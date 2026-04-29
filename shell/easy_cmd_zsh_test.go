package shell_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

// writeInstalledEasyCmdStub 在 $HOME/.local/bin/easy-cmd 写入可执行脚本，与桥接脚本固定解析路径一致。
func writeInstalledEasyCmdStub(t *testing.T, home string, script string) {
	t.Helper()
	bin := filepath.Join(home, ".local", "bin", "easy-cmd")
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
}

func TestEmbeddedShellScriptMatchesSourceBridgeScript(t *testing.T) {
	sourcePath := filepath.Join("..", "shell", "easy-cmd.zsh")
	embeddedPath := filepath.Join("..", "cmd", "easy-cmd", "assets", "easy-cmd.zsh")

	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("ReadFile source script failed: %v", err)
	}
	embedded, err := os.ReadFile(embeddedPath)
	if err != nil {
		t.Fatalf("ReadFile embedded script failed: %v", err)
	}

	if !bytes.Equal(source, embedded) {
		t.Fatalf("expected %s and %s to stay in sync", sourcePath, embeddedPath)
	}
}

func TestShellBridgeDoesNotExecuteOnCancelOrMalformedOutput(t *testing.T) {
	home := t.TempDir()
	marker := filepath.Join(home, "marker")
	script := "#!/bin/zsh\nexit 0\n"
	writeInstalledEasyCmdStub(t, home, script)

	cmd := exec.Command("zsh", "-lc", "source ./shell/easy-cmd.zsh; easy-cmd")
	cmd.Dir = filepath.Join("..")
	cmd.Env = append(os.Environ(), "HOME="+home, "MARKER="+marker)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("shell command failed: %v\n%s", err, out)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("expected cancel output not to execute command")
	}
}

func TestShellBridgePrintsSelectedCommandWithoutExecuting(t *testing.T) {
	home := t.TempDir()
	marker := filepath.Join(home, "marker")
	writeInstalledEasyCmdStub(t, home, "#!/bin/zsh\nprint 'touch "+marker+"'\n")

	cmd := exec.Command("zsh", "-lc", "source ./shell/easy-cmd.zsh; easy-cmd")
	cmd.Dir = filepath.Join("..")
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("shell command failed: %v\n%s", err, out)
	}
	if got := string(bytes.TrimSpace(out)); got != "touch "+marker {
		t.Fatalf("expected selected command on stdout, got %q", got)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("expected selected command not to execute")
	}
}

func TestShellWidgetRefillsBufferWithoutExecuting(t *testing.T) {
	home := t.TempDir()
	marker := filepath.Join(home, "marker")
	selectedCommand := "touch " + marker
	writeInstalledEasyCmdStub(t, home, "#!/bin/zsh\nprint '"+selectedCommand+"'\n")

	cmd := exec.Command("zsh", "-lc", "source ./shell/easy-cmd.zsh; function zle(){ :; }; BUFFER='list files'; easy-cmd-widget; print -r -- \"$BUFFER\"; print -r -- \"$CURSOR\"")
	cmd.Dir = filepath.Join("..")
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("shell command failed: %v\n%s", err, out)
	}

	lines := bytes.Split(bytes.TrimSpace(out), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("expected buffer and cursor output, got %q", out)
	}
	if got := string(lines[0]); got != selectedCommand {
		t.Fatalf("expected widget to refill buffer, got %q", got)
	}
	if got := string(lines[1]); got != strconv.Itoa(len(selectedCommand)) {
		t.Fatalf("expected widget to print cursor position, got %q", got)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("expected widget-selected command not to execute")
	}
}

func TestShellBridgePassesInitSubcommandThroughToBinary(t *testing.T) {
	home := t.TempDir()
	script := "#!/bin/zsh\nprint 'source /tmp/easy-cmd.zsh'\n"
	writeInstalledEasyCmdStub(t, home, script)

	cmd := exec.Command("zsh", "-lc", "source ./shell/easy-cmd.zsh; easy-cmd init")
	cmd.Dir = filepath.Join("..")
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("shell command failed: %v\n%s", err, out)
	}
	if got := string(bytes.TrimSpace(out)); got != "source /tmp/easy-cmd.zsh" {
		t.Fatalf("expected init output from binary, got %q", got)
	}
}

func TestShellBridgePassesQueryAsSeparateArgument(t *testing.T) {
	home := t.TempDir()
	argsFile := filepath.Join(home, "args.txt")
	script := "#!/bin/zsh\n: > " + argsFile + "\nfor arg in \"$@\"; do\n  print -r -- \"$arg\" >> " + argsFile + "\ndone\nexit 0\n"
	writeInstalledEasyCmdStub(t, home, script)

	cmd := exec.Command("zsh", "-lc", "source ./shell/easy-cmd.zsh; easy-cmd '列出当前目录下的文件'")
	cmd.Dir = filepath.Join("..")
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("shell command failed: %v\n%s", err, out)
	}

	rawArgs, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	lines := bytes.Split(bytes.TrimSpace(rawArgs), []byte("\n"))
	if len(lines) == 0 {
		t.Fatalf("expected forwarded args, got %q", rawArgs)
	}
	if !bytes.Equal(lines[0], []byte("pick")) {
		t.Fatalf("expected shell bridge to call pick subcommand, got %q", rawArgs)
	}
	if !bytes.Contains(rawArgs, []byte("--query")) {
		t.Fatalf("expected --query flag in args, got %q", rawArgs)
	}
	if !bytes.Contains(rawArgs, []byte("列出当前目录下的文件")) {
		t.Fatalf("expected query text as separate arg, got %q", rawArgs)
	}
	for _, line := range lines {
		if bytes.Equal(line, []byte(`--query "列出当前目录下的文件"`)) {
			t.Fatalf("expected query flag and value to be separate args, got %q", rawArgs)
		}
	}
}
