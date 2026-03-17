package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"log"

	"gopkg.in/yaml.v3"
)

// AppConfig 应用配置
type AppConfig struct {
	// 通用配置
	General GeneralConfig `yaml:"general" json:"general"`

	// 存储配置
	Storage StorageConfig `yaml:"storage" json:"storage"`

	// Web UI 配置
	Web WebConfig `yaml:"web" json:"web"`

	// AI 配置
	AI AIConfig `yaml:"ai" json:"ai"`

	// Langfuse 配置
	Langfuse LangfuseConfig `yaml:"langfuse" json:"langfuse"`

	// 告警配置
	Alert AlertConfig `yaml:"alert" json:"alert"`

	// 摄像头背后守卫配置
	CameraGuard CameraGuardConfig `yaml:"camera_guard" json:"camera_guard"`

	// 智能消息分析器配置
	IntelliAnalyzer IntelliAnalyzerConfig `yaml:"intelli_analyzer" json:"intelli_analyzer"`

	// 定时任务配置
	ScheduledTasks ScheduledTasksConfig `yaml:"scheduled_tasks" json:"scheduled_tasks"`

	// 弹幕配置
	Barrage BarrageConfig `yaml:"barrage" json:"barrage"`

	// 响应式AI链路配置
	ReactiveAI ReactiveAIConfig `yaml:"reactive_ai" json:"reactive_ai"`

	// 桌宠皮肤名称（空字符串表示使用默认经典皮肤）
	PetSkin string `yaml:"pet_skin" json:"pet_skin"`

	// 监听的应用列表
	Apps []AppConfigItem `yaml:"apps" json:"apps"`
}

// ReactiveAIConfig 响应式AI链路配置
type ReactiveAIConfig struct {
	// 是否启用响应式AI链路
	Enabled bool `yaml:"enabled" json:"enabled"`

	// 系统管家开关
	SystemGuardianEnabled bool `yaml:"system_guardian_enabled" json:"system_guardian_enabled"`

	// 用户习惯记录员开关
	UserTrackerEnabled bool `yaml:"user_tracker_enabled" json:"user_tracker_enabled"`

	// 摄像头巡查员开关
	CameraPatrolEnabled bool `yaml:"camera_patrol_enabled" json:"camera_patrol_enabled"`

	// 协调器洞察生成间隔（秒），默认60
	CoordinatorInterval int `yaml:"coordinator_interval" json:"coordinator_interval"`

	// AI 丞相汇报策略配置
	CoordinatorStrategy *CoordinatorReportStrategy `yaml:"coordinator_strategy" json:"coordinator_strategy"`

	// 应用秘书汇报策略配置
	SecretaryStrategy *SecretaryReportStrategy `yaml:"secretary_strategy" json:"secretary_strategy"`
}

// CoordinatorReportStrategy AI 丞相汇报策略配置
type CoordinatorReportStrategy struct {
	// 是否启用 AI 总结
	EnableAISummary bool `yaml:"enable_ai_summary" json:"enable_ai_summary"`
	// 最小汇报间隔（秒）
	MinReportInterval int `yaml:"min_report_interval" json:"min_report_interval"`
	// 最大连续汇报次数（防刷屏）
	MaxConsecutiveReports int `yaml:"max_consecutive_reports" json:"max_consecutive_reports"`
	// 连续汇报冷却时间（秒）
	ConsecutiveCooldown int `yaml:"consecutive_cooldown" json:"consecutive_cooldown"`
	// 高优先级事件立即汇报
	UrgentImmediate bool `yaml:"urgent_immediate" json:"urgent_immediate"`
	// 批量事件聚合窗口（秒）
	BatchWindow int `yaml:"batch_window" json:"batch_window"`
}

// SecretaryReportStrategy 应用秘书汇报策略配置
type SecretaryReportStrategy struct {
	// 是否启用 AI 总结
	EnableAISummary bool `yaml:"enable_ai_summary" json:"enable_ai_summary"`
	// 紧急消息立即汇报
	UrgentImmediate bool `yaml:"urgent_immediate" json:"urgent_immediate"`
	// 普通消息批量汇报间隔（秒）
	BatchInterval int `yaml:"batch_interval" json:"batch_interval"`
	// 最小汇报优先级（low/normal/high/urgent）
	MinReportPriority string `yaml:"min_report_priority" json:"min_report_priority"`
	// 同一发送者消息合并
	MergeSameSender bool `yaml:"merge_same_sender" json:"merge_same_sender"`
	// 会话去重（同一会话只汇报最新）
	SessionDeduplicate bool `yaml:"session_deduplicate" json:"session_deduplicate"`
}

// ScheduledTasksConfig 定时任务总配置
type ScheduledTasksConfig struct {
	// 是否启用定时任务调度器
	Enabled bool `yaml:"enabled" json:"enabled"`

	// 任务列表
	Tasks []ScheduledTaskConfig `yaml:"tasks" json:"tasks"`
}

// ScheduledTaskType 任务执行类型
type ScheduledTaskType string

const (
	// ScheduledTaskTypeBuiltin 内置任务（由 Go 代码实现）
	ScheduledTaskTypeBuiltin ScheduledTaskType = "builtin"
	// ScheduledTaskTypePython Python 脚本任务
	ScheduledTaskTypePython ScheduledTaskType = "python"
)

// ScheduledTaskConfig 单个定时任务配置
type ScheduledTaskConfig struct {
	// 任务唯一名称，用于 API 触发和日志标识
	Name string `yaml:"name" json:"name"`

	// 任务描述（可选）
	Description string `yaml:"description" json:"description"`

	// 是否启用该任务
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Cron 表达式，例如："0 10 * * *"（每天早上10点）
	// 支持标准 5 段 cron：分 时 日 月 周
	Cron string `yaml:"cron" json:"cron"`

	// 任务类型：builtin（内置）或 python（Python 脚本）
	Type ScheduledTaskType `yaml:"type" json:"type"`

	// 内置任务名称（type=builtin 时使用）
	// 目前支持：daily_profile_report（每日画像生成）
	BuiltinName string `yaml:"builtin_name" json:"builtin_name"`

	// Python 脚本路径（type=python 时使用）
	ScriptPath string `yaml:"script_path" json:"script_path"`

	// 传递给 Python 脚本的额外参数列表（type=python 时使用）
	ScriptArgs []string `yaml:"script_args" json:"script_args"`

	// 使用的 AI provider name（内置任务需要调用 AI 时使用，为空则用默认 provider）
	ProviderName string `yaml:"provider_name" json:"provider_name"`
}

// IntelliAnalyzerConfig 智能消息分析器配置
type IntelliAnalyzerConfig struct {
	// 是否启用智能分析
	Enabled bool `yaml:"enabled" json:"enabled"`

	// 分析间隔（秒），每隔多少秒扫描一次未分析消息，默认 120
	IntervalSeconds int `yaml:"interval_seconds" json:"interval_seconds"`

	// 每次最多分析多少条消息，默认 50
	BatchSize int `yaml:"batch_size" json:"batch_size"`

	// 使用的 AI provider name，为空时使用默认 provider
	ProviderName string `yaml:"provider_name" json:"provider_name"`

	// 更新个人画像的频率：每分析多少批次后更新一次画像，默认 5
	ProfileUpdateEveryBatches int `yaml:"profile_update_every_batches" json:"profile_update_every_batches"`
}

// BarrageFilterKeyword 弹幕过滤关键词
type BarrageFilterKeyword struct {
	// 关键词（大小写不敏感）
	Keyword string `yaml:"keyword" json:"keyword"`
}

// BarrageHighlightRule 弹幕特别关注规则
type BarrageHighlightRule struct {
	// 关键词（大小写不敏感）
	Keyword string `yaml:"keyword" json:"keyword"`
	// 高亮颜色（#RRGGBB 格式），为空时使用默认金色 #FFD700
	Color string `yaml:"color" json:"color"`
}

// BarrageConfig 弹幕配置
type BarrageConfig struct {
	// 过滤关键词列表：消息内容包含任意一个关键词时，不在弹幕中显示
	FilterKeywords []string `yaml:"filter_keywords" json:"filter_keywords"`

	// 特别关注规则列表：消息内容包含关键词时，使用特色颜色渲染弹幕
	HighlightRules []BarrageHighlightRule `yaml:"highlight_rules" json:"highlight_rules"`
}

// AlertConfig 告警规则配置
type AlertConfig struct {
	// 是否启用告警检测
	Enabled bool `yaml:"enabled" json:"enabled"`

	// 触发告警的关键词列表（大小写不敏感）
	Keywords []string `yaml:"keywords" json:"keywords"`

	// 过滤关键词列表（大小写不敏感）：消息内容命中任意一个过滤词时，即使匹配了告警关键词也不触发告警
	FilterKeywords []string `yaml:"filter_keywords" json:"filter_keywords"`
}

// CameraGuardConfig 摄像头背后守卫配置
type CameraGuardConfig struct {
	// 是否启用摄像头守卫功能
	Enabled bool `yaml:"enabled" json:"enabled"`

	// 检测间隔（秒），每隔多少秒抓一帧分析，默认 10
	IntervalSeconds int `yaml:"interval_seconds" json:"interval_seconds"`

	// 使用的 AI provider name（需支持视觉多模态，如 qwen-vl-plus）
	// 为空时使用默认 provider
	ProviderName string `yaml:"provider_name" json:"provider_name"`

	// 连续检测到人几次才触发告警（避免误报），默认 2
	ConfirmCount int `yaml:"confirm_count" json:"confirm_count"`

	// 告警冷却时间（秒），触发告警后多少秒内不再重复告警，默认 60
	CooldownSeconds int `yaml:"cooldown_seconds" json:"cooldown_seconds"`
}

// AIProvider AI 服务商类型
type AIProvider string

const (
	// ProviderOpenAI 标准 OpenAI 接口（也适用于所有 OpenAI 兼容接口）
	ProviderOpenAI AIProvider = "openai"
	// ProviderBailian 阿里云百炼 OpenAI 兼容接口（dashscope compatible-mode）
	ProviderBailian AIProvider = "bailian"
	// ProviderDashScope 阿里云 DashScope 原生接口
	ProviderDashScope AIProvider = "dashscope"
	// ProviderCoPaw 阿里开源 CoPaw 本地 Agent（http://localhost:8088）
	ProviderCoPaw AIProvider = "copaw"
)

// AIProviderConfig 单个 AI Provider 的配置
type AIProviderConfig struct {
	// Provider 唯一名称，用于业务侧按名称选择，例如：chat、summary、translate
	Name string `yaml:"name" json:"name"`

	// 服务商类型：openai / bailian / dashscope
	// - openai：标准 OpenAI 接口，也适用于所有 OpenAI 兼容接口（如 DeepSeek、Moonshot 等）
	// - bailian：阿里云百炼 OpenAI 兼容接口（推荐，base_url 填 dashscope compatible-mode 地址）
	// - dashscope：阿里云 DashScope 原生接口（使用 X-DashScope-SSE 等原生 header）
	Provider AIProvider `yaml:"provider" json:"provider"`

	// API Key
	APIKey string `yaml:"api_key" json:"api_key"`

	// 模型名称，例如：
	// - OpenAI：gpt-4o、gpt-4o-mini
	// - 百炼/通义：qwen-plus、qwen-turbo、qwen-max、qwen3-235b-a22b
	// - DeepSeek：deepseek-chat
	Model string `yaml:"model" json:"model"`

	// API 基础 URL
	// - openai 默认：https://api.openai.com/v1
	// - bailian 默认：https://dashscope.aliyuncs.com/compatible-mode/v1
	// - dashscope 默认：https://dashscope.aliyuncs.com/api/v1
	// - 自定义代理或其他兼容接口直接填写对应地址
	BaseURL string `yaml:"base_url" json:"base_url"`

	// 请求超时秒数（覆盖全局 timeout_seconds，0 表示使用全局配置）
	TimeoutSeconds int `yaml:"timeout_seconds" json:"timeout_seconds"`
}

// LangfuseConfig Langfuse 配置
type LangfuseConfig struct {
	// 是否启用 Langfuse 追踪
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Langfuse Public Key
	PublicKey string `yaml:"public_key" json:"public_key"`

	// Langfuse Secret Key
	SecretKey string `yaml:"secret_key" json:"secret_key"`

	// Langfuse Base URL，默认为 https://cloud.langfuse.com
	BaseURL string `yaml:"base_url" json:"base_url"`

	// 是否启用异步批量处理（高性能模式）
	AsyncEnabled bool `yaml:"async_enabled" json:"async_enabled"`

	// 批量处理配置
	BatchSize     int `yaml:"batch_size" json:"batch_size"`         // 批量大小，默认100
	FlushInterval int `yaml:"flush_interval" json:"flush_interval"` // 刷新间隔（秒），默认5
}

// AIConfig AI 智能体配置
type AIConfig struct {
	// 是否启用 AI 功能
	Enabled bool `yaml:"enabled" json:"enabled"`

	// 默认使用的 provider name，对应 providers 列表中某个 name
	// 未指定时使用 providers 列表中第一个
	DefaultProvider string `yaml:"default_provider" json:"default_provider"`

	// 最大上下文轮数（短期记忆，全局配置）
	MaxContextRounds int `yaml:"max_context_rounds" json:"max_context_rounds"`

	// 全局请求超时秒数，各 provider 可单独覆盖
	TimeoutSeconds int `yaml:"timeout_seconds" json:"timeout_seconds"`

	// Provider 列表，支持配置多个不同的 provider/model 组合
	Providers []AIProviderConfig `yaml:"providers" json:"providers"`
}

// GetProviderConfig 按 name 查找 provider 配置，name 为空时返回默认 provider
// 找不到时返回 nil
func (c *AIConfig) GetProviderConfig(name string) *AIProviderConfig {
	if len(c.Providers) == 0 {
		return nil
	}

	targetName := name
	if targetName == "" {
		targetName = c.DefaultProvider
	}

	// 按名称精确匹配
	for i := range c.Providers {
		if c.Providers[i].Name == targetName {
			return &c.Providers[i]
		}
	}

	// 找不到指定名称时，若有默认 provider 则回退到默认
	if name != "" && c.DefaultProvider != "" && name != c.DefaultProvider {
		for i := range c.Providers {
			if c.Providers[i].Name == c.DefaultProvider {
				return &c.Providers[i]
			}
		}
	}

	// 最终回退：返回列表中第一个
	return &c.Providers[0]
}

// GetEffectiveTimeout 获取 provider 的有效超时时间（provider 级别优先，否则使用全局）
func (c *AIConfig) GetEffectiveTimeout(providerCfg *AIProviderConfig) int {
	if providerCfg != nil && providerCfg.TimeoutSeconds > 0 {
		return providerCfg.TimeoutSeconds
	}
	if c.TimeoutSeconds > 0 {
		return c.TimeoutSeconds
	}
	return 30
}

// GeneralConfig 通用配置
type GeneralConfig struct {
	// 日志级别：debug, info, warn, error
	LogLevel string `yaml:"log_level" json:"log_level"`
	// 检查间隔（秒）
	CheckInterval int `yaml:"check_interval" json:"check_interval"`
	// 是否启用
	Enabled bool `yaml:"enabled" json:"enabled"`
}

// StorageConfig 存储配置
type StorageConfig struct {
	// SQLite 数据库路径
	DatabasePath string `yaml:"database_path" json:"database_path"`
	// 消息保留天数
	RetentionDays int `yaml:"retention_days" json:"retention_days"`
}

// WebConfig Web UI 配置
type WebConfig struct {
	// 是否启用 Web UI
	Enabled bool `yaml:"enabled" json:"enabled"`
	// 监听端口
	Port int `yaml:"port" json:"port"`
	// 绑定地址
	Host string `yaml:"host" json:"host"`
}

// AppConfigItem 单个应用的配置
type AppConfigItem struct {
	// 应用名称（自定义）
	Name string `yaml:"name" json:"name"`
	// 应用进程名（用于查找进程）
	ProcessName string `yaml:"process_name" json:"process_name"`
	// 应用显示名称（用于状态栏）
	DisplayName string `yaml:"display_name" json:"display_name"`
	// 是否启用
	Enabled bool `yaml:"enabled" json:"enabled"`
	// 图标路径（可选）
	IconPath string `yaml:"icon_path" json:"icon_path"`
	// 消息解析规则
	ParseRules ParseRules `yaml:"parse_rules" json:"parse_rules"`
	// 会话获取方式
	SessionConfig SessionConfig `yaml:"session_config" json:"session_config"`
	// DingTalk 钉钉专属配置（仅 name=dingtalk 时生效）
	DingTalk DingTalkConfig `yaml:"dingtalk" json:"dingtalk"`
	// ProcessMonitor 进程监控配置（通用）
	ProcessMonitor ProcessMonitorConfig `yaml:"process_monitor" json:"process_monitor"`
}

// ParseRules 消息解析规则
type ParseRules struct {
	// 发送者正则表达式
	SenderPattern string `yaml:"sender_pattern" json:"sender_pattern"`
	// 时间正则表达式
	TimePattern string `yaml:"time_pattern" json:"time_pattern"`
	// 内容提取方式：full, after_time, after_sender
	ContentMode string `yaml:"content_mode" json:"content_mode"`
	// 是否启用去重
	DedupEnabled bool `yaml:"dedup_enabled" json:"dedup_enabled"`
}

// SessionConfig 会话配置
type SessionConfig struct {
	// 会话名称获取方式：window_title, script, fixed
	Source string `yaml:"source" json:"source"`
	// 固定会话名（当 source=fixed 时使用）
	FixedName string `yaml:"fixed_name" json:"fixed_name"`
	// 获取会话的 AppleScript
	Script string `yaml:"script" json:"script"`
}

// DingTalkSourceMode 钉钉消息读取方式
type DingTalkSourceMode string

const (
	// DingTalkSourceModeAccessibility 使用 macOS Accessibility API 读取窗口文本（默认）
	DingTalkSourceModeAccessibility DingTalkSourceMode = "accessibility"
	// DingTalkSourceModeVLModel 截图后调用视觉模型识别消息（更准确）
	DingTalkSourceModeVLModel DingTalkSourceMode = "vlmodel"
)

// DingTalkConfig 钉钉专属配置
type DingTalkConfig struct {
	// SourceMode 消息读取方式：accessibility（默认）或 vlmodel
	// - accessibility：通过 macOS Accessibility API 读取窗口文本，速度快但可能不准确
	// - vlmodel：截图后调用视觉模型识别消息，准确但需要配置 vlmodel provider
	SourceMode DingTalkSourceMode `yaml:"source_mode" json:"source_mode"`
}

// ProcessMonitorConfig 进程监控配置（通用）
type ProcessMonitorConfig struct {
	// 是否启用 VLModel 识别优化
	// - false：使用 AppleScript/Accessibility API 读取窗口文本
	// - true：截图后调用视觉模型识别消息，更准确但需要配置 provider
	UseVLModel bool `yaml:"use_vlmodel" json:"use_vlmodel"`
	// VLModel Provider 名称（当 use_vlmodel=true 时生效）
	// 指定使用 AIConfig.Providers 中哪个 provider 进行视觉识别
	VLModelProvider string `yaml:"vlmodel_provider" json:"vlmodel_provider"`
	// VLModel 提示词名称（当 use_vlmodel=true 时生效）
	// 指定使用提示词管理中哪个提示词模板，留空则使用默认提示词
	VLModelPrompt string `yaml:"vlmodel_prompt" json:"vlmodel_prompt"`
}

// defaultAlertKeywords 默认告警关键词列表
var defaultAlertKeywords = []string{
	// 紧急程度
	"紧急", "urgent", "URGENT", "asap", "ASAP",
	// 需要响应
	"请尽快", "尽快", "马上", "立刻", "立即", "速回", "速看",
	// 重要标记
	"重要", "important", "IMPORTANT", "关键", "critical", "CRITICAL",
	// 故障告警
	"报警", "告警", "alert", "alarm", "故障", "异常", "error", "ERROR",
	"宕机", "down", "崩溃", "crash", "线上问题", "线上故障",
	// 截止时间
	"deadline", "DDL", "ddl", "截止", "今天必须", "今晚必须",
	// 等待回复
	"等你", "等你回复", "等你确认", "麻烦看一下", "帮忙看看",
	// @提醒
	"@我", "@你",
}

// 默认配置
var defaultConfig = AppConfig{
	General: GeneralConfig{
		LogLevel:      "info",
		CheckInterval: 3,
		Enabled:       true,
	},
	Storage: StorageConfig{
		DatabasePath:  "./data/messages.db",
		RetentionDays: 90,
	},
	Web: WebConfig{
		Enabled: true,
		Port:    8765,
		Host:    "127.0.0.1",
	},
	AI: AIConfig{
		Enabled:          true,
		DefaultProvider:  "default",
		MaxContextRounds: 10,
		TimeoutSeconds:   30,
		Providers: []AIProviderConfig{
			{
				Name:     "default",
				Provider: ProviderBailian,
				APIKey:   "",
				Model:    "qwen-plus",
				BaseURL:  "",
			},
		},
	},
	Alert: AlertConfig{
		Enabled:  true,
		Keywords: defaultAlertKeywords,
	},
	CameraGuard: CameraGuardConfig{
		Enabled:         false,
		IntervalSeconds: 10,
		ProviderName:    "",
		ConfirmCount:    2,
		CooldownSeconds: 60,
	},
	IntelliAnalyzer: IntelliAnalyzerConfig{
		Enabled:                   false,
		IntervalSeconds:           120,
		BatchSize:                 50,
		ProviderName:              "",
		ProfileUpdateEveryBatches: 5,
	},
	ReactiveAI: ReactiveAIConfig{
		Enabled:               false,
		SystemGuardianEnabled: true,
		UserTrackerEnabled:    true,
		CameraPatrolEnabled:   false,
		CoordinatorInterval:   60,
	},
	Apps: []AppConfigItem{
		{
			Name:        "dingtalk",
			ProcessName: "DingTalk",
			DisplayName: "钉钉",
			Enabled:     true,
			IconPath:    "",
			ParseRules: ParseRules{
				SenderPattern: `(\S+)\s+\d{1,2}:\d{2}`,
				TimePattern:   `\d{1,2}:\d{2}`,
				ContentMode:   "after_time",
				DedupEnabled:  true,
			},
			SessionConfig: SessionConfig{
				Source: "window_title",
			},
		},
		{
			Name:        "alidingding",
			ProcessName: "阿里钉",
			DisplayName: "阿里钉",
			Enabled:     true,
			IconPath:    "",
			ParseRules: ParseRules{
				SenderPattern: `(\S+)\s+\d{1,2}:\d{2}`,
				TimePattern:   `\d{1,2}:\d{2}`,
				ContentMode:   "after_time",
				DedupEnabled:  true,
			},
			SessionConfig: SessionConfig{
				Source: "window_title",
			},
		},
		{
			Name:        "wechat",
			ProcessName: "WeChat",
			DisplayName: "微信",
			Enabled:     true,
			IconPath:    "",
			ParseRules: ParseRules{
				SenderPattern: `(\S+)\s+\d{1,2}:\d{2}`,
				TimePattern:   `\d{1,2}:\d{2}`,
				ContentMode:   "after_time",
				DedupEnabled:  true,
			},
			SessionConfig: SessionConfig{
				Source: "window_title",
			},
		},
		{
			Name:        "qq",
			ProcessName: "QQ",
			DisplayName: "QQ",
			Enabled:     true,
			IconPath:    "",
			ParseRules: ParseRules{
				SenderPattern: `(\S+)\s+\d{1,2}:\d{2}`,
				TimePattern:   `\d{1,2}:\d{2}`,
				ContentMode:   "after_time",
				DedupEnabled:  true,
			},
			SessionConfig: SessionConfig{
				Source: "window_title",
			},
		},
	},
}

var (
	configInstance    *AppConfig
	configMutex       sync.RWMutex
	configPath        string
	configLastModTime time.Time
)

// Init 初始化配置
func Init(path string) (*AppConfig, error) {
	configPath = path

	// 如果配置文件不存在，创建默认配置
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := Save(); err != nil {
			return nil, fmt.Errorf("创建默认配置文件失败：%w", err)
		}
		fmt.Printf("已创建默认配置文件：%s\n", path)
	}

	// 加载配置
	if err := Load(); err != nil {
		return nil, err
	}

	// 确保数据目录存在
	dbDir := filepath.Dir(configInstance.Storage.DatabasePath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败：%w", err)
	}

	return configInstance, nil
}

// Load 从文件加载配置
func Load() error {
	configMutex.Lock()
	defer configMutex.Unlock()

	return loadLocked()
}

// loadLocked 在已持有写锁的情况下从文件加载配置（内部使用）
func loadLocked() error {
	info, err := os.Stat(configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件信息失败：%w", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败：%w", err)
	}

	cfg := defaultConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("解析配置文件失败：%w", err)
	}

	configInstance = &cfg
	configLastModTime = info.ModTime()
	return nil
}

// Save 保存配置到文件
// 注意：此函数不应在已持有 configMutex 锁的情况下调用
func Save() error {
	configMutex.RLock()
	defer configMutex.RUnlock()

	if configInstance == nil {
		configInstance = &defaultConfig
	}

	data, err := yaml.Marshal(configInstance)
	if err != nil {
		return fmt.Errorf("序列化配置失败：%w", err)
	}

	// 确保目录存在
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败：%w", err)
	}

	return nil
}

// saveLocked 在已持有锁的情况下保存配置到文件（内部使用）
func saveLocked() error {
	if configInstance == nil {
		configInstance = &defaultConfig
	}

	data, err := yaml.Marshal(configInstance)
	if err != nil {
		return fmt.Errorf("序列化配置失败：%w", err)
	}

	// 确保目录存在
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败：%w", err)
	}

	return nil
}

// Get 获取配置，每次调用时检查配置文件是否有变更，有则自动热重载
func Get() *AppConfig {
	// 先用读锁快速检查文件 mtime
	configMutex.RLock()
	currentPath := configPath
	lastMod := configLastModTime
	configMutex.RUnlock()

	if currentPath != "" {
		if info, err := os.Stat(currentPath); err == nil && info.ModTime().After(lastMod) {
			// 文件有变更，升级为写锁重新加载
			configMutex.Lock()
			// 双重检查：防止多个 goroutine 同时通过第一次检查后重复加载
			if info2, err2 := os.Stat(currentPath); err2 == nil && info2.ModTime().After(configLastModTime) {
				if err3 := loadLocked(); err3 != nil {
					log.Printf("[Config] 热重载配置文件失败，继续使用旧配置: %v", err3)
				} else {
					log.Printf("[Config] 配置文件已变更，热重载成功")
				}
			}
			configMutex.Unlock()
		}
	}

	configMutex.RLock()
	defer configMutex.RUnlock()
	return configInstance
}

// UpdateApp 更新应用配置
func UpdateApp(name string, app AppConfigItem) error {
	configMutex.Lock()
	defer configMutex.Unlock()

	for i, a := range configInstance.Apps {
		if a.Name == name {
			configInstance.Apps[i] = app
			return saveLocked()
		}
	}

	// 添加新应用
	configInstance.Apps = append(configInstance.Apps, app)
	return saveLocked()
}

// AddApp 添加应用
func AddApp(app AppConfigItem) error {
	configMutex.Lock()
	defer configMutex.Unlock()

	// 检查是否已存在
	for _, a := range configInstance.Apps {
		if a.Name == app.Name {
			return fmt.Errorf("应用 %s 已存在", app.Name)
		}
	}

	configInstance.Apps = append(configInstance.Apps, app)
	return saveLocked()
}

// RemoveApp 删除应用
func RemoveApp(name string) error {
	configMutex.Lock()
	defer configMutex.Unlock()

	newApps := []AppConfigItem{}
	for _, a := range configInstance.Apps {
		if a.Name != name {
			newApps = append(newApps, a)
		}
	}

	if len(newApps) == len(configInstance.Apps) {
		return fmt.Errorf("应用 %s 不存在", name)
	}

	configInstance.Apps = newApps
	return saveLocked()
}

// GetApp 获取应用配置
func GetApp(name string) (*AppConfigItem, error) {
	configMutex.RLock()
	defer configMutex.RUnlock()

	for _, a := range configInstance.Apps {
		if a.Name == name {
			return &a, nil
		}
	}
	return nil, fmt.Errorf("应用 %s 不存在", name)
}

// UpdateAIConfig 更新 AI 配置并持久化到文件
func UpdateAIConfig(aiCfg AIConfig) error {
	configMutex.Lock()
	configInstance.AI = aiCfg
	configMutex.Unlock()
	return Save()
}

// UpdateAlertConfig 更新告警配置并持久化到文件
func UpdateAlertConfig(alertCfg AlertConfig) error {
	configMutex.Lock()
	configInstance.Alert = alertCfg
	configMutex.Unlock()
	return Save()
}

// GetBarrageConfig 获取弹幕配置（线程安全）
func GetBarrageConfig() BarrageConfig {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return configInstance.Barrage
}

// UpdateBarrageConfig 更新弹幕配置并持久化到文件
func UpdateBarrageConfig(cfg BarrageConfig) error {
	configMutex.Lock()
	configInstance.Barrage = cfg
	configMutex.Unlock()
	return Save()
}

// GetAlertConfig 获取告警配置（线程安全）
func GetAlertConfig() AlertConfig {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return configInstance.Alert
}

// GetCameraGuardConfig 获取摄像头守卫配置（线程安全）
func GetCameraGuardConfig() CameraGuardConfig {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return configInstance.CameraGuard
}

// UpdateCameraGuardConfig 更新摄像头守卫配置并持久化
func UpdateCameraGuardConfig(cfg CameraGuardConfig) error {
	configMutex.Lock()
	configInstance.CameraGuard = cfg
	configMutex.Unlock()
	return Save()
}

// GetIntelliAnalyzerConfig 获取智能分析器配置（线程安全）
func GetIntelliAnalyzerConfig() IntelliAnalyzerConfig {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return configInstance.IntelliAnalyzer
}

// UpdateIntelliAnalyzerConfig 更新智能分析器配置并持久化
func UpdateIntelliAnalyzerConfig(cfg IntelliAnalyzerConfig) error {
	configMutex.Lock()
	configInstance.IntelliAnalyzer = cfg
	configMutex.Unlock()
	return Save()
}

// GetReactiveAIConfig 获取响应式AI链路配置（线程安全）
func GetReactiveAIConfig() ReactiveAIConfig {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return configInstance.ReactiveAI
}

// UpdateReactiveAIConfig 更新响应式AI链路配置并持久化
func UpdateReactiveAIConfig(cfg ReactiveAIConfig) error {
	configMutex.Lock()
	configInstance.ReactiveAI = cfg
	configMutex.Unlock()
	return Save()
}

// GetLangfuseConfig 获取Langfuse配置（线程安全）
func GetLangfuseConfig() LangfuseConfig {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return configInstance.Langfuse
}

// UpdateLangfuseConfig 更新Langfuse配置并持久化
func UpdateLangfuseConfig(cfg LangfuseConfig) error {
	configMutex.Lock()
	configInstance.Langfuse = cfg
	configMutex.Unlock()
	return Save()
}

// GetPetSkin 获取当前桌宠皮肤名称（线程安全）
func GetPetSkin() string {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return configInstance.PetSkin
}

// UpdatePetSkin 更新桌宠皮肤并持久化到文件
func UpdatePetSkin(skinName string) error {
	configMutex.Lock()
	configInstance.PetSkin = skinName
	configMutex.Unlock()
	return Save()
}

// GetEnabledApps 获取所有启用的应用
func GetEnabledApps() []AppConfigItem {
	configMutex.RLock()
	defer configMutex.RUnlock()

	enabled := []AppConfigItem{}
	for _, app := range configInstance.Apps {
		if app.Enabled {
			enabled = append(enabled, app)
		}
	}
	return enabled
}

// ToJSON 导出配置为 JSON
func (c *AppConfig) ToJSON() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

// GetCheckInterval 获取检查间隔（带默认值）
func (c *AppConfig) GetCheckInterval() time.Duration {
	if c.General.CheckInterval <= 0 {
		return 3 * time.Second
	}
	return time.Duration(c.General.CheckInterval) * time.Second
}
