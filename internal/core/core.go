package core

import (
	stdcontext "context"

	"easy-cmd/internal/protocol"
)

// Request 描述一次完整且独立的逻辑层调用。
type Request struct {
	SessionID     string `json:"session_id,omitempty"`
	Query         string `json:"query"`
	CWD           string `json:"cwd"`
	WorkspaceRoot string `json:"workspace_root,omitempty"`
}

type Activity struct {
	Title  string `json:"title"`
	Detail string `json:"detail,omitempty"`
}

type Candidate struct {
	Command              string             `json:"command"`
	Summary              string             `json:"summary"`
	RiskLevel            protocol.RiskLevel `json:"risk_level"`
	RequiresConfirmation bool               `json:"requires_confirmation"`
}

type Result struct {
	Message    string      `json:"message"`
	Candidates []Candidate `json:"candidates"`
	Activities []Activity  `json:"activities,omitempty"`
}

type Runner interface {
	Run(ctx stdcontext.Context, request Request) (Result, error)
}
