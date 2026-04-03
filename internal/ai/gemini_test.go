package ai

import (
	"encoding/json"
	"io"
	"testing"
)

func TestGeminiBuildRequest(t *testing.T) {
	g := Gemini{}
	req, err := g.BuildRequest(
		"https://generativelanguage.googleapis.com/v1beta/models",
		"AIza-test-key",
		ChatRequest{
			Model:        "gemini-2.0-flash",
			SystemPrompt: "you are helpful",
			UserContent:  "hello",
		},
	)
	if err != nil {
		t.Fatalf("BuildRequest failed: %v", err)
	}

	if req.Method != "POST" {
		t.Fatalf("expected POST, got %q", req.Method)
	}

	expectedURL := "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent?key=AIza-test-key"
	if req.URL.String() != expectedURL {
		t.Fatalf("unexpected URL: got %q, want %q", req.URL.String(), expectedURL)
	}

	if got := req.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("unexpected Content-Type header: %q", got)
	}

	if got := req.Header.Get("Authorization"); got != "" {
		t.Fatalf("expected no Authorization header, got %q", got)
	}

	raw, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	type part struct {
		Text string `json:"text"`
	}
	type content struct {
		Parts []part `json:"parts"`
	}
	type genConfig struct {
		Temperature float64 `json:"temperature"`
	}
	type body struct {
		SystemInstruction content   `json:"system_instruction"`
		Contents          []content `json:"contents"`
		GenerationConfig  genConfig `json:"generationConfig"`
	}

	var b body
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if len(b.SystemInstruction.Parts) != 1 || b.SystemInstruction.Parts[0].Text != "you are helpful" {
		t.Fatalf("unexpected system_instruction: %+v", b.SystemInstruction)
	}

	if len(b.Contents) != 1 {
		t.Fatalf("expected 1 content entry, got %d", len(b.Contents))
	}
	if len(b.Contents[0].Parts) != 1 || b.Contents[0].Parts[0].Text != "hello" {
		t.Fatalf("unexpected contents: %+v", b.Contents)
	}

	if b.GenerationConfig.Temperature != 0.3 {
		t.Fatalf("expected temperature 0.3, got %v", b.GenerationConfig.Temperature)
	}
}

func TestGeminiBuildRequestTrimsTrailingSlash(t *testing.T) {
	g := Gemini{}
	req, err := g.BuildRequest(
		"https://generativelanguage.googleapis.com/v1beta/models/",
		"key123",
		ChatRequest{Model: "gemini-2.0-flash", SystemPrompt: "s", UserContent: "u"},
	)
	if err != nil {
		t.Fatalf("BuildRequest failed: %v", err)
	}

	expectedURL := "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent?key=key123"
	if req.URL.String() != expectedURL {
		t.Fatalf("unexpected URL after trailing slash trim: got %q, want %q", req.URL.String(), expectedURL)
	}
}

func TestGeminiParseResponse(t *testing.T) {
	g := Gemini{}
	body := []byte(`{"candidates":[{"content":{"parts":[{"text":"ls -la"}]}}]}`)
	resp, err := g.ParseResponse(body)
	if err != nil {
		t.Fatalf("ParseResponse failed: %v", err)
	}
	if resp.Content != "ls -la" {
		t.Fatalf("expected content %q, got %q", "ls -la", resp.Content)
	}
}

func TestGeminiParseResponseEmptyCandidates(t *testing.T) {
	g := Gemini{}
	body := []byte(`{"candidates":[]}`)
	_, err := g.ParseResponse(body)
	if err == nil {
		t.Fatal("expected error for empty candidates")
	}
}

func TestGeminiParseResponseStripsCodeFence(t *testing.T) {
	g := Gemini{}
	body := []byte("{\n  \"candidates\": [{\"content\": {\"parts\": [{\"text\": \"```json\\n{\\\"key\\\":\\\"value\\\"}\\n```\"}]}}]\n}")
	resp, err := g.ParseResponse(body)
	if err != nil {
		t.Fatalf("ParseResponse failed: %v", err)
	}
	if resp.Content != `{"key":"value"}` {
		t.Fatalf("expected code fence stripped, got %q", resp.Content)
	}
}
