package environment

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
)

// Key is a variable of the .env: its name, its default, whether it is a secret, whether it is optional and the key whose value it depends on.
type Key struct {
	Name      string
	Default   string
	Secret    bool
	Optional  bool
	DependsOn string
}

// Required reports whether the key needs a value: it has no default and is not optional.
func (k Key) Required() bool {
	return !k.Optional && k.Default == ""
}

// Keys returns every variable of the .env, in the order setup asks for them.
func Keys() []Key {
	return []Key{
		{Name: "WEBHOOK_URL", Optional: true},
		{Name: "WEBHOOK_TOKEN", Secret: true, Optional: true, DependsOn: "WEBHOOK_URL"},
		{Name: "AWS_ACCESS_KEY_ID"},
		{Name: "AWS_SECRET_ACCESS_KEY", Secret: true},
		{Name: "AWS_REGION", Default: "us-east-1"},
		{Name: "INTERVAL_REPORT", Default: "10m"},
	}
}

// Prompt asks for the value of a key, showing its fallback, and returns the fallback on an empty answer.
type Prompt interface {
	Ask(name, fallback string, secret bool) (string, error)
}

// ConfigReaderService reads the configuration from the environment variables, .env, the defaults of the keys and, in Read, the prompt.
type ConfigReaderService struct {
	fsys     fs.FS
	keys     []Key
	defaults map[string]string
	getenv   func(string) string
	prompt   Prompt
}

// NewConfigReaderService returns a ConfigReaderService that reads .env from fsys, the keys, the environment variables through getenv and the answers through prompt.
func NewConfigReaderService(fsys fs.FS, keys []Key, getenv func(string) string, prompt Prompt) *ConfigReaderService {
	return &ConfigReaderService{fsys: fsys, keys: keys, defaults: defaults(keys), getenv: getenv, prompt: prompt}
}

// defaults returns the default of each of keys, by its name.
func defaults(keys []Key) map[string]string {
	values := map[string]string{}
	for _, key := range keys {
		values[key.Name] = key.Default
	}

	return values
}

// Get returns the value of each of names, by its name, from the environment variable, then .env, then the default, parsing .env once and without asking; it fails only when .env cannot be parsed.
func (s *ConfigReaderService) Get(names ...string) (map[string]string, error) {
	current, err := s.parse()
	if err != nil {
		return nil, err
	}

	values := map[string]string{}
	for _, name := range names {
		values[name] = s.get(name, current)
	}

	return values, nil
}

// get returns the environment variable name or, when that is empty, the value of the key in current or its default.
func (s *ConfigReaderService) get(name string, current map[string]string) string {
	if value := s.getenv(name); value != "" {
		return value
	}

	return s.fallback(name, current)
}

// Read returns the value of every key, from the environment variable or, when that is empty, from the prompt, with the value in .env or the default as fallback, leaving empty without asking a key whose dependency is empty, and fails listing the required keys left empty.
func (s *ConfigReaderService) Read() (map[string]string, error) {
	current, err := s.parse()
	if err != nil {
		return nil, err
	}

	values := map[string]string{}
	for _, key := range s.keys {
		value, err := s.value(key, current, values)
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

// value returns empty when the dependency of the key is empty in values; otherwise the environment variable of the key or, when that is empty, the answer of the prompt, with the value in current or the default as fallback.
func (s *ConfigReaderService) value(key Key, current, values map[string]string) (string, error) {
	if key.DependsOn != "" && values[key.DependsOn] == "" {
		return "", nil
	}

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

	return s.defaults[name]
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
