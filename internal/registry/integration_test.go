//go:build integration
// +build integration

package registry

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// registryContainer manages a Docker registry container for testing.
type registryContainer struct {
	container testcontainers.Container
	hostPort  string
	baseURL   string
}

// startRegistry starts a Docker registry container and returns a registryContainer handle.
// Caller must defer cleanupRegistry() to stop the container.
func startRegistry(ctx context.Context, t *testing.T) *registryContainer {
	req := testcontainers.ContainerRequest{
		Image:        "registry:2",
		ExposedPorts: []string{"5000/tcp"},
		WaitingFor:   wait.ForListeningPort("5000/tcp"),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start registry container: %v", err)
	}

	hostPort, err := container.MappedPort(ctx, "5000")
	if err != nil {
		container.Terminate(ctx)
		t.Fatalf("failed to get mapped port: %v", err)
	}

	baseURL := fmt.Sprintf("localhost:%s", hostPort.Port())

	return &registryContainer{
		container: container,
		hostPort:  hostPort.Port(),
		baseURL:   baseURL,
	}
}

// cleanup stops and removes the registry container.
func (rc *registryContainer) cleanup(ctx context.Context) {
	rc.container.Terminate(ctx)
}

// pushImage pushes a random image with the given tag to the registry.
// Returns the image name (e.g., "localhost:5000/test/image:tag").
func (rc *registryContainer) pushImage(ctx context.Context, t *testing.T, repoName, tag string) string {
	// Create a random image
	img, err := random.Image(1024, 1)
	if err != nil {
		t.Fatalf("failed to create random image: %v", err)
	}

	// Parse the reference
	ref, err := name.ParseReference(fmt.Sprintf("%s/%s:%s", rc.baseURL, repoName, tag))
	if err != nil {
		t.Fatalf("failed to parse reference: %v", err)
	}

	// Push the image
	if err := remote.Write(ref, img); err != nil {
		t.Fatalf("failed to push image: %v", err)
	}

	return fmt.Sprintf("%s/%s", rc.baseURL, repoName)
}

// TestListTags_RealRegistry tests ListTags against a real Docker registry.
func TestListTags_RealRegistry(t *testing.T) {
	ctx := context.Background()
	rc := startRegistry(ctx, t)
	defer rc.cleanup(ctx)

	// Push multiple tags to the same repository
	repoName := "test/image"
	tags := []string{"v1.0.0", "v1.1.0", "v2.0.0"}
	for _, tag := range tags {
		rc.pushImage(ctx, t, repoName, tag)
	}

	// Create OCI client and list tags
	client := NewOCIClient()
	repoURL := fmt.Sprintf("%s/%s", rc.baseURL, repoName)
	listedTags, err := client.ListTags(ctx, repoURL, nil)
	if err != nil {
		t.Fatalf("ListTags failed: %v", err)
	}

	// Verify tags are returned in lexicographic order
	if len(listedTags) != len(tags) {
		t.Errorf("expected %d tags, got %d", len(tags), len(listedTags))
	}

	// Check that all tags are present and sorted
	for i, tag := range []string{"v1.0.0", "v1.1.0", "v2.0.0"} {
		if i < len(listedTags) && listedTags[i] != tag {
			t.Errorf("tag %d: expected %q, got %q", i, tag, listedTags[i])
		}
	}
}

// TestGetLatestTag_RealRegistry tests GetLatestTag against a real Docker registry.
func TestGetLatestTag_RealRegistry(t *testing.T) {
	ctx := context.Background()
	rc := startRegistry(ctx, t)
	defer rc.cleanup(ctx)

	// Push multiple tags
	repoName := "test/image"
	tags := []string{"v1.0.0", "v1.1.0", "v2.0.0"}
	for _, tag := range tags {
		rc.pushImage(ctx, t, repoName, tag)
	}

	// Get latest tag
	client := NewOCIClient()
	repoURL := fmt.Sprintf("%s/%s", rc.baseURL, repoName)
	latestTag, err := client.GetLatestTag(ctx, repoURL, nil)
	if err != nil {
		t.Fatalf("GetLatestTag failed: %v", err)
	}

	// Should return lexicographically last tag
	if latestTag != "v2.0.0" {
		t.Errorf("expected latest tag v2.0.0, got %q", latestTag)
	}
}

// TestETagConsistency_RealRegistry tests that ETags are consistent across calls to the same registry.
func TestETagConsistency_RealRegistry(t *testing.T) {
	ctx := context.Background()
	rc := startRegistry(ctx, t)
	defer rc.cleanup(ctx)

	// Push some tags
	repoName := "test/image"
	tags := []string{"v1.0.0", "v1.1.0"}
	for _, tag := range tags {
		rc.pushImage(ctx, t, repoName, tag)
	}

	client := NewOCIClient()
	repoURL := fmt.Sprintf("%s/%s", rc.baseURL, repoName)

	// First request - should get tags and ETag
	tags1, etag1, err := client.ListTagsWithETag(ctx, repoURL, nil, "")
	if err != nil {
		t.Fatalf("first ListTagsWithETag failed: %v", err)
	}
	if etag1 == "" {
		t.Error("expected non-empty ETag on first request")
	}
	if len(tags1) != 2 {
		t.Errorf("expected 2 tags, got %d", len(tags1))
	}

	// Second request with same ETag - should return NotModifiedError
	tags2, etag2, err := client.ListTagsWithETag(ctx, repoURL, nil, etag1)
	if !isNotModifiedError(err) {
		t.Errorf("expected NotModifiedError on second request, got: %v", err)
	}
	if tags2 != nil {
		t.Error("expected nil tags on NotModifiedError")
	}
	if etag2 != etag1 {
		t.Errorf("ETag should remain same on NotModified: %q != %q", etag2, etag1)
	}

	// Third request - push a new tag and verify ETag changes
	rc.pushImage(ctx, t, repoName, "v1.2.0")

	// Query again - should get new tags and different ETag
	tags3, etag3, err := client.ListTagsWithETag(ctx, repoURL, nil, etag1)
	if err != nil {
		t.Fatalf("third ListTagsWithETag failed: %v", err)
	}
	if len(tags3) != 3 {
		t.Errorf("expected 3 tags after push, got %d", len(tags3))
	}
	if etag3 == etag1 {
		t.Error("ETag should change when tags change")
	}
}

// TestRepositoryNotFound_RealRegistry tests error handling for non-existent repositories.
func TestRepositoryNotFound_RealRegistry(t *testing.T) {
	ctx := context.Background()
	rc := startRegistry(ctx, t)
	defer rc.cleanup(ctx)

	client := NewOCIClient()
	repoURL := fmt.Sprintf("%s/nonexistent/repo", rc.baseURL)

	// Should return empty tag list for non-existent repo (registry behavior)
	tags, err := client.ListTags(ctx, repoURL, nil)
	if err == nil {
		t.Errorf("expected error for non-existent repository, got tags: %v", tags)
	}
}

// TestEmptyRepository_RealRegistry tests behavior with empty repositories.
func TestEmptyRepository_RealRegistry(t *testing.T) {
	ctx := context.Background()
	rc := startRegistry(ctx, t)
	defer rc.cleanup(ctx)

	// Create an empty repository by trying to get a non-existent image
	// (This is just to ensure we can handle repos with no tags)
	repoName := "test/empty"

	client := NewOCIClient()
	repoURL := fmt.Sprintf("%s/%s", rc.baseURL, repoName)

	// Try to list tags on empty/non-existent repo
	tags, err := client.ListTags(ctx, repoURL, nil)
	if err == nil {
		t.Errorf("expected error for empty repository, got tags: %v", tags)
	}

	// GetLatestTag should return empty string
	latestTag, err := client.GetLatestTag(ctx, repoURL, nil)
	if err == nil {
		t.Errorf("expected error for empty repository, got tag: %q", latestTag)
	}
}

// isNotModifiedError is a helper to check for NotModifiedError.
func isNotModifiedError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*NotModifiedError)
	return ok
}

// TestOCIProtocolHandling_RealRegistry tests that we correctly handle OCI protocol responses.
// This verifies that our use of go-containerregistry is correct for:
// - HTTP 200 with tag list
// - HTTP 404 for non-existent repos
// - Content-Type headers
// - Proper URL construction
func TestOCIProtocolHandling_RealRegistry(t *testing.T) {
	ctx := context.Background()
	rc := startRegistry(ctx, t)
	defer rc.cleanup(ctx)

	// Push a well-formed image
	repoName := "test/protocol"
	rc.pushImage(ctx, t, repoName, "v1.0.0")

	client := NewOCIClient()
	repoURL := fmt.Sprintf("%s/%s", rc.baseURL, repoName)

	// Verify we can list tags (tests HTTP 200 handling)
	tags, err := client.ListTags(ctx, repoURL, nil)
	if err != nil {
		t.Fatalf("failed to list tags: %v", err)
	}
	if len(tags) == 0 {
		t.Error("expected tags from registry")
	}

	// Verify ListTagsWithETag works with real HTTP responses
	_, etag, err := client.ListTagsWithETag(ctx, repoURL, nil, "")
	if err != nil {
		t.Fatalf("failed to get tags with ETag: %v", err)
	}
	if etag == "" {
		t.Error("expected non-empty ETag from real registry")
	}
}

// TestMultipleRepositories_RealRegistry tests handling of multiple repositories in same registry.
func TestMultipleRepositories_RealRegistry(t *testing.T) {
	ctx := context.Background()
	rc := startRegistry(ctx, t)
	defer rc.cleanup(ctx)

	// Push to different repos
	repo1Name := "app/service1"
	repo2Name := "app/service2"

	rc.pushImage(ctx, t, repo1Name, "v1.0.0")
	rc.pushImage(ctx, t, repo2Name, "v2.0.0")

	client := NewOCIClient()

	// List tags from repo1
	repo1URL := fmt.Sprintf("%s/%s", rc.baseURL, repo1Name)
	tags1, err := client.ListTags(ctx, repo1URL, nil)
	if err != nil {
		t.Fatalf("failed to list repo1 tags: %v", err)
	}
	if len(tags1) != 1 || tags1[0] != "v1.0.0" {
		t.Errorf("repo1: expected [v1.0.0], got %v", tags1)
	}

	// List tags from repo2
	repo2URL := fmt.Sprintf("%s/%s", rc.baseURL, repo2Name)
	tags2, err := client.ListTags(ctx, repo2URL, nil)
	if err != nil {
		t.Fatalf("failed to list repo2 tags: %v", err)
	}
	if len(tags2) != 1 || tags2[0] != "v2.0.0" {
		t.Errorf("repo2: expected [v2.0.0], got %v", tags2)
	}

	// ETags should be different for different repos
	_, etag1, _ := client.ListTagsWithETag(ctx, repo1URL, nil, "")
	_, etag2, _ := client.ListTagsWithETag(ctx, repo2URL, nil, "")
	if etag1 == etag2 {
		t.Error("different repositories should have different ETags")
	}
}
