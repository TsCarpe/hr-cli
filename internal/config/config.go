package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ConfigFile 持久化登录态。
// 顶层 = 当前学校/校区上下文(切学校会变),userInfo = 用户身份(跨学校稳定)。
type ConfigFile struct {
	HrToken       string    `json:"hrToken,omitempty"`
	Authorization string    `json:"authorization,omitempty"` // saas 凭证(会过期,诊断用)
	TenantId      string    `json:"tenantId,omitempty"`      // 租户
	SchoolId      string    `json:"schoolId,omitempty"`      // 当前学校
	CampusId      string    `json:"campusId,omitempty"`      // 当前校区
	SchoolName    string    `json:"schoolName,omitempty"`    // 展示
	CampusName    string    `json:"campusName,omitempty"`    // 展示
	SavedAt       time.Time `json:"savedAt,omitempty"`
	UserInfo      UserInfo  `json:"userInfo,omitempty"`
}

// UserInfo 跨学校稳定的用户身份。
// staffId 跟学校走(切学校会变),但归属"身份侧";userId 跨学校不变。
type UserInfo struct {
	AccountName string `json:"accountName,omitempty"`
	StaffId     string `json:"staffId,omitempty"`
	UserId      string `json:"userId,omitempty"`
}

// ResolveToken 解析链:--token flag > LISTEN_TOKEN env > ~/.hr-cli/config.json
func ResolveToken(flagToken string) string {
	if flagToken != "" {
		return flagToken
	}
	if env := os.Getenv("LISTEN_TOKEN"); env != "" {
		return env
	}
	if cfg, _ := LoadConfig(); cfg != nil && cfg.HrToken != "" {
		return cfg.HrToken
	}
	return ""
}

// ResolveSaasAuth 解析 saas 登录用 token。
// 仅来源:~/.haiclaw/saas-config.json 的 saasToken(由 haiclaw 工具生成)。
func ResolveSaasAuth() string {
	return readHaiclawSaasToken()
}

// readHaiclawSaasToken 读 ~/.haiclaw/saas-config.json 的 saasToken 字段。
// 文件不存在 / 解析失败 / 字段空 → 返回空串。
// 静默不打 log:此函数高频调用(internal/runner 每个 Authorization 请求都走)。
func readHaiclawSaasToken() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	path := filepath.Join(home, ".haiclaw", "saas-config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var cfg struct {
		SaasToken string `json:"saasToken"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ""
	}
	return cfg.SaasToken
}

// ProjectConfig 项目级配置(部署相关,跟用户身份分离)。
// 路径:<repo-root>/.hr-cli.json,gitignore。
// 跟 ConfigFile(登录态,~/.hr-cli/config.json)物理分离。
type ProjectConfig struct {
	BaseURL string `json:"baseURL,omitempty"`
}

// DefaultBaseURL 是 cobra flag --base-url 的默认值,所有兜底失败时用它。
const DefaultBaseURL = "http://localhost:8080"

// LoadProjectConfig 从当前工作目录向上递归查找 .hr-cli.json。
// 找不到返回 nil + nil error(视为未配置)。
func LoadProjectConfig() (*ProjectConfig, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("无法定位当前目录: %w", err)
	}
	for {
		path := filepath.Join(dir, ".hr-cli.json")
		data, err := os.ReadFile(path)
		if err == nil {
			var cfg ProjectConfig
			if err := json.Unmarshal(data, &cfg); err != nil {
				return nil, fmt.Errorf("解析项目配置失败 %s: %w", path, err)
			}
			return &cfg, nil
		}
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("读取项目配置失败 %s: %w", path, err)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, nil // 到达文件系统根
		}
		dir = parent
	}
}

// ResolveBaseURL 解析链: --base-url flag > HR_CLI_BASE_URL env > <repo>/.hr-cli.json > DefaultBaseURL
// 注意:cobra flag 的默认值必须是 ""(见 cmd/global_flags.go),否则无法区分「用户未传」vs「显式传了默认值」。
func ResolveBaseURL(flagBaseURL string) string {
	if flagBaseURL != "" {
		return flagBaseURL
	}
	if env := os.Getenv("HR_CLI_BASE_URL"); env != "" {
		return env
	}
	if cfg, _ := LoadProjectConfig(); cfg != nil && cfg.BaseURL != "" {
		return cfg.BaseURL
	}
	return DefaultBaseURL
}

// ConfigPath 返回 ~/.hr-cli/config.json 的绝对路径。
func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("无法定位用户目录: %w", err)
	}
	return filepath.Join(home, ".hr-cli", "config.json"), nil
}

// LoadConfig 读取配置文件。文件不存在返回 nil + nil error(视为未登录)。
func LoadConfig() (*ConfigFile, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取配置失败: %w", err)
	}
	var cfg ConfigFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	return &cfg, nil
}

// SaveConfig 写入完整 ConfigFile 到 ~/.hr-cli/config.json(权限 0600)。
// 写入失败不阻断登录成功(降级为只输出 token,提示用户手动 export)。
func SaveConfig(cfg ConfigFile) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}
	cfg.SavedAt = time.Now()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("写入配置失败: %w", err)
	}
	return nil
}
