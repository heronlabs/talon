package environment

import (
	"errors"
	"io/fs"
	"maps"
	"slices"
	"testing"
	"testing/fstest"
)

type fakeEnv map[string]string

func (f fakeEnv) Getenv(name string) string {
	return f[name]
}

type fakePrompt struct {
	answers map[string]string
	secrets map[string]bool
	err     error
}

func (f fakePrompt) Ask(name, fallback string, secret bool) (string, error) {
	if f.err != nil {
		return "", f.err
	}

	if f.secrets != nil {
		f.secrets[name] = secret
	}

	if answer, ok := f.answers[name]; ok {
		return answer, nil
	}

	return fallback, nil
}

func TestConfigReaderService(t *testing.T) {
	t.Run("Should list every key in the order setup asks for them", func(t *testing.T) {
		want := []Key{
			{Name: "WEBHOOK_URL", Optional: true},
			{Name: "WEBHOOK_TOKEN", Secret: true, Optional: true, DependsOn: "WEBHOOK_URL"},
			{Name: "AWS_ACCESS_KEY_ID"},
			{Name: "AWS_SECRET_ACCESS_KEY", Secret: true},
			{Name: "AWS_REGION", Default: "us-east-1"},
			{Name: "INTERVAL_REPORT", Default: "10m"},
		}

		keys := Keys()

		if !slices.Equal(keys, want) {
			t.Errorf("Keys() = %v, want %v", keys, want)
		}
	})

	t.Run("Should require a key without default", func(t *testing.T) {
		key := Key{Name: "WEBHOOK_URL"}

		required := key.Required()

		if !required {
			t.Errorf("Required() = %v, want %v", required, true)
		}
	})

	t.Run("Should not require a key with default", func(t *testing.T) {
		key := Key{Name: "AWS_REGION", Default: "us-east-1"}

		required := key.Required()

		if required {
			t.Errorf("Required() = %v, want %v", required, false)
		}
	})

	t.Run("Should read the environment variable without asking", func(t *testing.T) {
		keys := []Key{{Name: "WEBHOOK_URL"}, {Name: "AWS_REGION", Default: "us-east-1"}}
		env := fakeEnv{"WEBHOOK_URL": "https://webhook.example.com", "AWS_REGION": "sa-east-1"}
		reader := NewConfigReaderService(fstest.MapFS{}, keys, env.Getenv, fakePrompt{err: errors.New("asked")})
		want := map[string]string{"WEBHOOK_URL": "https://webhook.example.com", "AWS_REGION": "sa-east-1"}

		values, _ := reader.Read()

		if !maps.Equal(values, want) {
			t.Errorf("Read() = %v, want %v", values, want)
		}
	})

	t.Run("Should read the value of .env when the answer is empty", func(t *testing.T) {
		keys := []Key{{Name: "WEBHOOK_URL"}, {Name: "AWS_REGION", Default: "us-east-1"}}
		fsys := fstest.MapFS{".env": {Data: []byte("WEBHOOK_URL=https://webhook.example.com\nAWS_REGION = sa-east-1\n")}}
		reader := NewConfigReaderService(fsys, keys, fakeEnv{}.Getenv, fakePrompt{})
		want := map[string]string{"WEBHOOK_URL": "https://webhook.example.com", "AWS_REGION": "sa-east-1"}

		values, _ := reader.Read()

		if !maps.Equal(values, want) {
			t.Errorf("Read() = %v, want %v", values, want)
		}
	})

	t.Run("Should read the default when .env lacks the key", func(t *testing.T) {
		keys := []Key{{Name: "WEBHOOK_URL"}, {Name: "AWS_REGION", Default: "us-east-1"}}
		fsys := fstest.MapFS{".env": {Data: []byte("WEBHOOK_URL=https://webhook.example.com\n")}}
		reader := NewConfigReaderService(fsys, keys, fakeEnv{}.Getenv, fakePrompt{})
		want := map[string]string{"WEBHOOK_URL": "https://webhook.example.com", "AWS_REGION": "us-east-1"}

		values, _ := reader.Read()

		if !maps.Equal(values, want) {
			t.Errorf("Read() = %v, want %v", values, want)
		}
	})

	t.Run("Should read the typed answer over the fallback", func(t *testing.T) {
		keys := []Key{{Name: "WEBHOOK_URL"}, {Name: "AWS_REGION", Default: "us-east-1"}}
		fsys := fstest.MapFS{".env": {Data: []byte("WEBHOOK_URL=https://webhook.example.com\n")}}
		prompt := fakePrompt{answers: map[string]string{"WEBHOOK_URL": "https://other.example.com", "AWS_REGION": "sa-east-1"}}
		reader := NewConfigReaderService(fsys, keys, fakeEnv{}.Getenv, prompt)
		want := map[string]string{"WEBHOOK_URL": "https://other.example.com", "AWS_REGION": "sa-east-1"}

		values, _ := reader.Read()

		if !maps.Equal(values, want) {
			t.Errorf("Read() = %v, want %v", values, want)
		}
	})

	t.Run("Should read the answers when .env does not exist", func(t *testing.T) {
		keys := []Key{{Name: "WEBHOOK_URL"}, {Name: "AWS_REGION", Default: "us-east-1"}}
		prompt := fakePrompt{answers: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		reader := NewConfigReaderService(fstest.MapFS{}, keys, fakeEnv{}.Getenv, prompt)
		want := map[string]string{"WEBHOOK_URL": "https://webhook.example.com", "AWS_REGION": "us-east-1"}

		values, _ := reader.Read()

		if !maps.Equal(values, want) {
			t.Errorf("Read() = %v, want %v", values, want)
		}
	})

	t.Run("Should tell the prompt which keys are secret", func(t *testing.T) {
		secretKeys := []Key{{Name: "WEBHOOK_TOKEN", Secret: true}, {Name: "AWS_REGION", Default: "us-east-1"}}
		prompt := fakePrompt{answers: map[string]string{"WEBHOOK_TOKEN": "token"}, secrets: map[string]bool{}}
		reader := NewConfigReaderService(fstest.MapFS{}, secretKeys, fakeEnv{}.Getenv, prompt)
		want := map[string]bool{"WEBHOOK_TOKEN": true, "AWS_REGION": false}

		_, _ = reader.Read()

		if !maps.Equal(prompt.secrets, want) {
			t.Errorf("Read() asked with secret %v, want %v", prompt.secrets, want)
		}
	})

	t.Run("Should skip blank lines and comments of .env", func(t *testing.T) {
		keys := []Key{{Name: "WEBHOOK_URL"}, {Name: "AWS_REGION", Default: "us-east-1"}}
		fsys := fstest.MapFS{".env": {Data: []byte("# Format: KEY=VALUE\n\nWEBHOOK_URL=https://webhook.example.com\n")}}
		reader := NewConfigReaderService(fsys, keys, fakeEnv{}.Getenv, fakePrompt{})
		want := map[string]string{"WEBHOOK_URL": "https://webhook.example.com", "AWS_REGION": "us-east-1"}

		values, _ := reader.Read()

		if !maps.Equal(values, want) {
			t.Errorf("Read() = %v, want %v", values, want)
		}
	})

	t.Run("Should fail on a line of .env without KEY=VALUE", func(t *testing.T) {
		keys := []Key{{Name: "WEBHOOK_URL"}, {Name: "AWS_REGION", Default: "us-east-1"}}
		fsys := fstest.MapFS{".env": {Data: []byte("WEBHOOK_URL=https://webhook.example.com\nAWS_REGION\n")}}
		reader := NewConfigReaderService(fsys, keys, fakeEnv{}.Getenv, fakePrompt{})

		_, err := reader.Read()

		if err == nil || err.Error() != ".env line 2: want KEY=VALUE" {
			t.Errorf("Read() = %v, want %q", err, ".env line 2: want KEY=VALUE")
		}
	})

	t.Run("Should fail when .env cannot be read", func(t *testing.T) {
		keys := []Key{{Name: "WEBHOOK_URL"}, {Name: "AWS_REGION", Default: "us-east-1"}}
		fsys := fstest.MapFS{".env": {Mode: fs.ModeDir}}
		reader := NewConfigReaderService(fsys, keys, fakeEnv{}.Getenv, fakePrompt{})

		_, err := reader.Read()

		if !errors.Is(err, fs.ErrInvalid) {
			t.Errorf("Read() = %v, want %v", err, fs.ErrInvalid)
		}
	})

	t.Run("Should fail listing the required keys without a value", func(t *testing.T) {
		required := []Key{{Name: "WEBHOOK_URL"}, {Name: "WEBHOOK_TOKEN"}, {Name: "AWS_REGION", Default: "us-east-1"}}
		reader := NewConfigReaderService(fstest.MapFS{}, required, fakeEnv{}.Getenv, fakePrompt{})

		_, err := reader.Read()

		if err == nil || err.Error() != "missing WEBHOOK_URL, WEBHOOK_TOKEN" {
			t.Errorf("Read() = %v, want %q", err, "missing WEBHOOK_URL, WEBHOOK_TOKEN")
		}
	})

	t.Run("Should leave empty without asking a key whose dependency is empty", func(t *testing.T) {
		keys := []Key{{Name: "WEBHOOK_URL", Optional: true}, {Name: "WEBHOOK_TOKEN", Secret: true, Optional: true, DependsOn: "WEBHOOK_URL"}}
		fsys := fstest.MapFS{".env": {Data: []byte("WEBHOOK_TOKEN=token\n")}}
		env := fakeEnv{"WEBHOOK_TOKEN": "env-token"}
		prompt := fakePrompt{answers: map[string]string{"WEBHOOK_TOKEN": "typed-token"}}
		reader := NewConfigReaderService(fsys, keys, env.Getenv, prompt)
		want := map[string]string{"WEBHOOK_URL": "", "WEBHOOK_TOKEN": ""}

		values, _ := reader.Read()

		if !maps.Equal(values, want) {
			t.Errorf("Read() = %v, want %v", values, want)
		}
	})

	t.Run("Should ask for a key whose dependency has a value", func(t *testing.T) {
		keys := []Key{{Name: "WEBHOOK_URL", Optional: true}, {Name: "WEBHOOK_TOKEN", Secret: true, Optional: true, DependsOn: "WEBHOOK_URL"}}
		env := fakeEnv{"WEBHOOK_URL": "https://webhook.example.com"}
		prompt := fakePrompt{answers: map[string]string{"WEBHOOK_TOKEN": "token"}}
		reader := NewConfigReaderService(fstest.MapFS{}, keys, env.Getenv, prompt)
		want := map[string]string{"WEBHOOK_URL": "https://webhook.example.com", "WEBHOOK_TOKEN": "token"}

		values, _ := reader.Read()

		if !maps.Equal(values, want) {
			t.Errorf("Read() = %v, want %v", values, want)
		}
	})

	t.Run("Should get the value of the key in .env", func(t *testing.T) {
		keys := []Key{{Name: "WEBHOOK_URL"}, {Name: "AWS_REGION", Default: "us-east-1"}}
		fsys := fstest.MapFS{".env": {Data: []byte("WEBHOOK_URL=https://webhook.example.com\n")}}
		reader := NewConfigReaderService(fsys, keys, fakeEnv{}.Getenv, fakePrompt{})

		value, _ := reader.Get("WEBHOOK_URL")

		if value != "https://webhook.example.com" {
			t.Errorf("Get() = %q, want %q", value, "https://webhook.example.com")
		}
	})

	t.Run("Should get empty when .env lacks the key", func(t *testing.T) {
		keys := []Key{{Name: "WEBHOOK_URL"}, {Name: "AWS_REGION", Default: "us-east-1"}}
		fsys := fstest.MapFS{".env": {Data: []byte("AWS_REGION=sa-east-1\n")}}
		reader := NewConfigReaderService(fsys, keys, fakeEnv{}.Getenv, fakePrompt{})

		value, _ := reader.Get("WEBHOOK_URL")

		if value != "" {
			t.Errorf("Get() = %q, want %q", value, "")
		}
	})

	t.Run("Should get empty when .env does not exist", func(t *testing.T) {
		keys := []Key{{Name: "WEBHOOK_URL"}, {Name: "AWS_REGION", Default: "us-east-1"}}
		reader := NewConfigReaderService(fstest.MapFS{}, keys, fakeEnv{}.Getenv, fakePrompt{})

		value, _ := reader.Get("WEBHOOK_URL")

		if value != "" {
			t.Errorf("Get() = %q, want %q", value, "")
		}
	})

	t.Run("Should get empty for a key that is not in the keys", func(t *testing.T) {
		keys := []Key{{Name: "WEBHOOK_URL"}, {Name: "AWS_REGION", Default: "us-east-1"}}
		fsys := fstest.MapFS{".env": {Data: []byte("WEBHOOK_URL=https://webhook.example.com\n")}}
		reader := NewConfigReaderService(fsys, keys, fakeEnv{}.Getenv, fakePrompt{err: errors.New("asked")})

		value, err := reader.Get("AWS_ACCESS_KEY_ID")

		if value != "" || err != nil {
			t.Errorf("Get() = %q, %v; want %q, nil", value, err, "")
		}
	})

	t.Run("Should fail to get when .env cannot be parsed", func(t *testing.T) {
		keys := []Key{{Name: "WEBHOOK_URL"}, {Name: "AWS_REGION", Default: "us-east-1"}}
		fsys := fstest.MapFS{".env": {Data: []byte("WEBHOOK_URL=https://webhook.example.com\nAWS_REGION\n")}}
		reader := NewConfigReaderService(fsys, keys, fakeEnv{}.Getenv, fakePrompt{})

		_, err := reader.Get("WEBHOOK_URL")

		if err == nil || err.Error() != ".env line 2: want KEY=VALUE" {
			t.Errorf("Get() = %v, want %q", err, ".env line 2: want KEY=VALUE")
		}
	})

	t.Run("Should get the environment variable before .env", func(t *testing.T) {
		keys := []Key{{Name: "WEBHOOK_URL"}, {Name: "AWS_REGION", Default: "us-east-1"}}
		fsys := fstest.MapFS{".env": {Data: []byte("WEBHOOK_URL=https://webhook.example.com\n")}}
		env := fakeEnv{"WEBHOOK_URL": "https://env.example.com"}
		reader := NewConfigReaderService(fsys, keys, env.Getenv, fakePrompt{err: errors.New("asked")})

		value, _ := reader.Get("WEBHOOK_URL")

		if value != "https://env.example.com" {
			t.Errorf("Get() = %q, want %q", value, "https://env.example.com")
		}
	})

	t.Run("Should get the default when neither the environment nor .env has the key", func(t *testing.T) {
		keys := []Key{{Name: "WEBHOOK_URL"}, {Name: "AWS_REGION", Default: "us-east-1"}}
		fsys := fstest.MapFS{".env": {Data: []byte("WEBHOOK_URL=https://webhook.example.com\n")}}
		reader := NewConfigReaderService(fsys, keys, fakeEnv{}.Getenv, fakePrompt{err: errors.New("asked")})

		value, _ := reader.Get("AWS_REGION")

		if value != "us-east-1" {
			t.Errorf("Get() = %q, want %q", value, "us-east-1")
		}
	})

	t.Run("Should get the .env value before the default", func(t *testing.T) {
		keys := []Key{{Name: "WEBHOOK_URL"}, {Name: "AWS_REGION", Default: "us-east-1"}}
		fsys := fstest.MapFS{".env": {Data: []byte("AWS_REGION=sa-east-1\n")}}
		reader := NewConfigReaderService(fsys, keys, fakeEnv{}.Getenv, fakePrompt{err: errors.New("asked")})

		value, _ := reader.Get("AWS_REGION")

		if value != "sa-east-1" {
			t.Errorf("Get() = %q, want %q", value, "sa-east-1")
		}
	})

	t.Run("Should fail when the prompt fails", func(t *testing.T) {
		keys := []Key{{Name: "WEBHOOK_URL"}, {Name: "AWS_REGION", Default: "us-east-1"}}
		errAsk := errors.New("ask failed")
		reader := NewConfigReaderService(fstest.MapFS{}, keys, fakeEnv{}.Getenv, fakePrompt{err: errAsk})

		_, err := reader.Read()

		if !errors.Is(err, errAsk) {
			t.Errorf("Read() = %v, want %v", err, errAsk)
		}
	})
}
