// Transport wrapper that implements HTTP ETag-based caching.
package registry

import (
	"fmt"
	"net/http"
)

// etagRoundTripper wraps an http.RoundTripper to add ETag support.
// It automatically sets If-None-Match headers and captures ETag response headers.
type etagRoundTripper struct {
	base         http.RoundTripper
	lastETag     string
	capturedETag string
}

// newETagRoundTripper creates a new ETag-aware transport wrapper.
func newETagRoundTripper(base http.RoundTripper, lastETag string) *etagRoundTripper {
	return &etagRoundTripper{
		base:     base,
		lastETag: lastETag,
	}
}

// RoundTrip implements http.RoundTripper.
// Sets If-None-Match header if lastETag is set, and captures ETag from response.
// Detects HTTP error status codes and returns appropriate error types.
// Returns NotModifiedError if server returns 304 Not Modified.
// Returns NetworkError for 5xx status codes.
// Returns AuthError for 401/403 status codes.
func (t *etagRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Set If-None-Match header if we have a cached ETag
	if t.lastETag != "" {
		req.Header.Set("If-None-Match", t.lastETag)
	}

	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// Capture ETag header from response for future requests
	if etag := resp.Header.Get("ETag"); etag != "" {
		t.capturedETag = etag
	}

	// Detect HTTP error status codes and return appropriate error types
	switch resp.StatusCode {
	case http.StatusNotModified: // 304
		// Content hasn't changed, signal with NotModifiedError
		return nil, &NotModifiedError{}

	case http.StatusUnauthorized, http.StatusForbidden: // 401, 403
		// Authentication/authorization failure
		return nil, &AuthError{Err: fmt.Errorf("HTTP %d: %s", resp.StatusCode, http.StatusText(resp.StatusCode))}

	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout: // 502, 503, 504
		// Transient server errors
		return nil, &NetworkError{Err: fmt.Errorf("HTTP %d: %s", resp.StatusCode, http.StatusText(resp.StatusCode))}

	case http.StatusInternalServerError: // 500
		// Internal server error (might be transient)
		return nil, &NetworkError{Err: fmt.Errorf("HTTP %d: %s", resp.StatusCode, http.StatusText(resp.StatusCode))}
	}

	return resp, nil
}

// CapturedETag returns the ETag value captured from the last response.
func (t *etagRoundTripper) CapturedETag() string {
	return t.capturedETag
}

// GenerateFakeETag creates a deterministic ETag for testing purposes.
// This simulates what a real registry would return in the ETag header.
// Used by FakeClient implementations when simulating registry behavior.
func GenerateFakeETag(tags []string) string {
	if len(tags) == 0 {
		return `"empty"`
	}
	// Simple deterministic ETag: count of tags + first and last tag
	// In reality, registries use content hashes or version numbers
	return fmt.Sprintf(`"tags-%d-%s-%s"`, len(tags), tags[0], tags[len(tags)-1])
}
