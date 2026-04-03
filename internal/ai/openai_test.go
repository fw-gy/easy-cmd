package ai

import (
	"encoding/json"
	"io"
	"testing"
)

func TestOpenAIBuildRequest(t *testing.T) {
	o := OpenAI{}
	req, err := o.BuildRequest("https://api.example.com/v1/chat/completions", "test-key", ChatRequest{
		Model:        "gpt-4",
		SystemPrompt: "you are helpful",
		UserContent:  "hello",
	})
	if err != nil {
		t.Fatalf("BuildRequest failed: %v", err)
	}

	if req.Method != "POST" {
		t.Fatalf("expected POST, got %q", req.Method)
	}
	if req.URL.String() != "https://api.example.com/v1/chat/completions" {
		t.Fatalf("unexpected URL: %q", req.URL.String())
	}
	if got := req.Header.Get("Authorization"); got != "Bearer test-key" {
		t.Fatalf("unexpected Authorization header: %q", got)
	}
	if got := req.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("unexpected Content-Type header: %q", got)
	}

	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type body struct {
		Model       string    `json:"model"`
		Messages    []message `json:"messages"`
		Stream      bool      `json:"stream"`
		Temperature float64   `json:"temperature"`
	}

	raw, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	var b body
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if b.Model != "gpt-4" {
		t.Fatalf("expected model gpt-4, got %q", b.Model)
	}
	if b.Stream {
		t.Fatal("expected stream=false")
	}
	if b.Temperature != 0.3 {
		t.Fatalf("expected temperature 0.3, got %v", b.Temperature)
	}
	if len(b.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(b.Messages))
	}
	if b.Messages[0].Role != "system" || b.Messages[0].Content != "you are helpful" {
		t.Fatalf("unexpected system message: %+v", b.Messages[0])
	}
	if b.Messages[1].Role != "user" || b.Messages[1].Content != "hello" {
		t.Fatalf("unexpected user message: %+v", b.Messages[1])
	}
}

func TestOpenAIParseResponse(t *testing.T) {
	o := OpenAI{}
	body := []byte(`{"choices":[{"message":{"content":"ls -la"}}]}`)
	resp, err := o.ParseResponse(body)
	if err != nil {
		t.Fatalf("ParseResponse failed: %v", err)
	}
	if resp.Content != "ls -la" {
		t.Fatalf("expected content %q, got %q", "ls -la", resp.Content)
	}
}

func TestOpenAIParseResponseEmptyChoices(t *testing.T) {
	o := OpenAI{}
	body := []byte(`{"choices":[]}`)
	_, err := o.ParseResponse(body)
	if err == nil {
		t.Fatal("expected error for empty choices")
	}
}

func TestOpenAIParseResponseStripsCodeFence(t *testing.T) {
	o := OpenAI{}
	body := []byte("{\n  \"choices\": [{\"message\": {\"content\": \"```json\\n{\\\"key\\\":\\\"value\\\"}\\n```\"}}]\n}")
	resp, err := o.ParseResponse(body)
	if err != nil {
		t.Fatalf("ParseResponse failed: %v", err)
	}
	if resp.Content != `{"key":"value"}` {
		t.Fatalf("expected code fence stripped, got %q", resp.Content)
	}
}
