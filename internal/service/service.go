package service

import (
	stdcontext "context"
	"fmt"
	"time"

	"easy-cmd/internal/ai"
	"easy-cmd/internal/config"
	contextengine "easy-cmd/internal/context"
	"easy-cmd/internal/core"
	"easy-cmd/internal/protocol"
	"easy-cmd/internal/providers/filesystem"
	gitproviders "easy-cmd/internal/providers/git"
	"easy-cmd/internal/safety"
)

type engineRunner interface {
	Run(ctx stdcontext.Context, session protocol.SessionContext) (contextengine.RunResult, error)
}

type Service struct {
	engine engineRunner
}

func New(engine engineRunner) *Service {
	return &Service{engine: engine}
}

func NewDefault(cfg config.Config) *Service {
	registry := filesystem.Register(contextengine.Registry{}, filesystem.Options{MaxReadBytes: 64 * 1024})
	registry = gitproviders.Register(registry)
	engine := contextengine.NewEngine(registry, ai.New(cfg), contextengine.Options{MaxRounds: 3, MaxRequestsPerRound: 3})
	return New(engine)
}

func (s *Service) Run(ctx stdcontext.Context, request core.Request) (core.Result, error) {
	if s == nil || s.engine == nil {
		return core.Result{}, fmt.Errorf("service engine is not configured")
	}

	session := protocol.SessionContext{
		SessionID:     request.SessionID,
		UserQuery:     request.Query,
		CWD:           request.CWD,
		WorkspaceRoot: request.WorkspaceRoot,
	}
	if session.SessionID == "" {
		session.SessionID = fmt.Sprintf("sess-%d", time.Now().UnixNano())
	}
	if session.WorkspaceRoot == "" {
		session.WorkspaceRoot = session.CWD
	}

	result, err := s.engine.Run(ctx, session)
	if err != nil {
		return core.Result{}, err
	}

	candidates := safety.ClassifyAll(result.Turn.Candidates)
	out := core.Result{
		Message:    result.Turn.Message,
		Candidates: make([]core.Candidate, 0, len(candidates)),
		Activities: make([]core.Activity, 0, len(result.Activities)),
	}
	for _, candidate := range candidates {
		out.Candidates = append(out.Candidates, core.Candidate{
			Command:              candidate.Command,
			Summary:              candidate.Summary,
			RiskLevel:            candidate.RiskLevel,
			RequiresConfirmation: candidate.RequiresConfirmation,
		})
	}
	for _, activity := range result.Activities {
		out.Activities = append(out.Activities, core.Activity{
			Title:  activity.Title,
			Detail: activity.Detail,
		})
	}
	return out, nil
}
