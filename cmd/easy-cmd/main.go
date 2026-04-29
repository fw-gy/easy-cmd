package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"

	"easy-cmd/internal/cliapp"
	"easy-cmd/internal/protocol"
)

//go:embed assets/easy-cmd.zsh
var embeddedZshScript []byte

func main() {
	cliapp.EmbeddedZshScript = embeddedZshScript

	executablePath, err := os.Executable()
	if err != nil {
		fatalInit(err)
	}
	if handled, err := cliapp.HandleCommandDirectly(os.Args[1:], os.Stdout, executablePath); handled {
		if err != nil {
			fatalInit(err)
		}
		return
	}
	// 启动TUI交互
	if err := cliapp.RunInteractiveCLI(os.Args[1:], os.Stdout); err != nil {
		fatalCancel(err)
	}
}

// fatalCancel 即使在启动或运行失败时，也仍然输出一个 JSON cancel，
// 因为 shell 调用方依赖这种机器可读结果。
func fatalCancel(err error) {
	fmt.Fprintln(os.Stderr, "easy-cmd:", err)
	_ = json.NewEncoder(os.Stdout).Encode(protocol.AppOutput{Action: protocol.ActionCancel})
	os.Exit(1)
}

func fatalInit(err error) {
	fmt.Fprintln(os.Stderr, "easy-cmd:", err)
	os.Exit(1)
}
