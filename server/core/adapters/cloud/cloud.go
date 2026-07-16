// Package cloud defines the CloudProvider adapter interface (D7).
//
// The cloudcreds module uses this to validate cloud credentials and list
// available regions. The default implementation is a noop stub.
package cloud

import (
	"context"
	"fmt"
)

// Credentials holds the cloud-specific authentication material.
// The secret values are never logged or persisted in plaintext (D23).
type Credentials struct {
	Provider     string // aws | aliyun | azure | gcp
	AccessKey    string
	SecretKey    string
	SessionToken string
	OIDCToken    string
}

// CloudProvider abstracts cloud-specific operations so the platform can
// support multiple cloud vendors without code changes.
type CloudProvider interface {
	// ValidateCredentials checks if the credentials are valid and have
	// the minimum permissions required by the platform.
	ValidateCredentials(ctx context.Context, creds Credentials) error
	// ListRegions returns the regions enabled for the given credentials.
	ListRegions(ctx context.Context, creds Credentials) ([]string, error)
}

// NoopCloud is the default stub.
type NoopCloud struct{}

// ValidateCredentials returns a structured error.
func (NoopCloud) ValidateCredentials(_ context.Context, _ Credentials) error {
	return fmt.Errorf("cloud adapter not configured: set adapters.cloud.impl in config")
}

// ListRegions returns a structured error.
func (NoopCloud) ListRegions(_ context.Context, _ Credentials) ([]string, error) {
	return nil, fmt.Errorf("cloud adapter not configured: set adapters.cloud.impl in config")
}
