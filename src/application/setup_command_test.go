package application

import (
	"errors"
	"maps"
	"testing"
)

type fakeConfigReader struct {
	values map[string]string
	err    error
}

func (f fakeConfigReader) Read() (map[string]string, error) {
	return f.values, f.err
}

type fakeConfigWriter struct {
	written map[string]string
	err     error
}

func (f *fakeConfigWriter) Write(values map[string]string) error {
	f.written = values

	return f.err
}

type fakeAuthorizedKeys struct {
	written string
	err     error
}

func (f *fakeAuthorizedKeys) Write(publicKey string) error {
	f.written = publicKey

	return f.err
}

type fakeHostKey struct {
	key string
	err error
}

func (f fakeHostKey) Read() (string, error) {
	return f.key, f.err
}

func TestSetupCommand(t *testing.T) {
	values := map[string]string{"CMS_URL": "https://cms.example.com", "AWS_REGION": "us-east-1", "DEPLOY_SSH_PUBLIC_KEY": "ssh-ed25519 AAAA"}

	t.Run("Should set up the CLI", func(t *testing.T) {
		command := NewSetupCommand(fakeConfigReader{values: values}, &fakeConfigWriter{}, &fakeAuthorizedKeys{}, fakeHostKey{})

		_, err := command.Run()

		if err != nil {
			t.Errorf("Run() = %v, want nil", err)
		}
	})

	t.Run("Should write the values it read", func(t *testing.T) {
		writer := &fakeConfigWriter{}
		command := NewSetupCommand(fakeConfigReader{values: values}, writer, &fakeAuthorizedKeys{}, fakeHostKey{})

		_, _ = command.Run()

		if !maps.Equal(writer.written, values) {
			t.Errorf("Run() wrote %v, want %v", writer.written, values)
		}
	})

	t.Run("Should write the deploy key it read", func(t *testing.T) {
		authorizedKeys := &fakeAuthorizedKeys{}
		command := NewSetupCommand(fakeConfigReader{values: values}, &fakeConfigWriter{}, authorizedKeys, fakeHostKey{})

		_, _ = command.Run()

		if authorizedKeys.written != "ssh-ed25519 AAAA" {
			t.Errorf("Run() wrote the deploy key %q, want %q", authorizedKeys.written, "ssh-ed25519 AAAA")
		}
	})

	t.Run("Should return the host key", func(t *testing.T) {
		command := NewSetupCommand(fakeConfigReader{values: values}, &fakeConfigWriter{}, &fakeAuthorizedKeys{}, fakeHostKey{key: "ssh-ed25519 BBBB"})

		hostKey, _ := command.Run()

		if hostKey != "ssh-ed25519 BBBB" {
			t.Errorf("Run() = %q, want %q", hostKey, "ssh-ed25519 BBBB")
		}
	})

	t.Run("Should fail when the values cannot be read", func(t *testing.T) {
		errRead := errors.New("read failed")
		command := NewSetupCommand(fakeConfigReader{err: errRead}, &fakeConfigWriter{}, &fakeAuthorizedKeys{}, fakeHostKey{})

		_, err := command.Run()

		if !errors.Is(err, errRead) {
			t.Errorf("Run() = %v, want %v", err, errRead)
		}
	})

	t.Run("Should not write when the values cannot be read", func(t *testing.T) {
		writer := &fakeConfigWriter{}
		command := NewSetupCommand(fakeConfigReader{values: values, err: errors.New("read failed")}, writer, &fakeAuthorizedKeys{}, fakeHostKey{})

		_, _ = command.Run()

		if writer.written != nil {
			t.Errorf("Run() wrote %v, want nothing", writer.written)
		}
	})

	t.Run("Should fail when the deploy key cannot be written", func(t *testing.T) {
		errWrite := errors.New("write failed")
		command := NewSetupCommand(fakeConfigReader{values: values}, &fakeConfigWriter{}, &fakeAuthorizedKeys{err: errWrite}, fakeHostKey{})

		_, err := command.Run()

		if !errors.Is(err, errWrite) {
			t.Errorf("Run() = %v, want %v", err, errWrite)
		}
	})

	t.Run("Should not write .env when the deploy key cannot be written", func(t *testing.T) {
		writer := &fakeConfigWriter{}
		command := NewSetupCommand(fakeConfigReader{values: values}, writer, &fakeAuthorizedKeys{err: errors.New("write failed")}, fakeHostKey{})

		_, _ = command.Run()

		if writer.written != nil {
			t.Errorf("Run() wrote %v, want nothing", writer.written)
		}
	})

	t.Run("Should fail when .env cannot be written", func(t *testing.T) {
		errWrite := errors.New("write failed")
		command := NewSetupCommand(fakeConfigReader{values: values}, &fakeConfigWriter{err: errWrite}, &fakeAuthorizedKeys{}, fakeHostKey{})

		_, err := command.Run()

		if !errors.Is(err, errWrite) {
			t.Errorf("Run() = %v, want %v", err, errWrite)
		}
	})

	t.Run("Should fail when the host key cannot be read", func(t *testing.T) {
		errRead := errors.New("read failed")
		command := NewSetupCommand(fakeConfigReader{values: values}, &fakeConfigWriter{}, &fakeAuthorizedKeys{}, fakeHostKey{err: errRead})

		_, err := command.Run()

		if !errors.Is(err, errRead) {
			t.Errorf("Run() = %v, want %v", err, errRead)
		}
	})
}
