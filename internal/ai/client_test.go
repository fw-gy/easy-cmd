package ai_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"easy-cmd/internal/ai"
	"easy-cmd/internal/config"
	"easy-cmd/internal/protocol"
)

func TestClientSendsChatCompletionsRequest(t *testing.T) {
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type requestBody struct {
		Model       string    `json:"model"`
		Messages    []message `json:"messages"`
		Stream      bool      `json:"stream"`
		Temperature float64   `json:"temperature"`
	}

	var captured requestBody
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("unexpected authorization header: %q", got)
		}

		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll failed: %v", err)
		}
		if err := json.Unmarshal(raw, &captured); err != nil {
			t.Fatalf("json.Unmarshal failed: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices":[
				{"message":{"content":"{\"type\":\"assistant_turn\",\"message\":\"可以直接列目录。\",\"candidates\":[{\"command\":\"ls\",\"summary\":\"list\",\"risk_level\":\"low\",\"requires_confirmation\":false}]}" }}
			]
		}`))
	}))
	defer server.Close()

	client := ai.New(config.Config{BaseURL: server.URL, APIKey: "secret", Model: "glm-4.5-air"})
	_, err := client.NextResponse(context.Background(), protocol.SessionContext{
		SessionID:     "sess-1",
		UserQuery:     "列出当前目录文件",
		CWD:           "/tmp/project",
		WorkspaceRoot: "/tmp/project",
	})
	if err != nil {
		t.Fatalf("NextResponse failed: %v", err)
	}

	if captured.Model != "glm-4.5-air" {
		t.Fatalf("unexpected model: %q", captured.Model)
	}
	if captured.Stream {
		t.Fatal("expected stream=false")
	}
	if captured.Temperature != 0.3 {
		t.Fatalf("unexpected temperature: %v", captured.Temperature)
	}
	if len(captured.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(captured.Messages))
	}
	if captured.Messages[0].Role != "system" {
		t.Fatalf("unexpected system role: %q", captured.Messages[0].Role)
	}
	if captured.Messages[1].Role != "user" {
		t.Fatalf("unexpected user role: %q", captured.Messages[1].Role)
	}
	if captured.Messages[1].Content == "" {
		t.Fatal("expected user message content to be populated")
	}
	if captured.Messages[1].Content == `{"session_id":"sess-1"}` {
		t.Fatal("expected user message content to include the full session context")
	}
}

func TestClientRejectsNonJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not-json"))
	}))
	defer server.Close()

	client := ai.New(config.Config{BaseURL: server.URL, APIKey: "secret", Model: "test-model"})
	_, err := client.NextResponse(context.Background(), protocol.SessionContext{SessionID: "sess-1"})
	if err == nil {
		t.Fatal("expected invalid json response to fail")
	}
}

func TestClientRejectsWrongEnvelopeShape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"type\":\"unknown\"}"}}]}`))
	}))
	defer server.Close()

	client := ai.New(config.Config{BaseURL: server.URL, APIKey: "secret", Model: "test-model"})
	_, err := client.NextResponse(context.Background(), protocol.SessionContext{SessionID: "sess-1"})
	if err == nil {
		t.Fatal("expected wrong envelope shape to fail")
	}
}

func TestClientRejectsMissingRuntimeConfig(t *testing.T) {
	client := ai.New(config.Config{Model: "test-model"})

	_, err := client.NextResponse(context.Background(), protocol.SessionContext{SessionID: "sess-1"})
	if err == nil {
		t.Fatal("expected missing runtime config to fail")
	}
	if !strings.Contains(err.Error(), "config.json") {
		t.Fatalf("expected error to mention config.json, got %v", err)
	}
}

func TestClientExtractsEnvelopeFromChoiceMessageContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices":[
				{"message":{"content":"{\"type\":\"context_request\",\"requests\":[{\"id\":\"req-1\",\"provider\":\"filesystem.list\",\"args\":{\"path\":\".\",\"depth\":1},\"reason\":\"inspect\"}]}" }}
			]
		}`))
	}))
	defer server.Close()

	client := ai.New(config.Config{BaseURL: server.URL, APIKey: "secret", Model: "test-model"})
	raw, err := client.NextResponse(context.Background(), protocol.SessionContext{SessionID: "sess-1"})
	if err != nil {
		t.Fatalf("NextResponse failed: %v", err)
	}

	envelope, err := protocol.ParseEnvelope(raw)
	if err != nil {
		t.Fatalf("ParseEnvelope failed: %v", err)
	}
	if _, ok := envelope.(protocol.ContextRequestEnvelope); !ok {
		t.Fatalf("expected context request envelope, got %T", envelope)
	}
}

func TestClientExtractsAssistantTurnFromChoiceMessageContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices":[
				{"message":{"content":"{\"type\":\"assistant_turn\",\"message\":\"我把范围缩小到当前目录。\",\"candidates\":[{\"command\":\"find . -maxdepth 2 -type f\",\"summary\":\"find files\",\"risk_level\":\"low\",\"requires_confirmation\":false}]}" }}
			]
		}`))
	}))
	defer server.Close()

	client := ai.New(config.Config{BaseURL: server.URL, APIKey: "secret", Model: "test-model"})
	raw, err := client.NextResponse(context.Background(), protocol.SessionContext{SessionID: "sess-1"})
	if err != nil {
		t.Fatalf("NextResponse failed: %v", err)
	}

	envelope, err := protocol.ParseEnvelope(raw)
	if err != nil {
		t.Fatalf("ParseEnvelope failed: %v", err)
	}
	turn, ok := envelope.(protocol.AssistantTurnEnvelope)
	if !ok {
		t.Fatalf("expected assistant turn envelope, got %T", envelope)
	}
	if turn.Message == "" {
		t.Fatal("expected assistant message to be present")
	}
}
