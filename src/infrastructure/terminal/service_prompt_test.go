package terminal

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

type fakeEcho struct {
	hidden     bool
	err        error
	restoreErr error
	offs       int
	restored   bool
}

func (f *fakeEcho) Off() (func() error, bool, error) {
	f.offs++

	return f.restore, f.hidden, f.err
}

func (f *fakeEcho) restore() error {
	f.restored = true

	return f.restoreErr
}

func TestPromptService(t *testing.T) {
	t.Run("Should return the typed answer", func(t *testing.T) {
		prompt := NewPromptService(strings.NewReader(" sa-east-1 \n"), io.Discard, &fakeEcho{})

		answer, _ := prompt.Ask("AWS_REGION", "us-east-1", false)

		if answer != "sa-east-1" {
			t.Errorf("Ask() = %q, want %q", answer, "sa-east-1")
		}
	})

	t.Run("Should return the fallback when the answer is empty", func(t *testing.T) {
		prompt := NewPromptService(strings.NewReader("\n"), io.Discard, &fakeEcho{})

		answer, _ := prompt.Ask("AWS_REGION", "us-east-1", false)

		if answer != "us-east-1" {
			t.Errorf("Ask() = %q, want %q", answer, "us-east-1")
		}
	})

	t.Run("Should return the fallback when the input has ended", func(t *testing.T) {
		prompt := NewPromptService(strings.NewReader(""), io.Discard, &fakeEcho{})

		answer, err := prompt.Ask("AWS_REGION", "us-east-1", false)

		if err != nil || answer != "us-east-1" {
			t.Errorf("Ask() = %q, %v; want %q, nil", answer, err, "us-east-1")
		}
	})

	t.Run("Should answer one question per line", func(t *testing.T) {
		prompt := NewPromptService(strings.NewReader("first\nsecond\n"), io.Discard, &fakeEcho{})
		_, _ = prompt.Ask("WEBHOOK_URL", "", false)

		answer, _ := prompt.Ask("WEBHOOK_TOKEN", "", false)

		if answer != "second" {
			t.Errorf("Ask() = %q, want %q", answer, "second")
		}
	})

	t.Run("Should show the name and the fallback", func(t *testing.T) {
		output := &bytes.Buffer{}
		prompt := NewPromptService(strings.NewReader("\n"), output, &fakeEcho{})

		_, _ = prompt.Ask("AWS_REGION", "us-east-1", false)

		if output.String() != "AWS_REGION [us-east-1]: " {
			t.Errorf("Ask() wrote %q, want %q", output.String(), "AWS_REGION [us-east-1]: ")
		}
	})

	t.Run("Should show only the name when there is no fallback", func(t *testing.T) {
		output := &bytes.Buffer{}
		prompt := NewPromptService(strings.NewReader("\n"), output, &fakeEcho{})

		_, _ = prompt.Ask("WEBHOOK_URL", "", false)

		if output.String() != "WEBHOOK_URL: " {
			t.Errorf("Ask() wrote %q, want %q", output.String(), "WEBHOOK_URL: ")
		}
	})

	t.Run("Should hide the fallback of a secret", func(t *testing.T) {
		output := &bytes.Buffer{}
		prompt := NewPromptService(strings.NewReader("\n"), output, &fakeEcho{})

		_, _ = prompt.Ask("WEBHOOK_TOKEN", "token", true)

		if output.String() != "WEBHOOK_TOKEN [****]: " {
			t.Errorf("Ask() wrote %q, want %q", output.String(), "WEBHOOK_TOKEN [****]: ")
		}
	})

	t.Run("Should show only the name of a secret without fallback", func(t *testing.T) {
		output := &bytes.Buffer{}
		prompt := NewPromptService(strings.NewReader("\n"), output, &fakeEcho{})

		_, _ = prompt.Ask("WEBHOOK_TOKEN", "", true)

		if output.String() != "WEBHOOK_TOKEN: " {
			t.Errorf("Ask() wrote %q, want %q", output.String(), "WEBHOOK_TOKEN: ")
		}
	})

	t.Run("Should return the fallback of a secret when the answer is empty", func(t *testing.T) {
		prompt := NewPromptService(strings.NewReader("\n"), io.Discard, &fakeEcho{})

		answer, _ := prompt.Ask("WEBHOOK_TOKEN", "token", true)

		if answer != "token" {
			t.Errorf("Ask() = %q, want %q", answer, "token")
		}
	})

	t.Run("Should fail when the input cannot be read", func(t *testing.T) {
		errRead := errors.New("read failed")
		prompt := NewPromptService(iotest.ErrReader(errRead), io.Discard, &fakeEcho{})

		_, err := prompt.Ask("WEBHOOK_URL", "", false)

		if !errors.Is(err, errRead) {
			t.Errorf("Ask() = %v, want %v", err, errRead)
		}
	})

	t.Run("Should turn the echo off for a secret", func(t *testing.T) {
		echo := &fakeEcho{}
		prompt := NewPromptService(strings.NewReader("token\n"), io.Discard, echo)

		_, _ = prompt.Ask("WEBHOOK_TOKEN", "", true)

		if echo.offs != 1 {
			t.Errorf("Off() called %d times, want 1", echo.offs)
		}
	})

	t.Run("Should not turn the echo off for a key that is not a secret", func(t *testing.T) {
		echo := &fakeEcho{}
		prompt := NewPromptService(strings.NewReader("sa-east-1\n"), io.Discard, echo)

		_, _ = prompt.Ask("AWS_REGION", "", false)

		if echo.offs != 0 {
			t.Errorf("Off() called %d times, want 0", echo.offs)
		}
	})

	t.Run("Should return the answer of a secret typed with the echo off", func(t *testing.T) {
		prompt := NewPromptService(strings.NewReader(" token \n"), io.Discard, &fakeEcho{hidden: true})

		answer, _ := prompt.Ask("WEBHOOK_TOKEN", "", true)

		if answer != "token" {
			t.Errorf("Ask() = %q, want %q", answer, "token")
		}
	})

	t.Run("Should write a newline after the answer of a secret when the echo is off", func(t *testing.T) {
		output := &bytes.Buffer{}
		prompt := NewPromptService(strings.NewReader("token\n"), output, &fakeEcho{hidden: true})

		_, _ = prompt.Ask("WEBHOOK_TOKEN", "", true)

		if output.String() != "WEBHOOK_TOKEN: \n" {
			t.Errorf("Ask() wrote %q, want %q", output.String(), "WEBHOOK_TOKEN: \n")
		}
	})

	t.Run("Should write no newline after the answer of a secret when the echo was not turned off", func(t *testing.T) {
		output := &bytes.Buffer{}
		prompt := NewPromptService(strings.NewReader("token\n"), output, &fakeEcho{})

		_, _ = prompt.Ask("WEBHOOK_TOKEN", "", true)

		if output.String() != "WEBHOOK_TOKEN: " {
			t.Errorf("Ask() wrote %q, want %q", output.String(), "WEBHOOK_TOKEN: ")
		}
	})

	t.Run("Should turn the echo back on after the answer of a secret", func(t *testing.T) {
		echo := &fakeEcho{hidden: true}
		prompt := NewPromptService(strings.NewReader("token\n"), io.Discard, echo)

		_, _ = prompt.Ask("WEBHOOK_TOKEN", "", true)

		if !echo.restored {
			t.Errorf("Ask() left the echo off, want it back on")
		}
	})

	t.Run("Should turn the echo back on and fail when the input of a secret cannot be read", func(t *testing.T) {
		errRead := errors.New("read failed")
		echo := &fakeEcho{hidden: true}
		prompt := NewPromptService(iotest.ErrReader(errRead), io.Discard, echo)

		_, err := prompt.Ask("WEBHOOK_TOKEN", "", true)

		if !errors.Is(err, errRead) || !echo.restored {
			t.Errorf("Ask() = %v, echo back on %t; want %v, true", err, echo.restored, errRead)
		}
	})

	t.Run("Should fail when the echo cannot be turned off", func(t *testing.T) {
		errOff := errors.New("off failed")
		prompt := NewPromptService(strings.NewReader("token\n"), io.Discard, &fakeEcho{err: errOff})

		_, err := prompt.Ask("WEBHOOK_TOKEN", "", true)

		if !errors.Is(err, errOff) {
			t.Errorf("Ask() = %v, want %v", err, errOff)
		}
	})

	t.Run("Should fail when the echo cannot be turned back on", func(t *testing.T) {
		errRestore := errors.New("restore failed")
		prompt := NewPromptService(strings.NewReader("token\n"), io.Discard, &fakeEcho{hidden: true, restoreErr: errRestore})

		_, err := prompt.Ask("WEBHOOK_TOKEN", "", true)

		if !errors.Is(err, errRestore) {
			t.Errorf("Ask() = %v, want %v", err, errRestore)
		}
	})
}
