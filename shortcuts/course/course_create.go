package course

import (
	"context"
	"fmt"
	"github.com/TsCarpe/hr-cli/shortcuts/common"
	"strconv"
	"strings"
)

var Create = common.Shortcut{
	Service:     "course",
	Command:     "+create",
	Method:      "add",
	Description: "创建听评课邀请(智能默认 + 友好输出)",
	Risk:        "write",
	Flags: []common.Flag{
		{Name: "school-id", Desc: "学校 id"},
		{Name: "campus-id", Desc: "校区 id"},
		{Name: "teacher-id", Desc: "授课老师 id"},
		{Name: "teacher-name", Desc: "授课老师姓名"},
		{Name: "name", Desc: "课程名称"},
		{Name: "time", Desc: "授课时间,如 2026-06-28 00:00:00"},
		{Name: "node", Desc: "授课节次(数字,如 3)"},
		{Name: "node-display", Desc: "节次显示名,如 \"第三节\"", Default: ""},
		{Name: "course-tag-id", Desc: "课程标签 id"},
		{Name: "class-name", Desc: "授课班级"},
		{Name: "addr", Desc: "授课地址"},
		{Name: "listen-type", Desc: "听课方式", Default: "select"},
		{Name: "comment-paper-id", Desc: "评课量表 id", Default: "1"},
		{Name: "comment-paper-name", Desc: "评课量表名称", Default: "通用教师评课表"},
		{Name: "teach-group-id", Desc: "授课老师教研组 id"},
		{Name: "current-user-teach-group-id", Desc: "当前用户教研组 id"},
		{Name: "member-ids", Desc: "邀请人 id 列表(逗号分隔)"},
	},
	BuildBody: buildBody,
	After:     after,
}

func buildBody(ctx context.Context, rt *common.RuntimeContext) (map[string]any, error) {

	node, err := strconv.Atoi(rt.Str("node"))
	if err != nil {
		return nil, fmt.Errorf("node 格式非法: %q(应为数字,如 3)", rt.Str("node"))
	}

	body := map[string]any{
		// ===== 用户必传 flag =====
		"schoolId":    rt.Str("school-id"),
		"campusId":    rt.Str("campus-id"),
		"teacherId":   rt.Str("teacher-id"),
		"teacherName": rt.Str("teacher-name"),
		"name":        rt.Str("name"),
		"time":        rt.Str("time"),
		"node":        node, // 注意:YApi 是 integer,
		// 但传字符串 listen 也接受
		"courseTagId":             rt.Str("course-tag-id"),
		"className":               rt.Str("class-name"),
		"addr":                    rt.Str("addr"),
		"listenType":              rt.Str("listen-type"),
		"commentPaperId":          rt.Str("comment-paper-id"),
		"commentPaperName":        rt.Str("comment-paper-name"),
		"teachGroupId":            rt.Str("teach-group-id"),
		"currentUserTeachGroupId": rt.Str("current-user-teach-group-id"),

		// ===== 智能默认(M5 学到的) =====
		"isOutside":                   0,     // 默认非校外
		"appointmentRange":            "all", // 默认全校范围
		"outsideSchoolId":             "",    // 校外字段默认空
		"outsideSchoolName":           "",
		"teachPlanId":                 "",         // 教学计划默认空
		"courseInviteGroupAddReqList": []string{}, // 默认空数组
		"resourceList":                []any{},    // 默认空数组

		// ===== saas 字段自动复制 =====
		"saasSchoolId": rt.Str("school-id"), // = schoolId
		"saasCampusId": rt.Str("campus-id"), // = campusId
	}

	body["courseInviteMemberAddReqList"] = []string{}
	if mids := rt.Str("member-ids"); mids != "" {
		body["courseInviteMemberAddReqList"] = strings.Split(mids, ",")
	}

	return body, nil
}

func after(ctx context.Context, rt *common.RuntimeContext, result map[string]any) error {
	fmt.Println("✅ 邀课创建成功")
	fmt.Println()
	fmt.Printf("邀请码: %v\n", result["courseInviteCode"])
	fmt.Printf("(课程邀请已创建,授课老师和受邀评课人可凭此邀请码扫码评课)")
	return nil
}
