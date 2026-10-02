// Package docker reads the Docker Engine API through its unix socket, without the SDK.
package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/heronlabs/vps-agent/internal/core/domain"
)

// ErrUnexpectedStatus marks an Engine API answer other than 200.
var ErrUnexpectedStatus = errors.New("unexpected docker status")

// Client talks to the Engine API.
type Client struct {
	http         *http.Client
	baseURL      string
	statsTimeout time.Duration
	logger       *slog.Logger
}

// New builds a Client over any HTTP client; baseURL has no trailing slash. logger records the
// containers whose stats could not be read.
func New(httpClient *http.Client, baseURL string, statsTimeout time.Duration, logger *slog.Logger) *Client {
	return &Client{http: httpClient, baseURL: baseURL, statsTimeout: statsTimeout, logger: logger}
}

// NewUnix builds a Client that dials the engine's unix socket. requestTimeout bounds every
// call; statsTimeout bounds each container's stats, which take about a second.
func NewUnix(socketPath string, requestTimeout, statsTimeout time.Duration, logger *slog.Logger) *Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", socketPath)
		},
	}
	return New(&http.Client{Transport: transport, Timeout: requestTimeout}, "http://docker", statsTimeout, logger)
}

// Ping implements port.Docker.
func (c *Client) Ping(ctx context.Context) error {
	resp, err := c.get(ctx, "/_ping")
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

// Snapshot implements port.Docker. A container whose stats fail or time out has nil Stats and
// is logged.
func (c *Client) Snapshot(ctx context.Context) (domain.DockerSample, error) {
	var (
		services   []serviceDTO
		tasks      []taskDTO
		containers []containerDTO
		df         diskUsageDTO
	)
	reads := []struct {
		path string
		out  any
	}{
		{"/services", &services},
		{"/tasks", &tasks},
		{"/containers/json?all=1", &containers},
		{"/system/df", &df},
	}
	for _, r := range reads {
		if err := c.getJSON(ctx, r.path, r.out); err != nil {
			return domain.DockerSample{}, err
		}
	}
	sample := toSample(services, tasks, containers, df)
	c.fillStats(ctx, sample.Containers)
	return sample, nil
}

func (c *Client) fillStats(ctx context.Context, containers []domain.Container) {
	var wg sync.WaitGroup
	for i := range containers {
		if containers[i].State != domain.StateRunning {
			continue
		}
		wg.Go(func() { containers[i].Stats = c.stats(ctx, containers[i].ID) })
	}
	wg.Wait()
}

func (c *Client) stats(ctx context.Context, id string) *domain.ContainerStats {
	ctx, cancel := context.WithTimeout(ctx, c.statsTimeout)
	defer cancel()
	var s statsDTO
	if err := c.getJSON(ctx, "/containers/"+id+"/stats?stream=false", &s); err != nil {
		c.logger.WarnContext(ctx, "container stats unavailable",
			slog.String("logId", "docker.stats-failed"),
			slog.String("containerId", id),
			slog.String("errorName", fmt.Sprintf("%T", err)),
			slog.String("errorMessage", err.Error()))
		return nil
	}
	stats := s.toDomain()
	return &stats
}

func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	resp, err := c.get(ctx, path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode GET %s: %w", path, err)
	}
	return nil
}

func (c *Client) get(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("build GET %s: %w", path, err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("%w: GET %s: %d", ErrUnexpectedStatus, path, resp.StatusCode)
	}
	return resp, nil
}
