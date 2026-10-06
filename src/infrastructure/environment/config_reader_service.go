package environment

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/heronlabs/talon/src/infrastructure/terminal"
)

type Key struct {
	Name     string
	Default  string
	Secret   bool
	Optional bool
}

func (k Key) Required() bool {
	return k.Default == "" && !k.Optional
}

func Keys() []Key {
	return []Key{
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
}

type ConfigReader interface {
	Read() (map[string]string, error)
}

type ConfigReaderService struct {
	fsys   fs.FS
	keys   []Key
	getenv func(string) string
	prompt terminal.Prompt
}

func NewConfigReaderService(fsys fs.FS, keys []Key, getenv func(string) string, prompt terminal.Prompt) *ConfigReaderService {
	return &ConfigReaderService{fsys: fsys, keys: keys, getenv: getenv, prompt: prompt}
}

func (s *ConfigReaderService) Read() (map[string]string, error) {
	current, err := s.parse()
	if err != nil {
		return nil, err
	}

	values := map[string]string{}
	missing := []string{}

	for _, key := range s.keys {
		value := s.getenv(key.Name)
		if value == "" {
			fallback := current[key.Name]
			if fallback == "" {
				fallback = key.Default
			}

			value, err = s.prompt.Ask(key.Name, fallback, key.Secret)
			if err != nil {
				return nil, fmt.Errorf("prompt: %w", err)
			}
		}

		if key.Required() && value == "" {
			missing = append(missing, key.Name)
		}

		values[key.Name] = value
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing %s", strings.Join(missing, ", "))
	}

	return values, nil
}

func (s *ConfigReaderService) parse() (map[string]string, error) {
	values := map[string]string{}

	data, err := fs.ReadFile(s.fsys, ".env")
	if errors.Is(err, fs.ErrNotExist) {
		return values, nil
	}
	if err != nil {
		return nil, err
	}

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
