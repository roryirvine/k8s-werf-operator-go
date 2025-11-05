package registry

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// mockRoundTripper simulates HTTP responses for testing.
type mockRoundTripper struct {
	statusCode int
	etagValue  string
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp := &http.Response{
		StatusCode: m.statusCode,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("[]")),
	}

	if m.etagValue != "" {
		resp.Header.Set("ETag", m.etagValue)
	}

	return resp, nil
}

func TestETagRoundTripper_SetIfNoneMatch(t *testing.T) {
	// Verify that If-None-Match header is set when lastETag is provided
	mockRT := &mockRoundTripper{statusCode: 200, etagValue: `"new-etag"`}
	transport := newETagRoundTripper(mockRT, `"old-etag"`)

	req, _ := http.NewRequest("GET", "http://example.com/v2/repo/tags/list", nil)
	resp, err := transport.RoundTrip(req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that If-None-Match was set
	if req.Header.Get("If-None-Match") != `"old-etag"` {
		t.Errorf("If-None-Match header not set correctly: %s", req.Header.Get("If-None-Match"))
	}

	// Check that new ETag was captured
	if transport.CapturedETag() != `"new-etag"` {
		t.Errorf("ETag not captured: %s", transport.CapturedETag())
	}

	if err := resp.Body.Close(); err != nil {
		t.Errorf("failed to close response body: %v", err)
	}
}

func TestETagRoundTripper_NotModified_Error(t *testing.T) {
	// Verify that 304 Not Modified returns NotModifiedError
	mockRT := &mockRoundTripper{statusCode: 304, etagValue: `"same-etag"`}
	transport := newETagRoundTripper(mockRT, `"same-etag"`)

	req, _ := http.NewRequest("GET", "http://example.com/v2/repo/tags/list", nil)
	_, err := transport.RoundTrip(req)

	if _, ok := err.(*NotModifiedError); !ok {
		t.Errorf("expected NotModifiedError, got %T: %v", err, err)
	}

	// ETag should still be captured
	if transport.CapturedETag() != `"same-etag"` {
		t.Errorf("ETag not captured on 304: %s", transport.CapturedETag())
	}
}

func TestETagRoundTripper_Unauthorized_Error(t *testing.T) {
	// Verify that 401 returns AuthError
	mockRT := &mockRoundTripper{statusCode: 401}
	transport := newETagRoundTripper(mockRT, "")

	req, _ := http.NewRequest("GET", "http://example.com/v2/repo/tags/list", nil)
	_, err := transport.RoundTrip(req)

	if _, ok := err.(*AuthError); !ok {
		t.Errorf("expected AuthError for 401, got %T: %v", err, err)
	}
}

func TestETagRoundTripper_Forbidden_Error(t *testing.T) {
	// Verify that 403 returns AuthError
	mockRT := &mockRoundTripper{statusCode: 403}
	transport := newETagRoundTripper(mockRT, "")

	req, _ := http.NewRequest("GET", "http://example.com/v2/repo/tags/list", nil)
	_, err := transport.RoundTrip(req)

	if _, ok := err.(*AuthError); !ok {
		t.Errorf("expected AuthError for 403, got %T: %v", err, err)
	}
}

func TestETagRoundTripper_InternalServerError(t *testing.T) {
	// Verify that 500 returns NetworkError
	mockRT := &mockRoundTripper{statusCode: 500}
	transport := newETagRoundTripper(mockRT, "")

	req, _ := http.NewRequest("GET", "http://example.com/v2/repo/tags/list", nil)
	_, err := transport.RoundTrip(req)

	if _, ok := err.(*NetworkError); !ok {
		t.Errorf("expected NetworkError for 500, got %T: %v", err, err)
	}
}

func TestETagRoundTripper_ServiceUnavailable(t *testing.T) {
	// Verify that 503 returns NetworkError
	mockRT := &mockRoundTripper{statusCode: 503}
	transport := newETagRoundTripper(mockRT, "")

	req, _ := http.NewRequest("GET", "http://example.com/v2/repo/tags/list", nil)
	_, err := transport.RoundTrip(req)

	if _, ok := err.(*NetworkError); !ok {
		t.Errorf("expected NetworkError for 503, got %T: %v", err, err)
	}
}

func TestETagRoundTripper_Success(t *testing.T) {
	// Verify that 200 OK returns response normally
	mockRT := &mockRoundTripper{statusCode: 200, etagValue: `"new-etag"`}
	transport := newETagRoundTripper(mockRT, `"old-etag"`)

	req, _ := http.NewRequest("GET", "http://example.com/v2/repo/tags/list", nil)
	resp, err := transport.RoundTrip(req)

	if err != nil {
		t.Fatalf("unexpected error for 200: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	if transport.CapturedETag() != `"new-etag"` {
		t.Errorf("ETag not captured on 200: %s", transport.CapturedETag())
	}

	if err := resp.Body.Close(); err != nil {
		t.Errorf("failed to close response body: %v", err)
	}
}
