// Test utilities for the werf operator.
package controllers

import (
	"context"
	"sort"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/werf/k8s-werf-operator-go/internal/registry"
)

// FakeRegistry implements registry.Client for testing.
type FakeRegistry struct {
	// TagsByRepo maps repository URL to list of tags
	TagsByRepo map[string][]string

	// ErrorsByRepo maps repository URL to error that should be returned
	ErrorsByRepo map[string]error
}

// NewFakeRegistry creates a new fake registry for testing.
func NewFakeRegistry() *FakeRegistry {
	return &FakeRegistry{
		TagsByRepo:   make(map[string][]string),
		ErrorsByRepo: make(map[string]error),
	}
}

// SetTags sets the tags that will be returned for a given repository.
func (f *FakeRegistry) SetTags(repoURL string, tags []string) {
	// Sort tags to ensure consistent ordering
	sorted := make([]string, len(tags))
	copy(sorted, tags)
	sort.Strings(sorted)
	f.TagsByRepo[repoURL] = sorted
}

// SetError sets the error that will be returned for a given repository.
func (f *FakeRegistry) SetError(repoURL string, err error) {
	f.ErrorsByRepo[repoURL] = err
}

// ListTags returns predefined tags for testing.
func (f *FakeRegistry) ListTags(ctx context.Context, repoURL string, auth authn.Authenticator) ([]string, error) {
	if err, ok := f.ErrorsByRepo[repoURL]; ok {
		return nil, err
	}

	tags, ok := f.TagsByRepo[repoURL]
	if !ok {
		return []string{}, nil
	}

	return tags, nil
}

// GetLatestTag returns the latest tag by lexicographic order.
func (f *FakeRegistry) GetLatestTag(ctx context.Context, repoURL string, auth authn.Authenticator) (string, error) {
	tags, err := f.ListTags(ctx, repoURL, auth)
	if err != nil {
		return "", err
	}

	if len(tags) == 0 {
		return "", nil
	}

	return tags[len(tags)-1], nil
}

// Verify that FakeRegistry implements registry.Client
var _ registry.Client = (*FakeRegistry)(nil)
