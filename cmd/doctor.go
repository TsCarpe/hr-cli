package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"hr-cli/internal/config"
	"hr-cli/internal/runner"
)

// NewCmdDoctor 环境自检命令。
// AI agent 每次会话开始调一次,一次调用知道:配置在不在、token 空不空、
// baseURL 指向哪、schoolId 选没选、hrToken 是否真的有效(联网只读探活)。
// 任一 fail → exit 1,让 agent 用退出码判断环境就绪。
func NewCmdDoctor() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "环境自检:配置/token/连通性/schoolId",
		Long:  "检查 hr-cli 运行所需的一切是否就绪。--agent 模式下输出结构化 JSON。",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor()
		},
	}
}

type checkStatus string

const (
	statusOK   checkStatus = "ok"
	statusWarn checkStatus = "warn"
	statusFail checkStatus = "fail"
)

type checkItem struct {
	Name   string      `json:"name"`
	Status checkStatus `json:"status"`
	Detail string      `json:"detail"`
}

// userContext 输出当前用户与学校上下文给 AI agent(仅 --agent 模式)。
// 数据来自 ~/.hr-cli/config.json,凭证类(hrToken/authorization)不输出。
// 目的:agent 一次 doctor 调用拿到"我是谁 / 我在哪个学校",不必再 Read config.json 或反问用户。
type userContext struct {
	AccountName string `json:"accountName,omitempty"`
	StaffId     string `json:"staffId,omitempty"`
	UserId      string `json:"userId,omitempty"`
	TenantId    string `json:"tenantId,omitempty"`
	SchoolId    string `json:"schoolId,omitempty"`
	CampusId    string `json:"campusId,omitempty"`
	SchoolName  string `json:"schoolName,omitempty"`
	CampusName  string `json:"campusName,omitempty"`
}

func runDoctor() error {
	items := []checkItem{}
	var ctx *userContext

	// 1. 配置文件
	cfg, err := config.LoadConfig()
	if err != nil {
		items = append(items, checkItem{Name: "config", Status: statusFail, Detail: fmt.Sprintf("读取配置失败: %v", err)})
	} else if cfg == nil {
		items = append(items, checkItem{Name: "config", Status: statusFail, Detail: "未登录(~/.hr-cli/config.json 不存在),请先 hr-cli saas +login"})
	} else {
		items = append(items, checkItem{Name: "config", Status: statusOK, Detail: "~/.hr-cli/config.json 存在"})
		ctx = &userContext{
			AccountName: cfg.UserInfo.AccountName,
			StaffId:     cfg.UserInfo.StaffId,
			UserId:      cfg.UserInfo.UserId,
			TenantId:    cfg.TenantId,
			SchoolId:    cfg.SchoolId,
			CampusId:    cfg.CampusId,
			SchoolName:  cfg.SchoolName,
			CampusName:  cfg.CampusName,
		}
	}

	// 2. hrToken
	if cfg != nil {
		if cfg.HrToken == "" {
			items = append(items, checkItem{Name: "hrToken", Status: statusFail, Detail: "hrToken 为空,请重新 saas +login"})
		} else {
			items = append(items, checkItem{Name: "hrToken", Status: statusOK, Detail: maskToken(cfg.HrToken)})
		}
	}

	// 3. baseURL
	baseURL := config.ResolveBaseURL(globalFlagValues.baseURL)
	items = append(items, checkItem{Name: "baseURL", Status: statusOK, Detail: baseURL})

	// 4. schoolId / campusId
	if cfg != nil {
		switch {
		case cfg.SchoolId == "":
			items = append(items, checkItem{Name: "schoolId", Status: statusWarn, Detail: "未选择学校,shortcut 自动注入会缺失 schoolId"})
		case cfg.CampusId == "":
			items = append(items, checkItem{Name: "schoolId", Status: statusWarn, Detail: fmt.Sprintf("%s / 未选校区", cfg.SchoolName)})
		default:
			items = append(items, checkItem{Name: "schoolId", Status: statusOK, Detail: fmt.Sprintf("%s / %s", cfg.SchoolName, cfg.CampusName)})
		}
	}

	// 5. 连通性 + hrToken 真实有效性:发一次最轻只读请求
	if cfg != nil && cfg.HrToken != "" {
		items = append(items, pingConnectivity(baseURL, cfg.HrToken))
	}

	// 输出
	if globalFlagValues.agent {
		if err := outputDoctorJSON(items, ctx); err != nil {
			return err
		}
	} else {
		outputDoctorText(items)
	}

	// 退出码:任一 fail → exit 1
	for _, it := range items {
		if it.Status == statusFail {
			os.Exit(1)
		}
	}
	return nil
}

// pingConnectivity 发 listen_get_lists(最小分页)验证 hrToken 真实有效 + 网络可达。
// 失败时 detail 由调用者决定详略(见下方 TODO)。
//
// ponytail: 用 business 接口探活,不虚构 ping。失败原因(401/网络/5xx)直接透传后端消息。
func pingConnectivity(baseURL, token string) checkItem {
	opts := runner.Options{
		BaseURL: baseURL,
		Token:   token,
	}
	body := map[string]any{"page": 1, "pageSize": 1}
	_, err := runner.RunMethod("listen", "listen_get_lists", body, opts)
	if err != nil {
		// TODO(human): 连通性失败的 detail 文案
		return checkItem{Name: "connectivity", Status: statusFail, Detail: detailForConnFail(err)}
	}
	return checkItem{Name: "connectivity", Status: statusOK, Detail: "hrToken 有效,后端可达"}
}

func outputDoctorText(items []checkItem) {
	for _, it := range items {
		var tag string
		switch it.Status {
		case statusOK:
			tag = "[OK]  "
		case statusWarn:
			tag = "[WARN]"
		case statusFail:
			tag = "[FAIL]"
		}
		fmt.Printf("%s %-13s — %s\n", tag, it.Name, it.Detail)
	}
}

func outputDoctorJSON(items []checkItem, ctx *userContext) error {
	// ponytail: 未登录时输出 "context": null,agent 用 `if ctx == null` 判空,语义最清晰。
	payload := map[string]any{
		"checks":  items,
		"context": ctx, // ctx == nil 时序列化为 null
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(payload)
}

// maskToken 只显示首尾,中间用 ... 占位。诊断够用,不泄露完整凭证。
func maskToken(t string) string {
	if len(t) <= 12 {
		return "***"
	}
	return t[:6] + "..." + t[len(t)-4:]
}

// detailForConnFail 把连通性失败错误转成 detail 文案。
// TODO(human) 在这里实现引导式诊断文案:对 401/超时/5xx 给不同建议。
func detailForConnFail(err error) string {
	return err.Error()
}
