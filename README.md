# easy-cmd

[English](README.md) | [中文](README_CN.md)

`easy-cmd` is a zsh-first terminal assistant that opens a full-screen chat-style TUI, turns natural language into command options, and lets you keep refining the request until one command is worth executing in the current shell.

## Status

This is v1. It is macOS-first, zsh-only, and uses read-only context providers for AI context collection.

## Install

```bash
go mod tidy
go build -o ~/.local/bin/easy-cmd ./cmd/easy-cmd
```

For Homebrew-style installs, ship both the `easy-cmd` binary and `shell/easy-cmd.zsh` under `share/easy-cmd/easy-cmd.zsh`.

## Config

Create [`~/.easy-cmd/config.json`](/Users/Zhuanz/.easy-cmd/config.json):

```json
{
  "base_url": "https://open.bigmodel.cn/api/paas/v4/chat/completions",
  "api_key": "your-key",
  "model": "glm-4.5-air",
  "language": "zh-CN"
}
```

`base_url` and `api_key` are required. `model` is optional and defaults to `glm-4.5-air`. `language` is optional and defaults to `zh-CN`.

Supported `language` values:

- `zh-CN`
- `en-US`

The configured language applies to the built-in TUI chrome, such as placeholders, status text, confirmation prompts, and empty-state copy. AI-generated free-form responses are not translated by `easy-cmd`.

## zsh setup

Add this to `.zshrc`:

```zsh
eval "$(easy-cmd init zsh)"
```

`easy-cmd init zsh` prints the shell integration snippet for the current installation. It works for:

- Homebrew-style layouts where the script is installed at `share/easy-cmd/easy-cmd.zsh`
- Local development builds where the binary lives in `tmp/` and the script lives in `shell/`

If you need to source the script manually, `easy-cmd init zsh` will show the exact `source` path and `EASY_CMD_BIN` export it expects.

If your compiled binary is not on `PATH`, set `EASY_CMD_BIN` before sourcing the script:

```zsh
export EASY_CMD_BIN=/absolute/path/to/easy-cmd
```

## How it works

- `easy-cmd init zsh` prints the shell setup needed to enable the zsh widget for the current installation.
- The `easy-cmd` shell function launches the Go TUI with current directory and git root.
- The AI client sends a Chat Completions request with `system` and `user` messages, `stream=false`, and `temperature=0.3`.
- The model can either return `assistant_turn` or request more context through read-only providers.
- Each `assistant_turn` contains natural-language guidance plus up to 3 command cards.
- You can continue the conversation with follow-up instructions; older command groups stay visible but expire once a newer turn arrives.
- Selecting a command returns JSON to zsh, and the zsh widget refills the selected command into the current prompt buffer for you to inspect and run manually.
- Running `easy-cmd` directly prints the selected command to stdout instead of executing it.
- High-risk commands require an inline second confirmation in the TUI.

## v1 providers

- `filesystem.list`
- `filesystem.read_file`
- `filesystem.search`
- `path.stat`
- `git.status`
- `git.branch`

## Limitations

- No arbitrary shell-based context collection in v1
- No persistent history or feedback learning
- JSON parsing in zsh relies on `python3`
- The model must place the final easy-cmd JSON envelope inside `choices[0].message.content`
