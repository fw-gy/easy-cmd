package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"easy-cmd/internal/cliapp"
)

func TestMaybeHandleSubcommandInitCopiesBinaryWritesEnvAndPreservesConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	root := t.TempDir()
	execPath := filepath.Join(root, "easy-cmd")
	if err := os.WriteFile(execPath, []byte("binary"), 0o755); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	configPath := filepath.Join(home, ".easy-cmd", "config.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	existingConfig := `{"base_url":"https://example.com","api_key":"secret","model":"custom","language":"en-US"}`
	if err := os.WriteFile(configPath, []byte(existingConfig), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	var out bytes.Buffer
	cliapp.EmbeddedZshScript = embeddedZshScript
	handled, err := cliapp.HandleCommandDirectly([]string{"init"}, &out, execPath)
	if err != nil {
		t.Fatalf("handlerCommandDirectly failed: %v", err)
	}
	if !handled {
		t.Fatal("expected init to be handled")
	}
	if out.Len() != 0 {
		t.Fatalf("expected init to write nothing to stdout, got %q", out.String())
	}

	installedBin := filepath.Join(home, ".local", "bin", "easy-cmd")
	gotBin, err := os.ReadFile(installedBin)
	if err != nil {
		t.Fatalf("ReadFile installed bin failed: %v", err)
	}
	if string(gotBin) != "binary" {
		t.Fatalf("unexpected installed binary contents %q", string(gotBin))
	}

	shareScript := filepath.Join(home, ".local", "share", "easy-cmd", "easy-cmd.zsh")
	if _, err := os.Stat(shareScript); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected init not to install separate share script, got err=%v", err)
	}

	scriptPath := filepath.Join(home, ".easy-cmd", "script.zsh")
	scriptBody, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("ReadFile script.zsh failed: %v", err)
	}
	scriptStr := string(scriptBody)
	if !strings.Contains(scriptStr, "easy-cmd-widget") {
		t.Fatalf("expected bridge script in script.zsh, got %q", scriptStr)
	}
	if !strings.Contains(scriptStr, "zle -N easy-cmd-widget") {
		t.Fatalf("expected zle widget registration in script.zsh, got %q", scriptStr)
	}

	gotConfig, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(gotConfig) != existingConfig {
		t.Fatalf("expected config to be preserved, got %q", string(gotConfig))
	}
}

func TestMaybeHandleSubcommandInitRejectsExtraArgs(t *testing.T) {
	var out bytes.Buffer
	handled, err := cliapp.HandleCommandDirectly([]string{"init", "zsh"}, &out, "/tmp/easy-cmd")
	if !handled {
		t.Fatal("expected init subcommand to be handled")
	}
	if err == nil {
		t.Fatal("expected error for obsolete init zsh")
	}
	if !strings.Contains(err.Error(), "usage") {
		t.Fatalf("expected usage error, got %v", err)
	}
}

func TestHandlerCommandDirectlyDoesNotHandleInteractiveModeWithoutArgs(t *testing.T) {
	var out bytes.Buffer
	handled, err := cliapp.HandleCommandDirectly(nil, &out, "/tmp/easy-cmd")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if handled {
		t.Fatal("expected interactive mode to fall through when no subcommand is provided")
	}
}
