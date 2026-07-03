package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/TsCarpe/hr-cli/internal/registry"
	"github.com/TsCarpe/hr-cli/shortcuts"
)

// NewCmdWhich 意图发现命令。按关键词模糊搜索,同时返回 shortcut 和 service.method。
// 是 schema(列全部)的意图化补充:agent 不用背命令名,hr-cli which 查教师 → 命中。
func NewCmdWhich() *cobra.Command {
	return &cobra.Command{
		Use:   "which <capability> [...]",
		Short: "按关键词找命令(意图发现)",
		Long:  "模糊匹配 shortcut 和 service.method 的描述/名字。--agent 模式输出 JSON 数组。",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWhich(args)
		},
	}
}

type whichHit struct {
	Type        string `json:"type"` // "shortcut" | "method"
	Service     string `json:"service"`
	Command     string `json:"command"` // shortcut: "+create"; method: "add"
	Description string `json:"description"`
	Risk        string `json:"risk,omitempty"`
}

func runWhich(args []string) error {
	// 把多个关键词合一,小写化(中文 Contains 不受大小写影响,英文友好)
	q := strings.ToLower(strings.Join(args, " "))

	// 同义词扩展:业务术语 → 描述文本里的近义词。
	// ponytail: 硬编码映射,够用就行。条目多了再抽成单独文件。
	queries := expandQuery(q)

	hits := []whichHit{}
	for _, qry := range queries {
		hits = append(hits, searchShortcuts(qry)...)
		hits = append(hits, searchMethods(qry)...)
	}
	hits = dedupHits(hits)

	if globalFlagValues.agent {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		return enc.Encode(hits)
	}
	return outputWhichText(hits, q)
}

// expandQuery 把业务术语扩展成多个近义查询词。
// 用户输入 "邀课" → 实际描述里写的是 "邀请",直接 Contains 搜不到,
// 这里补一层同义词映射,避免改 shortcut 描述污染 --help。
func expandQuery(q string) []string {
	// 同义词表:输入 → 补充搜索词(原始词也保留)
	synonyms := map[string][]string{
		"邀课":  {"邀请", "创建"},
		"查教师": {"搜教师", "查询教师", "教师"},
		"建课":  {"创建课", "添加课", "课程"},
		"听课":  {"评课", "听评课"},
	}
	result := []string{q}
	if extra, ok := synonyms[q]; ok {
		result = append(result, extra...)
	}
	return result
}

func dedupHits(hits []whichHit) []whichHit {
	seen := map[string]bool{}
	out := hits[:0]
	for _, h := range hits {
		key := h.Type + "|" + h.Service + "|" + h.Command
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, h)
	}
	return out
}

// searchShortcuts 在 shortcut 集合里找命中。
// 字段:Description / Service / Command 都参与匹配。
func searchShortcuts(q string) []whichHit {
	var hits []whichHit
	for _, sc := range shortcuts.All() {
		hay := strings.ToLower(sc.Description + " " + sc.Service + " " + sc.Command)
		if strings.Contains(hay, q) {
			hits = append(hits, whichHit{
				Type:        "shortcut",
				Service:     sc.Service,
				Command:     sc.Command,
				Description: sc.Description,
				Risk:        sc.Risk,
			})
		}
	}
	return hits
}

// searchMethods 在元数据的 service.method 里找命中。
// 字段:Method.Name / Method.Description / Service.Name / Service.Title。
func searchMethods(q string) []whichHit {
	reg, err := registry.Load()
	if err != nil {
		return nil
	}
	var hits []whichHit
	for _, svc := range reg.Services {
		for _, m := range svc.Methods {
			hay := strings.ToLower(m.Name + " " + m.Description + " " + svc.Name + " " + svc.Title)
			if strings.Contains(hay, q) {
				hits = append(hits, whichHit{
					Type:        "method",
					Service:     svc.Name,
					Command:     m.Name,
					Description: m.Description,
					Risk:        m.Risk,
				})
			}
		}
	}
	return hits
}

func outputWhichText(hits []whichHit, q string) error {
	var scHits, mHits []whichHit
	for _, h := range hits {
		if h.Type == "shortcut" {
			scHits = append(scHits, h)
		} else {
			mHits = append(mHits, h)
		}
	}

	if len(scHits) == 0 && len(mHits) == 0 {
		fmt.Printf("未命中 %q。建议:\n", q)
		fmt.Println("  hr-cli schema              列全部 service")
		fmt.Println("  hr-cli schema <service>    列某 service 的 method")
		return nil
	}

	if len(scHits) > 0 {
		fmt.Println("Shortcut 命中:")
		for _, h := range scHits {
			risk := ""
			if h.Risk == "write" {
				risk = "(写操作)"
			}
			fmt.Printf("  %s %s  — %s %s\n", h.Service, h.Command, h.Description, risk)
			fmt.Printf("    用法: hr-cli %s %s --help\n", h.Service, h.Command)
		}
		fmt.Println()
	}
	if len(mHits) > 0 {
		fmt.Println("Service.method 命中:")
		for _, h := range mHits {
			fmt.Printf("  %s %s  — %s\n", h.Service, h.Command, h.Description)
			fmt.Printf("    用法: hr-cli %s %s --help\n", h.Service, h.Command)
		}
		fmt.Println()
	}
	fmt.Printf("共命中 %d 条。\n", len(hits))
	return nil
}
