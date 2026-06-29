package shortcuts

import (
	"github.com/spf13/cobra"

	"hr-cli/internal/runner"
	"hr-cli/shortcuts/common"
	"hr-cli/shortcuts/course"
	"hr-cli/shortcuts/groupmanage"
	"hr-cli/shortcuts/saas"
)

// All 返回所有已注册的 shortcut。
// 新增 shortcut 时,在这里 append。
func All() []*common.Shortcut {
	return []*common.Shortcut{
		&course.Create,
		&groupmanage.SearchTeacher,
		&saas.Login,
	}
}

// NewCmds 把所有 shortcut 转成 cobra 命令并按 service 分组挂载。
// optsFactory 由 cmd 层提供(读取 globalFlags 构造 runner.Options)。
//
// 返回 map[service 名][]*cobra.Command,cmd 层把它们挂到对应的 service 命令下。
// 例如 course.Create → 挂到 root 的 course 子命令下,形成 `hr-cli course +create`。
func NewCmds(optsFactory func() runner.Options) map[string][]*cobra.Command {
	grouped := make(map[string][]*cobra.Command)
	for _, sc := range All() {
		cmd := common.NewCmdShortcut(sc, optsFactory)
		grouped[sc.Service] = append(grouped[sc.Service], cmd)
	}
	return grouped
}