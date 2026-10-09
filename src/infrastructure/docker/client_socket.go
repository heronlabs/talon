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

// ErrUnexpectedStatus is the error that an answer of the Engine API other than 200 wraps.
var ErrUnexpectedStatus = errors.New("unexpected status")

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

// Get requests path from the Engine API and decodes its JSON answer into target, unless target is nil; an answer other than 200 wraps ErrUnexpectedStatus.
func (c *SocketClient) Get(path string, target any) error {
	body, err := c.Open(path)
	if err != nil {
		return err
	}
	defer body.Close()

	if target == nil {
		return nil
	}

	return json.NewDecoder(body).Decode(target)
}

// Open requests path from the Engine API and returns the body of its answer, which the caller closes; an answer other than 200 wraps ErrUnexpectedStatus.
func (c *SocketClient) Open(path string) (io.ReadCloser, error) {
	response, err := c.client.Get(engineURL + path)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		_ = response.Body.Close()
		return nil, fmt.Errorf("%w: %d", ErrUnexpectedStatus, response.StatusCode)
	}

	return response.Body, nil
}
