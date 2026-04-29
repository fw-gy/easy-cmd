# easy-cmd

[English](README.md) | [中文](README_CN.md)

你是否有这样的困扰，明明知道自己想做什么，却怎么也想不起来命令该怎么写？`easy-cmd` 就是为这个场景准备的。你只需要用自然语言描述任务，它就会结合当前上下文给出候选命令和对应的解释供你选择。

## 为什么你会需要它

- 你记得“要做什么”，但不记得命令细节和参数。
- 你想在终端里直接解决问题，而不是频繁切去搜索引擎。
- 你希望 AI 帮你起草命令，但最终执行权还在自己手里。
- 你希望工具理解当前目录、文件和 git 状态，而不是盲猜。

`easy-cmd` 不是一个自动帮你执行命令的黑盒代理，而是一个始终让你保持确认权的命令起草助手。

## 快速开始

最简单的方式是下载 GitHub Release 中的二进制，然后执行：

```bash
chmod +x ./easy-cmd
./easy-cmd init
```

把下面这行加入 `.zshrc`：

```zsh
source ~/.easy-cmd/script.zsh
```

重新加载 shell 后，先运行一次 `easy-cmd`。如果 `~/.easy-cmd/config.json` 不存在、格式不合法，或者缺少 `base_url` / `api_key`，程序会先打开一个 TUI 配置向导，帮你完成配置并写入文件。之后就可以正常使用，例如：

- `找出当前目录里最大的 10 个文件`
- `看看我现在在哪个分支，还有没有未提交改动`
- `把这里所有 png 文件打包成一个 zip`

`easy-cmd` 会打开一个全屏 TUI，读取当前工作区的安全上下文，并返回最多 3 个候选命令。你可以继续追问、细化要求，最后选中一个命令，把它回填到当前终端里，你只需回车即可执行。

## 架构

`easy-cmd` 现在拆成了“一次性逻辑层 + UI 包装层”：

- `internal/service` 接收一次请求，负责收集只读上下文、调用模型、做本地安全分级，并返回结构化结果。
- `internal/app` 只负责 Bubble Tea TUI 的输入、渲染和命令选择。
- `easy-cmd run` 暴露了同一套逻辑层能力，不需要启动 TUI。

如果要非交互调用，可以直接执行：

```bash
easy-cmd run --query "找出当前目录里最大的 10 个文件" --cwd "$PWD" --workspace-root "$PWD"
```

`easy-cmd run` 保持非交互语义；如果配置文件缺失或不完整，它会直接报错，不会弹出配置 TUI。

## 工作原理

- `easy-cmd init` 会安装二进制、shell 集成，并写入 `~/.easy-cmd/script.zsh` 以启用 zsh 小部件；配置文件仍在第一次交互式运行时再创建。
- `easy-cmd` shell 函数会带上当前目录和 git 根目录来启动 Go TUI。
- TUI 本身只是对一次性逻辑层的包装，也就是 `easy-cmd run` 使用的同一套服务。
- 在进入命令 TUI 前，交互式启动路径会先加载 `~/.easy-cmd/config.json`；如果文件缺失、格式错误或缺少必要字段，就先进入配置向导 TUI。
- AI 客户端会发送一个 Chat Completions 请求，其中包含 `system` 和 `user` 消息，并设置 `stream=false`、`temperature=0.3`。
- 模型既可以直接返回 `assistant_turn`，也可以通过只读 provider 请求更多上下文。
- 每个 `assistant_turn` 都包含自然语言说明，以及最多 3 张命令卡片。
- 你仍然可以在 TUI 里继续追问，但每次追问都会被当作一次全新的一次性逻辑调用；旧的命令组仍然可见，但在新的回复出现后会过期。
- 选择某个命令后，程序会把 JSON 返回给 zsh，而 zsh 小部件会把选中的命令重新填回当前提示符缓冲区，供你检查并手动执行。
- 直接运行 `easy-cmd run` 时，它会把结构化结果 JSON 打印到 stdout，而不是直接执行。
- 高风险命令需要在 TUI 中进行一次额外的行内确认。
