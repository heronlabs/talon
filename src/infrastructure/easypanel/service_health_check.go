package easypanel

import (
	"context"
	"errors"
)

// getUserOperation is the operation of the Easypanel API that returns the user of the token.
const getUserOperation = "getUser"

// ErrTokenRefused is the error of a token for which getUser answers no user.
var ErrTokenRefused = errors.New("token refused")

// HealthCheckService checks the token of the Easypanel API.
type HealthCheckService struct {
	client *APIClient
}

// NewHealthCheckService returns a HealthCheckService that requests the Easypanel API through client.
func NewHealthCheckService(client *APIClient) *HealthCheckService {
	return &HealthCheckService{client: client}
}

// Check fails when getUser of the Easypanel API at endpoint fails with token and ctx, or with ErrTokenRefused when it answers no user, as Easypanel does for a token it refuses.
func (s *HealthCheckService) Check(ctx context.Context, endpoint, token string) error {
	var user *User
	if err := s.client.Get(ctx, endpoint, token, getUserOperation, nil, &user); err != nil {
		return err
	}

	if user == nil {
		return ErrTokenRefused
	}

	return nil
}
