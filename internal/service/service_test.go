package service_test

import (
	stdcontext "context"
	"testing"

	contextengine "easy-cmd/internal/context"
	"easy-cmd/internal/core"
	"easy-cmd/internal/protocol"
	"easy-cmd/internal/service"
)

func TestServiceRunBuildsOneShotSessionAndClassifiesCandidates(t *testing.T) {
	engine := &stubEngine{
		result: contextengine.RunResult{
			Turn: protocol.AssistantTurnEnvelope{
				Type:    "assistant_turn",
				Message: "危险命令。",
				Candidates: []protocol.CommandCandidate{
					{Command: "rm -rf build", Summary: "remove build"},
				},
			},
			Activities: []contextengine.Activity{
				{Title: "Ran filesystem.list", Detail: "path=."},
			},
		},
	}

	svc := service.New(engine)
	result, err := svc.Run(stdcontext.Background(), core.Request{
		Query:         "删除构建目录",
		CWD:           "/repo",
		WorkspaceRoot: "/workspace",
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if engine.session.UserQuery != "删除构建目录" {
		t.Fatalf("unexpected query %q", engine.session.UserQuery)
	}
	if engine.session.CWD != "/repo" {
		t.Fatalf("unexpected cwd %q", engine.session.CWD)
	}
	if engine.session.WorkspaceRoot != "/workspace" {
		t.Fatalf("unexpected workspace root %q", engine.session.WorkspaceRoot)
	}
	if engine.session.SessionID == "" {
		t.Fatal("expected service to assign session id")
	}
	if len(engine.session.Conversation) != 0 {
		t.Fatalf("expected one-shot session without conversation, got %#v", engine.session.Conversation)
	}

	if result.Message != "危险命令。" {
		t.Fatalf("unexpected result message %q", result.Message)
	}
	if len(result.Activities) != 1 || result.Activities[0].Title != "Ran filesystem.list" {
		t.Fatalf("unexpected activities %#v", result.Activities)
	}
	if len(result.Candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(result.Candidates))
	}
	if result.Candidates[0].RiskLevel != protocol.RiskHigh {
		t.Fatalf("expected high risk classification, got %q", result.Candidates[0].RiskLevel)
	}
	if !result.Candidates[0].RequiresConfirmation {
		t.Fatal("expected service to require confirmation for high-risk candidate")
	}
}

func TestServiceRunDefaultsWorkspaceRootToCWD(t *testing.T) {
	engine := &stubEngine{
		result: contextengine.RunResult{
			Turn: protocol.AssistantTurnEnvelope{
				Type:    "assistant_turn",
				Message: "只读命令。",
				Candidates: []protocol.CommandCandidate{
					{Command: "pwd", Summary: "show cwd"},
				},
			},
		},
	}

	svc := service.New(engine)
	_, err := svc.Run(stdcontext.Background(), core.Request{
		Query: "显示当前目录",
		CWD:   "/repo",
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if engine.session.WorkspaceRoot != "/repo" {
		t.Fatalf("expected workspace root to default to cwd, got %q", engine.session.WorkspaceRoot)
	}
}

type stubEngine struct {
	result  contextengine.RunResult
	err     error
	session protocol.SessionContext
}

func (s *stubEngine) Run(_ stdcontext.Context, session protocol.SessionContext) (contextengine.RunResult, error) {
	s.session = session
	if s.err != nil {
		return contextengine.RunResult{}, s.err
	}
	return s.result, nil
}
