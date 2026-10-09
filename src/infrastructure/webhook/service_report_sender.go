package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/heronlabs/talon/src/core/entity"
)

// The headers of the request, the media type of its body and the prefix of the token in Authorization.
const (
	contentTypeHeader   = "Content-Type"
	contentTypeJSON     = "application/json"
	authorizationHeader = "Authorization"
	bearerPrefix        = "Bearer "
)

// ErrUnexpectedStatus is the error that an answer of the webhook outside 2xx wraps.
var ErrUnexpectedStatus = errors.New("unexpected status")

// ReportSenderService posts the report to the webhook.
type ReportSenderService struct {
	client *http.Client
}

// NewReportSenderService returns a ReportSenderService that posts through client.
func NewReportSenderService(client *http.Client) *ReportSenderService {
	return &ReportSenderService{client: client}
}

// Send posts the report as JSON to endpoint with ctx, with Authorization: Bearer <token> only when there is a token, and returns the reply of the webhook; an answer outside 2xx wraps ErrUnexpectedStatus.
func (s *ReportSenderService) Send(ctx context.Context, endpoint, token string, report *entity.Report) (*ReportResponse, error) {
	request, err := newRequest(ctx, endpoint, token, report)
	if err != nil {
		return nil, err
	}

	response, err := s.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if err := checkStatus(response.StatusCode); err != nil {
		return nil, err
	}

	return decodeResponse(response.Body)
}

// newRequest returns the POST of the report as JSON to endpoint with ctx, with Authorization: Bearer <token> only when there is a token.
func newRequest(ctx context.Context, endpoint, token string, report *entity.Report) (*http.Request, error) {
	body, err := json.Marshal(report)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	request.Header.Set(contentTypeHeader, contentTypeJSON)
	if token != "" {
		request.Header.Set(authorizationHeader, bearerPrefix+token)
	}

	return request, nil
}

// checkStatus fails with ErrUnexpectedStatus when status is outside 2xx.
func checkStatus(status int) error {
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return fmt.Errorf("%w: %d", ErrUnexpectedStatus, status)
	}

	return nil
}

// decodeResponse returns the reply of the webhook decoded from body.
func decodeResponse(body io.Reader) (*ReportResponse, error) {
	var reply ReportResponse
	if err := json.NewDecoder(body).Decode(&reply); err != nil {
		return nil, err
	}

	return &reply, nil
}
