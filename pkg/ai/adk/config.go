package adk

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ConfigManager ADK 配置管理器
type ConfigManager struct {
	configDir    string
	agentConfig  *ADKAgentConfig
	skillConfigs map[string]*ADKSkillConfig
}

// ADKAgentConfig ADK Agent 配置（避免与 agent.go 中的 AgentConfig 冲突）
type ADKAgentConfig struct {
	// 系统配置
	System SystemConfig `yaml:"system"`

	// LLM 配置
	LLM LLMConfig `yaml:"llm"`

	// Agent 团队配置
	Agents []AgentItemConfig `yaml:"agents"`

	// 技能目录配置
	Skills SkillsDirConfig `yaml:"skills"`

	// 工作流配置
	Workflows []WorkflowConfig `yaml:"workflows"`

	// 内存配置
	Memory MemoryConfig `yaml:"memory"`

	// 事件配置
	Events EventsConfig `yaml:"events"`
}

// SystemConfig 系统配置
type SystemConfig struct {
	Name     string `yaml:"name"`
	Version  string `yaml:"version"`
	LogLevel string `yaml:"log_level"`
}

// LLMConfig LLM 配置
type LLMConfig struct {
	Provider    string  `yaml:"provider"`
	Model       string  `yaml:"model"`
	APIKey      string  `yaml:"api_key"`
	Temperature float64 `yaml:"temperature"`
	MaxTokens   int     `yaml:"max_tokens"`
}

// AgentItemConfig 单个 Agent 配置
type AgentItemConfig struct {
	Name        string                 `yaml:"name"`
	Type        string                 `yaml:"type"`
	Description string                 `yaml:"description"`
	Instruction string                 `yaml:"instruction"`
	Skills      []string               `yaml:"skills"`
	Specialty   string                 `yaml:"specialty"`
	Metadata    map[string]interface{} `yaml:"metadata"`
}

// SkillsDirConfig 技能目录配置
type SkillsDirConfig struct {
	Directory string           `yaml:"directory"`
	AutoLoad  bool             `yaml:"auto_load"`
	Installed []InstalledSkill `yaml:"installed"`
}

// InstalledSkill 已安装技能
type InstalledSkill struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Source  string `yaml:"source"`
}

// WorkflowConfig 工作流配置
type WorkflowConfig struct {
	Name        string         `yaml:"name"`
	Description string         `yaml:"description"`
	Steps       []WorkflowStep `yaml:"steps"`
}

// WorkflowStep 工作流步骤
type WorkflowStep struct {
	Agent  string `yaml:"agent"`
	Action string `yaml:"action"`
}

// MemoryConfig 内存配置
type MemoryConfig struct {
	Type       string `yaml:"type"`
	Path       string `yaml:"path"`
	MaxHistory int    `yaml:"max_history"`
}

// EventsConfig 事件配置
type EventsConfig struct {
	Enabled    bool `yaml:"enabled"`
	BufferSize int  `yaml:"buffer_size"`
}

// ADKSkillConfig 技能配置（避免与 skills.go 中的 SkillConfig 冲突）
type ADKSkillConfig struct {
	Name        string                 `yaml:"name"`
	Version     string                 `yaml:"version"`
	Description string                 `yaml:"description"`
	Type        string                 `yaml:"type"`
	Entry       string                 `yaml:"entry"`
	Parameters  []ParameterConfig      `yaml:"parameters"`
	Returns     []ReturnConfig         `yaml:"returns"`
	Config      map[string]interface{} `yaml:"config"`
}

// ParameterConfig 参数配置
type ParameterConfig struct {
	Name        string      `yaml:"name"`
	Type        string      `yaml:"type"`
	Description string      `yaml:"description"`
	Required    bool        `yaml:"required"`
	Default     interface{} `yaml:"default"`
}

// ReturnConfig 返回值配置
type ReturnConfig struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	Description string `yaml:"description"`
}

// NewConfigManager 创建配置管理器
func NewConfigManager(configDir string) *ConfigManager {
	return &ConfigManager{
		configDir:    configDir,
		skillConfigs: make(map[string]*ADKSkillConfig),
	}
}

// Init 初始化配置目录和默认配置文件
func (cm *ConfigManager) Init() error {
	// 创建配置目录
	if err := os.MkdirAll(cm.configDir, 0755); err != nil {
		return fmt.Errorf("创建 ADK 配置目录失败: %w", err)
	}

	// 创建技能目录
	skillsDir := filepath.Join(cm.configDir, "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return fmt.Errorf("创建技能目录失败: %w", err)
	}

	// 创建数据目录
	dataDir := filepath.Join(cm.configDir, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}

	// 初始化默认 Agent 配置
	agentConfigPath := filepath.Join(cm.configDir, "agent_config.yaml")
	if _, err := os.Stat(agentConfigPath); os.IsNotExist(err) {
		if err := cm.createDefaultAgentConfig(agentConfigPath); err != nil {
			return fmt.Errorf("创建默认 Agent 配置失败: %w", err)
		}
	}

	// 加载 Agent 配置
	if err := cm.LoadAgentConfig(agentConfigPath); err != nil {
		return fmt.Errorf("加载 Agent 配置失败: %w", err)
	}

	// 初始化示例技能
	exampleSkillDir := filepath.Join(skillsDir, "example-skill")
	if _, err := os.Stat(exampleSkillDir); os.IsNotExist(err) {
		if err := cm.createExampleSkill(exampleSkillDir); err != nil {
			return fmt.Errorf("创建示例技能失败: %w", err)
		}
	}

	return nil
}

// createDefaultAgentConfig 创建默认 Agent 配置文件
func (cm *ConfigManager) createDefaultAgentConfig(path string) error {
	config := &ADKAgentConfig{
		System: SystemConfig{
			Name:     "luffot-adk-system",
			Version:  "1.0.0",
			LogLevel: "info",
		},
		LLM: LLMConfig{
			Provider:    "google",
			Model:       "gemini-2.0-flash",
			APIKey:      "${GEMINI_API_KEY}",
			Temperature: 0.7,
			MaxTokens:   4096,
		},
		Agents: []AgentItemConfig{
			{
				Name:        "coordinator",
				Type:        "coordinator",
				Description: "负责任务协调和Agent调度",
			},
			{
				Name:        "planner",
				Type:        "planner",
				Description: "负责任务规划和策略制定",
				Instruction: `你是一个智能规划Agent。
你的职责:
1. 分析复杂任务，理解目标和约束
2. 将任务分解为可执行的步骤
3. 为每个步骤分配合适的Agent和技能
4. 识别步骤间的依赖关系
5. 生成详细的执行计划`,
			},
			{
				Name:        "executor",
				Type:        "executor",
				Description: "负责任务执行",
				Skills:      []string{"example-skill"},
			},
			{
				Name:        "reviewer",
				Type:        "reviewer",
				Description: "负责结果审查和质量控制",
			},
			{
				Name:        "code-expert",
				Type:        "specialist",
				Description: "代码领域专家",
				Specialty:   "code",
				Instruction: `你是代码专家，擅长代码审查、重构和优化。
你的专长包括:
- 代码质量评估
- 性能优化建议
- 安全漏洞检测
- 最佳实践指导`,
			},
		},
		Skills: SkillsDirConfig{
			Directory: filepath.Join(cm.configDir, "skills"),
			AutoLoad:  true,
			Installed: []InstalledSkill{
				{
					Name:    "example-skill",
					Version: "1.0.0",
					Source:  "builtin",
				},
			},
		},
		Workflows: []WorkflowConfig{
			{
				Name:        "code-review",
				Description: "代码审查工作流",
				Steps: []WorkflowStep{
					{Agent: "planner", Action: "analyze"},
					{Agent: "code-expert", Action: "review"},
					{Agent: "reviewer", Action: "validate"},
				},
			},
			{
				Name:        "task-execution",
				Description: "标准任务执行工作流",
				Steps: []WorkflowStep{
					{Agent: "planner", Action: "plan"},
					{Agent: "executor", Action: "execute"},
					{Agent: "reviewer", Action: "review"},
				},
			},
		},
		Memory: MemoryConfig{
			Type:       "sqlite",
			Path:       filepath.Join(cm.configDir, "data", "memory.db"),
			MaxHistory: 100,
		},
		Events: EventsConfig{
			Enabled:    true,
			BufferSize: 1000,
		},
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// createExampleSkill 创建示例技能
func (cm *ConfigManager) createExampleSkill(dir string) error {
	// 创建技能目录
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 创建 skill.yaml
	skillConfig := &ADKSkillConfig{
		Name:        "example-skill",
		Version:     "1.0.0",
		Description: "示例技能，展示如何配置和使用技能",
		Type:        "lua",
		Entry:       "main.lua",
		Parameters: []ParameterConfig{
			{
				Name:        "message",
				Type:        "string",
				Description: "输入消息",
				Required:    true,
			},
			{
				Name:        "count",
				Type:        "number",
				Description: "重复次数",
				Required:    false,
				Default:     1,
			},
		},
		Returns: []ReturnConfig{
			{
				Name:        "result",
				Type:        "string",
				Description: "处理结果",
			},
			{
				Name:        "status",
				Type:        "string",
				Description: "执行状态",
			},
		},
		Config: map[string]interface{}{
			"timeout": 30,
			"retry":   3,
		},
	}

	skillData, err := yaml.Marshal(skillConfig)
	if err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(dir, "skill.yaml"), skillData, 0644); err != nil {
		return err
	}

	// 创建 main.lua
	luaContent := `-- 示例 Lua 技能
-- 该脚本会被 ADK 引擎加载并执行

-- execute 函数是入口点，必须实现
-- @param input table 输入参数表
-- @return table 返回结果表
function execute(input)
    -- 获取输入参数
    local message = input.message or "Hello"
    local count = input.count or 1
    
    -- 处理逻辑
    local result = ""
    for i = 1, count do
        result = result .. message .. " "
    end
    
    -- 返回结果
    return {
        result = result:gsub("%s+$", ""),  -- 去除末尾空格
        status = "success",
        processed_at = os.date("%Y-%m-%d %H:%M:%S")
    }
end

-- 可选：初始化函数，在技能加载时调用
function init()
    print("Example skill initialized")
    return true
end

-- 可选：健康检查函数
function health_check()
    return {
        status = "healthy",
        timestamp = os.time()
    }
end
`

	if err := os.WriteFile(filepath.Join(dir, "main.lua"), []byte(luaContent), 0644); err != nil {
		return err
	}

	return nil
}

// LoadAgentConfig 加载 Agent 配置
func (cm *ConfigManager) LoadAgentConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	config := &ADKAgentConfig{}
	if err := yaml.Unmarshal(data, config); err != nil {
		return err
	}

	cm.agentConfig = config
	return nil
}

// SaveAgentConfig 保存 Agent 配置
func (cm *ConfigManager) SaveAgentConfig() error {
	if cm.agentConfig == nil {
		return fmt.Errorf("没有可保存的配置")
	}

	path := filepath.Join(cm.configDir, "agent_config.yaml")
	data, err := yaml.Marshal(cm.agentConfig)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// GetAgentConfig 获取 Agent 配置
func (cm *ConfigManager) GetAgentConfig() *ADKAgentConfig {
	return cm.agentConfig
}

// UpdateAgentConfig 更新 Agent 配置
func (cm *ConfigManager) UpdateAgentConfig(config *ADKAgentConfig) error {
	cm.agentConfig = config
	return cm.SaveAgentConfig()
}

// GetConfigDir 获取配置目录
func (cm *ConfigManager) GetConfigDir() string {
	return cm.configDir
}

// GetSkillsDir 获取技能目录
func (cm *ConfigManager) GetSkillsDir() string {
	if cm.agentConfig != nil && cm.agentConfig.Skills.Directory != "" {
		return cm.agentConfig.Skills.Directory
	}
	return filepath.Join(cm.configDir, "skills")
}

// GetMemoryPath 获取内存数据库路径
func (cm *ConfigManager) GetMemoryPath() string {
	if cm.agentConfig != nil && cm.agentConfig.Memory.Path != "" {
		return cm.agentConfig.Memory.Path
	}
	return filepath.Join(cm.configDir, "data", "memory.db")
}
