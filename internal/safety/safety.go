package safety

import (
	"regexp"
	"strings"

	"easy-cmd/internal/protocol"
)

var (
	highPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(^|\s)sudo(\s|$)`),
		regexp.MustCompile(`(^|\s)rm(\s|$)`),
		regexp.MustCompile(`git\s+reset`),
		regexp.MustCompile(`git\s+clean`),
		regexp.MustCompile(`(^|\s)(>|>>)(\s|$)`),
		regexp.MustCompile(`(^|\s)mv\s+.*\s+-f(\s|$)`),
		regexp.MustCompile(`(^|\s)cp\s+.*\s+-f(\s|$)`),
	}
	mediumPatterns = []*regexp.Regexp{
		regexp.MustCompile(`&&|\|\||\|`),
		regexp.MustCompile(`(^|\s)(mv|cp)(\s|$)`),
	}
)

func Classify(candidate protocol.CommandCandidate) protocol.CommandCandidate {
	command := strings.TrimSpace(candidate.Command)
	switch {
	case matchesAny(command, highPatterns):
		candidate.RiskLevel = protocol.RiskHigh
		candidate.RequiresConfirmation = true
	case matchesAny(command, mediumPatterns):
		candidate.RiskLevel = protocol.RiskMedium
	case looksReadOnly(command):
		candidate.RiskLevel = protocol.RiskLow
	default:
		candidate.RiskLevel = protocol.RiskMedium
	}
	return candidate
}

func ClassifyAll(candidates []protocol.CommandCandidate) []protocol.CommandCandidate {
	out := make([]protocol.CommandCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		out = append(out, Classify(candidate))
	}
	return out
}

func matchesAny(command string, patterns []*regexp.Regexp) bool {
	for _, pattern := range patterns {
		if pattern.MatchString(command) {
			return true
		}
	}
	return false
}

func looksReadOnly(command string) bool {
	for _, prefix := range []string{"ls", "cat", "find", "rg", "grep", "git status", "git branch", "pwd"} {
		if strings.HasPrefix(command, prefix) {
			return true
		}
	}
	return false
}
