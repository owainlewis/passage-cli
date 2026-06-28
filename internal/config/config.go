package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	EnvAPIURL = "PASSAGE_API_URL"
	EnvToken  = "PASSAGE_TOKEN"

	DefaultAPIURL = "https://passage.md"
)

type Config struct {
	APIURL string `json:"api_url"`
	Token  string `json:"token"`
}

type Source struct {
	APIURL string
	Token  string
}

type LoadResult struct {
	Config Config
	Source Source
	Path   string
}

func DefaultDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "passage"), nil
}

func Path(dir string) string {
	return filepath.Join(dir, "config.json")
}

func Load(dir string, env map[string]string) (LoadResult, error) {
	path := Path(dir)
	cfg := Config{APIURL: DefaultAPIURL}
	source := Source{APIURL: "default"}

	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return LoadResult{}, fmt.Errorf("read config: %w", err)
		}
		if cfg.APIURL == "" {
			cfg.APIURL = DefaultAPIURL
			source.APIURL = "default"
		} else {
			source.APIURL = "config"
		}
		if cfg.Token != "" {
			source.Token = "config"
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return LoadResult{}, fmt.Errorf("read config: %w", err)
	}

	if value := strings.TrimSpace(env[EnvAPIURL]); value != "" {
		cfg.APIURL = value
		source.APIURL = "env"
	}
	if value := strings.TrimSpace(env[EnvToken]); value != "" {
		cfg.Token = value
		source.Token = "env"
	}

	cfg.APIURL = normalizeAPIURL(cfg.APIURL)
	return LoadResult{Config: cfg, Source: source, Path: path}, nil
}

func Save(dir string, cfg Config) error {
	cfg.APIURL = normalizeAPIURL(cfg.APIURL)
	if err := validateAPIURL(cfg.APIURL); err != nil {
		return err
	}
	if strings.TrimSpace(cfg.Token) == "" {
		return errors.New("token is required")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	path := Path(dir)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

func validateAPIURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("invalid API URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("API URL must start with http:// or https://")
	}
	if parsed.Host == "" {
		return errors.New("API URL must include a host")
	}
	return nil
}

func normalizeAPIURL(value string) string {
	value = strings.TrimSpace(value)
	return strings.TrimRight(value, "/")
}

func RedactToken(token string) string {
	if token == "" {
		return ""
	}
	runes := []rune(token)
	if len(runes) <= 8 {
		return "****"
	}
	return string(runes[:4]) + "..." + string(runes[len(runes)-4:])
}

func EnvMap() map[string]string {
	return map[string]string{
		EnvAPIURL: os.Getenv(EnvAPIURL),
		EnvToken:  os.Getenv(EnvToken),
	}
}
