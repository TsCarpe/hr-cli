package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version 在构建时通过 -ldflags "-X cmd.Version=..." 注入;
// 未注入时为 dev,标识本地开发构建。
var Version = "dev"

func NewCmdVersion() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "打印 hr-cli 版本",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("hr-cli", Version)
			return nil
		},
	}
}
