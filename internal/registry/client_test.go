package registry

import (
	"context"
	"testing"
)

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

	// Invalid URL should return error
	_, err := client.GetLatestTag(ctx, "invalid://url", nil)
	if err == nil {
		t.Error("expected error for invalid URL, got nil")
	}
}
