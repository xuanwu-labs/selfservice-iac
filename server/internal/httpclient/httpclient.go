// Package httpclient provides outbound HTTP clients with optional OAuth2
// client-credentials authentication. Used for OIDC providers, cloud APIs,
// and SCM (GitHub/GitLab) webhooks — anywhere the server calls out with an
// access token (D43/D10/D23).
package httpclient

import (
	"context"
	"fmt"
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// AuthConfig holds OAuth2 client-credentials parameters.
type AuthConfig struct {
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// New returns an *http.Client with OTel trace propagation (task 11.5): every
// outbound HTTP request becomes a span child of the caller's trace.
//
// Without auth (cfg == nil): a plain client (still traced).
// With auth: an oauth2-wrapped client that auto-refreshes tokens via the
// client-credentials grant (machine-to-machine; no user context).
func New(ctx context.Context, cfg *AuthConfig) *http.Client {
	// Wrap the default transport with otelhttp so outbound calls propagate
	// the active trace context (D41 — one trace across gin→pgx→http-out).
	transport := otelhttp.NewTransport(http.DefaultTransport)
	if cfg == nil {
		return &http.Client{Transport: transport}
	}
	oc := &clientcredentials.Config{
		TokenURL:     cfg.TokenURL,
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Scopes:       cfg.Scopes,
	}
	c := oc.Client(ctx)
	// oauth2's client uses its own transport; layer otelhttp on top.
	c.Transport = otelhttp.NewTransport(c.Transport)
	return c
}

// NewWithTransport is like New but lets the caller inject a transport (e.g.
// otelhttp.NewTransport for trace propagation, per D41 task 11.5).
func NewWithTransport(ctx context.Context, cfg *AuthConfig, base http.RoundTripper) *http.Client {
	c := New(ctx, cfg)
	if base != nil {
		c.Transport = base
	}
	return c
}

// TokenSource exposes the raw oauth2.TokenSource for callers that need to
// attach a token manually (e.g. to a non-HTTP protocol). Returns nil if cfg
// is nil (no auth).
func TokenSource(ctx context.Context, cfg *AuthConfig) oauth2.TokenSource {
	if cfg == nil {
		return nil
	}
	oc := &clientcredentials.Config{
		TokenURL:     cfg.TokenURL,
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Scopes:       cfg.Scopes,
	}
	return oc.TokenSource(ctx)
}

// Token retrieves a single access token from the client-credentials grant.
// Useful for testing the provider config without a full HTTP roundtrip.
func Token(ctx context.Context, cfg *AuthConfig) (*oauth2.Token, error) {
	ts := TokenSource(ctx, cfg)
	if ts == nil {
		return nil, fmt.Errorf("no auth config provided")
	}
	return ts.Token()
}
