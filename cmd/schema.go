package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"hr-cli/internal/registry"
	"sort"
	"strings"
)

func NewCmdSchema() *cobra.Command {
	return &cobra.Command{
		Use:   "schema [service[.method]]",
		Short: "查看 service / method 的参数结构",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSchema(args)
		},
	}
}

func runSchema(args []string) error {
	reg, err := registry.Load()
	if err != nil {
		return err
	}

	switch {
	case len(args) == 0:
		return listServices(reg)
	case !strings.Contains(args[0], "."):
		// 不含 ".",是 service 名:hr-cli schema course
		return listMethods(reg, args[0])
	default:
		// 含 ".",是 service.method:hr-cli schema course.add
		parts := strings.SplitN(args[0], ".", 2)
		if len(parts) < 2 || parts[1] == "" {
			return fmt.Errorf("invalid argument: %q, expected service.method (e.g. course.add)", args[0])
		}
		return showMethodSchema(reg, parts[0], parts[1])
	}
}

func listServices(reg *registry.Registry) error {
	fmt.Println("services:")
	for _, svc := range reg.Services {
		fmt.Printf("  - %s (%s): %d methods\n", svc.Name, svc.Title, len(svc.Methods))
	}
	return nil
}

func listMethods(reg *registry.Registry, svcName string) error {
	svc := findService(reg, svcName)
	if svc == nil {
		return fmt.Errorf("service not found: %s", svcName)
	}

	fmt.Printf("service: %s (%s)\n", svc.Name, svc.Title)
	fmt.Printf("basePath: %s\n", svc.BasePath)
	fmt.Println("methods:")

	names := make([]string, 0, len(svc.Methods))
	for name := range svc.Methods {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		mtd := svc.Methods[name]
		fmt.Printf("  - %s (%s) [%s, auth=%v]\n", mtd.Name, mtd.Description, mtd.Risk, mtd.RequiresAuth)
	}

	return nil
}

func findService(reg *registry.Registry, svcName string) *registry.Service {
	for _, svc := range reg.Services {
		if svc.Name == svcName {
			return svc
		}
	}

	return nil
}

func showMethodSchema(reg *registry.Registry, svcName, mtdName string) error {
	svc := findService(reg, svcName)
	if svc == nil {
		return fmt.Errorf("service not found: %s", svcName)
	}

	mtd := svc.Methods[mtdName]
	if mtd == nil {
		return fmt.Errorf("method not found: %s", mtdName)
	}

	fmt.Printf("method: %s\n", mtd.Name)
	fmt.Printf("%s %s%s\n", mtd.HTTPMethod, svc.BasePath, mtd.Path)
	fmt.Printf("risk: %s | requiresAuth: %v\n\n", mtd.Risk, mtd.RequiresAuth)

	if mtd.RequestSchema != nil {
		fmt.Println("request body (" + mtd.RequestSchema.Type + "):")
		printSchema(mtd.RequestSchema, 1)
	}

	if mtd.ResponseSchema != nil {
		fmt.Println("\nresponse (" + mtd.ResponseSchema.Type + "):")
		if mtd.ResponseSchema.Description != "" {
			fmt.Printf("  %s\n", mtd.ResponseSchema.Description)
		}
		printSchema(mtd.ResponseSchema, 1)
	}

	return nil
}

func printSchema(s *registry.Schema, indent int) {
	if s == nil || s.Properties == nil {
		return
	}

	prefix := strings.Repeat("  ", indent)

	// 先排序 properties(保证输出顺序稳定,便于人读)
	names := make([]string, 0, len(s.Properties))
	for name := range s.Properties {
		names = append(names, name)
	}
	sort.Strings(names)

	requiredSet := make(map[string]bool)
	for _, r := range s.Required {
		requiredSet[r] = true
	}

	for _, name := range names {
		field := s.Properties[name]
		requiredMark := ""
		if requiredSet[name] {
			requiredMark = "  [required]"
		}

		fmt.Printf("%s- %s  %s%s  %s\n", prefix, name, field.Type, requiredMark, field.Description)

		// 枚举展示
		if len(field.Enum) > 0 {
			// 拼成 "select(选人) | appointment(预约) | other(其他)"
			pairs := make([]string, len(field.Enum))
			for i, v := range field.Enum {
				label := ""
				if i < len(field.EnumLabels) {
					label = field.EnumLabels[i]
				}
				if label != "" {
					pairs[i] = fmt.Sprintf("%s(%s)", v, label)
				} else {
					pairs[i] = v
				}
			}
			fmt.Printf("%s  enum: %s\n", prefix, strings.Join(pairs, " | "))
		}

		// 嵌套:数组递归打印 items
		if field.Type == "array" && field.Items != nil {
			fmt.Printf("%s  items (%s):\n", prefix, field.Items.Type)
			printSchema(field.Items, indent+2) // ← 递归
		}
	}

}
