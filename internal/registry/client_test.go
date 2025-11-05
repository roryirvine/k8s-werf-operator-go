package registry

import (
	"context"
	"fmt"
	"sort"
	"testing"
)

// FakeClient implements Client for testing without network access.
type FakeClient struct {
	tags map[string][]string
	errs map[string]error
}

// NewFakeClient creates a fake registry client for testing.
func NewFakeClient() *FakeClient {
	return &FakeClient{
		tags: make(map[string][]string),
		errs: make(map[string]error),
	}
}

// SetTags sets predefined tags for a repository (unsorted).
func (f *FakeClient) SetTags(repoURL string, tags []string) {
	// Store unsorted to test that ListTags sorts them
	f.tags[repoURL] = tags
}

// SetError sets an error to return for a repository.
func (f *FakeClient) SetError(repoURL string, err error) {
	f.errs[repoURL] = err
}

// ListTags returns predefined tags sorted lexicographically (like OCIClient).
func (f *FakeClient) ListTags(ctx context.Context, repoURL string, auth interface{}) ([]string, error) {
	if err, ok := f.errs[repoURL]; ok {
		return nil, err
	}
	tags, ok := f.tags[repoURL]
	if !ok {
		return []string{}, nil
	}
	// Copy and sort like OCIClient does
	result := make([]string, len(tags))
	copy(result, tags)
	sort.Strings(result)
	return result, nil
}

// GetLatestTag returns the lexicographically last tag.
func (f *FakeClient) GetLatestTag(ctx context.Context, repoURL string, auth interface{}) (string, error) {
	tags, err := f.ListTags(ctx, repoURL, auth)
	if err != nil {
		return "", err
	}
	if len(tags) == 0 {
		return "", nil
	}
	return tags[len(tags)-1], nil
}

// ListTagsWithETag returns tags with ETag support.
// Simulates HTTP ETag behavior by generating a deterministic ETag based on tag content.
func (f *FakeClient) ListTagsWithETag(
	ctx context.Context,
	repoURL string,
	auth interface{},
	lastETag string,
) ([]string, string, error) {
	tags, err := f.ListTags(ctx, repoURL, auth)
	if err != nil {
		return nil, "", err
	}

	// Generate a simple deterministic ETag for these tags
	// In real HTTP, this comes from response headers; we simulate it here
	currentETag := GenerateFakeETag(tags)

	// If ETag matches, return NotModifiedError (simulating HTTP 304)
	if lastETag != "" && currentETag == lastETag {
		return nil, currentETag, &NotModifiedError{}
	}

	return tags, currentETag, nil
}

func TestListTags_InvalidURL(t *testing.T) {
	client := NewOCIClient()
	ctx := context.Background()

	_, err := client.ListTags(ctx, "invalid://url", nil)
	if err == nil {
		t.Error("expected error for invalid URL, got nil")
	}
}

func TestGetLatestTag_InvalidURL(t *testing.T) {
	client := NewOCIClient()
	ctx := context.Background()

	_, err := client.GetLatestTag(ctx, "invalid://url", nil)
	if err == nil {
		t.Error("expected error for invalid URL, got nil")
	}
}

func TestGetLatestTag_LexicographicOrdering(t *testing.T) {
	tests := []struct {
		name     string
		tags     []string
		wantLast string
		desc     string
	}{
		{
			name:     "single tag",
			tags:     []string{"v1.0.0"},
			wantLast: "v1.0.0",
			desc:     "Returns single tag when only one available",
		},
		{
			name:     "ascending versions (semantic)",
			tags:     []string{"v1.0.0", "v1.1.0", "v1.2.0"},
			wantLast: "v1.2.0",
			desc:     "Lexicographic order matches semantic order for this case",
		},
		{
			name:     "descending versions (semantic)",
			tags:     []string{"v1.2.0", "v1.1.0", "v1.0.0"},
			wantLast: "v1.2.0",
			desc:     "Sorted: v1.0.0 < v1.1.0 < v1.2.0",
		},
		{
			name:     "mixed version numbers",
			tags:     []string{"v1.9.0", "v1.10.0", "v1.2.0"},
			wantLast: "v1.9.0",
			desc:     "Sorted: v1.10.0 < v1.2.0 < v1.9.0 (lexicographic bug: 9 > 10)",
		},
		{
			name:     "semver quirk with majors",
			tags:     []string{"v2.0.0", "v1.99.0", "v3.0.0"},
			wantLast: "v3.0.0",
			desc:     "Lexicographic order is 'correct' when major versions differ",
		},
		{
			name:     "rc and release tags",
			tags:     []string{"v1.0.0-rc1", "v1.0.0", "v1.0.0-rc2"},
			wantLast: "v1.0.0-rc2",
			desc:     "Sorted: v1.0.0 < v1.0.0-rc1 < v1.0.0-rc2 (shorter string sorts first)",
		},
		{
			name:     "numeric-only tags",
			tags:     []string{"1.2.3", "1.10.0", "1.2.10"},
			wantLast: "1.2.3",
			desc:     "Lexicographic order without 'v' prefix also has semver issues",
		},
		{
			name:     "empty repository",
			tags:     []string{},
			wantLast: "",
			desc:     "Empty tag list returns empty string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewFakeClient()
			client.SetTags("test-repo", tt.tags)

			got, err := client.GetLatestTag(context.Background(), "test-repo", nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.wantLast {
				t.Errorf("got %q, want %q - %s", got, tt.wantLast, tt.desc)
			}
		})
	}
}

func TestListTags_SortingBehavior(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
		desc  string
	}{
		{
			name:  "already sorted",
			input: []string{"v1.0.0", "v1.1.0", "v1.2.0"},
			want:  []string{"v1.0.0", "v1.1.0", "v1.2.0"},
			desc:  "Returns tags in same order when already sorted",
		},
		{
			name:  "reverse order",
			input: []string{"v1.2.0", "v1.1.0", "v1.0.0"},
			want:  []string{"v1.0.0", "v1.1.0", "v1.2.0"},
			desc:  "Sorts reverse order to ascending",
		},
		{
			name:  "unsorted",
			input: []string{"v1.9.0", "v1.2.0", "v1.10.0"},
			want:  []string{"v1.10.0", "v1.2.0", "v1.9.0"},
			desc:  "Sorts unsorted tags lexicographically",
		},
		{
			name:  "with prerelease",
			input: []string{"v1.0.0", "v1.0.0-rc1", "v1.0.0-beta"},
			want:  []string{"v1.0.0", "v1.0.0-beta", "v1.0.0-rc1"},
			desc:  "Prerelease tags sort after release due to suffix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewFakeClient()
			// Set unsorted tags
			client.tags["test-repo"] = tt.input

			// ListTags should return them sorted
			got, err := client.ListTags(context.Background(), "test-repo", nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tt.want) {
				t.Errorf("len mismatch: got %d items, want %d", len(got), len(tt.want))
				return
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("item %d: got %q, want %q - %s", i, got[i], tt.want[i], tt.desc)
				}
			}
		})
	}
}

func TestGetLatestTag_ErrorHandling(t *testing.T) {
	tests := []struct {
		name    string
		repoURL string
		setErr  error
		wantErr bool
	}{
		{
			name:    "repository not found",
			repoURL: "nonexistent/repo",
			setErr:  fmt.Errorf("repository not found"),
			wantErr: true,
		},
		{
			name:    "network error",
			repoURL: "flaky/repo",
			setErr:  fmt.Errorf("connection timeout"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewFakeClient()
			client.SetError(tt.repoURL, tt.setErr)

			_, err := client.GetLatestTag(context.Background(), tt.repoURL, nil)

			if (err != nil) != tt.wantErr {
				t.Errorf("wantErr %v, got err %v", tt.wantErr, err)
			}
		})
	}
}

func TestListTagsWithETag(t *testing.T) {
	tests := []struct {
		name              string
		tags              []string
		lastETag          string
		expectNotModified bool
		expectError       bool
		desc              string
	}{
		{
			name:              "first request (no lastETag)",
			tags:              []string{"v1.0.0", "v1.1.0"},
			lastETag:          "",
			expectNotModified: false,
			expectError:       false,
			desc:              "First request returns tags and new ETag",
		},
		{
			name:              "second request (same tags, matching ETag)",
			tags:              []string{"v1.0.0", "v1.1.0"},
			lastETag:          "", // Will be filled with calculated ETag
			expectNotModified: true,
			expectError:       false,
			desc:              "Same tags should return NotModifiedError with matching ETag",
		},
		{
			name:              "tags changed",
			tags:              []string{"v1.0.0", "v1.1.0", "v1.2.0"},
			lastETag:          "oldETagValue1234",
			expectNotModified: false,
			expectError:       false,
			desc:              "New tag added, ETag should be different",
		},
		{
			name:              "empty repository",
			tags:              []string{},
			lastETag:          "",
			expectNotModified: false,
			expectError:       false,
			desc:              "Empty repository returns empty tag list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewFakeClient()
			client.SetTags("test-repo", tt.tags)
			ctx := context.Background()

			// For the "same tags" test, first calculate the ETag
			if tt.name == "second request (same tags, matching ETag)" {
				_, etag, _ := client.ListTagsWithETag(ctx, "test-repo", nil, "")
				tt.lastETag = etag
			}

			tags, etag, err := client.ListTagsWithETag(ctx, "test-repo", nil, tt.lastETag)

			// Check error
			if tt.expectNotModified {
				_, isNotModified := err.(*NotModifiedError)
				if !isNotModified {
					t.Errorf("expected NotModifiedError, got %v", err)
				}
				if tags != nil {
					t.Errorf("expected nil tags on NotModifiedError, got %v", tags)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(tags) != len(tt.tags) {
					t.Errorf("tag count mismatch: got %d, want %d", len(tags), len(tt.tags))
				}
			}

			// ETag should always be returned
			if etag == "" && len(tt.tags) > 0 {
				t.Errorf("expected non-empty ETag for non-empty tag list")
			}
		})
	}
}

func TestETagConsistency(t *testing.T) {
	// Verify that same tags always produce same ETag
	tags := []string{"v1.2.0", "v1.0.0", "v1.1.0"}
	client := NewFakeClient()
	client.SetTags("test-repo", tags)
	ctx := context.Background()

	// First request
	_, etag1, _ := client.ListTagsWithETag(ctx, "test-repo", nil, "")

	// Second request with same tags (order might be different in input)
	_, etag2, _ := client.ListTagsWithETag(ctx, "test-repo", nil, "")

	if etag1 != etag2 {
		t.Errorf("same tags produced different ETags: %s vs %s", etag1, etag2)
	}

	// Third request with same lastETag should return NotModifiedError
	_, etag3, err := client.ListTagsWithETag(ctx, "test-repo", nil, etag1)
	_, isNotModified := err.(*NotModifiedError)
	if !isNotModified {
		t.Errorf("expected NotModifiedError on matching ETag, got %v", err)
	}
	if etag3 != etag1 {
		t.Errorf("ETag should remain same on NotModifiedError")
	}
}
