package safety_test

import (
	"testing"

	"easy-cmd/internal/protocol"
	"easy-cmd/internal/safety"
)

func TestClassifyMarksDestructiveCommandsHighRisk(t *testing.T) {
	candidate := protocol.CommandCandidate{Command: "rm -rf build"}
	got := safety.Classify(candidate)
	if got.RiskLevel != protocol.RiskHigh {
		t.Fatalf("expected high risk, got %q", got.RiskLevel)
	}
	if !got.RequiresConfirmation {
		t.Fatal("expected high risk command to require confirmation")
	}
}

func TestClassifyMarksReadCommandsLowRisk(t *testing.T) {
	candidate := protocol.CommandCandidate{Command: "ls -la"}
	got := safety.Classify(candidate)
	if got.RiskLevel != protocol.RiskLow {
		t.Fatalf("expected low risk, got %q", got.RiskLevel)
	}
}
