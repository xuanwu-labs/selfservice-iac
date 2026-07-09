package auth_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xuanwu-labs/selfservice-iac/server/internal/auth"
)

// ============== argon2id ==============

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := auth.HashPasswordDefault("correct horse battery staple")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(hash, "argon2id$"), "hash must be PHC argon2id")

	assert.NoError(t, auth.VerifyPassword("correct horse battery staple", hash))
}

func TestVerifyPasswordRejectsWrongPassword(t *testing.T) {
	hash, err := auth.HashPasswordDefault("s3cret")
	require.NoError(t, err)

	err = auth.VerifyPassword("wrong", hash)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match")
}

func TestHashIsUniquePerCall(t *testing.T) {
	h1, _ := auth.HashPasswordDefault("same")
	h2, _ := auth.HashPasswordDefault("same")
	assert.NotEqual(t, h1, h2, "salts must differ → hashes differ")
}

func TestVerifyPasswordRejectsMalformedHash(t *testing.T) {
	err := auth.VerifyPassword("x", "not-a-hash")
	require.ErrorIs(t, err, auth.ErrInvalidHashFormat)
}

// Fast params to keep the test suite snappy (production uses defaults).
var fastParams = auth.Argon2Params{Memory: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32}

func TestCustomParamsHashAndVerify(t *testing.T) {
	hash, err := auth.HashPassword("pw", fastParams)
	require.NoError(t, err)
	assert.NoError(t, auth.VerifyPassword("pw", hash))
	assert.Error(t, auth.VerifyPassword("other", hash))
}

// ============== JWT ==============

func TestJWTSignAndVerify(t *testing.T) {
	s := auth.NewJWTSigner([]byte("test-secret-32-bytes-ok-ok-ok"), "aether")

	claims := auth.Claims{UserID: "u1", TeamID: "t1"}
	tok, err := s.Sign(claims, time.Hour)
	require.NoError(t, err)
	require.NotEmpty(t, tok)

	got, err := s.Verify(tok)
	require.NoError(t, err)
	assert.Equal(t, "u1", got.UserID)
	assert.Equal(t, "t1", got.TeamID)
	assert.Equal(t, "aether", got.Issuer)
}

func TestJWTRejectsExpired(t *testing.T) {
	s := auth.NewJWTSigner([]byte("test-secret-32-bytes-ok-ok-ok"), "aether")
	tok, err := s.Sign(auth.Claims{UserID: "u1"}, -1*time.Minute) // already expired
	require.NoError(t, err)

	_, err = s.Verify(tok)
	require.ErrorIs(t, err, auth.ErrInvalidToken)
}

func TestJWTRejectsWrongSecret(t *testing.T) {
	signer := auth.NewJWTSigner([]byte("secret-A"), "aether")
	verifier := auth.NewJWTSigner([]byte("secret-B"), "aether")

	tok, err := signer.Sign(auth.Claims{UserID: "u1"}, time.Hour)
	require.NoError(t, err)

	_, err = verifier.Verify(tok)
	require.ErrorIs(t, err, auth.ErrInvalidToken)
}

func TestJWTRejectsTamperedToken(t *testing.T) {
	s := auth.NewJWTSigner([]byte("secret"), "aether")
	tok, err := s.Sign(auth.Claims{UserID: "u1"}, time.Hour)
	require.NoError(t, err)

	// Flip the last byte to corrupt the signature.
	tampered := tok[:len(tok)-1]
	if tok[len(tok)-1] == 'A' {
		tampered += "B"
	} else {
		tampered += "A"
	}
	_, err = s.Verify(tampered)
	require.ErrorIs(t, err, auth.ErrInvalidToken)
}
