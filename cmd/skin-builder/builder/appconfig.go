package builder

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// AppConfig skin-builder 程序配置
type AppConfig struct {
	RemoveBG RemoveBGConfig `yaml:"removebg"`
}

// RemoveBGConfig remove.bg API 配置
type RemoveBGConfig struct {
	// APIKey remove.bg 的 API Key，在 https://www.remove.bg/api 申请
	APIKey string `yaml:"api_key"`
	// BaseURL API 基础地址，留空使用默认值
	BaseURL string `yaml:"base_url"`
}

// defaultRemoveBGBaseURL remove.bg 默认 API 地址
const defaultRemoveBGBaseURL = "https://api.remove.bg/v1.0"

// EffectiveBaseURL 返回有效的 API 基础地址（空时使用默认值）
func (c *RemoveBGConfig) EffectiveBaseURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	return defaultRemoveBGBaseURL
}

// IsConfigured 返回 remove.bg 是否已配置 API Key
func (c *RemoveBGConfig) IsConfigured() bool {
	return c.APIKey != ""
}

// LoadAppConfig 从指定路径加载配置文件，文件不存在时返回空配置（不报错）
func LoadAppConfig(configPath string) (*AppConfig, error) {
	cfg := &AppConfig{}

	data, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		// 配置文件不存在，返回空配置
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败：%w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败：%w", err)
	}

	return cfg, nil
}

// DefaultConfigPath 返回默认配置文件路径（与可执行文件同目录）
func DefaultConfigPath() string {
	// 优先查找可执行文件同目录
	execPath, err := os.Executable()
	if err == nil {
		candidate := filepath.Join(filepath.Dir(execPath), "skin-builder.yaml")
		if _, statErr := os.Stat(candidate); statErr == nil {
			return candidate
		}
	}
	// 回退到当前工作目录下的 cmd/skin-builder/skin-builder.yaml
	return filepath.Join("cmd", "skin-builder", "skin-builder.yaml")
}
