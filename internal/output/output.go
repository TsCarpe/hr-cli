package output

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/itchyny/gojq"
)

type Output struct {
	writer  io.Writer
	format  string
	jq      string
	compact bool
}

// NewOutput 创建输出器。
// compact=true 时 JSON 紧凑输出(无缩进),供 --agent / AI 消费。
func NewOutput(w io.Writer, format, jq string, compact bool) *Output {
	return &Output{writer: w, format: format, jq: jq, compact: compact}
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
	return o.encode(data, true)
}

// encode 序列化单个值。singleTop=true 时遵循 compact 设置(顶层美化),
// 否则强制紧凑(NDJSON 多结果分支)。
func (o *Output) encode(data any, singleTop bool) error {
	enc := json.NewEncoder(o.writer)
	if singleTop && !o.compact {
		enc.SetIndent("", "  ")
	}
	enc.SetEscapeHTML(false)
	return enc.Encode(data)
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
		return o.encode(results[0], true)
	default:
		// 多结果始终 NDJSON 风格(每行紧凑),不受 compact 影响
		for _, r := range results {
			if err := o.encode(r, false); err != nil {
				return err
			}
		}
		return nil
	}
}
