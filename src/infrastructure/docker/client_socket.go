package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// engineURL and socketNetwork are the base URL of the requests to the Engine API and the network of its socket.
const (
	engineURL     = "http://docker"
	socketNetwork = "unix"
)

// ErrUnexpectedStatus is the error that an answer of the Engine API other than 200 and 404 wraps.
var ErrUnexpectedStatus = errors.New("unexpected status")

// ErrNotFound is the error of an answer 404 of the Engine API.
var ErrNotFound = errors.New("not found")

// SocketClient requests the Engine API over the unix socket of the Docker Engine.
type SocketClient struct {
	client *http.Client
}

// NewSocketClient returns a SocketClient that dials socketPath and gives up on a request after timeout.
func NewSocketClient(socketPath string, timeout time.Duration) *SocketClient {
	dialer := &net.Dialer{}

	return &SocketClient{client: &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return dialer.DialContext(ctx, socketNetwork, socketPath)
			},
		},
	}}
}

// Get requests path from the Engine API with ctx and decodes its JSON answer into target, unless target is nil; an answer 404 is ErrNotFound and any other than 200 wraps ErrUnexpectedStatus.
func (c *SocketClient) Get(ctx context.Context, path string, target any) error {
	body, err := c.Open(ctx, path)
	if err != nil {
		return err
	}
	defer body.Close()

	if target == nil {
		return nil
	}

	return json.NewDecoder(body).Decode(target)
}

// Open requests path from the Engine API with ctx and returns the body of its answer, which the caller closes; an answer 404 is ErrNotFound and any other than 200 wraps ErrUnexpectedStatus.
func (c *SocketClient) Open(ctx context.Context, path string) (io.ReadCloser, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, engineURL+path, nil)
	if err != nil {
		return nil, err
	}

	response, err := c.client.Do(request)
	if err != nil {
		return nil, err
	}

	if err := checkStatus(response.StatusCode); err != nil {
		_ = response.Body.Close()
		return nil, err
	}

	return response.Body, nil
}

// checkStatus returns nil on 200, ErrNotFound on 404 and an error wrapping ErrUnexpectedStatus on any other status.
func checkStatus(status int) error {
	switch status {
	case http.StatusOK:
		return nil
	case http.StatusNotFound:
		return ErrNotFound
	default:
		return fmt.Errorf("%w: %d", ErrUnexpectedStatus, status)
	}
}
