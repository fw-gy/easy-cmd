package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"easy-cmd/internal/i18n"
)

const DefaultModel = "glm-4.5-air"

type Config struct {
	BaseURL  string `json:"base_url"`
	APIKey   string `json:"api_key"`
	Model    string `json:"model"`
	Language string `json:"language"`
}

func Load(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %q: %w", path, err)
	}

	if cfg.BaseURL == "" {
		return Config{}, errors.New("config base_url is required")
	}
	if cfg.APIKey == "" {
		return Config{}, errors.New("config api_key is required")
	}
	if cfg.Model == "" {
		cfg.Model = DefaultModel
	}
	if cfg.Language == "" {
		cfg.Language = string(i18n.DefaultLanguage)
	}
	if !i18n.IsSupported(cfg.Language) {
		return Config{}, fmt.Errorf("config language %q is not supported", cfg.Language)
	}

	return cfg, nil
}
