package groupmanage

import (
	"context"
	"encoding/json"
	"fmt"

	"hr-cli/shortcuts/common"
)

// SearchTeacher 按姓名模糊查询教研组教师。
// 返回的教师按教研组分组,供用户选择(AI 会引导用户选)。
var SearchTeacher = common.Shortcut{
	Service:     "groupmanage",
	Command:     "+search-teacher",
	Method:      "member_get_lists",
	Description: "按姓名模糊查询教研组教师(返回教研组 + 教师列表)",
	Risk:        "read",
	Flags: []common.Flag{
		{Name: "school-id", Desc: "学校 id"},
		{Name: "campus-id", Desc: "校区 id"},
		{Name: "name", Desc: "教师姓名(支持模糊匹配)"},
		{Name: "teach-group-id", Desc: "教研组 id(可选,精确查某组)"},
	},
	BuildBody: buildSearchTeacherBody,
	After:     afterSearchTeacher,
}

func buildSearchTeacherBody(ctx context.Context, rt *common.RuntimeContext) (map[string]any, error) {
	body := map[string]any{
		"schoolId":     rt.Str("school-id"),
		"campusId":     rt.Str("campus-id"),
		"staffNameDim": rt.Str("name"),
	}
	if id := rt.Str("teach-group-id"); id != "" {
		body["teachGroupId"] = id
	}
	return body, nil
}

func afterSearchTeacher(ctx context.Context, rt *common.RuntimeContext, result map[string]any) error {
	groups, _ := result["teachGroupList"].([]any)

	// 统计总人数(跨教研组去重)
	seen := make(map[string]bool)
	for _, g := range groups {
		if group, ok := g.(map[string]any); ok {
			if staffList, ok := group["staffList"].([]any); ok {
				for _, s := range staffList {
					if staff, ok := s.(map[string]any); ok {
						if id, _ := staff["staffId"].(string); id != "" {
							seen[id] = true
						}
					}
				}
			}
		}
	}

	if len(seen) == 0 {
		fmt.Printf("未找到匹配 %q 的教师\n", rt.Str("name"))
		return nil
	}

	// 友好区:给用户看,不含技术 ID
	fmt.Printf("找到 %d 位匹配 %q 的教师:\n\n", len(seen), rt.Str("name"))
	for _, g := range groups {
		group, _ := g.(map[string]any)
		groupName, _ := group["teachGroupName"].(string)

		fmt.Printf("教研组: %s\n", groupName)

		staffList, _ := group["staffList"].([]any)
		for _, s := range staffList {
			staff, _ := s.(map[string]any)
			name, _ := staff["staffName"].(string)
			fmt.Printf("  - %s\n", name)
		}
		fmt.Println()
	}

	// internal 区:给 Claude 后续调用用,转述给用户时 MUST 省略
	// 只保留评课流程必需字段(staffId/teachGroupId/姓名),过滤 mobile 等隐私
	internalGroups := []map[string]any{}
	for _, g := range groups {
		group, _ := g.(map[string]any)
		grp := map[string]any{
			"teachGroupId":   group["teachGroupId"],
			"teachGroupName": group["teachGroupName"],
		}
		staffList, _ := group["staffList"].([]any)
		staffOut := []map[string]any{}
		for _, s := range staffList {
			staff, _ := s.(map[string]any)
			staffOut = append(staffOut, map[string]any{
				"staffId":   staff["staffId"],
				"staffName": staff["staffName"],
			})
		}
		grp["staffList"] = staffOut
		internalGroups = append(internalGroups, grp)
	}
	fmt.Println("[internal]")
	internal, _ := json.Marshal(map[string]any{"teachGroupList": internalGroups})
	fmt.Println(string(internal))

	return nil
}
