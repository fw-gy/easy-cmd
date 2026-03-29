function _easy_cmd_resolve_bin() {
  if [[ -n "${EASY_CMD_BIN:-}" ]]; then
    print -r -- "$EASY_CMD_BIN"
    return 0
  fi

  local resolved
  resolved="$(whence -p easy-cmd 2>/dev/null)"
  if [[ -n "$resolved" ]]; then
    print -r -- "$resolved"
    return 0
  fi

  print -u2 -- "easy-cmd: binary not found; set EASY_CMD_BIN or add easy-cmd to PATH"
  return 1
}

function _easy_cmd_git_root() {
  command git -C "$PWD" rev-parse --show-toplevel 2>/dev/null
}

function _easy_cmd_parse_json_field() {
  local payload="$1"
  local field="$2"
  /usr/bin/python3 - "$payload" "$field" <<'PY'
import json
import sys

payload = json.loads(sys.argv[1])
field = sys.argv[2]
value = payload.get(field, "")
if isinstance(value, str):
    sys.stdout.write(value)
PY
}

function _easy_cmd_selected_command() {
  local bin
  bin="$(_easy_cmd_resolve_bin)" || return 1

  local workspace_root
  workspace_root="$(_easy_cmd_git_root)"

  local query="$*"
  if [[ -z "$query" && -n "${BUFFER:-}" ]]; then
    query="$BUFFER"
  fi

  local -a cmd_args
  cmd_args=(--cwd "$PWD")
  if [[ -n "$workspace_root" ]]; then
    cmd_args+=(--workspace-root "$workspace_root")
  fi
  if [[ -n "$query" ]]; then
    cmd_args+=(--query "$query")
  fi

  local output
  output="$("$bin" "${cmd_args[@]}")" || return $?

  local action
  action="$(_easy_cmd_parse_json_field "$output" "action")" || return 1
  if [[ "$action" != "execute" ]]; then
    return 0
  fi

  local selected_command
  selected_command="$(_easy_cmd_parse_json_field "$output" "selected_command")" || return 1
  if [[ -z "$selected_command" ]]; then
    print -u2 -- "easy-cmd: selected_command missing from output"
    return 1
  fi

  print -r -- "$selected_command"
}

function easy-cmd() {
  if [[ "${1:-}" == "init" ]]; then
    local bin
    bin="$(_easy_cmd_resolve_bin)" || return 1
    command "$bin" "$@"
    return $?
  fi

  local selected_command
  selected_command="$(_easy_cmd_selected_command "$@")" || return $?
  if [[ -n "$selected_command" ]]; then
    print -r -- "$selected_command"
  fi
}

function easy-cmd-widget() {
  local query="$BUFFER"
  local selected_command

  BUFFER=""
  zle -I

  selected_command="$(_easy_cmd_selected_command "$query")" || return $?
  if [[ -n "$selected_command" ]]; then
    BUFFER="$selected_command"
    CURSOR=${#BUFFER}
  else
    BUFFER="$query"
    CURSOR=${#BUFFER}
  fi

  zle reset-prompt
}

zle -N easy-cmd-widget
