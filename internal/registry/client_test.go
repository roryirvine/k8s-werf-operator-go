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
