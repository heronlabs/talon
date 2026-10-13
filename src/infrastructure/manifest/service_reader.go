package manifest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// The header and the prefix of the token in Authorization.
const (
	authorizationHeader = "Authorization"
	bearerPrefix        = "Bearer "
)

// ErrUnexpectedStatus is the error that an answer of the manifest endpoint outside 2xx wraps.
var ErrUnexpectedStatus = errors.New("unexpected status")

// ReaderService reads the manifest of the host.
type ReaderService struct {
	client *http.Client
}

// NewReaderService returns a ReaderService that requests through client.
func NewReaderService(client *http.Client) *ReaderService {
	return &ReaderService{client: client}
}

// Read gets the manifest at endpoint with ctx, with Authorization: Bearer <token> only when there is a token, and returns it decoded; an answer outside 2xx wraps ErrUnexpectedStatus.
func (s *ReaderService) Read(ctx context.Context, endpoint, token string) (*Manifest, error) {
	request, err := newRequest(ctx, endpoint, token)
	if err != nil {
		return nil, err
	}

	response, err := s.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if err := checkStatus(response.StatusCode); err != nil {
		return nil, err
	}

	return decodeManifest(response.Body)
}

// newRequest returns the GET of endpoint with ctx, with Authorization: Bearer <token> only when there is a token.
func newRequest(ctx context.Context, endpoint, token string) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, err
	}

	if token != "" {
		request.Header.Set(authorizationHeader, bearerPrefix+token)
	}

	return request, nil
}

// checkStatus fails with ErrUnexpectedStatus when status is outside 2xx.
func checkStatus(status int) error {
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return fmt.Errorf("%w: %d", ErrUnexpectedStatus, status)
	}

	return nil
}

// decodeManifest returns the manifest decoded from body.
func decodeManifest(body io.Reader) (*Manifest, error) {
	var manifest Manifest
	if err := json.NewDecoder(body).Decode(&manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}
