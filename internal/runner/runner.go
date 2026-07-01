package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"hr-cli/internal/client"
	"hr-cli/internal/config"
	"hr-cli/internal/registry"
)

func RunMethod(svcName, mtdName string, body map[string]any, opts Options) (*Result, error) {

	reg, err := registry.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load registry: %w", err)
	}

	var svc *registry.Service
	for _, s := range reg.Services {
		if s.Name == svcName {
			svc = s
			break
		}
	}
	if svc == nil {
		return nil, fmt.Errorf("service: %s 不存在", svcName)
	}

	mtd := svc.Methods[mtdName]
	if mtd == nil {
		return nil, fmt.Errorf("method: %s 不存在", mtdName)
	}

	fullPath := svc.BasePath + mtd.Path

	// 按 method 元数据的 authHeader 决定 token 来源:
	// - "Authorization"(saas 接口)→ 读 SAAS_AUTH env
	// - 其他/空(默认 hrToken)→ 走 ResolveToken(flag > env > config)
	authHeader := mtd.AuthHeader
	var token string
	if authHeader == "Authorization" {
		token = config.ResolveSaasAuth()
	} else {
		token = config.ResolveToken(opts.Token)
		authHeader = "hrToken"
	}

	// ponytail: 自动注入 schoolId/campusId 默认值。
	// 仅当元数据声明接受该字段且 body 缺失时注入;用户显式传的值优先。
	// ceiling: 严格后端可能拒绝多余字段,届时改用 requestSchema.required 精确判断。
	if mtd.RequestSchema != nil && mtd.RequestSchema.Properties != nil {
		if body == nil {
			body = map[string]any{}
		}
		props := mtd.RequestSchema.Properties
		if _, ok := props["schoolId"]; ok && (body["schoolId"] == nil || body["schoolId"] == "") {
			cfg, _ := config.LoadConfig()
			if cfg == nil || cfg.SchoolId == "" {
				return nil, fmt.Errorf("缺少 schoolId,请先 hr-cli saas +login 或在 --data 显式传入")
			}
			body["schoolId"] = cfg.SchoolId
		}
		if _, ok := props["campusId"]; ok && (body["campusId"] == nil || body["campusId"] == "") {
			cfg, _ := config.LoadConfig()
			if cfg == nil || cfg.CampusId == "" {
				return nil, fmt.Errorf("缺少 campusId,请先 hr-cli saas +login 或在 --data 显式传入")
			}
			body["campusId"] = cfg.CampusId
		}
	}

	var bodyBytes []byte
	if body != nil {
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal body: %w", err)
		}
	}

	if opts.DryRun {
		fmt.Printf("[DRY-RUN] %s %s%s\n", mtd.HTTPMethod, opts.BaseURL, fullPath)
		fmt.Printf("%s: %s\n", authHeader, maskToken(token))
		fmt.Printf("Body: %s\n", string(bodyBytes))
		return &Result{}, nil
	}

	c := client.NewClient(opts.BaseURL)

	status, respData, err := c.Do(context.Background(), mtd.HTTPMethod, fullPath, bodyBytes, authHeader, token)
	if err != nil {
		return nil, err
	}

	// 解析 data 成 map
	var dataMap map[string]any
	if len(respData) > 0 {
		json.Unmarshal(respData, &dataMap)
	}

	return &Result{
		Status: status,
		Data:   dataMap,
		Raw:    respData,
	}, nil
}

type Options struct {
	DryRun  bool
	BaseURL string
	Token   string
	Compact bool // --agent 联动:shortcuts 层据此开 compact 输出
}

type Result struct {
	Status int
	Data   map[string]any
	Raw    json.RawMessage
}

// maskToken 脱敏 token,只显示前 8 位 + ...
// 防止 dry-run 时把完整 token 打到日志
func maskToken(token string) string {
	if len(token) <= 8 {
		return token
	}
	return token[:8] + "..."
}
