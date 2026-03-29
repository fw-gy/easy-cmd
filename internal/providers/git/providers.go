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
