package core

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

func TestSetupService(t *testing.T) {
	t.Run("Should set up the CLI", func(t *testing.T) {
		values := map[string]string{"WEBHOOK_URL": "https://webhook.example.com", "AWS_REGION": "us-east-1"}
		service := NewSetupService(fakeConfigReader{values: values}, &fakeConfigWriter{})

		err := service.Configure()

		if err != nil {
			t.Errorf("Configure() = %v, want nil", err)
		}
	})

	t.Run("Should write the values it read", func(t *testing.T) {
		values := map[string]string{"WEBHOOK_URL": "https://webhook.example.com", "AWS_REGION": "us-east-1"}
		writer := &fakeConfigWriter{}
		service := NewSetupService(fakeConfigReader{values: values}, writer)

		_ = service.Configure()

		if !maps.Equal(writer.written, values) {
			t.Errorf("Configure() wrote %v, want %v", writer.written, values)
		}
	})

	t.Run("Should fail when the values cannot be read", func(t *testing.T) {
		errRead := errors.New("read failed")
		service := NewSetupService(fakeConfigReader{err: errRead}, &fakeConfigWriter{})

		err := service.Configure()

		if !errors.Is(err, errRead) {
			t.Errorf("Configure() = %v, want %v", err, errRead)
		}
	})

	t.Run("Should not write when the values cannot be read", func(t *testing.T) {
		values := map[string]string{"WEBHOOK_URL": "https://webhook.example.com", "AWS_REGION": "us-east-1"}
		writer := &fakeConfigWriter{}
		service := NewSetupService(fakeConfigReader{values: values, err: errors.New("read failed")}, writer)

		_ = service.Configure()

		if writer.written != nil {
			t.Errorf("Configure() wrote %v, want nothing", writer.written)
		}
	})

	t.Run("Should fail when .env cannot be written", func(t *testing.T) {
		values := map[string]string{"WEBHOOK_URL": "https://webhook.example.com", "AWS_REGION": "us-east-1"}
		errWrite := errors.New("write failed")
		service := NewSetupService(fakeConfigReader{values: values}, &fakeConfigWriter{err: errWrite})

		err := service.Configure()

		if !errors.Is(err, errWrite) {
			t.Errorf("Configure() = %v, want %v", err, errWrite)
		}
	})
}
