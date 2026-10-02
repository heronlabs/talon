// Package github reads the agent's releases through the GitHub REST API.
package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/heronlabs/vps-agent/internal/core/domain"
)

// Errors of the releases API.
var (
	ErrUnexpectedStatus = errors.New("unexpected github status")
	ErrAssetTooLarge    = errors.New("release asset is too large")
)

// TokenSource hands out the token the API is called with; config.File is one.
type TokenSource interface {
	GitHubToken() (string, error)
}

// Client reads releases of one repository.
type Client struct {
	http     *http.Client
	repoURL  string
	tokens   TokenSource
	maxAsset int64
}

// New builds a Client for repoURL (https://api.github.com/repos/<owner>/<repo>, no trailing
// slash). The token is read from tokens on every call, so nothing reads it before it is needed.
// Assets over maxAsset bytes are refused. Go's client drops the Authorization header when an
// asset download redirects to another host.
func New(httpClient *http.Client, repoURL string, tokens TokenSource, maxAsset int64) *Client {
	return &Client{http: httpClient, repoURL: repoURL, tokens: tokens, maxAsset: maxAsset}
}

type releaseDTO struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"assets"`
}

// Release implements port.ReleaseSource; version "" is the latest release.
func (c *Client) Release(ctx context.Context, version string) (domain.Release, error) {
	path := "/releases/latest"
	if version != "" {
		path = "/releases/tags/v" + url.PathEscape(version)
	}
	resp, err := c.get(ctx, path, "application/vnd.github+json")
	if err != nil {
		return domain.Release{}, err
	}
	defer resp.Body.Close()
	var dto releaseDTO
	if err := json.NewDecoder(resp.Body).Decode(&dto); err != nil {
		return domain.Release{}, fmt.Errorf("decode GET %s: %w", path, err)
	}
	rel := domain.Release{Tag: dto.TagName}
	for _, a := range dto.Assets {
		rel.Assets = append(rel.Assets, domain.Asset{ID: a.ID, Name: a.Name})
	}
	return rel, nil
}

// Download implements port.ReleaseSource.
func (c *Client) Download(ctx context.Context, asset domain.Asset) ([]byte, error) {
	resp, err := c.get(ctx, fmt.Sprintf("/releases/assets/%d", asset.ID), "application/octet-stream")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, c.maxAsset+1))
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", asset.Name, err)
	}
	if int64(len(data)) > c.maxAsset {
		return nil, fmt.Errorf("%w: %s is over %d bytes", ErrAssetTooLarge, asset.Name, c.maxAsset)
	}
	return data, nil
}

// get calls the API. Errors name the path, never the token.
func (c *Client) get(ctx context.Context, path, accept string) (*http.Response, error) {
	token, err := c.tokens.GitHubToken()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.repoURL+path, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("build GET %s: %w", path, err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", accept)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("%w: GET %s: %s", ErrUnexpectedStatus, path, resp.Status)
	}
	return resp, nil
}
