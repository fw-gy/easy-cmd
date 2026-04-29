package cliapp

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"easy-cmd/internal/app"
	"easy-cmd/internal/config"
	"easy-cmd/internal/core"
	"easy-cmd/internal/onboarding"
	"easy-cmd/internal/protocol"
	"easy-cmd/internal/service"
)

var EmbeddedZshScript []byte

// HandleCommandDirectly 判断本次是否直接在命令行处理
// 如果直接在命令行处理，那么就执行对应的操作并返回 true
// 如果不需要直接在命令行处理，那么就返回 false
func HandleCommandDirectly(args []string, stdout io.Writer, executablePath string) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}
	switch args[0] {
	case "init":
		if len(args) != 1 {
			return true, errors.New("usage: easy-cmd init")
		}
		return true, runInit(executablePath)
	case "run":
		return true, runCommand(context.Background(), stdout, args[1:])
	case "pick":
		return true, pickCommand(context.Background(), stdout, args[1:])
	default:
		return false, nil
	}
}

func RunInteractiveCLI(args []string, stdout io.Writer) error {
	request, err := parseFlags("interactive", args)
	if err != nil {
		return err
	}
	cfg, err := loadOrInitConfig()
	if err != nil {
		return err
	}
	output, err := runInteractiveSession(cfg, request)
	if err != nil {
		return err
	}
	return json.NewEncoder(stdout).Encode(output)
}

// runInit 安装可执行文件、zsh 集成脚本与配置文件到用户目录
func runInit(executablePath string) error {
	binPath, scriptPath, err := getInstallationPaths()
	if err != nil {
		return err
	}
	if err := installExecutable(executablePath, binPath); err != nil {
		return err
	}
	if err := writeFile(scriptPath, EmbeddedZshScript, 0o644); err != nil {
		return fmt.Errorf("write zsh script: %w", err)
	}
	if _, err := loadOrInitConfig(); err != nil {
		return fmt.Errorf("configure: %w", err)
	}
	return nil
}

func runCommand(ctx context.Context, stdout io.Writer, args []string) error {
	request, err := parseFlags("run", args)
	if err != nil {
		return err
	}
	if strings.TrimSpace(request.Query) == "" {
		return errors.New("run requires --query")
	}
	cfg, err := loadOrInitConfig()
	if err != nil {
		return err
	}
	result, err := service.NewDefault(cfg).Run(ctx, request)
	if err != nil {
		return err
	}
	return json.NewEncoder(stdout).Encode(result)
}

func pickCommand(_ context.Context, stdout io.Writer, args []string) error {
	request, err := parseFlags("pick", args)
	if err != nil {
		return err
	}
	cfg, err := loadOrInitConfig()
	if err != nil {
		return err
	}
	appOutput, err := runInteractiveSession(cfg, request)
	if err != nil {
		return err
	}
	if appOutput.Action != protocol.ActionExecute || strings.TrimSpace(appOutput.SelectedCommand) == "" {
		return nil
	}
	_, err = fmt.Fprintln(stdout, appOutput.SelectedCommand)
	return err
}

func parseFlags(name string, args []string) (core.Request, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var request core.Request

	fs.StringVar(&request.CWD, "cwd", "", "current working directory")
	fs.StringVar(&request.WorkspaceRoot, "workspace-root", "", "workspace root")
	fs.StringVar(&request.Query, "query", "", "query")
	if err := fs.Parse(args); err != nil {
		return core.Request{}, err
	}
	return completeRequestPaths(request)
}

func loadOrInitConfig() (config.Config, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return onboarding.InitConfig(os.Stderr)
	}
	return cfg, nil
}

func completeRequestPaths(request core.Request) (core.Request, error) {
	if request.CWD == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return core.Request{}, fmt.Errorf("get cwd: %w", err)
		}
		request.CWD = cwd
	}
	if request.WorkspaceRoot == "" {
		request.WorkspaceRoot = request.CWD
	}
	return request, nil
}

func runInteractiveSession(cfg config.Config, request core.Request) (protocol.AppOutput, error) {
	model := app.New(app.Dependencies{
		Runner:       service.NewDefault(cfg),
		BaseRequest:  core.Request{CWD: request.CWD, WorkspaceRoot: request.WorkspaceRoot},
		InitialQuery: request.Query,
		Language:     cfg.Language,
	})
	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithOutput(os.Stderr))
	finalModel, err := program.Run()
	if err != nil {
		return protocol.AppOutput{}, err
	}

	typed, ok := finalModel.(app.Model)
	if !ok {
		return protocol.AppOutput{}, fmt.Errorf("unexpected final model type %T", finalModel)
	}
	return typed.Output(), nil
}

// getInstallationPaths 返回可执行文件和 zsh 集成脚本的路径
func getInstallationPaths() (string, string, error) {
	homePath, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("resolve home directory failed: %w", err)
	}
	return filepath.Join(homePath, ".local", "bin", "easy-cmd"), filepath.Join(homePath, ".easy-cmd", "script.zsh"), nil
}

func installExecutable(sourcePath string, targetPath string) error {
	if filepath.Clean(sourcePath) == filepath.Clean(targetPath) {
		return nil
	}

	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("read current executable: %w", err)
	}
	defer source.Close()

	info, err := source.Stat()
	if err != nil {
		return fmt.Errorf("stat current executable: %w", err)
	}
	mode := info.Mode().Perm()
	if mode == 0 {
		mode = 0o755
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("install binary: %w", err)
	}

	tempFile, err := os.CreateTemp(filepath.Dir(targetPath), ".easy-cmd-*")
	if err != nil {
		return fmt.Errorf("install binary: %w", err)
	}
	tempPath := tempFile.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()

	if _, err := io.Copy(tempFile, source); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("install binary: %w", err)
	}
	if err := tempFile.Chmod(mode); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("install binary: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("install binary: %w", err)
	}
	if err := os.Rename(tempPath, targetPath); err != nil {
		return fmt.Errorf("install binary: %w", err)
	}
	return nil
}

func writeFile(path string, contents []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, contents, mode); err != nil {
		return err
	}
	return nil
}
