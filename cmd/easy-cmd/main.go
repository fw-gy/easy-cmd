package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"easy-cmd/internal/ai"
	"easy-cmd/internal/app"
	"easy-cmd/internal/config"
	contextengine "easy-cmd/internal/context"
	"easy-cmd/internal/protocol"
	"easy-cmd/internal/providers/filesystem"
	gitproviders "easy-cmd/internal/providers/git"
	"easy-cmd/internal/safety"
)

type loader struct {
	engine *contextengine.Engine
}

func (l loader) Load(ctx context.Context, session protocol.SessionContext) (contextengine.RunResult, error) {
	result, err := l.engine.Run(ctx, session)
	if err != nil {
		return contextengine.RunResult{}, err
	}
	result.Turn.Candidates = safety.ClassifyAll(result.Turn.Candidates)
	return result, nil
}

func main() {
	executablePath, err := os.Executable()
	if err != nil {
		fatalInit(err)
	}
	if handled, err := maybeHandleSubcommand(os.Args[1:], os.Stdout, executablePath); handled {
		if err != nil {
			fatalInit(err)
		}
		return
	}

	var cwd string
	var workspaceRoot string
	var query string
	var configPath string

	flag.StringVar(&cwd, "cwd", "", "current working directory")
	flag.StringVar(&workspaceRoot, "workspace-root", "", "workspace root")
	flag.StringVar(&query, "query", "", "initial query")
	flag.StringVar(&configPath, "config", defaultConfigPath(), "path to config.json")
	flag.Parse()

	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			fatalCancel(fmt.Errorf("get cwd: %w", err))
		}
	}
	if workspaceRoot == "" {
		workspaceRoot = cwd
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fatalCancel(err)
	}

	registry := filesystem.Register(contextengine.Registry{}, filesystem.Options{MaxReadBytes: 64 * 1024})
	registry = gitproviders.Register(registry)
	engine := contextengine.NewEngine(registry, ai.New(cfg), contextengine.Options{MaxRounds: 3, MaxRequestsPerRound: 3})

	model := app.New(app.Dependencies{
		Loader: loader{engine: engine},
		BaseSession: protocol.SessionContext{
			SessionID:     newSessionID(),
			CWD:           cwd,
			WorkspaceRoot: workspaceRoot,
		},
		InitialQuery: query,
		Language:     cfg.Language,
	})

	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithOutput(os.Stderr))
	finalModel, err := program.Run()
	if err != nil {
		fatalCancel(err)
	}

	typed, ok := finalModel.(app.Model)
	if !ok {
		fatalCancel(fmt.Errorf("unexpected final model type %T", finalModel))
	}

	if err := json.NewEncoder(os.Stdout).Encode(typed.Output()); err != nil {
		fatalCancel(err)
	}
}

func maybeHandleSubcommand(args []string, stdout io.Writer, executablePath string) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}
	if args[0] != "init" {
		return false, nil
	}
	if len(args) != 2 {
		return true, errors.New("usage: easy-cmd init zsh")
	}

	switch args[1] {
	case "zsh":
		script, err := zshInitScript(executablePath)
		if err != nil {
			return true, err
		}
		_, err = io.WriteString(stdout, script)
		return true, err
	default:
		return true, fmt.Errorf("unsupported shell %q", args[1])
	}
}

func zshInitScript(executablePath string) (string, error) {
	shellScriptPath, err := findShellIntegrationScript(executablePath)
	if err != nil {
		return "", err
	}

	lines := []string{
		"export EASY_CMD_BIN=" + shellQuote(executablePath),
		"source " + shellQuote(shellScriptPath),
		"bindkey '^G' easy-cmd-widget",
	}
	return strings.Join(lines, "\n") + "\n", nil
}

func findShellIntegrationScript(executablePath string) (string, error) {
	execDir := filepath.Dir(filepath.Clean(executablePath))
	candidates := []string{
		filepath.Clean(filepath.Join(execDir, "..", "share", "easy-cmd", "easy-cmd.zsh")),
		filepath.Clean(filepath.Join(execDir, "..", "shell", "easy-cmd.zsh")),
	}

	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("shell integration script not found for %q", executablePath)
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func fatalCancel(err error) {
	fmt.Fprintln(os.Stderr, "easy-cmd:", err)
	_ = json.NewEncoder(os.Stdout).Encode(protocol.AppOutput{Action: protocol.ActionCancel})
	os.Exit(1)
}

func fatalInit(err error) {
	fmt.Fprintln(os.Stderr, "easy-cmd:", err)
	os.Exit(1)
}

func defaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		return filepath.Join(home, ".easy-cmd", "config.json")
	}
	currentUser, err := user.Current()
	if err == nil && currentUser.HomeDir != "" {
		return filepath.Join(currentUser.HomeDir, ".easy-cmd", "config.json")
	}
	return ".easy-cmd/config.json"
}

func newSessionID() string {
	return fmt.Sprintf("sess-%d", time.Now().UnixNano())
}
