package easypanel

import (
	"errors"
	"net/http"
	"reflect"
	"testing"
	"time"
)

func TestHealthCheckService(t *testing.T) {
	t.Run("Should request getUser with Authorization: Bearer <token>", func(t *testing.T) {
		endpoint, received := startEasypanel(t, map[string]easypanelAnswer{
			"/api/getUser?": {status: http.StatusOK, body: `{"email":"admin@example.com"}`},
		})
		service := NewHealthCheckService(NewAPIClient(&http.Client{Timeout: time.Second}))

		_ = service.Check(t.Context(), endpoint, "token")

		if !reflect.DeepEqual(received.requested, []string{"/api/getUser?"}) || received.authorization != "Bearer token" {
			t.Errorf("Check() requested %v with Authorization %q, want %v with %q", received.requested, received.authorization, []string{"/api/getUser?"}, "Bearer token")
		}
	})

	t.Run("Should pass when getUser answers a user", func(t *testing.T) {
		endpoint, _ := startEasypanel(t, map[string]easypanelAnswer{
			"/api/getUser?": {status: http.StatusOK, body: `{"email":"admin@example.com"}`},
		})
		service := NewHealthCheckService(NewAPIClient(&http.Client{Timeout: time.Second}))

		err := service.Check(t.Context(), endpoint, "token")

		if err != nil {
			t.Errorf("Check() = %v, want nil", err)
		}
	})

	t.Run("Should fail with ErrTokenRefused when getUser answers null", func(t *testing.T) {
		endpoint, _ := startEasypanel(t, map[string]easypanelAnswer{
			"/api/getUser?": {status: http.StatusOK, body: `null`},
		})
		service := NewHealthCheckService(NewAPIClient(&http.Client{Timeout: time.Second}))

		err := service.Check(t.Context(), endpoint, "wrong")

		if !errors.Is(err, ErrTokenRefused) {
			t.Errorf("Check() = %v, want %v", err, ErrTokenRefused)
		}
	})

	t.Run("Should fail with ErrUnexpectedStatus prefixed with getUser when getUser answers outside 2xx", func(t *testing.T) {
		endpoint, _ := startEasypanel(t, map[string]easypanelAnswer{
			"/api/getUser?": {status: http.StatusInternalServerError, body: `{}`},
		})
		service := NewHealthCheckService(NewAPIClient(&http.Client{Timeout: time.Second}))

		err := service.Check(t.Context(), endpoint, "token")

		if !errors.Is(err, ErrUnexpectedStatus) || err.Error() != "getUser: unexpected status: 500" {
			t.Errorf("Check() = %v, want %q", err, "getUser: unexpected status: 500")
		}
	})
}
