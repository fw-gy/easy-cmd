package ai

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// OpenAI implements Provider for OpenAI-compatible chat-completions APIs
// (also used by ZhiPu, DeepSeek, and other compatible providers).
type OpenAI struct{}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Stream      bool            `json:"stream"`
	Temperature float64         `json:"temperature"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// BuildRequest creates an HTTP POST request to baseURL with the OpenAI
// chat-completions JSON body and the required headers.
func (OpenAI) BuildRequest(baseURL string, apiKey string, chat ChatRequest) (*http.Request, error) {
	body, err := json.Marshal(openAIRequest{
		Model: chat.Model,
		Messages: []openAIMessage{
			{Role: "system", Content: chat.SystemPrompt},
			{Role: "user", Content: chat.UserContent},
		},
		Stream:      false,
		Temperature: 0.3,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal openai request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build openai request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	return req, nil
}

// ParseResponse extracts the model's text content from an OpenAI-compatible
// response body, stripping any markdown code fences from the result.
func (OpenAI) ParseResponse(body []byte) (ChatResponse, error) {
	var resp openAIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return ChatResponse{}, fmt.Errorf("parse openai response: %w", err)
	}
	if len(resp.Choices) == 0 {
		return ChatResponse{}, errors.New("openai response did not include any choices")
	}
	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	if content == "" {
		return ChatResponse{}, errors.New("openai response content is empty")
	}
	return ChatResponse{Content: trimCodeFence(content)}, nil
}
