package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// newTestKey returns a fresh RSA key for test tokens. The OIDCVerifier test
// hook WithStaticKeyFunc bypasses the JWKS fetch and uses this key directly.
func newTestKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	return k
}

// signTestToken builds an RS256 JWT with the supplied claims.
func signTestToken(t *testing.T, key *rsa.PrivateKey, claims OIDCClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return s
}

// staticKeyFuncFor returns a jwt.Keyfunc that always returns the test public
// key but enforces RS256 (mirrors the production algorithm guard).
func staticKeyFuncFor(pub *rsa.PublicKey) jwt.Keyfunc {
	return func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return pub, nil
	}
}

func TestOIDCVerifier_ValidToken(t *testing.T) {
	key := newTestKey(t)
	// iss/aud empty → checks disabled; we exercise the signature + claim path.
	v := NewOIDCVerifier("", "", "", WithStaticKeyFunc(staticKeyFuncFor(&key.PublicKey)))

	tok := signTestToken(t, key, OIDCClaims{
		Subject: "user-123",
		Email:   "alice@example.com",
		Name:    "Alice",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-time.Minute)),
		},
	})

	claims, err := v.VerifyToken(context.Background(), "Bearer "+tok)
	if err != nil {
		t.Fatalf("expected valid token, got error: %v", err)
	}
	if claims.Subject != "user-123" {
		t.Errorf("expected sub user-123, got %q", claims.Subject)
	}
	if claims.Email != "alice@example.com" {
		t.Errorf("expected email alice@example.com, got %q", claims.Email)
	}
}

func TestOIDCVerifier_ExpiredTokenRejected(t *testing.T) {
	key := newTestKey(t)
	v := NewOIDCVerifier("", "", "", WithStaticKeyFunc(staticKeyFuncFor(&key.PublicKey)))

	tok := signTestToken(t, key, OIDCClaims{
		Subject: "user-expired",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)), // already expired
		},
	})

	_, err := v.VerifyToken(context.Background(), tok)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

func TestOIDCVerifier_InvalidSignatureRejected(t *testing.T) {
	// Token signed with key A, verifier configured with key B.
	keyA := newTestKey(t)
	keyB := newTestKey(t)

	v := NewOIDCVerifier("", "", "", WithStaticKeyFunc(staticKeyFuncFor(&keyB.PublicKey)))

	tok := signTestToken(t, keyA, OIDCClaims{
		Subject: "user-forged",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})

	_, err := v.VerifyToken(context.Background(), tok)
	if err == nil {
		t.Fatal("expected signature error, got nil")
	}
}

func TestOIDCVerifier_IssuerMismatch(t *testing.T) {
	key := newTestKey(t)
	v := NewOIDCVerifier("", "", "trusted-issuer", WithStaticKeyFunc(staticKeyFuncFor(&key.PublicKey)))

	tok := signTestToken(t, key, OIDCClaims{
		Subject: "u",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "wrong-issuer",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})

	// With a static key func, jwt.ParseWithClaims does NOT check iss unless
	// we use WithIssuer; our VerifyToken does the iss check itself after
	// a successful parse.
	_, err := v.VerifyToken(context.Background(), tok)
	if err == nil {
		t.Fatal("expected iss mismatch error, got nil")
	}
}

func TestOIDCVerifier_AudienceMismatch(t *testing.T) {
	key := newTestKey(t)
	v := NewOIDCVerifier("", "my-client", "", WithStaticKeyFunc(staticKeyFuncFor(&key.PublicKey)))

	tok := signTestToken(t, key, OIDCClaims{
		Subject: "u",
		RegisteredClaims: jwt.RegisteredClaims{
			Audience:  []string{"someone-else"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})

	_, err := v.VerifyToken(context.Background(), tok)
	if err == nil {
		t.Fatal("expected aud mismatch error, got nil")
	}
}

func TestOIDCVerifier_EmptyToken(t *testing.T) {
	key := newTestKey(t)
	v := NewOIDCVerifier("", "", "", WithStaticKeyFunc(staticKeyFuncFor(&key.PublicKey)))

	_, err := v.VerifyToken(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty token, got nil")
	}

	_, err = v.VerifyToken(context.Background(), "Bearer ")
	if err == nil {
		t.Fatal("expected error for bare 'Bearer ', got nil")
	}
}

// jwksServerFor stands up an httptest server that serves a JWKS document for
// the supplied RSA public key under the supplied kid. Returns the JWKS URL.
func jwksServerFor(t *testing.T, pub *rsa.PublicKey, kid string) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		doc := jwksResponse{Keys: []jwksKey{{
			KTY: "RSA",
			KID: kid,
			Alg: "RS256",
			Use: "sig",
			N:   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
			E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
		}}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL + "/jwks"
}

func TestOIDCVerifier_JWKSPathValidAndForged(t *testing.T) {
	// Exercise the real JWKS fetch + cache path end-to-end against an
	// httptest server, including the algorithm-confusion guard.
	key := newTestKey(t)
	jwksURL := jwksServerFor(t, &key.PublicKey, "test-kid")

	v := NewOIDCVerifier(jwksURL, "", "")

	// Token with the matching kid → must verify.
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, OIDCClaims{
		Subject: "from-jwks",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	tok.Header["kid"] = "test-kid"
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	claims, err := v.VerifyToken(context.Background(), signed)
	if err != nil {
		t.Fatalf("expected valid via JWKS, got: %v", err)
	}
	if claims.Subject != "from-jwks" {
		t.Errorf("expected sub from-jwks, got %q", claims.Subject)
	}

	// Forged token signed by a different key but claiming the same kid →
	// signature verification must fail.
	forged := newTestKey(t)
	fTok := jwt.NewWithClaims(jwt.SigningMethodRS256, OIDCClaims{
		Subject: "forged",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	fTok.Header["kid"] = "test-kid"
	fSigned, err := fTok.SignedString(forged)
	if err != nil {
		t.Fatalf("sign forged: %v", err)
	}
	if _, err := v.VerifyToken(context.Background(), fSigned); err == nil {
		t.Fatal("expected signature error for forged token, got nil")
	}
}

func TestOIDCVerifier_RejectsHMACAlgorithm(t *testing.T) {
	// The production keyFunc enforces an RS* algorithm guard so the classic
	// algorithm-confusion attack (sign an HS256 token with the RSA public
	// key's bytes) fails. Build the resolver against a real (httptest) JWKS
	// URL and probe the guard directly.
	key := newTestKey(t)
	jwksURL := jwksServerFor(t, &key.PublicKey, "test-kid")
	v := NewOIDCVerifier(jwksURL, "", "")

	kf, err := v.keyFunc(context.Background())
	if err != nil {
		t.Fatalf("keyFunc: %v", err)
	}
	hs256 := &jwt.Token{Header: map[string]any{"alg": "HS256", "typ": "JWT"}, Method: jwt.SigningMethodHS256}
	if _, err := kf(hs256); err == nil {
		t.Fatal("expected keyFunc to reject HS256 (algorithm-confusion guard), got nil")
	}
}
