package github_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/heronlabs/vps-agent/internal/core/domain"
	"github.com/heronlabs/vps-agent/internal/infrastructure/github"
	"github.com/heronlabs/vps-agent/internal/testkit"
)

const (
	fakePAT     = "github-pat-fake"
	releaseJSON = `{"tag_name":"v0.3.0","name":"ignored","assets":[{"id":1,"name":"checksums.txt","size":90},{"id":3,"name":"vps-agent_0.3.0_linux_amd64"}]}`
)

var errNoToken = errors.New("no token")

type tokens struct{ err error }

func (s tokens) GitHubToken() (string, error) { return fakePAT, s.err }

type route struct {
	status int
	body   string
}

// server answers each path from routes and records the headers of the last request.
func server(t *testing.T, routes map[string]route) (*httptest.Server, *http.Header) {
	t.Helper()
	last := &http.Header{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*last = r.Header.Clone()
		rt, ok := routes[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(rt.status)
		_, _ = io.WriteString(w, rt.body)
	}))
	t.Cleanup(srv.Close)
	return srv, last
}

func client(srv *httptest.Server) *github.Client {
	return github.New(srv.Client(), srv.URL+"/repos/heronlabs/vps-agent", tokens{}, 4)
}

var latest = map[string]route{
	"/repos/heronlabs/vps-agent/releases/latest":   {http.StatusOK, releaseJSON},
	"/repos/heronlabs/vps-agent/releases/assets/3": {http.StatusOK, "abcd"},
	"/repos/heronlabs/vps-agent/releases/assets/4": {http.StatusOK, "abcde"},
}

func TestRelease_ReadsTheLatestRelease(t *testing.T) {
	srv, _ := server(t, latest)

	got := testkit.Must(client(srv).Release(context.Background(), ""))

	testkit.Equal(t, domain.Release{Tag: "v0.3.0", Assets: []domain.Asset{
		{ID: 1, Name: "checksums.txt"}, {ID: 3, Name: "vps-agent_0.3.0_linux_amd64"},
	}}, got)
}

func TestRelease_ReadsTheReleaseOfAVersion(t *testing.T) {
	srv, _ := server(t, map[string]route{
		"/repos/heronlabs/vps-agent/releases/tags/v0.2.1": {http.StatusOK, `{"tag_name":"v0.2.1"}`},
	})

	got := testkit.Must(client(srv).Release(context.Background(), "0.2.1"))

	testkit.Equal(t, domain.Release{Tag: "v0.2.1"}, got)
}

func TestRelease_SendsTheToken(t *testing.T) {
	srv, last := server(t, latest)

	_ = testkit.Must(client(srv).Release(context.Background(), ""))

	testkit.Equal(t, "Bearer "+fakePAT, last.Get("Authorization"))
}

func TestRelease_AsksForJSON(t *testing.T) {
	srv, last := server(t, latest)

	_ = testkit.Must(client(srv).Release(context.Background(), ""))

	testkit.Equal(t, "application/vnd.github+json", last.Get("Accept"))
}

func TestRelease_FailsWithoutAToken(t *testing.T) {
	srv, _ := server(t, latest)

	_, err := github.New(srv.Client(), srv.URL, tokens{err: errNoToken}, 4).Release(context.Background(), "")

	testkit.ErrorIs(t, err, errNoToken)
}

func TestRelease_FailsOnAMissingRelease(t *testing.T) {
	srv, _ := server(t, latest)

	_, err := client(srv).Release(context.Background(), "9.9.9")

	testkit.ErrorIs(t, err, github.ErrUnexpectedStatus)
}

func TestRelease_KeepsTheTokenOutOfTheError(t *testing.T) {
	srv, _ := server(t, latest)

	_, err := client(srv).Release(context.Background(), "9.9.9")

	testkit.Equal(t, "unexpected github status: GET /releases/tags/v9.9.9: 404 Not Found", err.Error())
}

func TestRelease_FailsOnABodyThatIsNotJSON(t *testing.T) {
	srv, _ := server(t, map[string]route{"/repos/heronlabs/vps-agent/releases/latest": {http.StatusOK, "<html>"}})

	_, err := client(srv).Release(context.Background(), "")

	testkit.Equal(t, true, strings.HasPrefix(err.Error(), "decode GET /releases/latest: "))
}

func TestRelease_FailsWhenTheServerIsDown(t *testing.T) {
	srv, _ := server(t, latest)
	srv.Close()

	_, err := client(srv).Release(context.Background(), "")

	testkit.Equal(t, true, strings.HasPrefix(err.Error(), "GET /releases/latest: "))
}

func TestRelease_FailsOnAnInvalidURL(t *testing.T) {
	_, err := github.New(http.DefaultClient, "http://\x7f", tokens{}, 4).Release(context.Background(), "")

	testkit.Equal(t, true, strings.HasPrefix(err.Error(), "build GET /releases/latest: "))
}

func TestDownload_ReturnsAnAssetOfExactlyTheLimit(t *testing.T) {
	srv, _ := server(t, latest)

	got := testkit.Must(client(srv).Download(context.Background(), domain.Asset{ID: 3, Name: "bin"}))

	testkit.Equal(t, "abcd", string(got))
}

func TestDownload_AsksForTheRawAsset(t *testing.T) {
	srv, last := server(t, latest)

	_ = testkit.Must(client(srv).Download(context.Background(), domain.Asset{ID: 3, Name: "bin"}))

	testkit.Equal(t, "application/octet-stream", last.Get("Accept"))
}

func TestDownload_RefusesAnAssetOverTheLimit(t *testing.T) {
	srv, _ := server(t, latest)

	_, err := client(srv).Download(context.Background(), domain.Asset{ID: 4, Name: "bin"})

	testkit.ErrorIs(t, err, github.ErrAssetTooLarge)
}

func TestDownload_FailsOnAMissingAsset(t *testing.T) {
	srv, _ := server(t, latest)

	_, err := client(srv).Download(context.Background(), domain.Asset{ID: 5, Name: "bin"})

	testkit.ErrorIs(t, err, github.ErrUnexpectedStatus)
}

func TestDownload_FailsWhenTheBodyIsCutShort(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "4")
		_, _ = io.WriteString(w, "ab")
	}))
	t.Cleanup(srv.Close)

	_, err := client(srv).Download(context.Background(), domain.Asset{ID: 3, Name: "bin"})

	testkit.ErrorIs(t, err, io.ErrUnexpectedEOF)
}

func TestDownload_DropsTheTokenWhenRedirectedToAnotherHost(t *testing.T) {
	storage, last := server(t, map[string]route{"/blob": {http.StatusOK, "abcd"}})
	storageURL := strings.Replace(storage.URL, "127.0.0.1", "localhost", 1) + "/blob"
	api := httptest.NewServer(http.RedirectHandler(storageURL, http.StatusFound))
	t.Cleanup(api.Close)

	_ = testkit.Must(github.New(http.DefaultClient, api.URL, tokens{}, 4).Download(context.Background(), domain.Asset{ID: 3}))

	testkit.Equal(t, "", last.Get("Authorization"))
}
