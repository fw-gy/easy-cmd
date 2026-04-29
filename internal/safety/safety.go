package safety

import (
	"regexp"
	"strings"

	"easy-cmd/internal/protocol"
)

var (
	// 高风险模式对应可能删除数据、覆盖文件或需要更高权限的命令。
	// 这类命令一律要求显式确认。
	highPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(^|\s)sudo(\s|$)`),
		regexp.MustCompile(`(^|\s)rm(\s|$)`),
		regexp.MustCompile(`git\s+reset`),
		regexp.MustCompile(`git\s+clean`),
		regexp.MustCompile(`(^|\s)(>|>>)(\s|$)`),
		regexp.MustCompile(`(^|\s)mv\s+.*\s+-f(\s|$)`),
		regexp.MustCompile(`(^|\s)cp\s+.*\s+-f(\s|$)`),
	}
	// 中风险模式对应会修改状态、或把多条命令串起来执行的情况，
	// 用户通常没法一眼看清它的影响。
	mediumPatterns = []*regexp.Regexp{
		regexp.MustCompile(`&&|\|\||\|`),
		regexp.MustCompile(`(^|\s)(mv|cp)(\s|$)`),
	}
)

// Classify 会给模型生成的命令补一个本地安全标签。
// 这样即使模型低估了风险，最终风险判断也仍然是确定的。
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

// looksReadOnly 的策略是故意保守的：只有很小的一组白名单会被判成
// “低风险”，其他命令除非命中更危险的规则，否则都回落到中风险。
func looksReadOnly(command string) bool {
	for _, prefix := range []string{"ls", "cat", "find", "rg", "grep", "git status", "git branch", "pwd"} {
		if strings.HasPrefix(command, prefix) {
			return true
		}
	}
	return false
}
