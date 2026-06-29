package common

import (
	"context"

	"github.com/spf13/cobra"

	"hr-cli/internal/output"
	"hr-cli/internal/runner"
)

// NewCmdShortcut 把一个 Shortcut 注册成 cobra 命令。
// optsFactory 由 cmd 层提供,负责用 globalFlags 构造 runner.Options。
//
// 设计:shortcuts 包不直接访问 cmd 包的 globalFlags(避免循环依赖),
// 而是通过 optsFactory 闭包"被动接收"。
func NewCmdShortcut(sc *Shortcut, optsFactory func() runner.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   sc.Command, // "+create"
		Short: sc.Description,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := optsFactory()
			opts.DryRun, _ = cmd.Flags().GetBool("dry-run")
			return runShortcut(sc, cmd, opts)
		},
	}

	// 注册 shortcut 自己的 flag
	for _, f := range sc.Flags {
		cmd.Flags().String(f.Name, f.Default, f.Desc)
	}
	cmd.Flags().Bool("dry-run", false, "预览请求, 不实际发送")

	return cmd
}

// runShortcut 执行 shortcut 的完整流程:
// 1. 从 cobra 收集 flag 值 → RuntimeContext
// 2. 调 BuildBody 组装请求体
// 3. 调 runMethod(走 service 层)
// 4. 调 After 加工输出
func runShortcut(sc *Shortcut, cmd *cobra.Command, opts runner.Options) error {
	ctx := context.Background()

	// 1. 收集 flag 值
	flags := make(map[string]string, len(sc.Flags))
	for _, f := range sc.Flags {
		v, _ := cmd.Flags().GetString(f.Name)
		flags[f.Name] = v
	}

	// 2. 构造 RuntimeContext,注入 runMethod 闭包
	rt := &RuntimeContext{
		flags: flags,
		runMethod: func(service, method string, body map[string]any) (*runner.Result, error) {
			return runner.RunMethod(service, method, body, opts)
		},
		output: output.NewOutput(cmd.OutOrStdout(), "", ""),
	}

	// 3. BuildBody
	body, err := sc.BuildBody(ctx, rt)
	if err != nil {
		return err
	}

	// 4. 调 service 层
	result, err := rt.runMethod(sc.Service, sc.Method, body)
	if err != nil {
		return err
	}

	// 5. After 加工输出(如果有)
	if sc.After != nil && result.Data != nil {
		if err := sc.After(ctx, rt, result.Data); err != nil {
			return err
		}
	}

	return nil
}