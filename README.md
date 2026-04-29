# easy-cmd

[English](README.md) | [中文](README_CN.md)

Ever freeze the moment you open a terminal because you know what you want to do, but not the exact command to type? `easy-cmd` is built for that moment. Describe the task in plain language, let the assistant turn it into a few command options, and keep full control before anything runs.

## Quick Start

The simplest setup is to download the GitHub Release binary and run:

```bash
chmod +x ./easy-cmd
./easy-cmd init
```

This installs:

- `~/.local/bin/easy-cmd`
- `~/.easy-cmd/script.zsh` (zsh bridge functions, widget registration, and `^G` keybinding)

Then:

1. Make sure `~/.local/bin` is on your `PATH`
2. Add this to `.zshrc`:

```zsh
source ~/.easy-cmd/script.zsh
```

3. Reload your shell and run `easy-cmd` once. If `~/.easy-cmd/config.json` is missing, invalid, or missing `base_url` / `api_key`, a TUI setup flow will guide you through configuration and write the file for you.

If you prefer building from source:

```bash
go mod tidy
go build -o ~/.local/bin/easy-cmd ./cmd/easy-cmd
```

## Architecture

`easy-cmd` now has a one-shot logic layer and a UI wrapper:

- `internal/service` accepts a single request, gathers read-only context, calls the model, applies local safety classification, and returns structured results.
- `internal/app` is only the Bubble Tea wrapper that collects input, renders results, and lets you choose a command.
- `easy-cmd run` exposes the same logic layer without the TUI.

## First Use

Open your terminal and trigger `easy-cmd` from zsh. Then type something like:

- `find the 10 largest files in this directory`
- `show me which branch I'm on and whether I have uncommitted changes`
- `compress all png files here into a zip`

`easy-cmd` opens a full-screen TUI, reads safe context from your current workspace, and returns up to three command options. You can refine the request in natural language, choose one result, and inspect the final command before running it in your shell. Each refinement is executed as a fresh one-shot logic request.

For non-interactive use:

```bash
easy-cmd run --query "find the 10 largest files here" --cwd "$PWD" --workspace-root "$PWD"
```

`easy-cmd run` stays non-interactive. If the config file is missing or incomplete, it returns an error instead of launching the setup TUI.

## Why `easy-cmd`

- You remember intent, not flags.
- You want help inside the terminal, not in a browser tab.
- You still want to review the final command yourself.
- You want the assistant to use the current directory and git state as context.

This is not an auto-executing shell agent. It is a command drafting assistant that stays in the loop with you.

## How It Works

- `easy-cmd init` installs the binary, shell integration, and `~/.easy-cmd/script.zsh` (widget setup); config creation stays on the first interactive run.
- The `easy-cmd` shell function launches the Go TUI with the current directory and git root.
- The TUI is a thin wrapper over the one-shot service layer exposed by `easy-cmd run`.
- Before entering the command TUI, the interactive path loads `~/.easy-cmd/config.json` and launches a configuration TUI if the file is missing, invalid, or incomplete.
- The AI client sends a Chat Completions request with `system` and `user` messages, `stream=false`, and `temperature=0.3`.
- The model can either return `assistant_turn` or request more context through read-only providers.
- Each `assistant_turn` contains natural-language guidance plus up to 3 command cards.
- You can keep refining in the TUI, but each follow-up is handled as a fresh one-shot request; older command groups stay visible but expire once a newer turn arrives.
- Selecting a command returns JSON to zsh, and the zsh widget refills the selected command into the current prompt buffer for manual review and execution.
- Running `easy-cmd run` directly prints the structured logic result JSON to stdout instead of executing anything.
- High-risk commands require an inline second confirmation in the TUI.

## v1 Providers

- `filesystem.list`
- `filesystem.read_file`
- `filesystem.search`
- `path.stat`
- `git.status`
- `git.branch`

## Status

This is v1. It is macOS-first, zsh-only, and uses read-only context providers for AI context collection.

## Limitations

- No arbitrary shell-based context collection in v1
- No persistent history or feedback learning
- JSON parsing in zsh relies on `python3`
- The model must place the final easy-cmd JSON envelope inside `choices[0].message.content`
