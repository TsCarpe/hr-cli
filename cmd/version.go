package cmd

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// 版本号由 Go 工具链自动注入:
//   - go install pkg@vX.Y.Z → "vX.Y.Z"
//   - 本地 go build          → "(devel)",回退用 vcs.revision
func version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	// 本地开发构建:用 commit hash 兜底
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" && s.Value != "" {
			if len(s.Value) > 8 {
				return s.Value[:8]
			}
			return s.Value
		}
	}
	return "(devel)"
}

func NewCmdVersion() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "打印 hr-cli 版本",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("hr-cli", version())
			return nil
		},
	}
}
