package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestDo_Success 验证 200 业务成功场景
func TestDo_Success(t *testing.T) {
	// 起一个 mock listen 服务,返回标准 ResultJson(200)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求路径
		if r.URL.Path != "/listen/v1/course/invite/add" {
			t.Errorf("意外路径: %s", r.URL.Path)
		}
		// 验证 hrToken header
		if r.Header.Get("hrToken") != "test-token" {
			t.Errorf("hrToken header 缺失或错误: %q", r.Header.Get("hrToken"))
		}
		// 验证 Content-Type
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type 错误: %q", r.Header.Get("Content-Type"))
		}

		// 返回 listen 风格的 ResultJson
		resp := map[string]interface{}{
			"status":  200,
			"message": "成功",
			"success": true,
			"data": map[string]interface{}{
				"courseInviteId":   "invite-123",
				"courseInviteCode": "code-abc",
				"courseInfoId":     "info-456",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := NewClient(server.URL)
	status, data, err := c.Do(context.Background(), "POST", "/listen/v1/course/invite/add",
		[]byte(`{"schoolId":"1"}`), "hrToken", "test-token")

	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
	if status != 200 {
		t.Errorf("业务状态码错误: 得到 %d, 期望 200", status)
	}

	// 验证 data 字段
	var resp map[string]interface{}
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("解析 data 失败: %v", err)
	}
	if resp["courseInviteId"] != "invite-123" {
		t.Errorf("courseInviteId 错误: %v", resp["courseInviteId"])
	}
}

// TestDo_Unauthorized 验证 401 业务未登录场景
// 业务失败(status != 200)时 client 返回 error(含后端 message),
// 上层靠 error 判断失败,不会尝试解析 data。
func TestDo_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"status":  401,
			"message": "未登录",
			"success": false,
			"data":    nil,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := NewClient(server.URL)
	status, _, err := c.Do(context.Background(), "POST", "/test", []byte(`{}`), "hrToken", "")

	if err == nil {
		t.Fatal("期望返回 error(业务失败 status=401),实际 nil")
	}
	if status != 401 {
		t.Errorf("状态码错误: 得到 %d, 期望 401", status)
	}
}

// TestDo_NoToken_StillWorks 验证不传 token 时不报错(由上层决定是否需要)
func TestDo_NoToken_StillWorks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// mock 不检查 token,直接返回 200
		fmt.Fprintf(w, `{"status":200,"message":"ok","success":true,"data":null}`)
	}))
	defer server.Close()

	c := NewClient(server.URL)
	status, _, err := c.Do(context.Background(), "GET", "/test", nil, "hrToken", "")

	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
	if status != 200 {
		t.Errorf("状态码错误: 得到 %d, 期望 200", status)
	}
}

// TestDo_ServerError 验证 HTTP 5xx 场景
func TestDo_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer server.Close()

	c := NewClient(server.URL)
	_, _, err := c.Do(context.Background(), "POST", "/test", []byte(`{}`), "hrToken", "tok")

	if err == nil {
		t.Fatal("期望返回错误,实际 nil")
	}
}
