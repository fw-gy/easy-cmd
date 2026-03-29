# easy-cmd

[English](README.md) | [中文](README_CN.md)

`easy-cmd` 是一个以 zsh 为优先的终端助手。它会打开一个全屏、聊天式的 TUI，把自然语言请求转换成候选命令，并允许你继续细化需求，直到筛出一个适合在当前 shell 中执行的命令。

## 状态

这是 v1 版本。它目前优先支持 macOS、仅支持 zsh，并使用只读的上下文 provider 来收集 AI 所需的上下文。

## 安装

```bash
go mod tidy
go build -o ~/.local/bin/easy-cmd ./cmd/easy-cmd
```

如果按 Homebrew 风格安装，需要同时分发 `easy-cmd` 二进制以及位于 `share/easy-cmd/easy-cmd.zsh` 的 `shell/easy-cmd.zsh`。

## 配置

创建 [`~/.easy-cmd/config.json`](/Users/Zhuanz/.easy-cmd/config.json)：

```json
{
  "base_url": "https://open.bigmodel.cn/api/paas/v4/chat/completions",
  "api_key": "your-key",
  "model": "glm-4.5-air",
  "language": "zh-CN"
}
```

`base_url` 和 `api_key` 是必填项。`model` 是可选项，默认值为 `glm-4.5-air`。`language` 是可选项，默认值为 `zh-CN`。

支持的 `language` 取值：

- `zh-CN`
- `en-US`

配置的语言会应用到内置 TUI 的界面文案，例如占位提示、状态文本、确认提示和空状态说明。AI 生成的自由文本回复不会被 `easy-cmd` 翻译。

## zsh 设置

把下面这行加入 `.zshrc`：

```zsh
eval "$(easy-cmd init zsh)"
```

`easy-cmd init zsh` 会为当前安装方式输出对应的 shell 集成片段。它支持以下两种布局：

- Homebrew 风格安装，脚本位于 `share/easy-cmd/easy-cmd.zsh`
- 本地开发构建，二进制位于 `tmp/`，脚本位于 `shell/`

如果你需要手动 `source` 这个脚本，`easy-cmd init zsh` 会给出准确的 `source` 路径以及它期望的 `EASY_CMD_BIN` 导出内容。

如果编译后的二进制不在 `PATH` 中，请在 `source` 脚本之前先设置 `EASY_CMD_BIN`：

```zsh
export EASY_CMD_BIN=/absolute/path/to/easy-cmd
```

## 工作原理

- `easy-cmd init zsh` 会输出启用当前安装环境下 zsh 小部件所需的 shell 设置。
- `easy-cmd` shell 函数会带上当前目录和 git 根目录来启动 Go TUI。
- AI 客户端会发送一个 Chat Completions 请求，其中包含 `system` 和 `user` 消息，并设置 `stream=false`、`temperature=0.3`。
- 模型既可以直接返回 `assistant_turn`，也可以通过只读 provider 请求更多上下文。
- 每个 `assistant_turn` 都包含自然语言说明，以及最多 3 张命令卡片。
- 你可以继续通过追问来细化要求；旧的命令组仍然可见，但在新的回复出现后会过期。
- 选择某个命令后，程序会把 JSON 返回给 zsh，而 zsh 小部件会把选中的命令重新填回当前提示符缓冲区，供你检查并手动执行。
- 直接运行 `easy-cmd` 时，它会把选中的命令打印到 stdout，而不是直接执行。
- 高风险命令需要在 TUI 中进行一次额外的行内确认。

## v1 provider

- `filesystem.list`
- `filesystem.read_file`
- `filesystem.search`
- `path.stat`
- `git.status`
- `git.branch`

## 限制

- v1 不支持任意基于 shell 的上下文收集
- 没有持久化历史记录或反馈学习
- zsh 中的 JSON 解析依赖 `python3`
- 模型必须把最终的 easy-cmd JSON envelope 放在 `choices[0].message.content` 中
