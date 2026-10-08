package docker

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

// Ping fails when the Engine API does not answer the ping with 200.
func (s *HealthCheckService) Ping() error {
	return s.client.Get(pingPath, nil)
}
