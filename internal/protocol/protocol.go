package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
)

type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

type Action string

const (
	ActionExecute Action = "execute"
	ActionCancel  Action = "cancel"
)

type SessionContext struct {
	// SessionID 用来标识一次 easy-cmd 交互运行。
	SessionID        string                `json:"session_id"`
	// UserQuery 是当前这轮正在处理的最新用户请求。
	UserQuery        string                `json:"user_query"`
	// CWD 是这次请求发起时所在的 shell 目录。
	CWD              string                `json:"cwd"`
	// WorkspaceRoot 用来限制 context provider，防止它们越出当前项目。
	WorkspaceRoot    string                `json:"workspace_root,omitempty"`
	// Conversation 保存历史用户/助手对话，供后续追问时参考。
	Conversation     []ConversationMessage `json:"conversation,omitempty"`
	// CollectedContext 保存当前 session 已经取回过的 provider 输出。
	CollectedContext []ContextResult       `json:"collected_context,omitempty"`
	// RequestHistory 让模型知道哪些上下文请求已经发起过。
	RequestHistory   []ContextRequest      `json:"request_history,omitempty"`
}

type ConversationMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ContextRequestEnvelope struct {
	Type     string           `json:"type"`
	Requests []ContextRequest `json:"requests"`
}

type ContextRequest struct {
	ID       string          `json:"id"`
	Provider string          `json:"provider"`
	Args     json.RawMessage `json:"args"`
	Reason   string          `json:"reason"`
}

type ContextResultEnvelope struct {
	Type    string          `json:"type"`
	Results []ContextResult `json:"results"`
}

type ContextResult struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
	OK       bool   `json:"ok"`
	Data     any    `json:"data,omitempty"`
	Error    string `json:"error,omitempty"`
}

type AssistantTurnEnvelope struct {
	Type       string             `json:"type"`
	Message    string             `json:"message"`
	Candidates []CommandCandidate `json:"candidates"`
}

type CommandCandidate struct {
	Command              string    `json:"command"`
	Summary              string    `json:"summary"`
	RiskLevel            RiskLevel `json:"risk_level"`
	RequiresConfirmation bool      `json:"requires_confirmation"`
}

type AppOutput struct {
	Action          Action `json:"action"`
	SelectedCommand string `json:"selected_command,omitempty"`
}

type envelopeHeader struct {
	Type string `json:"type"`
}

// ParseEnvelope 是 AI 客户端和上下文引擎共用的统一 JSON 入口。
// 它会先读取轻量级 type 头，再解析成后续流程需要的具体结构。
func ParseEnvelope(raw []byte) (any, error) {
	var header envelopeHeader
	if err := json.Unmarshal(raw, &header); err != nil {
		return nil, fmt.Errorf("parse envelope header: %w", err)
	}

	switch header.Type {
	case "context_request":
		var out ContextRequestEnvelope
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, fmt.Errorf("parse context request: %w", err)
		}
		if len(out.Requests) == 0 {
			return nil, errors.New("context_request must contain at least one request")
		}
		return out, nil
	case "context_result":
		var out ContextResultEnvelope
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, fmt.Errorf("parse context result: %w", err)
		}
		return out, nil
	case "assistant_turn":
		var out AssistantTurnEnvelope
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, fmt.Errorf("parse assistant turn: %w", err)
		}
		// 这个 UI 的核心就是从具体命令里做选择，所以 assistant_turn
		// 如果没有候选命令，就视为非法数据。
		if len(out.Candidates) == 0 {
			return nil, errors.New("assistant_turn must contain at least one candidate")
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported envelope type %q", header.Type)
	}
}
