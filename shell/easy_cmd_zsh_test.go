package shell_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"easy-cmd/internal/protocol"
)

func TestShellBridgeDoesNotExecuteOnCancelOrMalformedOutput(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker")
	binary := filepath.Join(dir, "stub.sh")
	script := "#!/bin/zsh\nprint '{\"action\":\"cancel\"}'\n"
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cmd := exec.Command("zsh", "-lc", "source ./shell/easy-cmd.zsh; EASY_CMD_BIN="+binary+"; easy-cmd")
	cmd.Dir = filepath.Join("..")
	cmd.Env = append(os.Environ(), "MARKER="+marker)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("shell command failed: %v\n%s", err, out)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("expected cancel output not to execute command")
	}
}

func TestShellBridgePrintsSelectedCommandWithoutExecuting(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker")
	payload, _ := json.Marshal(protocol.AppOutput{
		Action:          protocol.ActionExecute,
		SelectedCommand: "touch " + marker,
	})

	binary := filepath.Join(dir, "stub.sh")
	var buf bytes.Buffer
	buf.WriteString("#!/bin/zsh\nprint '")
	buf.Write(payload)
	buf.WriteString("'\n")
	if err := os.WriteFile(binary, buf.Bytes(), 0o755); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cmd := exec.Command("zsh", "-lc", "source ./shell/easy-cmd.zsh; EASY_CMD_BIN="+binary+"; easy-cmd")
	cmd.Dir = filepath.Join("..")
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
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker")
	selectedCommand := "touch " + marker
	payload, _ := json.Marshal(protocol.AppOutput{
		Action:          protocol.ActionExecute,
		SelectedCommand: selectedCommand,
	})

	binary := filepath.Join(dir, "stub.sh")
	var buf bytes.Buffer
	buf.WriteString("#!/bin/zsh\nprint '")
	buf.Write(payload)
	buf.WriteString("'\n")
	if err := os.WriteFile(binary, buf.Bytes(), 0o755); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cmd := exec.Command("zsh", "-lc", "source ./shell/easy-cmd.zsh; function zle(){ :; }; EASY_CMD_BIN="+binary+"; BUFFER='list files'; easy-cmd-widget; print -r -- \"$BUFFER\"; print -r -- \"$CURSOR\"")
	cmd.Dir = filepath.Join("..")
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
	dir := t.TempDir()
	binary := filepath.Join(dir, "stub.sh")
	script := "#!/bin/zsh\nprint 'source /tmp/easy-cmd.zsh'\n"
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cmd := exec.Command("zsh", "-lc", "source ./shell/easy-cmd.zsh; EASY_CMD_BIN="+binary+"; easy-cmd init zsh")
	cmd.Dir = filepath.Join("..")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("shell command failed: %v\n%s", err, out)
	}
	if got := string(bytes.TrimSpace(out)); got != "source /tmp/easy-cmd.zsh" {
		t.Fatalf("expected init output from binary, got %q", got)
	}
}

func TestShellBridgePassesQueryAsSeparateArgument(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	binary := filepath.Join(dir, "stub.sh")
	script := "#!/bin/zsh\n: > " + argsFile + "\nfor arg in \"$@\"; do\n  print -r -- \"$arg\" >> " + argsFile + "\ndone\nprint '{\"action\":\"cancel\"}'\n"
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cmd := exec.Command("zsh", "-lc", "source ./shell/easy-cmd.zsh; EASY_CMD_BIN="+binary+"; easy-cmd '列出当前目录下的文件'")
	cmd.Dir = filepath.Join("..")
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
