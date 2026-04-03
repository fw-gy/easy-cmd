package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"easy-cmd/internal/i18n"
)

type Config struct {
	BaseURL  string `json:"base_url"`
	APIKey   string `json:"api_key"`
	Model    string `json:"model"`
	Language string `json:"language"`
	Provider string `json:"provider"`
}

func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		return filepath.Join(home, ".easy-cmd", "config.json")
	}
	currentUser, err := user.Current()
	if err == nil && currentUser.HomeDir != "" {
		return filepath.Join(currentUser.HomeDir, ".easy-cmd", "config.json")
	}
	return ".easy-cmd/config.json"
}

// Load 读取用户配置，补上稳定默认值，并尽早拒绝不支持的语言设置，
// 这样后续代码就可以默认拿到的是可用配置。
func load(path string) (Config, error) {
	cfg, err := ReadRaw(path)
	if err != nil {
		return Config{}, err
	}

	cfg = ApplyDefaults(cfg)
	if !i18n.IsSupported(cfg.Language) {
		return Config{}, fmt.Errorf("config language %q is not supported", cfg.Language)
	}

	return cfg, nil
}

func ReadRaw(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %q: %w", path, err)
	}
	return cfg, nil
}

const DefaultModel = "glm-4.5-air"

func ApplyDefaults(cfg Config) Config {
	if cfg.Language == "" {
		cfg.Language = string(i18n.DefaultLanguage)
	}
	if cfg.Model == "" {
		cfg.Model = DefaultModel
	}
	if cfg.Provider == "" {
		cfg.Provider = "openai"
	}
	return cfg
}

func MissingRuntimeFields(cfg Config) []string {
	var missing []string
	if strings.TrimSpace(cfg.BaseURL) == "" {
		missing = append(missing, "base_url")
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		missing = append(missing, "api_key")
	}
	return missing
}

func IsRuntimeReady(cfg Config) bool {
	return len(MissingRuntimeFields(cfg)) == 0
}

// LoadConfig 从默认路径加载配置，并确保配置已就绪（base_url 和 api_key 已设置）。
func LoadConfig() (Config, error) {
	cfg, err := load(DefaultPath())
	if err != nil {
		return Config{}, err
	}
	if !IsRuntimeReady(cfg) {
		return Config{}, errors.New("config is not runtime-ready")
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	cfg = ApplyDefaults(cfg)
	if !i18n.IsSupported(cfg.Language) {
		return fmt.Errorf("config language %q is not supported", cfg.Language)
	}

	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config %q: %w", path, err)
	}
	raw = append(raw, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir %q: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return fmt.Errorf("write config %q: %w", path, err)
	}
	return nil
}
