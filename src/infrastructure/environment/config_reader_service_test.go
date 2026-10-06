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
	keys := []Key{{Name: "CMS_URL"}, {Name: "AWS_REGION", Default: "us-east-1"}}

	t.Run("Should list every key in the order setup asks for them", func(t *testing.T) {
		want := []Key{
			{Name: "CMS_URL"},
			{Name: "CMS_AGENT_TOKEN", Secret: true},
			{Name: "CMS_AGENT_HOST_NAME"},
			{Name: "EASYPANEL_URL", Default: "http://127.0.0.1:3000"},
			{Name: "EASYPANEL_TOKEN", Secret: true},
			{Name: "AWS_ACCESS_KEY_ID"},
			{Name: "AWS_SECRET_ACCESS_KEY", Secret: true},
			{Name: "AWS_REGION", Default: "us-east-1"},
			{Name: "INTERVAL_RECONCILE", Default: "5m"},
			{Name: "INTERVAL_REPORT", Default: "10m"},
			{Name: "DEPLOY_SSH_PUBLIC_KEY", Optional: true},
		}

		keys := Keys()

		if !slices.Equal(keys, want) {
			t.Errorf("Keys() = %v, want %v", keys, want)
		}
	})

	t.Run("Should require a key without default", func(t *testing.T) {
		key := Key{Name: "CMS_URL"}

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

	t.Run("Should not require an optional key without default", func(t *testing.T) {
		key := Key{Name: "DEPLOY_SSH_PUBLIC_KEY", Optional: true}

		required := key.Required()

		if required {
			t.Errorf("Required() = %v, want %v", required, false)
		}
	})

	t.Run("Should read the environment variable without asking", func(t *testing.T) {
		env := fakeEnv{"CMS_URL": "https://cms.example.com", "AWS_REGION": "sa-east-1"}
		reader := NewConfigReaderService(fstest.MapFS{}, keys, env.Getenv, fakePrompt{err: errors.New("asked")})
		want := map[string]string{"CMS_URL": "https://cms.example.com", "AWS_REGION": "sa-east-1"}

		values, _ := reader.Read()

		if !maps.Equal(values, want) {
			t.Errorf("Read() = %v, want %v", values, want)
		}
	})

	t.Run("Should read the value of .env when the answer is empty", func(t *testing.T) {
		fsys := fstest.MapFS{".env": {Data: []byte("CMS_URL=https://cms.example.com\nAWS_REGION = sa-east-1\n")}}
		reader := NewConfigReaderService(fsys, keys, fakeEnv{}.Getenv, fakePrompt{})
		want := map[string]string{"CMS_URL": "https://cms.example.com", "AWS_REGION": "sa-east-1"}

		values, _ := reader.Read()

		if !maps.Equal(values, want) {
			t.Errorf("Read() = %v, want %v", values, want)
		}
	})

	t.Run("Should read the default when .env lacks the key", func(t *testing.T) {
		fsys := fstest.MapFS{".env": {Data: []byte("CMS_URL=https://cms.example.com\n")}}
		reader := NewConfigReaderService(fsys, keys, fakeEnv{}.Getenv, fakePrompt{})
		want := map[string]string{"CMS_URL": "https://cms.example.com", "AWS_REGION": "us-east-1"}

		values, _ := reader.Read()

		if !maps.Equal(values, want) {
			t.Errorf("Read() = %v, want %v", values, want)
		}
	})

	t.Run("Should read the typed answer over the fallback", func(t *testing.T) {
		fsys := fstest.MapFS{".env": {Data: []byte("CMS_URL=https://cms.example.com\n")}}
		prompt := fakePrompt{answers: map[string]string{"CMS_URL": "https://other.example.com", "AWS_REGION": "sa-east-1"}}
		reader := NewConfigReaderService(fsys, keys, fakeEnv{}.Getenv, prompt)
		want := map[string]string{"CMS_URL": "https://other.example.com", "AWS_REGION": "sa-east-1"}

		values, _ := reader.Read()

		if !maps.Equal(values, want) {
			t.Errorf("Read() = %v, want %v", values, want)
		}
	})

	t.Run("Should read the answers when .env does not exist", func(t *testing.T) {
		prompt := fakePrompt{answers: map[string]string{"CMS_URL": "https://cms.example.com"}}
		reader := NewConfigReaderService(fstest.MapFS{}, keys, fakeEnv{}.Getenv, prompt)
		want := map[string]string{"CMS_URL": "https://cms.example.com", "AWS_REGION": "us-east-1"}

		values, _ := reader.Read()

		if !maps.Equal(values, want) {
			t.Errorf("Read() = %v, want %v", values, want)
		}
	})

	t.Run("Should tell the prompt which keys are secret", func(t *testing.T) {
		secretKeys := []Key{{Name: "CMS_AGENT_TOKEN", Secret: true}, {Name: "AWS_REGION", Default: "us-east-1"}}
		prompt := fakePrompt{answers: map[string]string{"CMS_AGENT_TOKEN": "token"}, secrets: map[string]bool{}}
		reader := NewConfigReaderService(fstest.MapFS{}, secretKeys, fakeEnv{}.Getenv, prompt)
		want := map[string]bool{"CMS_AGENT_TOKEN": true, "AWS_REGION": false}

		_, _ = reader.Read()

		if !maps.Equal(prompt.secrets, want) {
			t.Errorf("Read() asked with secret %v, want %v", prompt.secrets, want)
		}
	})

	t.Run("Should skip blank lines and comments of .env", func(t *testing.T) {
		fsys := fstest.MapFS{".env": {Data: []byte("# Format: KEY=VALUE\n\nCMS_URL=https://cms.example.com\n")}}
		reader := NewConfigReaderService(fsys, keys, fakeEnv{}.Getenv, fakePrompt{})
		want := map[string]string{"CMS_URL": "https://cms.example.com", "AWS_REGION": "us-east-1"}

		values, _ := reader.Read()

		if !maps.Equal(values, want) {
			t.Errorf("Read() = %v, want %v", values, want)
		}
	})

	t.Run("Should fail on a line of .env without KEY=VALUE", func(t *testing.T) {
		fsys := fstest.MapFS{".env": {Data: []byte("CMS_URL=https://cms.example.com\nAWS_REGION\n")}}
		reader := NewConfigReaderService(fsys, keys, fakeEnv{}.Getenv, fakePrompt{})

		_, err := reader.Read()

		if err == nil || err.Error() != ".env line 2: want KEY=VALUE" {
			t.Errorf("Read() = %v, want %q", err, ".env line 2: want KEY=VALUE")
		}
	})

	t.Run("Should fail when .env cannot be read", func(t *testing.T) {
		fsys := fstest.MapFS{".env": {Mode: fs.ModeDir}}
		reader := NewConfigReaderService(fsys, keys, fakeEnv{}.Getenv, fakePrompt{})

		_, err := reader.Read()

		if !errors.Is(err, fs.ErrInvalid) {
			t.Errorf("Read() = %v, want %v", err, fs.ErrInvalid)
		}
	})

	t.Run("Should fail listing the required keys without a value", func(t *testing.T) {
		required := []Key{{Name: "CMS_URL"}, {Name: "CMS_AGENT_TOKEN"}, {Name: "AWS_REGION", Default: "us-east-1"}}
		reader := NewConfigReaderService(fstest.MapFS{}, required, fakeEnv{}.Getenv, fakePrompt{})

		_, err := reader.Read()

		if err == nil || err.Error() != "missing CMS_URL, CMS_AGENT_TOKEN" {
			t.Errorf("Read() = %v, want %q", err, "missing CMS_URL, CMS_AGENT_TOKEN")
		}
	})

	t.Run("Should fail when the prompt fails", func(t *testing.T) {
		errAsk := errors.New("ask failed")
		reader := NewConfigReaderService(fstest.MapFS{}, keys, fakeEnv{}.Getenv, fakePrompt{err: errAsk})

		_, err := reader.Read()

		if !errors.Is(err, errAsk) {
			t.Errorf("Read() = %v, want %v", err, errAsk)
		}
	})
}
