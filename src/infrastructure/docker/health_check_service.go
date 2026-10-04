package docker

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

var ErrUnexpectedStatus = errors.New("unexpected status")

type HealthCheck interface {
	Ping() error
}

type HealthCheckService struct {
	client *http.Client
}

func NewHealthCheckService(socketPath string, timeout time.Duration) *HealthCheckService {
	dialer := &net.Dialer{}

	return &HealthCheckService{client: &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return dialer.DialContext(ctx, "unix", socketPath)
			},
		},
	}}
}

func (s *HealthCheckService) Ping() error {
	response, err := s.client.Get("http://docker/_ping")
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %d", ErrUnexpectedStatus, response.StatusCode)
	}

	return nil
}
