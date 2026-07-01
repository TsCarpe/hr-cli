package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"hr-cli/internal/config"
	"hr-cli/internal/output"
	"hr-cli/internal/registry"
	"hr-cli/internal/runner"
	"os"
)

// NewCmdService 从 registry 元数据生成所有 service 命令。
// 每个 service 直接挂到 root(不要 service 父层),让 hr-cli course add 直接可用。
// shortcutCmds 按 service 名分组,挂到对应 service 命令下(如 course +create)。
func NewCmdService(shortcutCmds map[string][]*cobra.Command) []*cobra.Command {
	reg, err := registry.Load()
	if err != nil {
		// 元数据加载失败是编译期就该发现的问题,直接 panic 让 main 退出
		panic(fmt.Errorf("加载元数据失败: %w", err))
	}

	var cmds []*cobra.Command
	for _, svc := range reg.Services {
		svcCmd := buildServiceCommand(svc)
		// 把该 service 下的 shortcut 挂上
		for _, sc := range shortcutCmds[svc.Name] {
			svcCmd.AddCommand(sc)
		}
		cmds = append(cmds, svcCmd)
	}
	return cmds
}

func buildServiceCommand(svc *registry.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:   svc.Name,
		Short: svc.Title,
	}

	for _, mtd := range svc.Methods {
		cmd.AddCommand(buildMethodCommand(svc, mtd))
	}

	return cmd
}

func buildMethodCommand(svc *registry.Service, mtd *registry.Method) *cobra.Command {
	cmd := &cobra.Command{
		Use:   mtd.Name,
		Short: mtd.Description,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMethod(svc, mtd, cmd)
		},
	}

	cmd.Flags().String("data", "", "请求体 json")
	cmd.Flags().Bool("dry-run", false, "预览请求, 不实际发送")

	return cmd
}

func runMethod(svc *registry.Service, mtd *registry.Method, cmd *cobra.Command) error {

	data, _ := cmd.Flags().GetString("data")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	opts := runner.Options{
		DryRun:  dryRun,
		BaseURL: config.ResolveBaseURL(globalFlagValues.baseURL),
		Token:   globalFlagValues.token,
	}

	var body map[string]any
	if data != "" {
		if err := json.Unmarshal([]byte(data), &body); err != nil {
			return fmt.Errorf("--data JSON 解析失败: %w", err)
		}
	}

	result, err := runner.RunMethod(svc.Name, mtd.Name, body, opts)

	if err != nil {
		return err
	}

	// DryRun 时 RunMethod 已经打印过请求信息,不再输出
	if opts.DryRun {
		return nil
	}

	o := output.NewOutput(os.Stdout, globalFlagValues.format, globalFlagValues.jq, globalFlagValues.agent)
	return o.Write(result.Raw)
}
