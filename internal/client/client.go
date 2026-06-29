package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
}

func NewClient(baseURL string) *Client {

	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: time.Second * 30}}
}

// Do 发送 HTTP 请求并解析 ResultJson。
// authHeader 指定认证 header 名:默认 "hrToken";saas 接口传 "Authorization"。
func (c *Client) Do(ctx context.Context, method, path string, body []byte, authHeader, token string) (int, json.RawMessage, error) {

	url := c.baseURL + path

	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return 0, nil, fmt.Errorf("构造请求失败: %w", err)
	}

	headerName := authHeader
	if headerName == "" {
		headerName = "hrToken"
	}
	if token != "" {
		req.Header.Set(headerName, token)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("读取响应失败: %w", err)
	}
	if resp.StatusCode >= http.StatusInternalServerError {
		return resp.StatusCode, nil, fmt.Errorf("服务器错误(HTTP:%d):%s", resp.StatusCode, string(respBody))
	}
	var result ResultJson
	if err := json.Unmarshal(respBody, &result); err != nil {
		return resp.StatusCode, nil, fmt.Errorf("解析响应体失败: %w, (原始响应体: %s)", err, string(respBody))
	}

	// 业务失败时,把 message 也带回去(便于上层诊断)
	if result.Status != 200 {
		return result.Status, result.Data, fmt.Errorf("业务失败(status=%d): %s", result.Status, result.Message)
	}

	return result.Status, result.Data, nil
}
