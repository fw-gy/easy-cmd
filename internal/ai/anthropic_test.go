package ai

import (
	"encoding/json"
	"io"
	"testing"
)

func TestAnthropicBuildRequest(t *testing.T) {
	a := Anthropic{}
	req, err := a.BuildRequest("https://api.anthropic.com/v1/messages", "test-key", ChatRequest{
		Model:        "claude-3-5-sonnet-20241022",
		SystemPrompt: "you are helpful",
		UserContent:  "hello",
	})
	if err != nil {
		t.Fatalf("BuildRequest failed: %v", err)
	}

	if req.Method != "POST" {
		t.Fatalf("expected POST, got %q", req.Method)
	}
	if req.URL.String() != "https://api.anthropic.com/v1/messages" {
		t.Fatalf("unexpected URL: %q", req.URL.String())
	}
	if got := req.Header.Get("x-api-key"); got != "test-key" {
		t.Fatalf("unexpected x-api-key header: %q", got)
	}
	if got := req.Header.Get("anthropic-version"); got != "2023-06-01" {
		t.Fatalf("unexpected anthropic-version header: %q", got)
	}
	if got := req.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("unexpected Content-Type header: %q", got)
	}
	if got := req.Header.Get("Authorization"); got != "" {
		t.Fatalf("expected no Authorization header, got %q", got)
	}

	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type body struct {
		Model     string    `json:"model"`
		MaxTokens int       `json:"max_tokens"`
		System    string    `json:"system"`
		Messages  []message `json:"messages"`
	}

	raw, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	var b body
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if b.Model != "claude-3-5-sonnet-20241022" {
		t.Fatalf("expected model claude-3-5-sonnet-20241022, got %q", b.Model)
	}
	if b.MaxTokens != 4096 {
		t.Fatalf("expected max_tokens 4096, got %d", b.MaxTokens)
	}
	if b.System != "you are helpful" {
		t.Fatalf("expected system %q, got %q", "you are helpful", b.System)
	}
	if len(b.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(b.Messages))
	}
	if b.Messages[0].Role != "user" || b.Messages[0].Content != "hello" {
		t.Fatalf("unexpected user message: %+v", b.Messages[0])
	}
}

func TestAnthropicParseResponse(t *testing.T) {
	a := Anthropic{}
	body := []byte(`{"content":[{"type":"text","text":"ls -la"}]}`)
	resp, err := a.ParseResponse(body)
	if err != nil {
		t.Fatalf("ParseResponse failed: %v", err)
	}
	if resp.Content != "ls -la" {
		t.Fatalf("expected content %q, got %q", "ls -la", resp.Content)
	}
}

func TestAnthropicParseResponseEmptyContent(t *testing.T) {
	a := Anthropic{}
	body := []byte(`{"content":[]}`)
	_, err := a.ParseResponse(body)
	if err == nil {
		t.Fatal("expected error for empty content array")
	}
}

func TestAnthropicParseResponseStripsCodeFence(t *testing.T) {
	a := Anthropic{}
	body := []byte("{\n  \"content\": [{\"type\": \"text\", \"text\": \"```json\\n{\\\"key\\\":\\\"value\\\"}\\n```\"}]\n}")
	resp, err := a.ParseResponse(body)
	if err != nil {
		t.Fatalf("ParseResponse failed: %v", err)
	}
	if resp.Content != `{"key":"value"}` {
		t.Fatalf("expected code fence stripped, got %q", resp.Content)
	}
}
