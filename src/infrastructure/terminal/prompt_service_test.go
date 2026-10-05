package terminal

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

func TestPromptService(t *testing.T) {
	t.Run("Should return the typed answer", func(t *testing.T) {
		prompt := NewPromptService(strings.NewReader(" sa-east-1 \n"), io.Discard)

		answer, _ := prompt.Ask("AWS_REGION", "us-east-1")

		if answer != "sa-east-1" {
			t.Errorf("Ask() = %q, want %q", answer, "sa-east-1")
		}
	})

	t.Run("Should return the fallback when the answer is empty", func(t *testing.T) {
		prompt := NewPromptService(strings.NewReader("\n"), io.Discard)

		answer, _ := prompt.Ask("AWS_REGION", "us-east-1")

		if answer != "us-east-1" {
			t.Errorf("Ask() = %q, want %q", answer, "us-east-1")
		}
	})

	t.Run("Should return the fallback when the input has ended", func(t *testing.T) {
		prompt := NewPromptService(strings.NewReader(""), io.Discard)

		answer, err := prompt.Ask("AWS_REGION", "us-east-1")

		if err != nil || answer != "us-east-1" {
			t.Errorf("Ask() = %q, %v; want %q, nil", answer, err, "us-east-1")
		}
	})

	t.Run("Should answer one question per line", func(t *testing.T) {
		prompt := NewPromptService(strings.NewReader("first\nsecond\n"), io.Discard)
		_, _ = prompt.Ask("CMS_URL", "")

		answer, _ := prompt.Ask("CMS_AGENT_TOKEN", "")

		if answer != "second" {
			t.Errorf("Ask() = %q, want %q", answer, "second")
		}
	})

	t.Run("Should show the name and the fallback", func(t *testing.T) {
		output := &bytes.Buffer{}
		prompt := NewPromptService(strings.NewReader("\n"), output)

		_, _ = prompt.Ask("AWS_REGION", "us-east-1")

		if output.String() != "AWS_REGION [us-east-1]: " {
			t.Errorf("Ask() wrote %q, want %q", output.String(), "AWS_REGION [us-east-1]: ")
		}
	})

	t.Run("Should show only the name when there is no fallback", func(t *testing.T) {
		output := &bytes.Buffer{}
		prompt := NewPromptService(strings.NewReader("\n"), output)

		_, _ = prompt.Ask("CMS_URL", "")

		if output.String() != "CMS_URL: " {
			t.Errorf("Ask() wrote %q, want %q", output.String(), "CMS_URL: ")
		}
	})

	t.Run("Should fail when the input cannot be read", func(t *testing.T) {
		errRead := errors.New("read failed")
		prompt := NewPromptService(iotest.ErrReader(errRead), io.Discard)

		_, err := prompt.Ask("CMS_URL", "")

		if !errors.Is(err, errRead) {
			t.Errorf("Ask() = %v, want %v", err, errRead)
		}
	})
}
