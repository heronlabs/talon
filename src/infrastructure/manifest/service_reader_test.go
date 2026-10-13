package manifest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"syscall"
	"testing"
	"time"
)

type manifestRequest struct {
	method string
	path   string
	header http.Header
}

func startManifest(t *testing.T, status int, reply string) (string, *manifestRequest) {
	t.Helper()

	received := &manifestRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.method = r.Method
		received.path = r.URL.Path
		received.header = r.Header
		w.WriteHeader(status)
		_, _ = io.WriteString(w, reply)
	}))
	t.Cleanup(server.Close)

	return server.URL, received
}

func TestReaderService(t *testing.T) {
	t.Run("Should get the manifest at the endpoint", func(t *testing.T) {
		manifestURL, received := startManifest(t, http.StatusOK, `{"data":{}}`)
		service := NewReaderService(&http.Client{Timeout: time.Second})

		_, _ = service.Read(t.Context(), manifestURL+"/api/infra-manifests/vps-01", "")

		if received.method != http.MethodGet || received.path != "/api/infra-manifests/vps-01" {
			t.Errorf("Read() requested %s %s, want %s %s", received.method, received.path, http.MethodGet, "/api/infra-manifests/vps-01")
		}
	})

	t.Run("Should set Authorization: Bearer <token> when there is a token", func(t *testing.T) {
		manifestURL, received := startManifest(t, http.StatusOK, `{"data":{}}`)
		service := NewReaderService(&http.Client{Timeout: time.Second})

		_, _ = service.Read(t.Context(), manifestURL, "token")

		if received.header.Get("Authorization") != "Bearer token" {
			t.Errorf("Read() sent Authorization %q, want %q", received.header.Get("Authorization"), "Bearer token")
		}
	})

	t.Run("Should send no Authorization header without a token", func(t *testing.T) {
		manifestURL, received := startManifest(t, http.StatusOK, `{"data":{}}`)
		service := NewReaderService(&http.Client{Timeout: time.Second})

		_, _ = service.Read(t.Context(), manifestURL, "")

		if received.header == nil || received.header.Values("Authorization") != nil {
			t.Errorf("Read() sent headers %v, want a request without Authorization", received.header)
		}
	})

	t.Run("Should return the decoded manifest", func(t *testing.T) {
		manifestURL, _ := startManifest(t, http.StatusOK, `{"data":{"host":"vps-01","clients":[{"slug":"shop","lifecycle":"active","secretsRoleArn":"arn:aws:iam::123456789012:role/talon-secrets-reader","schema":{"services":[{"type":"postgres","data":{"serviceName":"db","image":"postgres:18","resources":{"memoryLimit":512}}}]}}]}}`)
		service := NewReaderService(&http.Client{Timeout: time.Second})
		role := "arn:aws:iam::123456789012:role/talon-secrets-reader"
		client := Client{Slug: "shop", Lifecycle: "active", SecretsRoleArn: &role}
		client.Schema.Services = []Service{{Type: "postgres", Data: map[string]any{"serviceName": "db", "image": "postgres:18", "resources": map[string]any{"memoryLimit": 512.0}}}}
		want := Manifest{}
		want.Data.Host = "vps-01"
		want.Data.Clients = []Client{client}

		manifest, _ := service.Read(t.Context(), manifestURL, "")

		if manifest == nil || !reflect.DeepEqual(*manifest, want) {
			t.Errorf("Read() = %+v, want %+v", manifest, want)
		}
	})

	t.Run("Should accept a 2xx other than 200", func(t *testing.T) {
		manifestURL, _ := startManifest(t, http.StatusAccepted, `{"data":{}}`)
		service := NewReaderService(&http.Client{Timeout: time.Second})

		_, err := service.Read(t.Context(), manifestURL, "")

		if err != nil {
			t.Errorf("Read() = %v, want nil", err)
		}
	})

	t.Run("Should fail with ErrUnexpectedStatus on 401", func(t *testing.T) {
		manifestURL, _ := startManifest(t, http.StatusUnauthorized, `{"error":"unauthorized"}`)
		service := NewReaderService(&http.Client{Timeout: time.Second})

		_, err := service.Read(t.Context(), manifestURL, "wrong")

		if !errors.Is(err, ErrUnexpectedStatus) {
			t.Errorf("Read() = %v, want %v", err, ErrUnexpectedStatus)
		}
	})

	t.Run("Should fail with ErrUnexpectedStatus on 300", func(t *testing.T) {
		manifestURL, _ := startManifest(t, http.StatusMultipleChoices, `{"data":{}}`)
		service := NewReaderService(&http.Client{Timeout: time.Second})

		_, err := service.Read(t.Context(), manifestURL, "")

		if !errors.Is(err, ErrUnexpectedStatus) {
			t.Errorf("Read() = %v, want %v", err, ErrUnexpectedStatus)
		}
	})

	t.Run("Should fail when the answer is not JSON", func(t *testing.T) {
		manifestURL, _ := startManifest(t, http.StatusOK, "manifest")
		service := NewReaderService(&http.Client{Timeout: time.Second})
		var syntaxError *json.SyntaxError

		_, err := service.Read(t.Context(), manifestURL, "")

		if !errors.As(err, &syntaxError) {
			t.Errorf("Read() = %v, want a %T", err, syntaxError)
		}
	})

	t.Run("Should fail when the server cannot be reached", func(t *testing.T) {
		server := httptest.NewServer(http.NotFoundHandler())
		server.Close()
		service := NewReaderService(&http.Client{Timeout: time.Second})

		_, err := service.Read(t.Context(), server.URL, "")

		if !errors.Is(err, syscall.ECONNREFUSED) {
			t.Errorf("Read() = %v, want %v", err, syscall.ECONNREFUSED)
		}
	})

	t.Run("Should fail when ctx is done", func(t *testing.T) {
		manifestURL, _ := startManifest(t, http.StatusOK, `{"data":{}}`)
		service := NewReaderService(&http.Client{Timeout: time.Second})
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		_, err := service.Read(ctx, manifestURL, "")

		if !errors.Is(err, context.Canceled) {
			t.Errorf("Read() = %v, want %v", err, context.Canceled)
		}
	})

	t.Run("Should fail when the endpoint is invalid", func(t *testing.T) {
		service := NewReaderService(&http.Client{Timeout: time.Second})
		var urlError *url.Error

		_, err := service.Read(t.Context(), "://cms.example.com", "")

		if !errors.As(err, &urlError) || urlError.Op != "parse" {
			t.Errorf("Read() = %v, want a %T from parse", err, urlError)
		}
	})
}
