package config_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"easy-cmd/internal/config"
)

func TestLoadRequiresBaseURLAndAPIKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"base_url":"","api_key":""}`)

	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected missing fields to fail")
	}
}

func TestLoadAppliesDefaultModel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"base_url":"https://api.example.com/v1","api_key":"secret"}`)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Model == "" {
		t.Fatal("expected default model to be set")
	}
}

func TestLoadAppliesDefaultLanguage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"base_url":"https://api.example.com/v1","api_key":"secret"}`)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
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

	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected unsupported language to fail")
	}
}

func writeFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
}
