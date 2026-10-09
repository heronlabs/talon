package docker

import "context"

// pingPath is the path of the ping of the Engine API.
const pingPath = "/_ping"

// HealthCheckService pings the Docker Engine.
type HealthCheckService struct {
	client *SocketClient
}

// NewHealthCheckService returns a HealthCheckService that requests the Engine API through client.
func NewHealthCheckService(client *SocketClient) *HealthCheckService {
	return &HealthCheckService{client: client}
}

// Ping fails when the Engine API does not answer the ping with 200 before ctx is done.
func (s *HealthCheckService) Ping(ctx context.Context) error {
	return s.client.Get(ctx, pingPath, nil)
}
