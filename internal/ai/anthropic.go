package ai

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// Anthropic implements Provider for the Anthropic Messages API.
type Anthropic struct{}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

// BuildRequest creates an HTTP POST request to baseURL with the Anthropic
// Messages API JSON body and required headers.
func (Anthropic) BuildRequest(baseURL string, apiKey string, chat ChatRequest) (*http.Request, error) {
	body, err := json.Marshal(anthropicRequest{
		Model:     chat.Model,
		MaxTokens: 4096,
		System:    chat.SystemPrompt,
		Messages: []anthropicMessage{
			{Role: "user", Content: chat.UserContent},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal anthropic request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build anthropic request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	return req, nil
}

// ParseResponse extracts the model's text content from an Anthropic Messages API
// response body, stripping any markdown code fences from the result.
func (Anthropic) ParseResponse(body []byte) (ChatResponse, error) {
	var resp anthropicResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return ChatResponse{}, fmt.Errorf("parse anthropic response: %w", err)
	}
	if len(resp.Content) == 0 {
		return ChatResponse{}, errors.New("anthropic response did not include any content blocks")
	}

	var texts []string
	for _, block := range resp.Content {
		if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
			texts = append(texts, block.Text)
		}
	}
	if len(texts) == 0 {
		return ChatResponse{}, errors.New("anthropic response has no text content")
	}
	content := strings.TrimSpace(strings.Join(texts, "\n"))
	return ChatResponse{Content: trimCodeFence(content)}, nil
}
