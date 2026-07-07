package saas

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/TsCarpe/hr-cli/internal/config"
	"github.com/TsCarpe/hr-cli/shortcuts/common"
)

// Login 是 saas +login shortcut。
//
// 流程(混合调用):
//
//	步骤1: net/http 裸调 saas-auth tokenAuth → 拿 userId + accountName
//	步骤2: runMethod("saas","app_school_campus_get_lists") → 拿 schools[]
//	步骤3: 用户选择学校/校区(多个时列出)
//	步骤4: 框架自动调 runMethod("saas","login", body) ← BuildBody 返回 login body
//	步骤5: After 里 SaveToken + 输出
//
// 注意:BuildBody 里调 rt.runMethod 是合法的(调不同 method,不会递归 BuildBody)。
var Login = common.Shortcut{
	Service:     "saas",
	Command:     "+login",
	Method:      "login",
	Description: "saas 登录(haiclaw saas token → 自动换 hrToken 并持久化)",
	Risk:        "write",
	Flags: []common.Flag{
		{Name: "saas-url", Desc: "saas-auth 服务地址", Default: "http://10.30.5.53:31759"},
		{Name: "school-id", Desc: "学校 id(可选,跳过选择)"},
		{Name: "campus-id", Desc: "校区 id(可选,跳过选择)"},
	},
	BuildBody: buildBody,
	After:     after,
}

const defaultAppID = "6874934855898312704"

// tokenAuth 调 saas-auth 服务,用 Authorization 换 userId + accountName。
// 返回结构是 saas 的 CommonResult(不是 lesson 的 ResultJson),所以单独解析。
func tokenAuth(ctx context.Context, saasURL, authorization string) (userID, accountName string, err error) {
	body := map[string]string{"token": authorization}
	bodyBytes, _ := json.Marshal(body)

	url := strings.TrimRight(saasURL, "/") + "/saas-auth/sso/channel/token/auth"
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return "", "", fmt.Errorf("构造 tokenAuth 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("tokenAuth 请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("读取 tokenAuth 响应失败: %w", err)
	}
	if resp.StatusCode >= 500 {
		return "", "", fmt.Errorf("saas-auth 服务错误(HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	// CommonResult 结构: {success, status, data: {authenticationResource: {ucUserInfoVO: {id, name}}}}
	// 注意:id 是大整数(如 7040560941061681152),用 json.Number 保留原始字符串,
	// 避免 float64 解析导致精度丢失(变成 7.04e+18),后端会拒绝("userId 必须为数字")。
	var cr struct {
		Success bool `json:"success"`
		Status  int  `json:"status"`
		Data    struct {
			AuthenticationResource struct {
				UcUserInfoVO struct {
					ID   json.Number `json:"id"`
					Name string      `json:"name"`
				} `json:"ucUserInfoVO"`
			} `json:"authenticationResource"`
		} `json:"data"`
	}
	dec := json.NewDecoder(bytes.NewReader(respBody))
	dec.UseNumber()
	if err := dec.Decode(&cr); err != nil {
		return "", "", fmt.Errorf("解析 tokenAuth 响应失败: %w (原始: %s)", err, string(respBody))
	}
	if !cr.Success || cr.Status != 200 {
		return "", "", fmt.Errorf("tokenAuth 失败(status=%d): %s", cr.Status, string(respBody))
	}

	userID = cr.Data.AuthenticationResource.UcUserInfoVO.ID.String()
	accountName = cr.Data.AuthenticationResource.UcUserInfoVO.Name
	if userID == "" {
		return "", "", fmt.Errorf("tokenAuth 返回的 userId 为空(原始: %s)", string(respBody))
	}
	return userID, accountName, nil
}

func buildBody(ctx context.Context, rt *common.RuntimeContext) (map[string]any, error) {
	// 步骤1: 读 saas token(来源:~/.haiclaw/saas-config.json,由 haiclaw 工具生成)
	authorization := config.ResolveSaasAuth()
	if authorization == "" {
		return nil, fmt.Errorf("未找到 saas token:请先用 haiclaw 工具登录生成 ~/.haiclaw/saas-config.json")
	}

	saasURL := rt.Str("saas-url")
	if saasURL == "" {
		saasURL = "http://10.30.5.53:31759"
	}

	// 步骤1: 裸调 tokenAuth 换 userId
	userID, accountName, err := tokenAuth(ctx, saasURL, authorization)
	if err != nil {
		return nil, err
	}
	fmt.Printf("✓ 身份验证通过: %s (userId: %s)\n", accountName, userID)

	// 步骤2: 走元数据调 app_school_campus_get_lists
	listResult, err := rt.RunMethod("saas", "app_school_campus_get_lists", map[string]any{
		"userId":    userID,
		"saasAppId": defaultAppID,
	})
	if err != nil {
		return nil, fmt.Errorf("查学校校区失败: %w", err)
	}

	// 步骤3: 解析 schools[],让用户选择(或用 --school-id/--campus-id 跳过)
	schoolID, campusID, staffID, tenantID, schoolName, campusName, err := selectSchoolCampus(rt, listResult.Data)
	if err != nil {
		return nil, err
	}

	// 把登录后要展示的信息暂存到 RuntimeContext(通过 flags 传)
	rt.SetFlag("__accountName", accountName)
	rt.SetFlag("__schoolName", schoolName)
	rt.SetFlag("__campusName", campusName)
	rt.SetFlag("__staffId", staffID)
	rt.SetFlag("__userId", userID)
	rt.SetFlag("__tenantId", tenantID)
	rt.SetFlag("__schoolId", schoolID)
	rt.SetFlag("__campusId", campusID)
	rt.SetFlag("__authorization", authorization)

	// 步骤4: 返回 login 的 body,框架会自动调 runMethod("saas","login", body)
	return map[string]any{
		"authorization": authorization,
		"staffId":       staffID,
		"tenantId":      tenantID,
		"schoolId":      schoolID,
		"campusId":      campusID,
	}, nil
}

// selectSchoolCampus 从 schools[] 提取并选择学校/校区。
// 用户传了 --school-id/--campus-id 时直接用,否则按列表选择。
type campusInfo struct {
	ID, Name string
}

type schoolInfo struct {
	ID, Name, StaffID, TenantID string
	Campuses                    []campusInfo
}

func selectSchoolCampus(rt *common.RuntimeContext, data map[string]any) (schoolID, campusID, staffID, tenantID, schoolName, campusName string, err error) {
	schoolsRaw, ok := data["schools"].([]any)
	if !ok || len(schoolsRaw) == 0 {
		err = fmt.Errorf("未找到任何学校,请联系管理员分配学校权限")
		return
	}

	var schools []schoolInfo
	for _, s := range schoolsRaw {
		sm, ok := s.(map[string]any)
		if !ok {
			continue
		}
		si := schoolInfo{
			ID:       toString(sm["schoolId"]),
			Name:     toString(sm["schoolName"]),
			StaffID:  toString(sm["staffId"]),
			TenantID: toString(sm["tenantId"]),
		}
		if camps, ok := sm["campus"].([]any); ok {
			for _, c := range camps {
				if cm, ok := c.(map[string]any); ok {
					si.Campuses = append(si.Campuses, campusInfo{
						ID:   toString(cm["campusId"]),
						Name: toString(cm["campusName"]),
					})
				}
			}
		}
		schools = append(schools, si)
	}

	// 用户传了 --school-id 直接匹配
	if want := rt.Str("school-id"); want != "" {
		for _, s := range schools {
			if s.ID == want {
				schoolID, schoolName, staffID, tenantID = s.ID, s.Name, s.StaffID, s.TenantID
				if wantCampus := rt.Str("campus-id"); wantCampus != "" {
					for _, c := range s.Campuses {
						if c.ID == wantCampus {
							campusID, campusName = c.ID, c.Name
							return
						}
					}
					err = fmt.Errorf("校区 %s 不在学校 %s 内", wantCampus, s.Name)
					return
				}
				if len(s.Campuses) == 1 {
					campusID, campusName = s.Campuses[0].ID, s.Campuses[0].Name
				}
				return
			}
		}
		err = fmt.Errorf("未找到 school-id=%s 的学校", want)
		return
	}

	// 多学校/多校区选择逻辑
	if len(schools) == 1 {
		s := schools[0]
		schoolID, schoolName, staffID, tenantID = s.ID, s.Name, s.StaffID, s.TenantID
		if len(s.Campuses) <= 1 {
			if len(s.Campuses) == 1 {
				campusID, campusName = s.Campuses[0].ID, s.Campuses[0].Name
			}
			return
		}
		var idx int
		idx, err = promptSelect(fmt.Sprintf("选择 %s 的校区", s.Name), campusItems(s.Campuses), campusInternalJSON(s.Campuses))
		if err != nil {
			return
		}
		campusID, campusName = s.Campuses[idx].ID, s.Campuses[idx].Name
		return
	}

	// 多学校:先列学校让用户选,再处理校区
	items := make([]string, len(schools))
	for i, s := range schools {
		items[i] = fmt.Sprintf("%s(校区 %d 个)", s.Name, len(s.Campuses))
	}
	var sIdx int
	sIdx, err = promptSelect("学校", items, schoolInternalJSON(schools))
	if err != nil {
		return
	}
	s := schools[sIdx]
	schoolID, schoolName, staffID, tenantID = s.ID, s.Name, s.StaffID, s.TenantID

	if len(s.Campuses) <= 1 {
		if len(s.Campuses) == 1 {
			campusID, campusName = s.Campuses[0].ID, s.Campuses[0].Name
		}
		return
	}

	var cIdx int
	cIdx, err = promptSelect(fmt.Sprintf("选择 %s 的校区", s.Name), campusItems(s.Campuses), campusInternalJSON(s.Campuses))
	if err != nil {
		return
	}
	campusID, campusName = s.Campuses[cIdx].ID, s.Campuses[cIdx].Name
	return
}

func campusItems(cs []campusInfo) []string {
	items := make([]string, len(cs))
	for i, c := range cs {
		items[i] = c.Name
	}
	return items
}

// campusInternalJSON 把每个校区序列化成 JSON 对象字符串,供 [internal] 区使用。
func campusInternalJSON(cs []campusInfo) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		b, _ := json.Marshal(map[string]string{"campusId": c.ID, "campusName": c.Name})
		out[i] = string(b)
	}
	return out
}

// schoolInternalJSON 把每个学校序列化成 JSON 对象字符串,供 [internal] 区使用。
func schoolInternalJSON(ss []schoolInfo) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		b, _ := json.Marshal(map[string]string{
			"schoolId":   s.ID,
			"schoolName": s.Name,
			"staffId":    s.StaffID,
			"tenantId":   s.TenantID,
		})
		out[i] = string(b)
	}
	return out
}

// promptSelect 把候选项列成编号让用户选,返回选中的下标。
// 单候选项时直接返回 0(不读 stdin)。
// 友好区打名字(items);[internal] 区输出 JSON 数组(internalJSON 每项是
// 调用方序列化好的对象字符串),对齐 groupmanage +search-teacher 的范式,
// 供 AI agent 后续按用户选的序号直接反查到 ID。
func promptSelect(label string, items []string, internalJSON []string) (int, error) {
	if len(items) == 1 {
		return 0, nil
	}
	fmt.Printf("\n%s:\n", label)
	for i, it := range items {
		fmt.Printf("  %d. %s\n", i+1, it)
	}
	fmt.Print("\n[internal]\n")
	// 扁平结构:{"index":N,"label":"...","schoolId":...,...}。
	// - 扁平而非嵌套:对齐 groupmanage +search-teacher 的范式,AI 写 jq 更短。
	// - index 必备:AI 按"用户选 3"直接定位到数组第 3 项。
	// - label 留着:让 AI 能校验选中的是哪个中文名,防错配。
	// - 字段驼峰:与后端 API + groupmanage 一致。
	out := make([]map[string]any, len(items))
	for i := range items {
		var obj map[string]any
		_ = json.Unmarshal([]byte(internalJSON[i]), &obj)
		obj["index"] = i + 1
		obj["label"] = items[i]
		out[i] = obj
	}
	b, _ := json.Marshal(out)
	fmt.Println(string(b))
	fmt.Print("\n回复序号: ")

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return 0, fmt.Errorf("读取选择失败: %w", err)
	}
	line = strings.TrimSpace(line)
	var idx int
	if _, err := fmt.Sscanf(line, "%d", &idx); err != nil || idx < 1 || idx > len(items) {
		return 0, fmt.Errorf("无效的序号: %q(应在 1-%d 之间)", line, len(items))
	}
	return idx - 1, nil
}

func after(ctx context.Context, rt *common.RuntimeContext, result map[string]any) error {
	hrToken, _ := result["hrToken"].(string)
	if hrToken == "" {
		return fmt.Errorf("login 返回的 hrToken 为空: %v", result)
	}

	cfg := config.ConfigFile{
		HrToken:       hrToken,
		Authorization: rt.Str("__authorization"),
		TenantId:      rt.Str("__tenantId"),
		SchoolId:      rt.Str("__schoolId"),
		CampusId:      rt.Str("__campusId"),
		SchoolName:    rt.Str("__schoolName"),
		CampusName:    rt.Str("__campusName"),
		UserInfo: config.UserInfo{
			AccountName: rt.Str("__accountName"),
			StaffId:     rt.Str("__staffId"),
			UserId:      rt.Str("__userId"),
		},
	}

	fmt.Println("✅ 登录成功")
	fmt.Printf("账号: %s\n", cfg.UserInfo.AccountName)
	fmt.Printf("学校: %s / %s\n", cfg.SchoolName, cfg.CampusName)

	if err := config.SaveConfig(cfg); err != nil {
		fmt.Printf("⚠ 持久化失败(%v),请手动 export:\n", err)
		fmt.Printf("export LISTEN_TOKEN=%q\n", hrToken)
		return nil
	}
	fmt.Println("hrToken 已保存到 ~/.hr-cli/config.json,后续命令自动使用")
	return nil
}

func toString(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}
