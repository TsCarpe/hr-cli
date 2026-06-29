package output

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/itchyny/gojq"
)

type Output struct {
	writer io.Writer
	format string
	jq     string
}

func NewOutput(w io.Writer, format, jq string) *Output {
	return &Output{writer: w, format: format, jq: jq}
}

func (o *Output) Write(data any) error {
	switch o.format {
	case "json", "":
		return o.writeJSON(data)
	case "table":
		return fmt.Errorf("table 格式暂未实现")
	default:
		return fmt.Errorf("%s 格式暂不支持", o.format)
	}
}

func (o *Output) writeJSON(data any) error {
	if o.jq != "" {
		return o.applyJQ(data)
	}
	encoder := json.NewEncoder(o.writer)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(data)
}

// applyJQ 对 data 跑 jq 表达式,把结果序列化输出。
//   - 多结果(.[] | .foo)→ 每行一个 JSON(NDJSON 风格),便于下游/AI 处理
//   - 单结果 → 缩进美化输出,跟普通 --format json 一致
//   - ponytail: 空结果静默返回,exit 0,脚本管道友好
func (o *Output) applyJQ(data any) error {
	// gojq 需要 map/slice/scalar,不认 json.RawMessage;先解码一次
	if raw, ok := data.(json.RawMessage); ok {
		var decoded any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return fmt.Errorf("--jq 解析响应失败: %w", err)
		}
		data = decoded
	}

	query, err := gojq.Parse(o.jq)
	if err != nil {
		return fmt.Errorf("--jq 表达式语法错误: %w", err)
	}
	iter := query.Run(data)

	var results []any
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if e, isErr := v.(error); isErr {
			return fmt.Errorf("--jq 执行错误: %w", e)
		}
		results = append(results, v)
	}

	switch len(results) {
	case 0:
		return nil
	case 1:
		enc := json.NewEncoder(o.writer)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		return enc.Encode(results[0])
	default:
		enc := json.NewEncoder(o.writer)
		enc.SetEscapeHTML(false)
		for _, r := range results {
			if err := enc.Encode(r); err != nil {
				return err
			}
		}
		return nil
	}
}
