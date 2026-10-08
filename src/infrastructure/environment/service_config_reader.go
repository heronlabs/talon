package environment

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
)

// Key is a variable of the .env: its name, its default, whether it is a secret and whether it is optional.
type Key struct {
	Name     string
	Default  string
	Secret   bool
	Optional bool
}

// Required reports whether the key needs a value: it has no default and is not optional.
func (k Key) Required() bool {
	return !k.Optional && k.Default == ""
}

// Keys returns every variable of the .env, in the order setup asks for them.
func Keys() []Key {
	return []Key{
		{Name: "WEBHOOK_URL", Optional: true},
		{Name: "WEBHOOK_AGENT_TOKEN", Secret: true, Optional: true},
		{Name: "AWS_ACCESS_KEY_ID"},
		{Name: "AWS_SECRET_ACCESS_KEY", Secret: true},
		{Name: "AWS_REGION", Default: "us-east-1"},
		{Name: "INTERVAL_RECONCILE", Default: "5m"},
		{Name: "INTERVAL_REPORT", Default: "10m"},
	}
}

// Prompt asks for the value of a key, showing its fallback, and returns the fallback on an empty answer.
type Prompt interface {
	Ask(name, fallback string, secret bool) (string, error)
}

// ConfigReaderService reads the configuration from the environment variables, .env, the defaults of the keys and, in Read, the prompt.
type ConfigReaderService struct {
	fsys   fs.FS
	keys   []Key
	getenv func(string) string
	prompt Prompt
}

// NewConfigReaderService returns a ConfigReaderService that reads .env from fsys, the keys, the environment variables through getenv and the answers through prompt.
func NewConfigReaderService(fsys fs.FS, keys []Key, getenv func(string) string, prompt Prompt) *ConfigReaderService {
	return &ConfigReaderService{fsys: fsys, keys: keys, getenv: getenv, prompt: prompt}
}

// Get returns the value of one key from the environment variable, then .env, then the default, without asking; it fails only when .env cannot be parsed.
func (s *ConfigReaderService) Get(name string) (string, error) {
	current, err := s.parse()
	if err != nil {
		return "", err
	}

	if value := s.getenv(name); value != "" {
		return value, nil
	}

	return s.fallback(name, current), nil
}

// Read returns the value of every key, from the environment variable or, when that is empty, from the prompt, with the value in .env or the default as fallback, and fails listing the required keys left empty.
func (s *ConfigReaderService) Read() (map[string]string, error) {
	current, err := s.parse()
	if err != nil {
		return nil, err
	}

	values := map[string]string{}
	for _, key := range s.keys {
		value, err := s.value(key, current)
		if err != nil {
			return nil, err
		}

		values[key.Name] = value
	}

	if err := s.checkRequired(values); err != nil {
		return nil, err
	}

	return values, nil
}

// value returns the environment variable of the key or, when that is empty, the answer of the prompt, with the value in current or the default as fallback.
func (s *ConfigReaderService) value(key Key, current map[string]string) (string, error) {
	if value := s.getenv(key.Name); value != "" {
		return value, nil
	}

	value, err := s.prompt.Ask(key.Name, s.fallback(key.Name, current), key.Secret)
	if err != nil {
		return "", fmt.Errorf("prompt: %w", err)
	}

	return value, nil
}

// checkRequired fails listing the required keys whose value is empty.
func (s *ConfigReaderService) checkRequired(values map[string]string) error {
	missing := []string{}
	for _, key := range s.keys {
		if key.Required() && values[key.Name] == "" {
			missing = append(missing, key.Name)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing %s", strings.Join(missing, ", "))
	}

	return nil
}

// fallback returns the value of the key in current or, when that is empty, its default.
func (s *ConfigReaderService) fallback(name string, current map[string]string) string {
	if value := current[name]; value != "" {
		return value
	}

	for _, key := range s.keys {
		if key.Name == name {
			return key.Default
		}
	}

	return ""
}

// parse returns the values of .env, none when it does not exist.
func (s *ConfigReaderService) parse() (map[string]string, error) {
	data, err := fs.ReadFile(s.fsys, ".env")
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}

	return parseLines(data)
}

// parseLines returns the KEY=VALUE lines of data, trimmed, skipping blank lines and comments, and fails on a line without =.
func parseLines(data []byte) (map[string]string, error) {
	values := map[string]string{}
	for index, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		name, value, found := strings.Cut(line, "=")
		if !found {
			return nil, fmt.Errorf(".env line %d: want KEY=VALUE", index+1)
		}

		values[strings.TrimSpace(name)] = strings.TrimSpace(value)
	}

	return values, nil
}
