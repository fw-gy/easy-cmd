package filesystem_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	contextengine "easy-cmd/internal/context"
	"easy-cmd/internal/protocol"
	"easy-cmd/internal/providers/filesystem"
)

func TestReadFileRejectsBinaryAndOversizedFiles(t *testing.T) {
	root := t.TempDir()
	binaryPath := filepath.Join(root, "data.bin")
	if err := os.WriteFile(binaryPath, []byte{0x00, 0x01, 0x02}, 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	provider := filesystem.NewReadFileProvider(4)
	session := protocol.SessionContext{CWD: root, WorkspaceRoot: root}

	args, _ := json.Marshal(map[string]any{"path": "data.bin"})
	if _, err := provider.Run(context.Background(), session, args); err == nil {
		t.Fatal("expected binary file to be rejected")
	}

	largePath := filepath.Join(root, "large.txt")
	if err := os.WriteFile(largePath, []byte("12345"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	args, _ = json.Marshal(map[string]any{"path": "large.txt"})
	if _, err := provider.Run(context.Background(), session, args); err == nil {
		t.Fatal("expected oversized file to be rejected")
	}
}

func TestPathGuardsRejectTraversalOutsideWorkspace(t *testing.T) {
	root := t.TempDir()
	registry := filesystem.Register(contextengine.Registry{}, filesystem.Options{MaxReadBytes: 1024})
	session := protocol.SessionContext{CWD: root, WorkspaceRoot: root}

	raw, _ := json.Marshal(map[string]any{"path": "../secret.txt", "depth": 1})
	_, err := registry.Run(context.Background(), "filesystem.list", session, raw)
	if err == nil {
		t.Fatal("expected traversal outside workspace to fail")
	}
}
