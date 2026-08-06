// Package config loads and validates l402test's declarative endpoint files.
package config

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"

	"gopkg.in/yaml.v3"
)

const CurrentVersion = 1

// Config is the versioned, file-backed configuration for a playground.
type Config struct {
	Version   int        `yaml:"version"`
	Endpoints []Endpoint `yaml:"endpoints"`
}

// Endpoint describes one L402-protected HTTP resource.
type Endpoint struct {
	Name      string   `yaml:"name"`
	Method    string   `yaml:"method"`
	Path      string   `yaml:"path"`
	PriceSats int64    `yaml:"price_sats"`
	Response  Response `yaml:"response"`
}

// Response is returned after a request presents a valid L402 credential.
type Response struct {
	Status      int               `yaml:"status"`
	ContentType string            `yaml:"content_type"`
	Headers     map[string]string `yaml:"headers"`
	Body        string            `yaml:"body"`
}

// Load reads a YAML configuration file and validates it before returning it.
func Load(filename string) (Config, error) {
	contents, err := os.ReadFile(filename)
	if err != nil {
		return Config{}, fmt.Errorf("read configuration: %w", err)
	}
	var cfg Config
	decoder := yaml.NewDecoder(strings.NewReader(string(contents)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode configuration: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return Config{}, errors.New("configuration must contain exactly one YAML document")
	}
	if err := Validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate rejects ambiguous, unsafe, or incomplete endpoint definitions.
func Validate(cfg Config) error {
	if cfg.Version != CurrentVersion {
		return fmt.Errorf("configuration version must be %d", CurrentVersion)
	}
	if len(cfg.Endpoints) == 0 {
		return errors.New("configuration must define at least one endpoint")
	}

	seen := make(map[string]struct{}, len(cfg.Endpoints))
	names := make(map[string]struct{}, len(cfg.Endpoints))
	for index, endpoint := range cfg.Endpoints {
		prefix := fmt.Sprintf("endpoints[%d]", index)
		if strings.TrimSpace(endpoint.Name) == "" {
			return fmt.Errorf("%s.name is required", prefix)
		}
		if _, exists := names[endpoint.Name]; exists {
			return fmt.Errorf("%s.name duplicates %q", prefix, endpoint.Name)
		}
		names[endpoint.Name] = struct{}{}
		if !validMethod(endpoint.Method) {
			return fmt.Errorf("%s.method must be an HTTP method", prefix)
		}
		if !validPath(endpoint.Path) {
			return fmt.Errorf("%s.path must be an absolute clean URL path", prefix)
		}
		if endpoint.PriceSats <= 0 {
			return fmt.Errorf("%s.price_sats must be greater than zero", prefix)
		}
		if endpoint.Response.Status < http.StatusOK || endpoint.Response.Status > 599 {
			return fmt.Errorf("%s.response.status must be between 200 and 599", prefix)
		}
		if strings.TrimSpace(endpoint.Response.ContentType) == "" {
			return fmt.Errorf("%s.response.content_type is required", prefix)
		}
		for header, value := range endpoint.Response.Headers {
			if !validHeader(header, value) {
				return fmt.Errorf("%s.response.headers contains an invalid header", prefix)
			}
		}

		key := strings.ToUpper(endpoint.Method) + " " + endpoint.Path
		if _, exists := seen[key]; exists {
			return fmt.Errorf("%s duplicates %s", prefix, key)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func validMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead:
		return true
	default:
		return false
	}
}

func validPath(value string) bool {
	return strings.HasPrefix(value, "/") && path.Clean(value) == value && !strings.Contains(value, "//")
}

func validHeader(name, value string) bool {
	if strings.TrimSpace(name) == "" || strings.ContainsAny(name, "\r\n") || strings.ContainsAny(value, "\r\n") {
		return false
	}
	return http.CanonicalHeaderKey(name) != ""
}
