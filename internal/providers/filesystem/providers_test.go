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

func TestSearchSkipsIgnoredAndOversizedFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "node_modules"), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "match.txt"), []byte("needle here\n"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "ignored.txt"), []byte("needle in git dir\n"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "node_modules", "ignored.txt"), []byte("needle in node_modules\n"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "large.txt"), []byte("needle-1234567890-too-long"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "binary.bin"), []byte{0x00, 0x01, 0x02, 0x03}, 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	registry := filesystem.Register(contextengine.Registry{}, filesystem.Options{MaxReadBytes: 16})
	session := protocol.SessionContext{CWD: root, WorkspaceRoot: root}

	raw, _ := json.Marshal(map[string]any{"path": ".", "pattern": "needle", "max_results": 10})
	got, err := registry.Run(context.Background(), "filesystem.search", session, raw)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	data := got.(map[string]any)
	matches := data["matches"].([]map[string]any)
	if len(matches) != 1 {
		t.Fatalf("expected only one searchable match, got %#v", matches)
	}
	if matches[0]["path"] != "match.txt" {
		t.Fatalf("expected match from root text file, got %#v", matches[0])
	}
}
