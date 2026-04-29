function _easy_cmd_resolve_bin() {
  print -r -- "${HOME}/.local/bin/easy-cmd"
  return 0
}

function _easy_cmd_git_root() {
  # 如果当前目录在 git 仓库里，就把仓库根目录当作工作区边界传给
  # easy-cmd，确保上下文 provider 只在项目范围内活动。
  command git -C "$PWD" rev-parse --show-toplevel 2>/dev/null
}

function _easy_cmd_selected_command() {
  local bin
  bin="$(_easy_cmd_resolve_bin)" || return 1

  local workspace_root
  workspace_root="$(_easy_cmd_git_root)"

  local query="$*"
  if [[ -z "$query" && -n "${BUFFER:-}" ]]; then
    # widget 方式调用时不会传命令行参数，因此这里把当前 shell buffer
    # 当作自然语言查询文本使用。
    query="$BUFFER"
  fi

  local -a cmd_args
  # 交互本身完全由 easy-cmd 负责；shell 包装层这里只转发足够的环境信息，
  # 让它知道请求来自哪里。
  cmd_args=(pick --cwd "$PWD")
  if [[ -n "$workspace_root" ]]; then
    cmd_args+=(--workspace-root "$workspace_root")
  fi
  if [[ -n "$query" ]]; then
    cmd_args+=(--query "$query")
  fi

  local output
  output="$("$bin" "${cmd_args[@]}")" || return $?
  print -r -- "$output"
}

function easy-cmd() {
  # `easy-cmd init` 直接透传给真实子命令处理；其他调用都会走 TUI，
  # 再返回一份机器可读的已选命令结果。
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

  # 全屏 TUI 运行时先清空当前提示行；退出后再恢复为用户选中的命令，
  # 或原始输入内容。
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

# 将easy-cmd-widget注册为zsh widget，这样才能绑定快捷键
zle -N easy-cmd-widget
# 绑定快捷键 ctrl + g
bindkey '^G' easy-cmd-widget
