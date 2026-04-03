package ai

import (
	"testing"
)

func TestResolveProviderOpenAI(t *testing.T) {
	p, err := ResolveProvider("openai")
	if err != nil {
		t.Fatalf("ResolveProvider failed: %v", err)
	}
	if _, ok := p.(OpenAI); !ok {
		t.Fatalf("expected OpenAI, got %T", p)
	}
}

func TestResolveProviderAnthropic(t *testing.T) {
	p, err := ResolveProvider("anthropic")
	if err != nil {
		t.Fatalf("ResolveProvider failed: %v", err)
	}
	if _, ok := p.(Anthropic); !ok {
		t.Fatalf("expected Anthropic, got %T", p)
	}
}

func TestResolveProviderGemini(t *testing.T) {
	p, err := ResolveProvider("gemini")
	if err != nil {
		t.Fatalf("ResolveProvider failed: %v", err)
	}
	if _, ok := p.(Gemini); !ok {
		t.Fatalf("expected Gemini, got %T", p)
	}
}

func TestResolveProviderEmpty(t *testing.T) {
	p, err := ResolveProvider("")
	if err != nil {
		t.Fatalf("ResolveProvider failed for empty: %v", err)
	}
	if _, ok := p.(OpenAI); !ok {
		t.Fatalf("expected OpenAI for empty provider, got %T", p)
	}
}

func TestResolveProviderUnknown(t *testing.T) {
	_, err := ResolveProvider("unknown")
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}
