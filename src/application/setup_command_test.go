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

func TestSetupCommand(t *testing.T) {
	values := map[string]string{"CMS_URL": "https://cms.example.com", "AWS_REGION": "us-east-1"}

	t.Run("Should set up the CLI", func(t *testing.T) {
		command := NewSetupCommand(fakeConfigReader{values: values}, &fakeConfigWriter{})

		err := command.Run()

		if err != nil {
			t.Errorf("Run() = %v, want nil", err)
		}
	})

	t.Run("Should write the values it read", func(t *testing.T) {
		writer := &fakeConfigWriter{}
		command := NewSetupCommand(fakeConfigReader{values: values}, writer)

		_ = command.Run()

		if !maps.Equal(writer.written, values) {
			t.Errorf("Run() wrote %v, want %v", writer.written, values)
		}
	})

	t.Run("Should fail when the values cannot be read", func(t *testing.T) {
		errRead := errors.New("read failed")
		command := NewSetupCommand(fakeConfigReader{err: errRead}, &fakeConfigWriter{})

		err := command.Run()

		if !errors.Is(err, errRead) {
			t.Errorf("Run() = %v, want %v", err, errRead)
		}
	})

	t.Run("Should not write when the values cannot be read", func(t *testing.T) {
		writer := &fakeConfigWriter{}
		command := NewSetupCommand(fakeConfigReader{values: values, err: errors.New("read failed")}, writer)

		_ = command.Run()

		if writer.written != nil {
			t.Errorf("Run() wrote %v, want nothing", writer.written)
		}
	})

	t.Run("Should fail when .env cannot be written", func(t *testing.T) {
		errWrite := errors.New("write failed")
		command := NewSetupCommand(fakeConfigReader{values: values}, &fakeConfigWriter{err: errWrite})

		err := command.Run()

		if !errors.Is(err, errWrite) {
			t.Errorf("Run() = %v, want %v", err, errWrite)
		}
	})
}
