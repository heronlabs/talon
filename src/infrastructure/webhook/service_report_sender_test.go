package webhook

import (
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

	"github.com/heronlabs/talon/src/core/entity"
)

type webhookRequest struct {
	method string
	path   string
	header http.Header
	body   []byte
}

func startWebhook(t *testing.T, status int, reply string) (string, *webhookRequest) {
	t.Helper()

	received := &webhookRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.method = r.Method
		received.path = r.URL.Path
		received.header = r.Header
		received.body, _ = io.ReadAll(r.Body)
		w.WriteHeader(status)
		_, _ = io.WriteString(w, reply)
	}))
	t.Cleanup(server.Close)

	return server.URL, received
}

func TestReportSenderService(t *testing.T) {
	t.Run("Should post the report as JSON to the endpoint", func(t *testing.T) {
		webhookURL, received := startWebhook(t, http.StatusOK, `{"data":{}}`)
		service := NewReportSenderService(NewHTTPClient(time.Second))
		report := &entity.Report{Host: "vps-01", MeasuredAt: "2026-10-08T12:30:15.123456789Z", Agent: entity.Agent{Version: "1.2.3"}, Projects: []entity.Project{}}

		_, _ = service.Send(webhookURL+"/reports", "", report)
		var decoded entity.Report
		_ = json.Unmarshal(received.body, &decoded)

		if received.method != http.MethodPost || received.path != "/reports" || !reflect.DeepEqual(decoded, *report) {
			t.Errorf("Send() sent %s %s %+v, want %s %s %+v", received.method, received.path, decoded, http.MethodPost, "/reports", *report)
		}
	})

	t.Run("Should set Content-Type: application/json", func(t *testing.T) {
		webhookURL, received := startWebhook(t, http.StatusOK, `{"data":{}}`)
		service := NewReportSenderService(NewHTTPClient(time.Second))

		_, _ = service.Send(webhookURL, "", &entity.Report{})

		if received.header.Get("Content-Type") != "application/json" {
			t.Errorf("Send() sent Content-Type %q, want %q", received.header.Get("Content-Type"), "application/json")
		}
	})

	t.Run("Should set Authorization: Bearer <token> when there is a token", func(t *testing.T) {
		webhookURL, received := startWebhook(t, http.StatusOK, `{"data":{}}`)
		service := NewReportSenderService(NewHTTPClient(time.Second))

		_, _ = service.Send(webhookURL, "token", &entity.Report{})

		if received.header.Get("Authorization") != "Bearer token" {
			t.Errorf("Send() sent Authorization %q, want %q", received.header.Get("Authorization"), "Bearer token")
		}
	})

	t.Run("Should send no Authorization header without a token", func(t *testing.T) {
		webhookURL, received := startWebhook(t, http.StatusOK, `{"data":{}}`)
		service := NewReportSenderService(NewHTTPClient(time.Second))

		_, _ = service.Send(webhookURL, "", &entity.Report{})

		if received.header == nil || received.header.Values("Authorization") != nil {
			t.Errorf("Send() sent headers %v, want a request without Authorization", received.header)
		}
	})

	t.Run("Should return the decoded ReportResponse", func(t *testing.T) {
		webhookURL, _ := startWebhook(t, http.StatusOK, `{"data":{"host":"vps-01","measuredAt":"2026-10-08T12:30:15.123456789Z","outcome":"partial","matched":["shop"],"unmatched":["blog"]}}`)
		service := NewReportSenderService(NewHTTPClient(time.Second))
		want := ReportResponse{}
		want.Data.Host = "vps-01"
		want.Data.MeasuredAt = "2026-10-08T12:30:15.123456789Z"
		want.Data.Outcome = "partial"
		want.Data.Matched = []string{"shop"}
		want.Data.Unmatched = []string{"blog"}

		response, _ := service.Send(webhookURL, "", &entity.Report{})

		if response == nil || !reflect.DeepEqual(*response, want) {
			t.Errorf("Send() = %+v, want %+v", response, want)
		}
	})

	t.Run("Should accept a 2xx other than 200", func(t *testing.T) {
		webhookURL, _ := startWebhook(t, http.StatusAccepted, `{"data":{}}`)
		service := NewReportSenderService(NewHTTPClient(time.Second))

		_, err := service.Send(webhookURL, "", &entity.Report{})

		if err != nil {
			t.Errorf("Send() = %v, want nil", err)
		}
	})

	t.Run("Should fail with ErrUnexpectedStatus on 401", func(t *testing.T) {
		webhookURL, _ := startWebhook(t, http.StatusUnauthorized, `{"error":"unauthorized"}`)
		service := NewReportSenderService(NewHTTPClient(time.Second))

		_, err := service.Send(webhookURL, "wrong", &entity.Report{})

		if !errors.Is(err, ErrUnexpectedStatus) {
			t.Errorf("Send() = %v, want %v", err, ErrUnexpectedStatus)
		}
	})

	t.Run("Should fail with ErrUnexpectedStatus on 300", func(t *testing.T) {
		webhookURL, _ := startWebhook(t, http.StatusMultipleChoices, `{"data":{}}`)
		service := NewReportSenderService(NewHTTPClient(time.Second))

		_, err := service.Send(webhookURL, "", &entity.Report{})

		if !errors.Is(err, ErrUnexpectedStatus) {
			t.Errorf("Send() = %v, want %v", err, ErrUnexpectedStatus)
		}
	})

	t.Run("Should fail when the reply is not JSON", func(t *testing.T) {
		webhookURL, _ := startWebhook(t, http.StatusOK, "stored")
		service := NewReportSenderService(NewHTTPClient(time.Second))
		var syntaxError *json.SyntaxError

		_, err := service.Send(webhookURL, "", &entity.Report{})

		if !errors.As(err, &syntaxError) {
			t.Errorf("Send() = %v, want a %T", err, syntaxError)
		}
	})

	t.Run("Should fail when the server cannot be reached", func(t *testing.T) {
		server := httptest.NewServer(http.NotFoundHandler())
		server.Close()
		service := NewReportSenderService(NewHTTPClient(time.Second))

		_, err := service.Send(server.URL, "", &entity.Report{})

		if !errors.Is(err, syscall.ECONNREFUSED) {
			t.Errorf("Send() = %v, want %v", err, syscall.ECONNREFUSED)
		}
	})

	t.Run("Should fail when the endpoint is invalid", func(t *testing.T) {
		service := NewReportSenderService(NewHTTPClient(time.Second))
		var urlError *url.Error

		_, err := service.Send("://webhook.example.com", "", &entity.Report{})

		if !errors.As(err, &urlError) || urlError.Op != "parse" {
			t.Errorf("Send() = %v, want a %T from parse", err, urlError)
		}
	})
}
