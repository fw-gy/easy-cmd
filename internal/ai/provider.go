package ai

import (
	"fmt"
	"net/http"
	"strings"
)

// ChatRequest is the provider-agnostic representation of what
// ai.Client wants to send. Each Provider turns it into a
// provider-specific HTTP request.
type ChatRequest struct {
	Model        string
	SystemPrompt string
	UserContent  string
}

// ChatResponse is the provider-agnostic result: the raw text
// content the model returned, already extracted from the
// provider-specific envelope.
type ChatResponse struct {
	Content string
}

// Provider abstracts the wire-format differences between AI API
// protocols. ai.Client calls BuildRequest / ParseResponse and
// never touches HTTP headers or JSON shapes directly.
type Provider interface {
	// BuildRequest creates an http.Request with the correct URL,
	// headers, and body for this provider.
	BuildRequest(baseURL string, apiKey string, chat ChatRequest) (*http.Request, error)

	// ParseResponse reads the provider-specific HTTP response body
	// and extracts the model's text content.
	ParseResponse(body []byte) (ChatResponse, error)
}

// ResolveProvider returns the Provider implementation for the given
// provider name from config. Returns an error for unknown providers.
func ResolveProvider(name string) (Provider, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "openai", "":
		return OpenAI{}, nil
	case "anthropic":
		return Anthropic{}, nil
	case "gemini":
		return Gemini{}, nil
	default:
		return nil, fmt.Errorf("unsupported provider %q: valid options are openai, anthropic, gemini", name)
	}
}
