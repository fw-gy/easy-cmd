package protocol_test

import (
	"encoding/json"
	"testing"

	"easy-cmd/internal/protocol"
)

func TestParseEnvelopeRejectsNonJSON(t *testing.T) {
	_, err := protocol.ParseEnvelope([]byte("not-json"))
	if err == nil {
		t.Fatal("expected non-json input to fail")
	}
}

func TestParseEnvelopeRejectsUnknownType(t *testing.T) {
	_, err := protocol.ParseEnvelope([]byte(`{"type":"unknown"}`))
	if err == nil {
		t.Fatal("expected unknown type to fail")
	}
}

func TestParseEnvelopeParsesContextRequest(t *testing.T) {
	raw := []byte(`{
		"type":"context_request",
		"requests":[
			{"id":"req-1","provider":"filesystem.list","args":{"path":".","depth":1},"reason":"inspect"}
		]
	}`)

	got, err := protocol.ParseEnvelope(raw)
	if err != nil {
		t.Fatalf("ParseEnvelope failed: %v", err)
	}

	req, ok := got.(protocol.ContextRequestEnvelope)
	if !ok {
		t.Fatalf("expected ContextRequestEnvelope, got %T", got)
	}
	if len(req.Requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(req.Requests))
	}
	if req.Requests[0].Provider != "filesystem.list" {
		t.Fatalf("unexpected provider: %q", req.Requests[0].Provider)
	}
}

func TestParseEnvelopeRejectsLegacyCommandCandidates(t *testing.T) {
	_, err := protocol.ParseEnvelope([]byte(`{"type":"command_candidates","candidates":[{"command":"ls"}]}`))
	if err == nil {
		t.Fatal("expected legacy command_candidates envelope to fail")
	}
}

func TestAssistantTurnRoundTripJSON(t *testing.T) {
	in := protocol.AssistantTurnEnvelope{
		Type:    "assistant_turn",
		Message: "下面是几个可执行方案。",
		Candidates: []protocol.CommandCandidate{
			{
				Command:              "ls -la",
				Summary:              "List files",
				RiskLevel:            protocol.RiskLow,
				RequiresConfirmation: false,
			},
		},
	}

	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	got, err := protocol.ParseEnvelope(raw)
	if err != nil {
		t.Fatalf("ParseEnvelope failed: %v", err)
	}

	out, ok := got.(protocol.AssistantTurnEnvelope)
	if !ok {
		t.Fatalf("expected AssistantTurnEnvelope, got %T", got)
	}
	if out.Message == "" {
		t.Fatal("expected assistant message to be preserved")
	}
	if out.Candidates[0].RiskLevel != protocol.RiskLow {
		t.Fatalf("unexpected risk level: %q", out.Candidates[0].RiskLevel)
	}
}
