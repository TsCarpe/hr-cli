package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewCmdVersion() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "打印 hr-cli 版本",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("hr-cli v0.1.0")
			return nil
		},
	}
}
