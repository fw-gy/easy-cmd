package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"easy-cmd/internal/config"
	"easy-cmd/internal/protocol"
)

type Client struct {
	httpClient *http.Client
	config     config.Config
}

const systemPrompt = `You are easy-cmd, an AI assistant that generates shell command candidates and requests read-only context when needed.
You must respond with strict JSON only.
The only valid response shapes are:
1. {"type":"context_request","requests":[{"id":"...","provider":"...","args":{},"reason":"..."}]}
2. {"type":"assistant_turn","message":"...","candidates":[{"command":"...","summary":"...","risk_level":"low|medium|high","requires_confirmation":true|false}]}
The message should explain the command options in natural language and help the user decide.
Do not include markdown fences or explanatory text.`

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

type responseBody struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func New(cfg config.Config) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		config:     cfg,
	}
}

func (c *Client) NextResponse(ctx context.Context, session protocol.SessionContext) ([]byte, error) {
	sessionJSON, err := json.Marshal(session)
	if err != nil {
		return nil, fmt.Errorf("marshal session context: %w", err)
	}

	body, err := json.Marshal(requestBody{
		Model: c.config.Model,
		Messages: []message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: string(sessionJSON)},
		},
		Stream:      false,
		Temperature: 0.3,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal ai request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.BaseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build ai request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send ai request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read ai response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ai request failed with status %d: %s", resp.StatusCode, string(raw))
	}

	envelope, err := extractEnvelope(raw)
	if err != nil {
		return nil, err
	}
	if _, err := protocol.ParseEnvelope(envelope); err != nil {
		return nil, err
	}
	return envelope, nil
}

func extractEnvelope(raw []byte) ([]byte, error) {
	var response responseBody
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, fmt.Errorf("parse ai response: %w", err)
	}
	if len(response.Choices) == 0 {
		return nil, errors.New("ai response did not include any choices")
	}

	content := strings.TrimSpace(response.Choices[0].Message.Content)
	if content == "" {
		return nil, errors.New("ai response content is empty")
	}

	return []byte(trimCodeFence(content)), nil
}

func trimCodeFence(content string) string {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "```") {
		return content
	}

	lines := strings.Split(content, "\n")
	if len(lines) < 2 {
		return content
	}

	start := 1
	end := len(lines)
	if strings.HasPrefix(lines[len(lines)-1], "```") {
		end = len(lines) - 1
	}
	return strings.Join(lines[start:end], "\n")
}
