// Package config reads /etc/vps-agent/config.env.
package config

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"time"
)

// ErrInvalid marks a configuration file the agent refuses.
var ErrInvalid = errors.New("invalid config")

// ErrMissingToken marks a configuration without GITHUB_TOKEN, which update needs.
var ErrMissingToken = errors.New("GITHUB_TOKEN is missing from the config")

// Keys lists every key the configuration file accepts, in the order install.sh documents them.
func Keys() []string {
	return []string{
		"CMS_URL", "AGENT_TOKEN", "AGENT_HOST_NAME", "EASYPANEL_URL", "EASYPANEL_TOKEN",
		"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_REGION", "RECONCILE_INTERVAL", "REPORT_INTERVAL",
		"GITHUB_TOKEN",
	}
}

// Config is the agent configuration. Each subcommand checks only the fields it uses.
type Config struct {
	CMSURL             string
	AgentToken         string
	HostName           string
	EasypanelURL       string
	EasypanelToken     string
	AWSAccessKeyID     string
	AWSSecretAccessKey string
	AWSRegion          string
	ReconcileInterval  time.Duration
	ReportInterval     time.Duration
	GitHubToken        string
}

func defaults() Config {
	return Config{
		EasypanelURL:      "http://127.0.0.1:3000",
		AWSRegion:         "us-east-1",
		ReconcileInterval: 5 * time.Minute,
		ReportInterval:    10 * time.Minute,
	}
}

// Load reads name from fsys (os.DirFS("/etc/vps-agent") in production); the file must be mode 0600.
func Load(fsys fs.FS, name string) (Config, error) {
	info, err := fs.Stat(fsys, name)
	if err != nil {
		return Config{}, fmt.Errorf("stat config: %w", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		return Config{}, fmt.Errorf("%w: %s has mode %o, want 600", ErrInvalid, name, perm)
	}
	raw, err := fs.ReadFile(fsys, name)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	return Parse(bytes.NewReader(raw))
}

// Parse reads KEY=VALUE lines; blank lines and lines starting with # are skipped. Spaces around
// the key and the value are dropped, and so is one pair of double quotes around the value.
func Parse(r io.Reader) (Config, error) {
	cfg := defaults()
	seen := map[string]bool{}
	sc := bufio.NewScanner(r)
	for line := 1; sc.Scan(); line++ {
		text := strings.TrimSpace(sc.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		key, value, ok := strings.Cut(text, "=")
		if !ok {
			return Config{}, fmt.Errorf("%w: line %d has no '='", ErrInvalid, line)
		}
		key, value = strings.TrimSpace(key), unquote(strings.TrimSpace(value))
		if seen[key] {
			return Config{}, fmt.Errorf("%w: %s set twice", ErrInvalid, key)
		}
		seen[key] = true
		if err := cfg.set(key, value); err != nil {
			return Config{}, err
		}
	}
	if err := sc.Err(); err != nil {
		return Config{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	return cfg, nil
}

func unquote(v string) string {
	if len(v) >= 2 && strings.HasPrefix(v, `"`) && strings.HasSuffix(v, `"`) {
		return v[1 : len(v)-1]
	}
	return v
}

func (c *Config) set(key, value string) error {
	texts := map[string]*string{
		"CMS_URL":               &c.CMSURL,
		"AGENT_TOKEN":           &c.AgentToken,
		"AGENT_HOST_NAME":       &c.HostName,
		"EASYPANEL_URL":         &c.EasypanelURL,
		"EASYPANEL_TOKEN":       &c.EasypanelToken,
		"AWS_ACCESS_KEY_ID":     &c.AWSAccessKeyID,
		"AWS_SECRET_ACCESS_KEY": &c.AWSSecretAccessKey,
		"AWS_REGION":            &c.AWSRegion,
		"GITHUB_TOKEN":          &c.GitHubToken,
	}
	durations := map[string]*time.Duration{
		"RECONCILE_INTERVAL": &c.ReconcileInterval,
		"REPORT_INTERVAL":    &c.ReportInterval,
	}
	if target, ok := texts[key]; ok {
		*target = value
		return nil
	}
	target, ok := durations[key]
	if !ok {
		return fmt.Errorf("%w: unknown key %s", ErrInvalid, key)
	}
	d, err := time.ParseDuration(value)
	if err != nil || d <= 0 {
		return fmt.Errorf("%w: %s must be a positive duration, got %q", ErrInvalid, key, value)
	}
	*target = d
	return nil
}

// File is the configuration file Name inside Dir.
type File struct {
	Dir  fs.FS
	Name string
}

// Validate implements port.ConfigValidator.
func (f File) Validate() error {
	_, err := Load(f.Dir, f.Name)
	return err
}

// HostNameOr returns AGENT_HOST_NAME, and fallback() when the file does not exist or leaves the
// name unset. Any other problem with the file is an error, so a broken config never reports
// under the machine's own name.
func (f File) HostNameOr(fallback func() (string, error)) (string, error) {
	cfg, err := Load(f.Dir, f.Name)
	if errors.Is(err, fs.ErrNotExist) {
		return fallback()
	}
	if err != nil {
		return "", err
	}
	if cfg.HostName == "" {
		return fallback()
	}
	return cfg.HostName, nil
}

// GitHubToken returns GITHUB_TOKEN, the token update reads releases with.
func (f File) GitHubToken() (string, error) {
	cfg, err := Load(f.Dir, f.Name)
	if err != nil {
		return "", err
	}
	if cfg.GitHubToken == "" {
		return "", fmt.Errorf("%w: run install.sh again with GITHUB_TOKEN", ErrMissingToken)
	}
	return cfg.GitHubToken, nil
}

// Syntax checks configuration contents without reading a file.
type Syntax struct{}

// CheckSyntax implements port.ConfigSyntax.
func (Syntax) CheckSyntax(data []byte) error {
	_, err := Parse(bytes.NewReader(data))
	return err
}
