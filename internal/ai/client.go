package ai

import (
	"bufio"
	"context"
	"encoding/json"
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
	provider   Provider
}

const systemPrompt = `You are easy-cmd, an AI assistant that generates shell command candidates and requests read-only context when needed.
You must respond with strict JSON only.
The only valid response shapes are:
1. {"type":"context_request","requests":[{"id":"...","provider":"...","args":{},"reason":"..."}]}
2. {"type":"assistant_turn","message":"...","candidates":[{"command":"...","summary":"...","risk_level":"low|medium|high","requires_confirmation":true|false}]}
The message should explain the command options in natural language and help the user decide.
Do not include markdown fences or explanatory text.`

// New 创建基于 HTTP 的模型客户端。调用方通过配置传入接口地址和
// API Key，而不是把具体提供方细节写死在代码里。
func New(cfg config.Config) *Client {
	provider, err := ResolveProvider(cfg.Provider)
	if err != nil {
		provider = OpenAI{}
	}
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		config:     cfg,
		provider:   provider,
	}
}

// NextResponse 会把完整会话序列化为 JSON，发送到配置好的 AI 接口，并返回一个已经过校验的协议信封。
func (c *Client) NextResponse(ctx context.Context, session protocol.SessionContext) ([]byte, error) {
	if err := c.validateRuntimeConfig(); err != nil {
		return nil, err
	}

	sessionJSON, err := json.Marshal(session)
	if err != nil {
		return nil, fmt.Errorf("marshal session context: %w", err)
	}

	req, err := c.provider.BuildRequest(c.config.BaseURL, c.config.APIKey, ChatRequest{
		Model:        c.config.Model,
		SystemPrompt: systemPrompt,
		UserContent:  string(sessionJSON),
	})
	if err != nil {
		return nil, fmt.Errorf("build ai request: %w", err)
	}
	req = req.WithContext(ctx)

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

	chatResp, err := c.provider.ParseResponse(raw)
	if err != nil {
		return nil, err
	}

	envelope := []byte(chatResp.Content)
	if _, err := protocol.ParseEnvelope(envelope); err != nil {
		return nil, err
	}
	return envelope, nil
}

// validateRuntimeConfig 允许程序先启动，但如果用户还没填好必需的
// 接口配置，会在第一次发起网络请求前就直接报错。
func (c *Client) validateRuntimeConfig() error {
	if strings.TrimSpace(c.config.BaseURL) != "" && strings.TrimSpace(c.config.APIKey) != "" {
		return nil
	}

	return fmt.Errorf("missing configuration: please edit %s and set base_url and api_key", defaultConfigPath())
}

func defaultConfigPath() string {
	return config.DefaultPath()
}

// trimCodeFence 用来兼容那些把 JSON 包进单个 markdown 代码块的后端。
func trimCodeFence(content string) string {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "```") {
		return content
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	lines := make([]string, 0, 8)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
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
