// Package auth provides security primitives: password hashing (argon2id)
// and JSON Web Token signing/verification. Per D43, argon2id is the OWASP
// 2025-recommended password hash and golang-jwt/v5 is the JWT library.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/argon2"
)

// ============== argon2id password hashing ==============

// Argon2 parameters follow OWASP 2025 recommendations (D43).
// These are tunable per-deployment; the defaults target ~100ms on commodity
// hardware while keeping memory reasonable.
type Argon2Params struct {
	Memory      uint32 // KiB
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

// DefaultArgon2Params are OWASP-2025-aligned defaults.
var DefaultArgon2Params = Argon2Params{
	Memory:      64 * 1024, // 64 MiB
	Iterations:  3,
	Parallelism: 2,
	SaltLength:  16,
	KeyLength:   32,
}

// encodedHash layout: argon2id$memory=MiB,iterations=N,parallelism=P$salt$hash
// (PHC string format, base64 no-padding for salt/hash)

// HashPassword returns an argon2id PHC-style hash of the password.
func HashPassword(password string, p Argon2Params) (string, error) {
	salt, err := randomBytes(p.SaltLength)
	if err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, p.Iterations, p.Memory, p.Parallelism, p.KeyLength)
	return fmt.Sprintf("argon2id$m=%d,t=%d,p=%d$%s$%s",
		p.Memory, p.Iterations, p.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// HashPasswordDefault hashes with DefaultArgon2Params (convenience).
func HashPasswordDefault(password string) (string, error) {
	return HashPassword(password, DefaultArgon2Params)
}

// VerifyPassword checks a password against an argon2id PHC hash in constant time.
var (
	ErrInvalidHashFormat = errors.New("invalid argon2id hash format")
	ErrIncompatibleHash  = errors.New("incompatible argon2 variant (expected argon2id)")
)

// VerifyPassword reports whether the password matches the stored hash.
func VerifyPassword(password, encodedHash string) error {
	p, salt, hash, err := decodeHash(encodedHash)
	if err != nil {
		return err
	}
	otherKey := argon2.IDKey([]byte(password), salt, p.Iterations, p.Memory, p.Parallelism, p.KeyLength)
	if subtle.ConstantTimeCompare(hash, otherKey) != 1 {
		return errors.New("password does not match")
	}
	return nil
}

func decodeHash(encoded string) (Argon2Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	// Layout: argon2id$param-string$salt$hash → 4 parts after split.
	if len(parts) != 4 || parts[0] != "argon2id" {
		return Argon2Params{}, nil, nil, ErrInvalidHashFormat
	}
	if parts[1] == "" {
		return Argon2Params{}, nil, nil, ErrInvalidHashFormat
	}

	var p Argon2Params
	// parse "m=..,t=..,p=.."
	for _, field := range strings.Split(parts[1], ",") {
		kv := strings.SplitN(field, "=", 2)
		if len(kv) != 2 {
			return Argon2Params{}, nil, nil, ErrInvalidHashFormat
		}
		var v uint32
		var p8 uint8
		switch kv[0] {
		case "m":
			_, err := fmt.Sscanf(kv[1], "%d", &v)
			p.Memory = v
			if err != nil {
				return Argon2Params{}, nil, nil, ErrInvalidHashFormat
			}
		case "t":
			_, err := fmt.Sscanf(kv[1], "%d", &v)
			p.Iterations = v
			if err != nil {
				return Argon2Params{}, nil, nil, ErrInvalidHashFormat
			}
		case "p":
			_, err := fmt.Sscanf(kv[1], "%d", &p8)
			p.Parallelism = p8
			if err != nil {
				return Argon2Params{}, nil, nil, ErrInvalidHashFormat
			}
		default:
			return Argon2Params{}, nil, nil, ErrInvalidHashFormat
		}
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return Argon2Params{}, nil, nil, ErrInvalidHashFormat
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return Argon2Params{}, nil, nil, ErrInvalidHashFormat
	}
	p.SaltLength = uint32(len(salt))
	p.KeyLength = uint32(len(hash))
	return p, salt, hash, nil
}

func randomBytes(n uint32) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

// ============== JWT signing/verification ==============

// Claims is the standard JWT claims payload Aether issues.
type Claims struct {
	UserID string `json:"sub"`
	TeamID string `json:"team_id,omitempty"`
	jwt.RegisteredClaims
}

// JWTSigner signs and verifies HS256 tokens with a shared secret.
// (Phase 1: symmetric HS256 for simplicity; OIDC/RSA in D10 future work.)
type JWTSigner struct {
	secret []byte
	issuer string
}

// NewJWTSigner returns a signer bound to the given HMAC secret and issuer.
func NewJWTSigner(secret []byte, issuer string) *JWTSigner {
	return &JWTSigner{secret: secret, issuer: issuer}
}

// Sign issues a JWT valid for ttl, carrying the supplied claims.
func (s *JWTSigner) Sign(c Claims, ttl time.Duration) (string, error) {
	now := time.Now().UTC()
	c.Issuer = s.issuer
	c.IssuedAt = jwt.NewNumericDate(now)
	c.ExpiresAt = jwt.NewNumericDate(now.Add(ttl))
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return token.SignedString(s.secret)
}

var ErrInvalidToken = errors.New("invalid or expired token")

// Verify validates the token signature and expiration, returning the claims.
func (s *JWTSigner) Verify(tokenString string) (*Claims, error) {
	c := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, c, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	if !token.Valid {
		return nil, ErrInvalidToken
	}
	return c, nil
}
