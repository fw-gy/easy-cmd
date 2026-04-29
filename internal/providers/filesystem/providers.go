package filesystem

import (
	"bufio"
	stdcontext "context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

type searchProvider struct {
	maxReadBytes int
}

type statProvider struct{}

var errSearchLimitReached = errors.New("search limit reached")

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

// Register 注册模型可调用的只读文件系统工具。
// 每个 provider 都会基于当前工作区根目录来解析路径。
func Register(registry contextengine.Registry, options Options) contextengine.Registry {
	readMax := options.MaxReadBytes
	if readMax <= 0 {
		readMax = defaultMaxReadBytes
	}

	registry.Register("filesystem.list", listProvider{})
	registry.Register("filesystem.read_file", NewReadFileProvider(readMax))
	registry.Register("filesystem.search", searchProvider{maxReadBytes: readMax})
	registry.Register("path.stat", statProvider{})
	return registry
}

func NewReadFileProvider(maxReadBytes int) ReadFileProvider {
	if maxReadBytes <= 0 {
		maxReadBytes = defaultMaxReadBytes
	}
	return ReadFileProvider{maxReadBytes: maxReadBytes}
}

// listProvider 返回一个较浅的目录树，方便模型先了解项目结构，
// 再决定接下来要读哪些文件。
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
			// 一旦超过指定深度就停止继续向下遍历，避免大目录树占满上下文预算。
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

// ReadFileProvider 返回 UTF-8 文本文件内容，并带有大小上限。
// 这个上限是为了避免模型误把整个大文件塞进上下文。
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

// searchProvider 会在指定路径下的文本文件里做简单子串搜索，并返回
// 匹配行以及从 1 开始的行号。
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
		if d.IsDir() && shouldIgnoreDir(d.Name()) {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		stat, err := os.Stat(path)
		if err != nil || stat.Size() > int64(p.maxReadBytes) {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()

		sample := make([]byte, min(4096, p.maxReadBytes))
		n, readErr := file.Read(sample)
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return nil
		}
		if isBinary(sample[:n]) {
			return nil
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return nil
		}

		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 0, 4096), p.maxReadBytes)
		lineNumber := 0
		for scanner.Scan() {
			lineNumber++
			line := scanner.Text()
			if !strings.Contains(line, args.Pattern) {
				continue
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			matches = append(matches, map[string]any{
				"path":        rel,
				"line_number": lineNumber,
				"line":        line,
			})
			if len(matches) >= args.MaxResults {
				return errSearchLimitReached
			}
		}
		if err := scanner.Err(); err != nil {
			return nil
		}
		return nil
	})
	if err != nil && !errors.Is(err, errSearchLimitReached) {
		return nil, err
	}

	return map[string]any{
		"path":    args.Path,
		"pattern": args.Pattern,
		"matches": matches,
	}, nil
}

// statProvider 只返回文件元数据而不读取内容，方便模型低成本查看
// 大小、修改时间等信息。
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

// resolvePath 会把用户或模型提供的路径转成绝对路径，并强制要求它
// 仍然位于工作区根目录以内。
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
	// 同时拒绝 `..` 和 `../...` 形式的越界路径，确保 provider 即使收到
	// 绝对路径输入，也依然只能在工作区内部做只读访问。
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

func shouldIgnoreDir(name string) bool {
	return name == ".git" || name == "node_modules"
}

// isBinary 使用一个轻量启发式判断二进制文件：如果内容不是合法 UTF-8，
// 或包含 NUL 字节，通常就不适合当作源码文本发给模型。
func isBinary(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	if !utf8.Valid(data) {
		return true
	}
	return strings.IndexByte(string(data), 0) >= 0
}
