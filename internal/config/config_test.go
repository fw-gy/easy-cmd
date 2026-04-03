package config

import (
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadAllowsEmptyBaseURLAndAPIKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"base_url":"","api_key":""}`)

	cfg, err := load(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if cfg.BaseURL != "" {
		t.Fatalf("expected empty base_url, got %q", cfg.BaseURL)
	}
	if cfg.APIKey != "" {
		t.Fatalf("expected empty api_key, got %q", cfg.APIKey)
	}
}

func TestLoadAppliesDefaultModel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"base_url":"https://api.example.com/v1","api_key":"secret"}`)

	cfg, err := load(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if cfg.Model == "" {
		t.Fatal("expected default model to be set")
	}
}

func TestLoadAppliesDefaultLanguage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"base_url":"https://api.example.com/v1","api_key":"secret"}`)

	cfg, err := load(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	value := reflect.ValueOf(cfg)
	field := value.FieldByName("Language")
	if !field.IsValid() {
		t.Fatal("expected Config to expose a Language field")
	}
	if got := field.String(); got != "zh-CN" {
		t.Fatalf("expected default language zh-CN, got %q", got)
	}
}

func TestLoadRejectsUnsupportedLanguage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"base_url":"https://api.example.com/v1","api_key":"secret","language":"fr-FR"}`)

	_, err := load(path)
	if err == nil {
		t.Fatal("expected unsupported language to fail")
	}
}

func TestDefaultPathUsesHomeDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	got := DefaultPath()
	want := filepath.Join(home, ".easy-cmd", "config.json")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestDefaultPathFallsBackWhenHomeMissing(t *testing.T) {
	t.Setenv("HOME", "")

	currentUser, err := user.Current()
	if err != nil || currentUser.HomeDir == "" {
		t.Skip("current user home dir is unavailable")
	}

	got := DefaultPath()
	want := filepath.Join(currentUser.HomeDir, ".easy-cmd", "config.json")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestApplyDefaultsSetsProviderToOpenAI(t *testing.T) {
	cfg := ApplyDefaults(Config{})
	if cfg.Provider != "openai" {
		t.Fatalf("expected default provider 'openai', got %q", cfg.Provider)
	}
}

func TestApplyDefaultsPreservesExistingProvider(t *testing.T) {
	cfg := ApplyDefaults(Config{Provider: "anthropic"})
	if cfg.Provider != "anthropic" {
		t.Fatalf("expected provider 'anthropic', got %q", cfg.Provider)
	}
}

func writeFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
}
