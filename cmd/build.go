package cmd

import (
	"github.com/spf13/cobra"

	"hr-cli/internal/config"
	"hr-cli/internal/runner"
	"hr-cli/shortcuts"
)

// buildCmdTree 把所有子命令挂到 root 上。
// 新增子命令时,在这里 AddCommand。
func buildCmdTree(root *cobra.Command) {
	root.AddCommand(NewCmdVersion())

	// shortcut 命令(按 service 分组),optsFactory 延迟读取 globalFlags
	shortcutCmds := shortcuts.NewCmds(func() runner.Options {
		return runner.Options{
			BaseURL: config.ResolveBaseURL(globalFlagValues.baseURL),
			Token:   globalFlagValues.token,
		}
	})

	// 元数据驱动的 service 命令(course/add 等),并把对应 shortcut 挂到 service 下
	for _, cmd := range NewCmdService(shortcutCmds) {
		root.AddCommand(cmd)
	}

	// 添加 schema 命令
	root.AddCommand(NewCmdSchema())
}
