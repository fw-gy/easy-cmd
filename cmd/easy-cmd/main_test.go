package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMaybeHandleSubcommandInitZshForHomebrewLayout(t *testing.T) {
	root := t.TempDir()
	execPath := filepath.Join(root, "bin", "easy-cmd")
	scriptPath := filepath.Join(root, "share", "easy-cmd", "easy-cmd.zsh")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(scriptPath, []byte("# shell integration"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	var out bytes.Buffer
	handled, err := maybeHandleSubcommand([]string{"init", "zsh"}, &out, execPath)
	if err != nil {
		t.Fatalf("maybeHandleSubcommand failed: %v", err)
	}
	if !handled {
		t.Fatal("expected init zsh to be handled")
	}

	output := out.String()
	if !strings.Contains(output, "export EASY_CMD_BIN='"+execPath+"'") {
		t.Fatalf("expected EASY_CMD_BIN export in output, got %q", output)
	}
	if !strings.Contains(output, "source '"+scriptPath+"'") {
		t.Fatalf("expected script source in output, got %q", output)
	}
	if !strings.Contains(output, "bindkey '^G' easy-cmd-widget") {
		t.Fatalf("expected bindkey in output, got %q", output)
	}
}

func TestMaybeHandleSubcommandInitZshFallsBackToRepoShellScript(t *testing.T) {
	root := t.TempDir()
	execPath := filepath.Join(root, "tmp", "easy-cmd")
	scriptPath := filepath.Join(root, "shell", "easy-cmd.zsh")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(scriptPath, []byte("# shell integration"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	var out bytes.Buffer
	handled, err := maybeHandleSubcommand([]string{"init", "zsh"}, &out, execPath)
	if err != nil {
		t.Fatalf("maybeHandleSubcommand failed: %v", err)
	}
	if !handled {
		t.Fatal("expected init zsh to be handled")
	}

	if !strings.Contains(out.String(), "source '"+scriptPath+"'") {
		t.Fatalf("expected fallback shell script source in output, got %q", out.String())
	}
}
