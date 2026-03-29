package filesystem

import (
	stdcontext "context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	contextengine "easy-cmd/internal/context"
	"easy-cmd/internal/protocol"
)

const defaultMaxReadBytes = 64 * 1024

type Options struct {
	MaxReadBytes int
}

type listProvider struct{}

type ReadFileProvider struct {
	maxReadBytes int
}

type searchProvider struct{}

type statProvider struct{}

type listArgs struct {
	Path  string `json:"path"`
	Depth int    `json:"depth"`
}

type readFileArgs struct {
	Path string `json:"path"`
}

type searchArgs struct {
	Path       string `json:"path"`
	Pattern    string `json:"pattern"`
	MaxResults int    `json:"max_results"`
}

type statArgs struct {
	Path string `json:"path"`
}

func Register(registry contextengine.Registry, options Options) contextengine.Registry {
	readMax := options.MaxReadBytes
	if readMax <= 0 {
		readMax = defaultMaxReadBytes
	}

	registry.Register("filesystem.list", listProvider{})
	registry.Register("filesystem.read_file", NewReadFileProvider(readMax))
	registry.Register("filesystem.search", searchProvider{})
	registry.Register("path.stat", statProvider{})
	return registry
}

func NewReadFileProvider(maxReadBytes int) ReadFileProvider {
	if maxReadBytes <= 0 {
		maxReadBytes = defaultMaxReadBytes
	}
	return ReadFileProvider{maxReadBytes: maxReadBytes}
}

func (p listProvider) Run(ctx stdcontext.Context, session protocol.SessionContext, raw json.RawMessage) (any, error) {
	var args listArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if args.Path == "" {
		return nil, errors.New("path is required")
	}
	if args.Depth <= 0 {
		args.Depth = 1
	}

	resolved, root, err := resolvePath(session, args.Path)
	if err != nil {
		return nil, err
	}

	entries := make([]map[string]any, 0)
	err = filepath.WalkDir(resolved, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		rel, err := filepath.Rel(resolved, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if depth(rel) > args.Depth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		workspaceRel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		entries = append(entries, map[string]any{
			"name":          d.Name(),
			"type":          entryType(d),
			"relative_path": workspaceRel,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"path":    args.Path,
		"entries": entries,
	}, nil
}

func (p ReadFileProvider) Run(_ stdcontext.Context, session protocol.SessionContext, raw json.RawMessage) (any, error) {
	var args readFileArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if args.Path == "" {
		return nil, errors.New("path is required")
	}

	resolved, _, err := resolvePath(session, args.Path)
	if err != nil {
		return nil, err
	}

	stat, err := os.Stat(resolved)
	if err != nil {
		return nil, err
	}
	if stat.Size() > int64(p.maxReadBytes) {
		return nil, fmt.Errorf("file exceeds max read size of %d bytes", p.maxReadBytes)
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, err
	}
	if isBinary(data) {
		return nil, errors.New("binary files are not supported")
	}

	return map[string]any{
		"path":     args.Path,
		"contents": string(data),
	}, nil
}

func (p searchProvider) Run(ctx stdcontext.Context, session protocol.SessionContext, raw json.RawMessage) (any, error) {
	var args searchArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if args.Path == "" {
		return nil, errors.New("path is required")
	}
	if args.Pattern == "" {
		return nil, errors.New("pattern is required")
	}
	if args.MaxResults <= 0 {
		args.MaxResults = 20
	}

	resolved, root, err := resolvePath(session, args.Path)
	if err != nil {
		return nil, err
	}

	matches := make([]map[string]any, 0)
	err = filepath.WalkDir(resolved, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		data, err := os.ReadFile(path)
		if err != nil || isBinary(data) {
			return nil
		}
		lines := strings.Split(string(data), "\n")
		for idx, line := range lines {
			if !strings.Contains(line, args.Pattern) {
				continue
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			matches = append(matches, map[string]any{
				"path":        rel,
				"line_number": idx + 1,
				"line":        line,
			})
			if len(matches) >= args.MaxResults {
				return errors.New("search limit reached")
			}
		}
		return nil
	})
	if err != nil && err.Error() != "search limit reached" {
		return nil, err
	}

	return map[string]any{
		"path":    args.Path,
		"pattern": args.Pattern,
		"matches": matches,
	}, nil
}

func (p statProvider) Run(_ stdcontext.Context, session protocol.SessionContext, raw json.RawMessage) (any, error) {
	var args statArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if args.Path == "" {
		return nil, errors.New("path is required")
	}

	resolved, _, err := resolvePath(session, args.Path)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"path":     args.Path,
		"size":     info.Size(),
		"is_dir":   info.IsDir(),
		"mode":     info.Mode().String(),
		"mod_time": info.ModTime().Format(time.RFC3339),
	}, nil
}

func resolvePath(session protocol.SessionContext, requested string) (string, string, error) {
	root := session.WorkspaceRoot
	if root == "" {
		root = session.CWD
	}
	if root == "" {
		return "", "", errors.New("workspace root is required")
	}

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", "", err
	}

	var target string
	if filepath.IsAbs(requested) {
		target = requested
	} else {
		target = filepath.Join(session.CWD, requested)
	}

	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", "", err
	}
	rel, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil {
		return "", "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", "", fmt.Errorf("path %q escapes workspace root", requested)
	}
	return targetAbs, rootAbs, nil
}

func depth(rel string) int {
	return strings.Count(rel, string(os.PathSeparator)) + 1
}

func entryType(entry fs.DirEntry) string {
	if entry.IsDir() {
		return "dir"
	}
	return "file"
}

func isBinary(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	if !utf8.Valid(data) {
		return true
	}
	return strings.IndexByte(string(data), 0) >= 0
}
