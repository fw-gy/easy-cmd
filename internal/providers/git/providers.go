package git

import (
	stdcontext "context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	contextengine "easy-cmd/internal/context"
	"easy-cmd/internal/protocol"
)

type branchProvider struct{}
type statusProvider struct{}

// Register 只向模型暴露很小的一部分 git 信息：当前分支和 porcelain
// 状态，两者都限制在当前工作区根目录下。
func Register(registry contextengine.Registry) contextengine.Registry {
	registry.Register("git.branch", branchProvider{})
	registry.Register("git.status", statusProvider{})
	return registry
}

func (branchProvider) Run(ctx stdcontext.Context, session protocol.SessionContext, raw json.RawMessage) (any, error) {
	if len(raw) > 0 && string(raw) != "{}" {
		return nil, errors.New("git.branch does not accept args")
	}
	root := session.WorkspaceRoot
	if root == "" {
		root = session.CWD
	}

	// `git branch --show-current` 对大多数依赖分支信息的场景已经足够，
	// 同时默认不暴露提交历史或 diff 内容。
	out, err := exec.CommandContext(ctx, "git", "-C", root, "branch", "--show-current").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git branch: %w", err)
	}
	return map[string]any{"branch": strings.TrimSpace(string(out))}, nil
}

func (statusProvider) Run(ctx stdcontext.Context, session protocol.SessionContext, raw json.RawMessage) (any, error) {
	if len(raw) > 0 && string(raw) != "{}" {
		return nil, errors.New("git.status does not accept args")
	}
	root := session.WorkspaceRoot
	if root == "" {
		root = session.CWD
	}

	out, err := exec.CommandContext(ctx, "git", "-C", root, "status", "--short", "--branch").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git status: %w", err)
	}

	// porcelain 输出的第一行是分支元信息，后面的每一行才是文件状态，
	// 用来描述修改、新增或未跟踪文件。
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	result := map[string]any{
		"branch":  "",
		"changes": []string{},
	}
	if len(lines) > 0 {
		result["branch"] = strings.TrimPrefix(lines[0], "## ")
	}
	if len(lines) > 1 {
		result["changes"] = lines[1:]
	}
	return result, nil
}
