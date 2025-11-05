// Package registry provides OCI registry interactions for pulling bundle information.
package registry

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// NotModifiedError indicates that the registry returned 304 Not Modified,
// meaning the content hasn't changed since the last request (ETag matched).
// This is not an error condition - it means the cached tag list is still valid.
type NotModifiedError struct{}

func (e *NotModifiedError) Error() string {
	return "registry returned 304 Not Modified (cached response is valid)"
}

// NetworkError indicates a transient network failure (connectivity issue, timeout, etc).
// These errors are retry-able and should trigger exponential backoff.
type NetworkError struct {
	Err error
}

func (e *NetworkError) Error() string {
	return fmt.Sprintf("network error: %v", e.Err)
}

func (e *NetworkError) Unwrap() error {
	return e.Err
}

// AuthError indicates an authentication or authorization failure.
// These errors may be transient (registry temporarily down) or permanent (bad credentials).
// Should be retried but with a shorter window before marking as Failed.
type AuthError struct {
	Err error
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("authentication error: %v", e.Err)
}

func (e *AuthError) Unwrap() error {
	return e.Err
}

// Client defines the interface for OCI registry operations.
type Client interface {
	// ListTags returns all tags available in the given repository.
	// repoURL should be a full OCI repository URL (e.g., "ghcr.io/org/bundle").
	// auth is an optional authn.Authenticator; if nil, anonymous access is used.
	ListTags(ctx context.Context, repoURL string, auth authn.Authenticator) ([]string, error)

	// GetLatestTag returns the latest tag in the repository.
	// Returns empty string if no tags are found.
	GetLatestTag(ctx context.Context, repoURL string, auth authn.Authenticator) (string, error)

	// ListTagsWithETag returns tags with ETag-based caching support.
	// If lastETag matches the current tag list content, returns NotModifiedError
	// (not a real error - indicates cached response is still valid).
	// On success, returns the tag list and a new ETag for future requests.
	// auth is an optional authn.Authenticator; if nil, anonymous access is used.
	ListTagsWithETag(
		ctx context.Context,
		repoURL string,
		auth authn.Authenticator,
		lastETag string,
	) (tags []string, newETag string, err error)
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
//
// WARNING: Lexicographic ordering does NOT work correctly for semantic versioning:
//
//	v1.2.0 < v1.10.0 (lexicographically, but > semantically)
//	v2.0.0 < v1.99.0 (lexicographically, but > semantically)
//
// This is acceptable as MVP but will be replaced with proper semver constraint
// matching in Slice 5. Use version constraints in CRD spec as workaround
// (e.g., ">=1.0.0,<2.0.0" to avoid unwanted major version jumps).
func (c *OCIClient) GetLatestTag(ctx context.Context, repoURL string, auth authn.Authenticator) (string, error) {
	tags, err := c.ListTags(ctx, repoURL, auth)
	if err != nil {
		return "", err
	}

	if len(tags) == 0 {
		return "", nil
	}

	// Return the last tag (lexicographic order).
	return tags[len(tags)-1], nil
}

// CalculateETag returns a hash of the sorted tag list.
// Used as a simple ETag substitute for detecting tag list changes.
// The hash ensures that the same set of tags always produces the same ETag,
// even if they arrive in different order from the registry.
func CalculateETag(tags []string) string {
	// Sort for consistent hashing
	sortedTags := make([]string, len(tags))
	copy(sortedTags, tags)
	sort.Strings(sortedTags)

	// Create a single string from sorted tags and hash it
	tagString := strings.Join(sortedTags, ",")
	hash := sha256.Sum256([]byte(tagString))

	// Return first 16 characters of hex-encoded hash as ETag
	return fmt.Sprintf("%x", hash)[:16]
}

// ListTagsWithETag returns tags with ETag-based caching support.
// If the current tag list matches lastETag, returns NotModifiedError
// to indicate the cached response is still valid (no download needed).
// On success, returns the tag list and a new ETag for future requests.
func (c *OCIClient) ListTagsWithETag(
	ctx context.Context,
	repoURL string,
	auth authn.Authenticator,
	lastETag string,
) ([]string, string, error) {
	tags, err := c.ListTags(ctx, repoURL, auth)
	if err != nil {
		return nil, "", err
	}

	// Calculate ETag for current tag list
	currentETag := CalculateETag(tags)

	// If ETag matches, return NotModifiedError (cached response valid)
	if lastETag != "" && currentETag == lastETag {
		return nil, currentETag, &NotModifiedError{}
	}

	// Return new tag list and ETag
	return tags, currentETag, nil
}
