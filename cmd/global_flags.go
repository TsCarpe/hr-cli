package cmd

import (
	"github.com/spf13/cobra"

	"github.com/TsCarpe/hr-cli/internal/config"
)

// globalFlags 容纳所有全局 flag 的值。
// M1 阶段先定义字段,后续 milestone 逐步启用。

var globalFlagValues = &globalFlags{}

type globalFlags struct {
	token   string
	format  string
	baseURL string
	jq      string
	agent   bool
}

func registerGlobalFlags(root *cobra.Command, gf *globalFlags) {

	root.PersistentFlags().StringVar(&gf.token, "token", "", "认证 token (hrToken),优先级: --token > LISTEN_TOKEN env > 配置文件")
	// ponytail: base-url 默认空,由 config.ResolveBaseURL 兜底(flag > env > .hr-cli.json > DefaultBaseURL)。
	// 不在 cobra 层写死默认值,否则无法区分「用户未传」vs「显式传了 localhost」。
	root.PersistentFlags().StringVar(&gf.baseURL, "base-url", "", "listen 服务地址,优先级: --base-url > HR_CLI_BASE_URL env > .hr-cli.json > "+config.DefaultBaseURL)
	root.PersistentFlags().StringVar(&gf.format, "format", "json", "输出格式: json | table")
	root.PersistentFlags().StringVar(&gf.jq, "jq", "", "jq 表达式,过滤 JSON 输出")
	// --agent: AI agent 模式信号。当前仅联动 compact JSON 输出;
	// 后续可扩展(无色、非交互默认值)。一个 flag 替代记多个。
	root.PersistentFlags().BoolVar(&gf.agent, "agent", false, "AI agent 模式:强制 compact JSON 输出(无缩进)")
}
