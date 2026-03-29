package context_test

import (
	stdcontext "context"
	"encoding/json"
	"errors"
	"testing"

	contextengine "easy-cmd/internal/context"
	"easy-cmd/internal/protocol"
)

func TestEngineStopsOnCandidates(t *testing.T) {
	client := stubClient{
		responses: []string{
			`{"type":"assistant_turn","message":"Use ls.","candidates":[{"command":"ls","summary":"list","risk_level":"low","requires_confirmation":false}]}`,
		},
	}

	engine := contextengine.NewEngine(contextengine.Registry{}, client, contextengine.Options{MaxRounds: 3, MaxRequestsPerRound: 3})
	session := protocol.SessionContext{SessionID: "sess-1", UserQuery: "list files", CWD: "."}

	got, err := engine.Run(stdcontext.Background(), session)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if len(got.Turn.Candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(got.Turn.Candidates))
	}
	if got.Turn.Message == "" {
		t.Fatal("expected assistant message to be preserved")
	}
}

func TestEngineEnforcesRequestLimits(t *testing.T) {
	client := stubClient{
		responses: []string{
			`{"type":"context_request","requests":[
				{"id":"1","provider":"filesystem.list","args":{"path":"."},"reason":"a"},
				{"id":"2","provider":"filesystem.list","args":{"path":"."},"reason":"b"},
				{"id":"3","provider":"filesystem.list","args":{"path":"."},"reason":"c"},
				{"id":"4","provider":"filesystem.list","args":{"path":"."},"reason":"d"}
			]}`,
		},
	}

	engine := contextengine.NewEngine(contextengine.Registry{}, client, contextengine.Options{MaxRounds: 3, MaxRequestsPerRound: 3})
	_, err := engine.Run(stdcontext.Background(), protocol.SessionContext{SessionID: "sess-1", UserQuery: "too many", CWD: "."})
	if err == nil {
		t.Fatal("expected too many requests to fail")
	}
}

func TestEngineRejectsUnknownProvider(t *testing.T) {
	client := stubClient{
		responses: []string{
			`{"type":"context_request","requests":[{"id":"1","provider":"unknown","args":{},"reason":"inspect"}]}`,
		},
	}

	engine := contextengine.NewEngine(contextengine.Registry{}, client, contextengine.Options{MaxRounds: 3, MaxRequestsPerRound: 3})
	_, err := engine.Run(stdcontext.Background(), protocol.SessionContext{SessionID: "sess-1", UserQuery: "unknown", CWD: "."})
	if err == nil {
		t.Fatal("expected unknown provider to fail")
	}
}

func TestEngineRejectsProviderArgValidationErrors(t *testing.T) {
	registry := contextengine.Registry{}
	registry.Register("filesystem.list", contextengine.ProviderFunc(func(ctx stdcontext.Context, session protocol.SessionContext, raw json.RawMessage) (any, error) {
		var args struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, err
		}
		if args.Path == "" {
			return nil, errors.New("path required")
		}
		return map[string]any{"path": args.Path}, nil
	}))

	client := stubClient{
		responses: []string{
			`{"type":"context_request","requests":[{"id":"1","provider":"filesystem.list","args":{},"reason":"inspect"}]}`,
		},
	}

	engine := contextengine.NewEngine(registry, client, contextengine.Options{MaxRounds: 3, MaxRequestsPerRound: 3})
	_, err := engine.Run(stdcontext.Background(), protocol.SessionContext{SessionID: "sess-1", UserQuery: "bad args", CWD: "."})
	if err == nil {
		t.Fatal("expected validation failure")
	}
}

type stubClient struct {
	responses []string
	index     int
}

func (s stubClient) NextResponse(stdcontext.Context, protocol.SessionContext) ([]byte, error) {
	if s.index >= len(s.responses) {
		return nil, errors.New("no response configured")
	}
	out := s.responses[s.index]
	s.index++
	return []byte(out), nil
}
