# Repository Guidelines

## Project Structure & Module Organization
`cmd/easy-cmd/` contains the CLI entrypoint and app wiring. Core logic lives under `internal/`: `ai/` for chat completion calls, `app/` for the Bubble Tea TUI model, `context/` for provider orchestration, `protocol/` for JSON envelopes, `providers/` for read-only filesystem and git context, `config/` for config loading, and `safety/` for command risk checks. `shell/easy-cmd.zsh` is the zsh bridge, `examples/config.json` shows config shape, and `tmp/` is the local build output directory.

## Build, Test, and Development Commands
Use `make build` to compile the binary into `tmp/easy-cmd`. Use `make test` to run the full test suite with `go test ./...`. For local installation, `go build -o ~/.local/bin/easy-cmd ./cmd/easy-cmd` matches the README flow. To exercise the shell integration manually, source `shell/easy-cmd.zsh` from `.zshrc`, then bind the widget with `bindkey '^G' easy-cmd-widget`.

## Coding Style & Naming Conventions
Format all Go code with `gofmt`; this repo follows standard Go formatting and import grouping. Keep packages focused and prefer adding behavior inside the relevant `internal/*` package instead of pushing logic into `cmd/easy-cmd/main.go`. Exported identifiers should use PascalCase, unexported helpers should use lowerCamelCase, and test files should stay adjacent to the code they cover. Follow existing names such as `NewEngine`, `Load`, and `TestClientRejectsNonJSON`.

## Testing Guidelines
Tests use Go's standard `testing` package and live in `*_test.go` files, often in external test packages such as `ai_test` or `shell_test`. Add or update tests for any behavior change in the TUI model, provider layer, protocol parsing, or zsh bridge. Run `make test` before opening a PR; no separate coverage gate is defined in this snapshot.

## Commit & Pull Request Guidelines
This workspace snapshot does not include `.git`, so no local history is available to infer project-specific commit conventions. Use short, imperative commit subjects and keep each commit to one logical change, for example `shell: validate selected command before eval`. PRs should summarize the behavior change, list verification steps, note config changes such as `~/.easy-cmd/config.json`, and include screenshots or terminal captures for TUI or shell UX changes.

## Security & Configuration Tips
Never commit API keys; keep them in `~/.easy-cmd/config.json`. The filesystem provider is intentionally read-only and path-guarded, so preserve those restrictions when changing `internal/providers/filesystem`.
