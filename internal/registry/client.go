// Package registry provides OCI registry interactions for pulling bundle information.
package registry

import (
	"context"
	"fmt"
	"sort"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// Client defines the interface for OCI registry operations.
type Client interface {
	// ListTags returns all tags available in the given repository.
	// repoURL should be a full OCI repository URL (e.g., "ghcr.io/org/bundle").
	// auth is an optional authn.Authenticator; if nil, anonymous access is used.
	ListTags(ctx context.Context, repoURL string, auth authn.Authenticator) ([]string, error)

	// GetLatestTag returns the latest tag in the repository.
	// Returns empty string if no tags are found.
	GetLatestTag(ctx context.Context, repoURL string, auth authn.Authenticator) (string, error)
}

// OCIClient implements Client for OCI registries using go-containerregistry.
type OCIClient struct{}

// NewOCIClient creates a new OCI registry client.
func NewOCIClient() Client {
	return &OCIClient{}
}

// ListTags returns all tags in the OCI repository.
func (c *OCIClient) ListTags(ctx context.Context, repoURL string, auth authn.Authenticator) ([]string, error) {
	ref, err := name.NewRepository(repoURL)
	if err != nil {
		return nil, fmt.Errorf("invalid repository URL: %w", err)
	}

	tags, err := remote.List(ref, remote.WithContext(ctx), remote.WithAuth(auth))
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}

	sort.Strings(tags)
	return tags, nil
}

// GetLatestTag returns the latest tag by sorting tags lexicographically.
// For Slice 1, we use simple lexicographic ordering.
// Version constraint and semver logic will be added in future slices.
func (c *OCIClient) GetLatestTag(ctx context.Context, repoURL string, auth authn.Authenticator) (string, error) {
	tags, err := c.ListTags(ctx, repoURL, auth)
	if err != nil {
		return "", err
	}

	if len(tags) == 0 {
		return "", nil
	}

	// For MVP, return the last tag (lexicographic order).
	// This is a simplification that works for semver-like tags.
	return tags[len(tags)-1], nil
}
