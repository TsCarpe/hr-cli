package common

import (
	"context"
	"github.com/TsCarpe/hr-cli/internal/output"
	"github.com/TsCarpe/hr-cli/internal/runner"
)

type Shortcut struct {
	Service     string
	Command     string   // "+create" / "+search-teacher"
	Method      string   // 对应的 service method 名(add / member_get_lists)
	Flags       []Flag
	Description string
	Risk        string
	BuildBody   func(ctx context.Context, rt *RuntimeContext) (map[string]any, error)
	After       func(ctx context.Context, rt *RuntimeContext, result map[string]any) error
}

type Flag struct {
	Name    string
	Desc    string
	Default string
}

type RuntimeContext struct {
	flags     map[string]string
	runMethod func(service, method string, body map[string]any) (*runner.Result, error)
	output    *output.Output
}

func (rt *RuntimeContext) Str(name string) string {
	return rt.flags[name]
}

// RunMethod 暴露给 shortcut 内部调用其他 service/method(用于多步编排,如 saas +login)。
func (rt *RuntimeContext) RunMethod(service, method string, body map[string]any) (*runner.Result, error) {
	return rt.runMethod(service, method, body)
}

// SetFlag 暂存跨步骤数据(如登录后要展示的学校名)。shortcut 内部用。
func (rt *RuntimeContext) SetFlag(name, value string) {
	rt.flags[name] = value
}
