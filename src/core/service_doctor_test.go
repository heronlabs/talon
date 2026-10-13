package core

import (
	"context"
	"errors"
	"io/fs"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

type fakeHealthCheck struct {
	ctx context.Context
	err error
}

func (f *fakeHealthCheck) Ping(ctx context.Context) error {
	f.ctx = ctx

	return f.err
}

type fakeEasypanelHealthCheck struct {
	checked  bool
	ctx      context.Context
	endpoint string
	token    string
	err      error
}

func (f *fakeEasypanelHealthCheck) Check(ctx context.Context, endpoint, token string) error {
	f.checked = true
	f.ctx = ctx
	f.endpoint = endpoint
	f.token = token

	return f.err
}

func TestDoctorService(t *testing.T) {
	t.Run("Should pass when .env has mode 600 and Docker responds", func(t *testing.T) {
		service := NewDoctorService(fstest.MapFS{".env": {Mode: 0o600}}, &fakeHealthCheck{}, &fakeConfigGetter{}, &fakeEasypanelHealthCheck{})

		err := service.Check(t.Context())

		if err != nil {
			t.Errorf("Check() = %v, want nil", err)
		}
	})

	t.Run("Should fail when .env does not exist", func(t *testing.T) {
		service := NewDoctorService(fstest.MapFS{}, &fakeHealthCheck{}, &fakeConfigGetter{}, &fakeEasypanelHealthCheck{})

		err := service.Check(t.Context())

		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Check() = %v, want %v", err, fs.ErrNotExist)
		}
	})

	t.Run("Should fail when .env has mode other than 600", func(t *testing.T) {
		service := NewDoctorService(fstest.MapFS{".env": {Mode: 0o644}}, &fakeHealthCheck{}, &fakeConfigGetter{}, &fakeEasypanelHealthCheck{})

		err := service.Check(t.Context())

		if err == nil || err.Error() != "config: .env has mode 644, want 600" {
			t.Errorf("Check() = %v, want %q", err, "config: .env has mode 644, want 600")
		}
	})

	t.Run("Should ping with the ctx it was given", func(t *testing.T) {
		ctx := t.Context()
		healthCheck := &fakeHealthCheck{}
		service := NewDoctorService(fstest.MapFS{".env": {Mode: 0o600}}, healthCheck, &fakeConfigGetter{}, &fakeEasypanelHealthCheck{})

		_ = service.Check(ctx)

		if healthCheck.ctx != ctx {
			t.Errorf("Check() pinged with %v, want %v", healthCheck.ctx, ctx)
		}
	})

	t.Run("Should fail when Docker does not respond", func(t *testing.T) {
		errPing := errors.New("ping failed")
		service := NewDoctorService(fstest.MapFS{".env": {Mode: 0o600}}, &fakeHealthCheck{err: errPing}, &fakeConfigGetter{}, &fakeEasypanelHealthCheck{})

		err := service.Check(t.Context())

		if !errors.Is(err, errPing) {
			t.Errorf("Check() = %v, want %v", err, errPing)
		}
	})

	t.Run("Should read EASYPANEL_URL and EASYPANEL_TOKEN in one call", func(t *testing.T) {
		config := &fakeConfigGetter{}
		service := NewDoctorService(fstest.MapFS{".env": {Mode: 0o600}}, &fakeHealthCheck{}, config, &fakeEasypanelHealthCheck{})
		want := [][]string{{"EASYPANEL_URL", "EASYPANEL_TOKEN"}}

		_ = service.Check(t.Context())

		if !reflect.DeepEqual(config.requested, want) {
			t.Errorf("Check() requested %v, want %v", config.requested, want)
		}
	})

	t.Run("Should not check Easypanel without EASYPANEL_TOKEN", func(t *testing.T) {
		config := &fakeConfigGetter{values: map[string]string{"EASYPANEL_URL": "http://localhost:3000"}}
		easypanelHealthCheck := &fakeEasypanelHealthCheck{}
		service := NewDoctorService(fstest.MapFS{".env": {Mode: 0o600}}, &fakeHealthCheck{}, config, easypanelHealthCheck)

		_ = service.Check(t.Context())

		if easypanelHealthCheck.checked {
			t.Errorf("Check() checked Easypanel = %v, want %v", easypanelHealthCheck.checked, false)
		}
	})

	t.Run("Should check EASYPANEL_TOKEN against EASYPANEL_URL with the ctx it was given", func(t *testing.T) {
		ctx := t.Context()
		config := &fakeConfigGetter{values: map[string]string{"EASYPANEL_URL": "http://localhost:3000", "EASYPANEL_TOKEN": "easypanel-token"}}
		easypanelHealthCheck := &fakeEasypanelHealthCheck{}
		service := NewDoctorService(fstest.MapFS{".env": {Mode: 0o600}}, &fakeHealthCheck{}, config, easypanelHealthCheck)

		_ = service.Check(ctx)

		if easypanelHealthCheck.ctx != ctx || easypanelHealthCheck.endpoint != "http://localhost:3000" || easypanelHealthCheck.token != "easypanel-token" {
			t.Errorf("Check() checked %q with %q and %v, want %q with %q and %v", easypanelHealthCheck.endpoint, easypanelHealthCheck.token, easypanelHealthCheck.ctx, "http://localhost:3000", "easypanel-token", ctx)
		}
	})

	t.Run("Should pass when Easypanel accepts the token", func(t *testing.T) {
		config := &fakeConfigGetter{values: map[string]string{"EASYPANEL_URL": "http://localhost:3000", "EASYPANEL_TOKEN": "easypanel-token"}}
		service := NewDoctorService(fstest.MapFS{".env": {Mode: 0o600}}, &fakeHealthCheck{}, config, &fakeEasypanelHealthCheck{})

		err := service.Check(t.Context())

		if err != nil {
			t.Errorf("Check() = %v, want nil", err)
		}
	})

	t.Run("Should fail when Easypanel refuses the token", func(t *testing.T) {
		errCheck := errors.New("token refused")
		config := &fakeConfigGetter{values: map[string]string{"EASYPANEL_URL": "http://localhost:3000", "EASYPANEL_TOKEN": "wrong"}}
		service := NewDoctorService(fstest.MapFS{".env": {Mode: 0o600}}, &fakeHealthCheck{}, config, &fakeEasypanelHealthCheck{err: errCheck})

		err := service.Check(t.Context())

		if !errors.Is(err, errCheck) || !strings.HasPrefix(err.Error(), "easypanel: ") {
			t.Errorf("Check() = %v, want %v with prefix %q", err, errCheck, "easypanel: ")
		}
	})

	t.Run("Should fail when the configuration cannot be read", func(t *testing.T) {
		errGet := errors.New("get failed")
		config := &fakeConfigGetter{err: errGet}
		service := NewDoctorService(fstest.MapFS{".env": {Mode: 0o600}}, &fakeHealthCheck{}, config, &fakeEasypanelHealthCheck{})

		err := service.Check(t.Context())

		if !errors.Is(err, errGet) || !strings.HasPrefix(err.Error(), "config: ") {
			t.Errorf("Check() = %v, want %v with prefix %q", err, errGet, "config: ")
		}
	})
}
