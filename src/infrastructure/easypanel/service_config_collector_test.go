package easypanel

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

type easypanelAnswer struct {
	status int
	body   string
}

type easypanelRequests struct {
	requested     []string
	authorization string
}

func startEasypanel(t *testing.T, answers map[string]easypanelAnswer) (string, *easypanelRequests) {
	t.Helper()

	received := &easypanelRequests{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request := r.URL.Path + "?" + r.URL.RawQuery
		received.requested = append(received.requested, request)
		received.authorization = r.Header.Get("Authorization")
		answer, found := answers[request]
		if !found {
			answer = easypanelAnswer{status: http.StatusNotFound, body: `{"code":"NOT_FOUND","status":404,"message":"Service not found."}`}
		}

		w.WriteHeader(answer.status)
		_, _ = io.WriteString(w, answer.body)
	}))
	t.Cleanup(server.Close)

	return server.URL, received
}

func TestConfigCollectorService(t *testing.T) {
	t.Run("Should list the services with Authorization: Bearer <token>", func(t *testing.T) {
		endpoint, received := startEasypanel(t, map[string]easypanelAnswer{
			"/api/listProjectsAndServices?": {status: http.StatusOK, body: `{"projects":[],"services":[]}`},
		})
		service := NewConfigCollectorService(NewAPIClient(&http.Client{Timeout: time.Second}))

		_, _ = service.Collect(t.Context(), endpoint, "token")

		if !reflect.DeepEqual(received.requested, []string{"/api/listProjectsAndServices?"}) || received.authorization != "Bearer token" {
			t.Errorf("Collect() requested %v with Authorization %q, want %v with %q", received.requested, received.authorization, []string{"/api/listProjectsAndServices?"}, "Bearer token")
		}
	})

	t.Run("Should request the API under the path of the endpoint", func(t *testing.T) {
		endpoint, received := startEasypanel(t, map[string]easypanelAnswer{
			"/easypanel/api/listProjectsAndServices?": {status: http.StatusOK, body: `{"services":[]}`},
		})
		service := NewConfigCollectorService(NewAPIClient(&http.Client{Timeout: time.Second}))

		_, _ = service.Collect(t.Context(), endpoint+"/easypanel/", "token")

		if !reflect.DeepEqual(received.requested, []string{"/easypanel/api/listProjectsAndServices?"}) {
			t.Errorf("Collect() requested %v, want %v", received.requested, []string{"/easypanel/api/listProjectsAndServices?"})
		}
	})

	t.Run("Should return an app with its domains in the shape of the schema", func(t *testing.T) {
		endpoint, _ := startEasypanel(t, map[string]easypanelAnswer{
			"/api/listProjectsAndServices?":                           {status: http.StatusOK, body: `{"services":[{"name":"api","projectName":"shop","type":"app"}]}`},
			"/api/inspectAppService?projectName=shop&serviceName=api": {status: http.StatusOK, body: `{"name":"api","projectName":"shop","type":"app","source":{"type":"image","image":"nginx:latest"},"resources":{"memoryLimit":1024}}`},
			"/api/listDomains?projectName=shop&serviceName=api":       {status: http.StatusOK, body: `[{"certificateResolver":"","destinationType":"service","host":"shop.example.com","https":true,"id":"d1","middlewares":[],"path":"/","serviceDestination":{"path":"/app","port":80,"projectName":"shop","protocol":"http","serviceName":"api"},"wildcard":false}]`},
		})
		service := NewConfigCollectorService(NewAPIClient(&http.Client{Timeout: time.Second}))
		want := ServiceConfig{Type: "app", Data: map[string]any{
			"name":        "api",
			"projectName": "shop",
			"type":        "app",
			"source":      map[string]any{"type": "image", "image": "nginx:latest"},
			"resources":   map[string]any{"memoryLimit": 1024.0},
			"domains": []any{map[string]any{
				"host":                "shop.example.com",
				"https":               true,
				"port":                80.0,
				"path":                "/",
				"internalProtocol":    "http",
				"certificateResolver": "",
				"middlewares":         []any{},
				"wildcard":            false,
			}},
		}}

		snapshot, _ := service.Collect(t.Context(), endpoint, "token")

		if snapshot == nil || !reflect.DeepEqual(snapshot.Services["shop"]["api"], want) {
			t.Errorf("Collect() = %+v, want shop/api %+v", snapshot, want)
		}
	})

	t.Run("Should return a postgres service read with inspectPostgresService", func(t *testing.T) {
		endpoint, _ := startEasypanel(t, map[string]easypanelAnswer{
			"/api/listProjectsAndServices?":                               {status: http.StatusOK, body: `{"services":[{"name":"db","projectName":"shop","type":"postgres"}]}`},
			"/api/inspectPostgresService?projectName=shop&serviceName=db": {status: http.StatusOK, body: `{"name":"db","image":"postgres:18"}`},
		})
		service := NewConfigCollectorService(NewAPIClient(&http.Client{Timeout: time.Second}))
		want := ServiceConfig{Type: "postgres", Data: map[string]any{"name": "db", "image": "postgres:18"}}

		snapshot, _ := service.Collect(t.Context(), endpoint, "token")

		if snapshot == nil || !reflect.DeepEqual(snapshot.Services["shop"]["db"], want) {
			t.Errorf("Collect() = %+v, want shop/db %+v", snapshot, want)
		}
	})

	t.Run("Should return a mysql service read with inspectMySQLService", func(t *testing.T) {
		endpoint, _ := startEasypanel(t, map[string]easypanelAnswer{
			"/api/listProjectsAndServices?":                            {status: http.StatusOK, body: `{"services":[{"name":"db","projectName":"shop","type":"mysql"}]}`},
			"/api/inspectMySQLService?projectName=shop&serviceName=db": {status: http.StatusOK, body: `{"name":"db","image":"mysql:9"}`},
		})
		service := NewConfigCollectorService(NewAPIClient(&http.Client{Timeout: time.Second}))
		want := ServiceConfig{Type: "mysql", Data: map[string]any{"name": "db", "image": "mysql:9"}}

		snapshot, _ := service.Collect(t.Context(), endpoint, "token")

		if snapshot == nil || !reflect.DeepEqual(snapshot.Services["shop"]["db"], want) {
			t.Errorf("Collect() = %+v, want shop/db %+v", snapshot, want)
		}
	})

	t.Run("Should not list the domains of a service other than an app", func(t *testing.T) {
		endpoint, received := startEasypanel(t, map[string]easypanelAnswer{
			"/api/listProjectsAndServices?":                               {status: http.StatusOK, body: `{"services":[{"name":"db","projectName":"shop","type":"postgres"}]}`},
			"/api/inspectPostgresService?projectName=shop&serviceName=db": {status: http.StatusOK, body: `{"name":"db"}`},
		})
		service := NewConfigCollectorService(NewAPIClient(&http.Client{Timeout: time.Second}))
		want := []string{"/api/listProjectsAndServices?", "/api/inspectPostgresService?projectName=shop&serviceName=db"}

		_, _ = service.Collect(t.Context(), endpoint, "token")

		if !reflect.DeepEqual(received.requested, want) {
			t.Errorf("Collect() requested %v, want %v", received.requested, want)
		}
	})

	t.Run("Should leave out the services of other types", func(t *testing.T) {
		endpoint, received := startEasypanel(t, map[string]easypanelAnswer{
			"/api/listProjectsAndServices?": {status: http.StatusOK, body: `{"services":[{"name":"cache","projectName":"shop","type":"redis"}]}`},
		})
		service := NewConfigCollectorService(NewAPIClient(&http.Client{Timeout: time.Second}))

		snapshot, _ := service.Collect(t.Context(), endpoint, "token")

		if snapshot == nil || len(snapshot.Services) != 0 || len(received.requested) != 1 {
			t.Errorf("Collect() = %+v after requesting %v, want no service after listing them", snapshot, received.requested)
		}
	})

	t.Run("Should leave out the env, password, token and deploymentUrl of a service", func(t *testing.T) {
		endpoint, _ := startEasypanel(t, map[string]easypanelAnswer{
			"/api/listProjectsAndServices?":                               {status: http.StatusOK, body: `{"services":[{"name":"db","projectName":"shop","type":"postgres"}]}`},
			"/api/inspectPostgresService?projectName=shop&serviceName=db": {status: http.StatusOK, body: `{"image":"postgres:18","env":"KEY=value","password":"secret","token":"deploy-token","deploymentUrl":"http://easypanel.example.com/api/deploy/deploy-token"}`},
		})
		service := NewConfigCollectorService(NewAPIClient(&http.Client{Timeout: time.Second}))
		want := ServiceConfig{Type: "postgres", Data: map[string]any{"image": "postgres:18"}}

		snapshot, _ := service.Collect(t.Context(), endpoint, "token")

		if snapshot == nil || !reflect.DeepEqual(snapshot.Services["shop"]["db"], want) {
			t.Errorf("Collect() = %+v, want shop/db %+v", snapshot, want)
		}
	})

	t.Run("Should group the services by project and then by service", func(t *testing.T) {
		endpoint, _ := startEasypanel(t, map[string]easypanelAnswer{
			"/api/listProjectsAndServices?":                               {status: http.StatusOK, body: `{"services":[{"name":"db","projectName":"shop","type":"postgres"},{"name":"db","projectName":"blog","type":"mysql"},{"name":"cache","projectName":"shop","type":"mysql"}]}`},
			"/api/inspectPostgresService?projectName=shop&serviceName=db": {status: http.StatusOK, body: `{"image":"postgres:18"}`},
			"/api/inspectMySQLService?projectName=blog&serviceName=db":    {status: http.StatusOK, body: `{"image":"mysql:9"}`},
			"/api/inspectMySQLService?projectName=shop&serviceName=cache": {status: http.StatusOK, body: `{"image":"mysql:8"}`},
		})
		service := NewConfigCollectorService(NewAPIClient(&http.Client{Timeout: time.Second}))
		want := map[string]map[string]ServiceConfig{
			"shop": {
				"db":    {Type: "postgres", Data: map[string]any{"image": "postgres:18"}},
				"cache": {Type: "mysql", Data: map[string]any{"image": "mysql:8"}},
			},
			"blog": {"db": {Type: "mysql", Data: map[string]any{"image": "mysql:9"}}},
		}

		snapshot, _ := service.Collect(t.Context(), endpoint, "token")

		if snapshot == nil || !reflect.DeepEqual(snapshot.Services, want) {
			t.Errorf("Collect() = %+v, want %+v", snapshot, want)
		}
	})

	t.Run("Should accept a 2xx other than 200", func(t *testing.T) {
		endpoint, _ := startEasypanel(t, map[string]easypanelAnswer{
			"/api/listProjectsAndServices?": {status: http.StatusAccepted, body: `{"services":[]}`},
		})
		service := NewConfigCollectorService(NewAPIClient(&http.Client{Timeout: time.Second}))

		_, err := service.Collect(t.Context(), endpoint, "token")

		if err != nil {
			t.Errorf("Collect() = %v, want nil", err)
		}
	})

	t.Run("Should fail with ErrUnexpectedStatus prefixed with the operation when the services cannot be listed", func(t *testing.T) {
		endpoint, _ := startEasypanel(t, map[string]easypanelAnswer{
			"/api/listProjectsAndServices?": {status: http.StatusUnauthorized, body: `{"code":"UNAUTHORIZED","status":401,"message":"Unauthorized"}`},
		})
		service := NewConfigCollectorService(NewAPIClient(&http.Client{Timeout: time.Second}))

		_, err := service.Collect(t.Context(), endpoint, "wrong")

		if !errors.Is(err, ErrUnexpectedStatus) || err.Error() != "listProjectsAndServices: unexpected status: 401" {
			t.Errorf("Collect() = %v, want %q", err, "listProjectsAndServices: unexpected status: 401")
		}
	})

	t.Run("Should fail with ErrUnexpectedStatus on 300", func(t *testing.T) {
		endpoint, _ := startEasypanel(t, map[string]easypanelAnswer{
			"/api/listProjectsAndServices?": {status: http.StatusMultipleChoices, body: `{"services":[]}`},
		})
		service := NewConfigCollectorService(NewAPIClient(&http.Client{Timeout: time.Second}))

		_, err := service.Collect(t.Context(), endpoint, "token")

		if !errors.Is(err, ErrUnexpectedStatus) {
			t.Errorf("Collect() = %v, want %v", err, ErrUnexpectedStatus)
		}
	})

	t.Run("Should fail when a listed service is not found", func(t *testing.T) {
		endpoint, _ := startEasypanel(t, map[string]easypanelAnswer{
			"/api/listProjectsAndServices?": {status: http.StatusOK, body: `{"services":[{"name":"api","projectName":"shop","type":"app"}]}`},
		})
		service := NewConfigCollectorService(NewAPIClient(&http.Client{Timeout: time.Second}))

		_, err := service.Collect(t.Context(), endpoint, "token")

		if !errors.Is(err, ErrUnexpectedStatus) || err.Error() != "inspectAppService: unexpected status: 404" {
			t.Errorf("Collect() = %v, want %q", err, "inspectAppService: unexpected status: 404")
		}
	})

	t.Run("Should fail when the domains of an app cannot be listed", func(t *testing.T) {
		endpoint, _ := startEasypanel(t, map[string]easypanelAnswer{
			"/api/listProjectsAndServices?":                           {status: http.StatusOK, body: `{"services":[{"name":"api","projectName":"shop","type":"app"}]}`},
			"/api/inspectAppService?projectName=shop&serviceName=api": {status: http.StatusOK, body: `{"name":"api"}`},
			"/api/listDomains?projectName=shop&serviceName=api":       {status: http.StatusInternalServerError, body: `{}`},
		})
		service := NewConfigCollectorService(NewAPIClient(&http.Client{Timeout: time.Second}))

		_, err := service.Collect(t.Context(), endpoint, "token")

		if !errors.Is(err, ErrUnexpectedStatus) || err.Error() != "listDomains: unexpected status: 500" {
			t.Errorf("Collect() = %v, want %q", err, "listDomains: unexpected status: 500")
		}
	})

	t.Run("Should fail when the answer is not JSON", func(t *testing.T) {
		endpoint, _ := startEasypanel(t, map[string]easypanelAnswer{
			"/api/listProjectsAndServices?": {status: http.StatusOK, body: "services"},
		})
		service := NewConfigCollectorService(NewAPIClient(&http.Client{Timeout: time.Second}))
		var syntaxError *json.SyntaxError

		_, err := service.Collect(t.Context(), endpoint, "token")

		if !errors.As(err, &syntaxError) || !strings.HasPrefix(err.Error(), "listProjectsAndServices: ") {
			t.Errorf("Collect() = %v, want a %T prefixed with %q", err, syntaxError, "listProjectsAndServices: ")
		}
	})

	t.Run("Should fail when the server cannot be reached", func(t *testing.T) {
		server := httptest.NewServer(http.NotFoundHandler())
		server.Close()
		service := NewConfigCollectorService(NewAPIClient(&http.Client{Timeout: time.Second}))

		_, err := service.Collect(t.Context(), server.URL, "token")

		if !errors.Is(err, syscall.ECONNREFUSED) {
			t.Errorf("Collect() = %v, want %v", err, syscall.ECONNREFUSED)
		}
	})

	t.Run("Should fail when ctx is done", func(t *testing.T) {
		endpoint, _ := startEasypanel(t, map[string]easypanelAnswer{
			"/api/listProjectsAndServices?": {status: http.StatusOK, body: `{"services":[]}`},
		})
		service := NewConfigCollectorService(NewAPIClient(&http.Client{Timeout: time.Second}))
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		_, err := service.Collect(ctx, endpoint, "token")

		if !errors.Is(err, context.Canceled) {
			t.Errorf("Collect() = %v, want %v", err, context.Canceled)
		}
	})

	t.Run("Should fail when the endpoint is invalid", func(t *testing.T) {
		service := NewConfigCollectorService(NewAPIClient(&http.Client{Timeout: time.Second}))
		var urlError *url.Error

		_, err := service.Collect(t.Context(), "://easypanel.example.com", "token")

		if !errors.As(err, &urlError) || urlError.Op != "parse" {
			t.Errorf("Collect() = %v, want a %T from parse", err, urlError)
		}
	})
}
