package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func NewRoot() *cobra.Command {
	root := cobra.Command{
		Use:   "hr-cli",
		Short: "鸿儒cli",
		Long:  "hr-cli 把听评课(listen)业务能力封装成可被 AI Agent 直接调用的命令行工具。",
		// 业务错误时不重复打印 Usage/Error(cobra 默认会打,我们让 main 统一处理)
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	registerGlobalFlags(&root, globalFlagValues)

	buildCmdTree(&root)

	return &root
}

func Execute() int {

	root := NewRoot()

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		return 1
	}

	return 0
}
