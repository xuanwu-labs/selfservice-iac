// Package auth oidc_verifier.go: OIDC ID-token verification (design D3).
//
// Phase 1 is verify-only: we accept a Bearer JWT issued by a single trusted
// OIDC issuer, validate signature (via JWKS), exp, iss, aud, and surface the
// claims a downstream layer (IdentityService.GetByExternalID + BootstrapAdmin)
// uses to map to a platform identity.
//
// We deliberately use golang-jwt/v5 + a hand-rolled JWKS fetch rather than
// zitadel/oidc. zitadel/oidc is a full provider/client library and is
// overkill for verify-only (P2-10). ~140 lines here vs. pulling in the whole
// OIDC stack. zitadel/oidc remains in go.mod for Phase 2's full
// authorization-code flow.
//
// We also avoid lestrrat-go/jwx to keep the dependency surface minimal —
// stdlib crypto/rsa + encoding/json is enough for the common RSA + kid JWKS
// shape that real OIDC issuers (Auth0/Keycloak/Zitadel/Google) publish.
package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// OIDCClaims is the subset of an OIDC ID token / access token this platform
// needs at Phase 1. Additional claims (groups, picture, ...) can be added
// without breaking existing callers.
type OIDCClaims struct {
	Subject string `json:"sub"`
	Email   string `json:"email,omitempty"`
	Name    string `json:"name,omitempty"`
	// AuthorizedParty is the azp claim (OAuth client id the token was issued
	// to). Some IdPs require it when aud has more than one entry.
	AuthorizedParty string `json:"azp,omitempty"`
	jwt.RegisteredClaims
}

// jwksKey is the subset of a JWKS "keys" entry we need to reconstruct an
// RSA public key. This covers all OIDC providers that use RS256/384/512 —
// the only signing algorithms this verifier accepts.
type jwksKey struct {
	KTY string `json:"kty"`
	KID string `json:"kid"`
	Alg string `json:"alg,omitempty"`
	Use string `json:"use,omitempty"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// jwksResponse is the shape of a standard JWKS document.
type jwksResponse struct {
	Keys []jwksKey `json:"keys"`
}

// OIDCVerifier validates Bearer JWTs against a single OIDC issuer's JWKS.
type OIDCVerifier struct {
	jwksURL  string
	audience string
	issuer   string

	// httpClient is overridable for tests. Defaults to a 10s-timeout client.
	httpClient *http.Client

	mu      sync.RWMutex
	cache   map[string]*rsa.PublicKey // kid → key
	fetched time.Time                 // last successful full refresh
	ttl     time.Duration             // refresh interval (default 1h)

	// keyProvider is an optional override of the JWKS-backed key resolver.
	// When set (tests), VerifyToken uses it directly and skips the network.
	// Production leaves it nil and uses the JWKS cache.
	keyProvider jwt.Keyfunc
}

// Option configures an OIDCVerifier.
type Option func(*OIDCVerifier)

// WithHTTPClient overrides the default HTTP client used for JWKS fetches.
func WithHTTPClient(c *http.Client) Option {
	return func(v *OIDCVerifier) { v.httpClient = c }
}

// WithJWKSCacheTTL overrides the default 1h JWKS refresh interval.
func WithJWKSCacheTTL(d time.Duration) Option {
	return func(v *OIDCVerifier) { v.ttl = d }
}

// WithStaticKeyFunc bypasses the JWKS fetch and uses the supplied key
// resolver. This is the test hook: tests pass an HMAC/RSA key directly
// instead of standing up a JWKS endpoint.
func WithStaticKeyFunc(kf jwt.Keyfunc) Option {
	return func(v *OIDCVerifier) { v.keyProvider = kf }
}

// NewOIDCVerifier constructs an OIDCVerifier for a single issuer.
//
//   - jwksURL  : full URL to the issuer's JWKS endpoint
//     (e.g. "https://idp.example.com/.well-known/jwks.json")
//   - audience : expected aud claim (your client id); "" disables the check
//   - issuer   : expected iss claim; "" disables the check
//
// Disabling iss/aud checks is intended for development only — production
// should always set both.
func NewOIDCVerifier(jwksURL, audience, issuer string, opts ...Option) *OIDCVerifier {
	v := &OIDCVerifier{
		jwksURL:    jwksURL,
		audience:   audience,
		issuer:     issuer,
		cache:      make(map[string]*rsa.PublicKey),
		ttl:        time.Hour,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	for _, o := range opts {
		o(v)
	}
	return v
}

// VerifyToken parses and validates a Bearer JWT. The token may be the raw JWT
// string or the full "Bearer <jwt>" header value — leading "Bearer " is
// trimmed when present. On success the OIDCClaims are returned.
//
// Validation performed:
//   - signature (JWKS or static key)
//   - exp (rejects expired tokens)
//   - iss (when v.issuer != "")
//   - aud (when v.audience != "")
//   - signing algorithm is RS256/384/512 only (no "none", no HMAC-as-RSA
//     confusion attack)
//
// Any failure returns a non-nil error wrapping ErrInvalidToken.
func (v *OIDCVerifier) VerifyToken(ctx context.Context, bearerToken string) (*OIDCClaims, error) {
	if v == nil {
		return nil, errors.New("oidc: verifier is nil")
	}
	token := strings.TrimSpace(bearerToken)
	token = strings.TrimPrefix(token, "Bearer ")
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("%w: empty token", ErrInvalidToken)
	}

	kf, err := v.keyFunc(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: init key set: %v", ErrInvalidToken, err)
	}

	// Validation options. When iss/aud are configured we ask jwt.ParseWithClaims
	// to enforce them; this catches mismatches even with a static test key.
	parserOpts := []jwt.ParserOption{}
	if v.issuer != "" {
		parserOpts = append(parserOpts, jwt.WithIssuer(v.issuer))
	}
	if v.audience != "" {
		parserOpts = append(parserOpts, jwt.WithAudience(v.audience))
	}

	claims := &OIDCClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, kf, parserOpts...)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	if !parsed.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// keyFunc returns the jwt.Keyfunc the parser will call per-token. When a
// static key provider is set (tests) it is returned directly; otherwise we
// lazily fetch + cache the JWKS.
func (v *OIDCVerifier) keyFunc(ctx context.Context) (jwt.Keyfunc, error) {
	if v.keyProvider != nil {
		return v.keyProvider, nil
	}
	if v.jwksURL == "" {
		return nil, errors.New("jwks_url is empty")
	}

	// First call primes the cache so we have something to serve.
	if _, err := v.ensureCache(ctx, false); err != nil {
		return nil, err
	}

	return func(t *jwt.Token) (interface{}, error) {
		// Algorithm guard: only RSA is allowed. Rejecting HMAC / "none" here
		// closes the well-known JWT algorithm-confusion attacks.
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v (RSA required)", t.Header["alg"])
		}
		kid, _ := t.Header["kid"].(string)

		v.mu.RLock()
		key, ok := v.cache[kid]
		v.mu.RUnlock()
		if ok {
			return key, nil
		}

		// Cache miss → force a refresh and try once more.
		if _, err := v.ensureCache(ctx, true); err != nil {
			return nil, fmt.Errorf("refresh jwks on kid miss: %w", err)
		}
		v.mu.RLock()
		key, ok = v.cache[kid]
		v.mu.RUnlock()
		if !ok {
			return nil, fmt.Errorf("kid %q not in JWKS", kid)
		}
		return key, nil
	}, nil
}

// ensureCache populates v.cache from the JWKS endpoint. When force is false
// and the cache is fresh (younger than ttl), it is a no-op. Returns the
// number of keys currently cached.
func (v *OIDCVerifier) ensureCache(ctx context.Context, force bool) (int, error) {
	v.mu.RLock()
	fresh := !v.fetched.IsZero() && time.Since(v.fetched) < v.ttl
	cnt := len(v.cache)
	v.mu.RUnlock()
	if !force && fresh && cnt > 0 {
		return cnt, nil
	}

	set, err := fetchJWKS(ctx, v.httpClient, v.jwksURL)
	if err != nil {
		// If we already have a cache, serve stale rather than fail closed —
		// better to risk a stale key than to reject every request during a
		// brief JWKS outage.
		v.mu.RLock()
		stale := len(v.cache)
		v.mu.RUnlock()
		if stale > 0 {
			return stale, nil
		}
		return 0, err
	}

	next := make(map[string]*rsa.PublicKey, len(set.Keys))
	for _, k := range set.Keys {
		if k.KTY != "RSA" || k.KID == "" {
			continue
		}
		pk, err := jwkToRSAPublicKey(k)
		if err != nil {
			// Skip a malformed key but keep going — a single bad key must not
			// poison the whole cache.
			continue
		}
		next[k.KID] = pk
	}

	v.mu.Lock()
	v.cache = next
	v.fetched = time.Now()
	cnt = len(next)
	v.mu.Unlock()
	return cnt, nil
}

// fetchJWKS is the raw HTTP fetch + JSON parse. Separated out so tests can
// stub it via WithHTTPClient + httptest.Server.
func fetchJWKS(ctx context.Context, httpClient *http.Client, url string) (*jwksResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("jwks %s: status %d: %s", url, resp.StatusCode, string(body))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read jwks body: %w", err)
	}
	var out jwksResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("parse jwks: %w", err)
	}
	return &out, nil
}

// jwkToRSAPublicKey reconstructs an *rsa.PublicKey from the JWK base64url
// modulus + exponent. Per RFC 7518 §6.3.1, n and e are base64url unsigned
// big-endian.
func jwkToRSAPublicKey(k jwksKey) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("decode n: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("decode e: %w", err)
	}
	// e is typically the 3-byte big-endian encoding of 65537. big.Int.SetBytes
	// handles arbitrary length.
	eInt := new(big.Int).SetBytes(eBytes)
	if !eInt.IsInt64() {
		return nil, fmt.Errorf("exponent too large")
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: int(eInt.Int64()),
	}, nil
}
