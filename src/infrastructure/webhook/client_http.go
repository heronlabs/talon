package webhook

import (
	"net/http"
	"time"
)

// NewHTTPClient returns the *http.Client that gives up on a request to the webhook after timeout.
func NewHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}
