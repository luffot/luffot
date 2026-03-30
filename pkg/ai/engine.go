package ai

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	pcAgent "github.com/sipeed/picoclaw/pkg/agent"
	"github.com/sipeed/picoclaw/pkg/bus"
	pcConfig "github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/providers"
	pcTools "github.com/sipeed/picoclaw/pkg/tools"

	"github.com/luffot/luffot/pkg/config"
)

// PicoClawEngine 封装 PicoClaw AgentLoop，作为 Luffot 的 AI 引擎
type PicoClawEngine struct {
	mu        sync.RWMutex
	agentLoop *pcAgent.AgentLoop
	msgBus    *bus.MessageBus
	provider  providers.LLMProvider
	modelID   string
	running   bool
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewPicoClawEngine 根据 Luffot 的 AI 配置创建 PicoClaw 引擎
func NewPicoClawEngine(aiCfg *config.AIConfig) (*PicoClawEngine, error) {
	if !aiCfg.Enabled || len(aiCfg.Providers) == 0 {
		return nil, fmt.Errorf("AI 未启用或未配置 provider")
	}

	// 找到默认 provider 配置
	providerCfg := aiCfg.GetProviderConfig("")
	if providerCfg == nil {
		return nil, fmt.Errorf("未找到默认 AI provider 配置")
	}

	// 将 Luffot 的 provider 配置转换为 PicoClaw ModelConfig
	modelCfg := convertToModelConfig(providerCfg)

	// 创建 PicoClaw LLM Provider
	llmProvider, modelID, err := providers.CreateProviderFromConfig(modelCfg)
	if err != nil {
		return nil, fmt.Errorf("创建 PicoClaw LLM Provider 失败: %w", err)
	}

	// 构建 PicoClaw Config
	pcCfg := buildPicoClawConfig(aiCfg, modelCfg)

	// 创建消息总线
	msgBus := bus.NewMessageBus()

	// 创建 AgentLoop
	agentLoop := pcAgent.NewAgentLoop(pcCfg, msgBus, llmProvider)

	ctx, cancel := context.WithCancel(context.Background())

	engine := &PicoClawEngine{
		agentLoop: agentLoop,
		msgBus:    msgBus,
		provider:  llmProvider,
		modelID:   modelID,
		ctx:       ctx,
		cancel:    cancel,
	}

	log.Printf("[PicoClaw] 引擎初始化完成，model=%s, provider=%s", modelID, providerCfg.Provider)
	return engine, nil
}

// RegisterTool 注册自定义工具到 PicoClaw AgentLoop
func (e *PicoClawEngine) RegisterTool(tool pcTools.Tool) {
	e.agentLoop.RegisterTool(tool)
	log.Printf("[PicoClaw] 注册工具: %s", tool.Name())
}

// Start 启动 PicoClaw AgentLoop（后台运行）
func (e *PicoClawEngine) Start() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return
	}
	e.running = true

	go func() {
		if err := e.agentLoop.Run(e.ctx); err != nil {
			log.Printf("[PicoClaw] AgentLoop 运行结束: %v", err)
		}
	}()

	log.Println("[PicoClaw] AgentLoop 已启动")
}

// ProcessDirect 直接处理用户输入，返回 AI 回复（同步阻塞）
func (e *PicoClawEngine) ProcessDirect(ctx context.Context, content, sessionKey string) (string, error) {
	return e.agentLoop.ProcessDirect(ctx, content, sessionKey)
}

// ProcessDirectWithChannel 使用指定 channel 处理用户输入
func (e *PicoClawEngine) ProcessDirectWithChannel(ctx context.Context, content, sessionKey, channel, chatID string) (string, error) {
	return e.agentLoop.ProcessDirectWithChannel(ctx, content, sessionKey, channel, chatID)
}

// ChatDirect 简化的对话接口，使用默认 session
func (e *PicoClawEngine) ChatDirect(ctx context.Context, userInput string) (string, error) {
	return e.ProcessDirect(ctx, userInput, "luffot-chat")
}

// ChatWithSession 使用指定 session 进行对话
func (e *PicoClawEngine) ChatWithSession(ctx context.Context, userInput, sessionKey string) (string, error) {
	return e.ProcessDirect(ctx, userInput, sessionKey)
}

// Stop 停止 PicoClaw 引擎
func (e *PicoClawEngine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return
	}

	e.agentLoop.Stop()
	e.cancel()
	e.running = false
	log.Println("[PicoClaw] 引擎已停止")
}

// Close 关闭引擎并释放资源
func (e *PicoClawEngine) Close() {
	e.Stop()
	e.agentLoop.Close()
	log.Println("[PicoClaw] 引擎资源已释放")
}

// GetAgentLoop 获取底层 AgentLoop 实例（供高级用法使用）
func (e *PicoClawEngine) GetAgentLoop() *pcAgent.AgentLoop {
	return e.agentLoop
}

// GetProvider 获取底层 LLM Provider
func (e *PicoClawEngine) GetProvider() providers.LLMProvider {
	return e.provider
}

// convertToModelConfig 将 Luffot 的 AIProviderConfig 转换为 PicoClaw 的 ModelConfig
func convertToModelConfig(providerCfg *config.AIProviderConfig) *pcConfig.ModelConfig {
	// 构建 PicoClaw 的 model 标识符（protocol/model 格式）
	protocol := mapProviderToProtocol(providerCfg.Provider)
	modelIdentifier := protocol + "/" + providerCfg.Model

	modelCfg := &pcConfig.ModelConfig{
		ModelName: providerCfg.Name,
		Model:     modelIdentifier,
		APIBase:   resolveBaseURLForPicoClaw(providerCfg),
	}

	if providerCfg.APIKey != "" {
		modelCfg.SetAPIKey(providerCfg.APIKey)
	}

	if providerCfg.TimeoutSeconds > 0 {
		modelCfg.RequestTimeout = providerCfg.TimeoutSeconds
	}

	return modelCfg
}

// mapProviderToProtocol 将 Luffot 的 provider 类型映射为 PicoClaw 的 protocol 前缀
func mapProviderToProtocol(provider config.AIProvider) string {
	switch provider {
	case config.ProviderOpenAI:
		return "openai"
	case config.ProviderBailian:
		return "openai" // 百炼使用 OpenAI 兼容接口
	case config.ProviderDashScope:
		return "openai" // DashScope compatible-mode 也走 OpenAI 兼容
	case config.ProviderCoPaw:
		return "openai" // CoPaw 使用 OpenAI 兼容接口
	default:
		return "openai"
	}
}

// resolveBaseURLForPicoClaw 根据 provider 配置返回 PicoClaw 使用的 API base URL
func resolveBaseURLForPicoClaw(providerCfg *config.AIProviderConfig) string {
	if providerCfg.BaseURL != "" {
		return strings.TrimRight(providerCfg.BaseURL, "/")
	}
	switch providerCfg.Provider {
	case config.ProviderOpenAI:
		return "https://api.openai.com/v1"
	case config.ProviderBailian:
		return "https://dashscope.aliyuncs.com/compatible-mode/v1"
	case config.ProviderDashScope:
		return "https://dashscope.aliyuncs.com/compatible-mode/v1"
	case config.ProviderCoPaw:
		return "http://localhost:8088/v1"
	default:
		return "https://api.openai.com/v1"
	}
}

// buildPicoClawConfig 构建 PicoClaw 的完整配置
func buildPicoClawConfig(aiCfg *config.AIConfig, defaultModel *pcConfig.ModelConfig) *pcConfig.Config {
	// 加载系统 prompt
	systemPrompt := loadSystemPrompt()
	if userProfile := loadUserProfileForPrompt(); userProfile != "" {
		systemPrompt += fmt.Sprintf(`\n\n---\n以下是主人的个人画像，请在回答时将其作为背景参考：\n<主人画像>\n%s\n</主人画像>`, userProfile)
	}

	// 构建 workspace 路径
	workspace := filepath.Join(os.Getenv("HOME"), ".luffot", "picoclaw")
	os.MkdirAll(workspace, 0755)

	// 构建所有 model 配置
	modelList := make([]*pcConfig.ModelConfig, 0, len(aiCfg.Providers))
	for i := range aiCfg.Providers {
		mc := convertToModelConfig(&aiCfg.Providers[i])
		modelList = append(modelList, mc)
	}

	// 确定默认 model name
	defaultModelName := defaultModel.ModelName
	if defaultModelName == "" && len(modelList) > 0 {
		defaultModelName = modelList[0].ModelName
	}

	pcCfg := &pcConfig.Config{
		ModelList: modelList,
		Agents: pcConfig.AgentsConfig{
			Defaults: pcConfig.AgentDefaults{
				ModelName:                 defaultModelName,
				Workspace:                 workspace,
				SummarizeMessageThreshold: aiCfg.MaxContextRounds * 2, // 超过此消息数时触发摘要
			},
			List: []pcConfig.AgentConfig{
				{
					ID:        "default",
					Default:   true,
					Name:      "luffot-agent",
					Workspace: workspace,
				},
			},
		},
	}

	// 系统 prompt 通过 AgentLoop 的 system prompt 机制注入
	// PicoClaw 的 AgentConfig 不直接支持 SystemPrompt 字段，
	// 系统 prompt 在 NewPicoClawEngine 中通过 AgentLoop 配置传入
	_ = systemPrompt // systemPrompt 将在后续版本中通过 AgentLoop API 注入

	return pcCfg
}

// convertAllProviders 将所有 Luffot provider 配置转换为 PicoClaw provider 映射
// 返回 providerName -> (LLMProvider, modelID) 的映射
func convertAllProviders(aiCfg *config.AIConfig) map[string]providerEntry {
	result := make(map[string]providerEntry)
	for i := range aiCfg.Providers {
		pCfg := &aiCfg.Providers[i]
		if pCfg.APIKey == "" {
			continue
		}
		modelCfg := convertToModelConfig(pCfg)
		llmProvider, modelID, err := providers.CreateProviderFromConfig(modelCfg)
		if err != nil {
			log.Printf("[PicoClaw] 创建 provider %s 失败: %v", pCfg.Name, err)
			continue
		}
		result[pCfg.Name] = providerEntry{
			provider: llmProvider,
			modelID:  modelID,
		}
	}
	return result
}

// providerEntry 存储一个 provider 实例及其对应的 model ID
type providerEntry struct {
	provider providers.LLMProvider
	modelID  string
}
