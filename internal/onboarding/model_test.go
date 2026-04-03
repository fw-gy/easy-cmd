package onboarding

import (
	"strings"
	"testing"

	"easy-cmd/internal/config"
)

func TestModelSubmitReturnsCompletedConfig(t *testing.T) {
	model := New(config.Config{
		BaseURL:  "https://example.com",
		APIKey:   "secret",
		Model:    "custom",
		Language: "en-US",
	}, "")

	next, err := model.submit()
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}
	if next.errMsg != "" {
		t.Fatalf("expected no inline error, got %q", next.errMsg)
	}
	if next.result.Cancelled {
		t.Fatal("did not expect cancellation")
	}
	if next.result.Config.BaseURL != "https://example.com" {
		t.Fatalf("unexpected base_url %q", next.result.Config.BaseURL)
	}
	if next.result.Config.APIKey != "secret" {
		t.Fatalf("unexpected api_key %q", next.result.Config.APIKey)
	}
}

func TestModelSubmitRejectsMissingRequiredFields(t *testing.T) {
	model := New(config.Config{}, "")

	next, err := model.submit()
	if err == nil {
		t.Fatal("expected validation error")
	}
	if next.errMsg == "" {
		t.Fatal("expected inline error message")
	}
	if next.result.Config.BaseURL != "" || next.result.Config.APIKey != "" {
		t.Fatalf("did not expect result config on validation failure, got %#v", next.result.Config)
	}
}

func TestViewDoesNotRenderLoadReason(t *testing.T) {
	model := New(config.Config{}, `read config "/tmp/config.json": no such file or directory`)

	got := model.View()
	if got == "" {
		t.Fatal("expected non-empty view")
	}
	if strings.Contains(got, "read config") {
		t.Fatalf("expected load reason to stay hidden, got %q", got)
	}
}

func TestViewHighlightsFocusedField(t *testing.T) {
	model := New(config.Config{}, "")

	got := model.View()
	if !strings.Contains(got, "Base URL") {
		t.Fatalf("expected focused label in view, got %q", got)
	}
	if strings.Contains(got, "› Base URL") {
		t.Fatalf("expected no › prefix on label, got %q", got)
	}
}

func TestModelSubmitIncludesProvider(t *testing.T) {
	model := New(config.Config{
		BaseURL:  "https://example.com",
		APIKey:   "secret",
		Model:    "custom",
		Language: "en-US",
		Provider: "anthropic",
	}, "")

	next, err := model.submit()
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}
	if next.result.Config.Provider != "anthropic" {
		t.Fatalf("expected provider 'anthropic', got %q", next.result.Config.Provider)
	}
}

func TestModelSubmitDefaultsProviderToOpenAI(t *testing.T) {
	model := New(config.Config{
		BaseURL:  "https://example.com",
		APIKey:   "secret",
		Model:    "custom",
		Language: "en-US",
	}, "")

	next, err := model.submit()
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}
	if next.result.Config.Provider != "openai" {
		t.Fatalf("expected provider 'openai', got %q", next.result.Config.Provider)
	}
}
