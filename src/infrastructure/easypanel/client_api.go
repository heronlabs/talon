package easypanel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
)

// rootPath and apiPath are the root of a path and the path of the Easypanel API under its URL, and the header and the prefix of the token in Authorization.
const (
	rootPath            = "/"
	apiPath             = "api"
	authorizationHeader = "Authorization"
	bearerPrefix        = "Bearer "
)

// ErrUnexpectedStatus is the error that an answer of the Easypanel API outside 2xx wraps.
var ErrUnexpectedStatus = errors.New("unexpected status")

// APIClient requests the operations of the Easypanel API.
type APIClient struct {
	client *http.Client
}

// NewAPIClient returns an APIClient that requests through client.
func NewAPIClient(client *http.Client) *APIClient {
	return &APIClient{client: client}
}

// Get requests operation with query from the Easypanel API at endpoint, GET <endpoint>/api/<operation>, with ctx and Authorization: Bearer <token>, and decodes its JSON answer into target; its error is prefixed with operation, and an answer outside 2xx wraps ErrUnexpectedStatus.
func (c *APIClient) Get(ctx context.Context, endpoint, token, operation string, query url.Values, target any) error {
	if err := c.get(ctx, endpoint, token, operation, query, target); err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}

	return nil
}

// get requests operation with query from the Easypanel API at endpoint with ctx and token and decodes its JSON answer into target.
func (c *APIClient) get(ctx context.Context, endpoint, token, operation string, query url.Values, target any) error {
	request, err := newRequest(ctx, endpoint, token, operation, query)
	if err != nil {
		return err
	}

	response, err := c.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if err := checkStatus(response.StatusCode); err != nil {
		return err
	}

	return json.NewDecoder(response.Body).Decode(target)
}

// newRequest returns the GET of operation with query under the API path of endpoint, with ctx and Authorization: Bearer <token>.
func newRequest(ctx context.Context, endpoint, token, operation string, query url.Values) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, err
	}

	request.URL.Path = path.Join(rootPath, request.URL.Path, apiPath, operation)
	request.URL.RawQuery = query.Encode()
	request.Header.Set(authorizationHeader, bearerPrefix+token)

	return request, nil
}

// checkStatus fails with ErrUnexpectedStatus when status is outside 2xx.
func checkStatus(status int) error {
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return fmt.Errorf("%w: %d", ErrUnexpectedStatus, status)
	}

	return nil
}
