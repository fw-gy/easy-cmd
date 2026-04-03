package ai

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// Gemini implements Provider for Google's Gemini API.
type Gemini struct{}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiRequest struct {
	SystemInstruction geminiContent   `json:"system_instruction"`
	Contents          []geminiContent `json:"contents"`
	GenerationConfig  geminiGenConfig `json:"generationConfig"`
}

type geminiGenConfig struct {
	Temperature float64 `json:"temperature"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

// BuildRequest creates an HTTP POST request to the Gemini generateContent
// endpoint. The model name and API key are embedded in the URL; no
// Authorization header is set.
func (Gemini) BuildRequest(baseURL string, apiKey string, chat ChatRequest) (*http.Request, error) {
	body, err := json.Marshal(geminiRequest{
		SystemInstruction: geminiContent{
			Parts: []geminiPart{{Text: chat.SystemPrompt}},
		},
		Contents: []geminiContent{
			{Parts: []geminiPart{{Text: chat.UserContent}}},
		},
		GenerationConfig: geminiGenConfig{Temperature: 0.3},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal gemini request: %w", err)
	}

	url := strings.TrimRight(baseURL, "/") + "/" + chat.Model + ":generateContent?key=" + apiKey
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build gemini request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

// ParseResponse extracts the model's text content from a Gemini API response
// body, joining multiple parts with newlines and stripping markdown code fences.
func (Gemini) ParseResponse(body []byte) (ChatResponse, error) {
	var resp geminiResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return ChatResponse{}, fmt.Errorf("parse gemini response: %w", err)
	}
	if len(resp.Candidates) == 0 {
		return ChatResponse{}, errors.New("gemini response did not include any candidates")
	}

	parts := resp.Candidates[0].Content.Parts
	var texts []string
	for _, part := range parts {
		if t := strings.TrimSpace(part.Text); t != "" {
			texts = append(texts, t)
		}
	}
	if len(texts) == 0 {
		return ChatResponse{}, errors.New("gemini response has no text content")
	}
	content := strings.TrimSpace(strings.Join(texts, "\n"))
	return ChatResponse{Content: trimCodeFence(content)}, nil
}
