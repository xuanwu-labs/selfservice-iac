package httpclient_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xuanwu-labs/selfservice-iac/server/internal/httpclient"
)

// startMockTokenServer starts a fake OAuth2 token endpoint that always issues
// a fixed access token. Returns the server URL.
func startMockTokenServer(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// oauth2 requires application/json content-type to parse the body.
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "fake-token-xyz",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// TestTokenFetch verifies the oauth2 client-credentials grant retrieves a token.
func TestTokenFetch(t *testing.T) {
	tokenURL := startMockTokenServer(t)

	cfg := &httpclient.AuthConfig{
		TokenURL:     tokenURL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Scopes:       []string{"read"},
	}
	tok, err := httpclient.Token(context.Background(), cfg)
	require.NoError(t, err)
	assert.Equal(t, "fake-token-xyz", tok.AccessToken)
	assert.Equal(t, "Bearer", tok.TokenType)
}

// TestTokenFetchNoConfig verifies calling Token without auth returns an error.
func TestTokenFetchNoConfig(t *testing.T) {
	_, err := httpclient.Token(context.Background(), nil)
	require.Error(t, err)
}

// TestNewClientWithoutAuth verifies a nil config yields a plain working client.
func TestNewClientWithoutAuth(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := httpclient.New(context.Background(), nil)
	resp, err := c.Get(srv.URL)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.True(t, called, "plain client must reach the server")
}

// TestNewClientWithAuthMakesAuthenticatedRequest verifies the oauth2 client
// attaches the bearer token to outbound requests.
func TestNewClientWithAuthMakesAuthenticatedRequest(t *testing.T) {
	tokenURL := startMockTokenServer(t)

	var gotAuth string
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(200)
	}))
	defer apiSrv.Close()

	cfg := &httpclient.AuthConfig{
		TokenURL:     tokenURL,
		ClientID:     "cid",
		ClientSecret: "csec",
	}
	c := httpclient.New(context.Background(), cfg)
	resp, err := c.Get(apiSrv.URL)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Contains(t, gotAuth, "Bearer", "authenticated client must send bearer token")
}
