package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "github.com/AEKurt/langfuse-go"
	"github.com/luffot/luffot/pkg/config"
	"github.com/luffot/luffot/pkg/prompt"
)

// userProfilePath 用户画像文件路径：~/.luffot/.my_profile
var userProfilePath = filepath.Join(os.Getenv("HOME"), ".luffot", ".my_profile")

// loadUserProfileForPrompt 读取用户画像文件，供注入 system prompt 使用。
// 文件不存在或读取失败时返回空字符串，不影响正常对话。
func loadUserProfileForPrompt() string {
	data, err := os.ReadFile(userProfilePath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// loadSystemPrompt 从 ~/.luffot/prompt/agent_system.md 动态加载系统人设 prompt。
// 文件不存在时自动回退到内置默认值，保证程序始终可用。
func loadSystemPrompt() string {
	content, err := prompt.Load("agent_system")
	if err != nil {
		log.Printf("[Agent] 加载系统 prompt 失败，使用内置默认值: %v", err)
		return prompt.DefaultContent("agent_system")
	}
	return content
}

// ChatMessage 单条对话消息
type ChatMessage struct {
	Role             string `json:"role"`
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
}

// openAIRequest OpenAI 兼容接口请求体
type openAIRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

// openAIResponse OpenAI 兼容接口响应体
type openAIResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Agent AI 智能体，负责与 LLM 交互
type Agent struct {
	mu     sync.Mutex
	memory *Memory

	// 当前是否正在思考（异步请求进行中）
	isThinking bool
	// 最新的 AI 回复（供 PetSprite 读取展示）
	latestReply string
	// 回复完成回调（通知 PetSprite 展示完整回复，用于非流式模式）
	onReply func(reply string)
	// 流式 token 回调（每收到一个 token 片段就调用一次）
	onToken func(token string)
	// Langfuse 追踪会话 ID
	traceID string
}

// NewAgent 创建 AI 智能体
// onReply：回复完成时的回调（非流式模式使用）
// onToken：流式 token 回调，每收到一个片段调用一次（nil 则退化为非流式）
func NewAgent(memory *Memory, onReply func(reply string), onToken func(token string)) *Agent {
	return &Agent{
		memory:  memory,
		onReply: onReply,
		onToken: onToken,
	}
}

// aiConfig 实时读取最新的 AI 配置（每次调用都从全局配置中获取，支持热重载）
func (a *Agent) aiConfig() *config.AIConfig {
	return &config.Get().AI
}

// IsEnabled 是否启用 AI 功能（至少有一个 provider 配置了 APIKey）
func (a *Agent) IsEnabled() bool {
	cfg := a.aiConfig()
	if !cfg.Enabled {
		return false
	}
	for _, p := range cfg.Providers {
		if p.APIKey != "" {
			return true
		}
	}
	return false
}

// newHTTPClient 根据 provider 配置创建 HTTP 客户端
func (a *Agent) newHTTPClient(providerCfg *config.AIProviderConfig) *http.Client {
	timeout := a.aiConfig().GetEffectiveTimeout(providerCfg)
	return &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}
}

// IsThinking 是否正在思考中（线程安全）
func (a *Agent) IsThinking() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.isThinking
}

// GetLatestReply 获取最新回复（线程安全）
func (a *Agent) GetLatestReply() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.latestReply
}

// Chat 使用默认 provider 发起对话（异步，不阻塞调用方）
// 若 onToken 不为 nil 则走流式模式，逐 token 回调；否则走非流式模式，回复完成后一次性回调 onReply。
func (a *Agent) Chat(userInput string) {
	a.ChatWithProvider(userInput, "")
}

// ChatWithProvider 使用指定 provider name 发起对话（异步，不阻塞调用方）
// providerName 为空时使用默认 provider。
// 若 onToken 不为 nil 则走流式模式，逐 token 回调；否则走非流式模式，回复完成后一次性回调 onReply。
func (a *Agent) ChatWithProvider(userInput string, providerName string) {
	if !a.IsEnabled() {
		hint := "主人，我还没有配置 API Key 哦～请在 config.yaml 里填上 API Key 吧！🔑"
		log.Printf("[AI] 未启用或未配置 API Key")
		if a.onReply != nil {
			a.onReply(hint)
		}
		return
	}

	providerCfg := a.aiConfig().GetProviderConfig(providerName)
	if providerCfg == nil {
		hint := "主人，找不到对应的 AI provider 配置哦～请检查 config.yaml 里的 providers 配置吧！🔑"
		log.Printf("[AI] 找不到 provider 配置，providerName=%s", providerName)
		if a.onReply != nil {
			a.onReply(hint)
		}
		return
	}

	a.mu.Lock()
	if a.isThinking {
		a.mu.Unlock()
		return // 正在思考中，忽略新输入
	}
	a.isThinking = true
	a.mu.Unlock()

	go func() {
		defer func() {
			a.mu.Lock()
			a.isThinking = false
			a.mu.Unlock()
		}()

		// 创建 Trace 追踪整个对话流程（业务功能级别）
		lc := GetLangfuseClient()
		var traceCtx *TraceContext
		if lc.IsEnabled() {
			traceCtx, _ = StartTrace(context.Background(), "agent-chat", "user", userInput, map[string]interface{}{
				"provider": providerCfg.Provider,
				"model":    providerCfg.Model,
			})
		}

		// 优先走流式模式
		if a.onToken != nil {
			fullReply, err := a.callLLMStream(userInput, providerCfg, a.onToken, traceCtx)
			if err != nil {
				log.Printf("[AI] 流式调用 LLM 失败: %v", err)
				errMsg := fmt.Sprintf("呜呜，小钉出错了 😢\n错误详情：%s", err.Error())
				if a.onReply != nil {
					a.onReply(errMsg)
				}
				// 结束 Trace
				if traceCtx != nil {
					traceCtx.End(context.Background(), errMsg)
				}
				return
			}
			a.memory.AddTurn(userInput, fullReply)
			a.mu.Lock()
			a.latestReply = fullReply
			a.mu.Unlock()
			// 流式完成后通知 onReply（用于将完整回复写入消息列表）
			if a.onReply != nil {
				a.onReply(fullReply)
			}
			// 结束 Trace
			if traceCtx != nil {
				traceCtx.End(context.Background(), fullReply)
			}
			return
		}

		// 非流式模式
		reply, err := a.callLLM(userInput, providerCfg, traceCtx)
		if err != nil {
			log.Printf("[AI] 调用 LLM 失败: %v", err)
			reply = fmt.Sprintf("呜呜，小钉出错了 😢\n错误详情：%s", err.Error())
		}

		a.memory.AddTurn(userInput, reply)

		a.mu.Lock()
		a.latestReply = reply
		a.mu.Unlock()

		if a.onReply != nil {
			a.onReply(reply)
		}

		// 结束 Trace
		if traceCtx != nil {
			traceCtx.End(context.Background(), reply)
		}
	}()
}

// callLLMStream 流式调用 LLM，每收到一个 token 片段就调用 onToken 回调
// 返回完整的回复文本
// traceCtx: 可选的 Trace 上下文，如果提供则在此 Trace 下创建 Generation
func (a *Agent) callLLMStream(userInput string, providerCfg *config.AIProviderConfig, onToken func(token string), traceCtx *TraceContext) (string, error) {
	messages := a.buildMessages(userInput)
	timeout := a.aiConfig().GetEffectiveTimeout(providerCfg)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	// 创建Langfuse Generation（在传入的 Trace 下）
	lc := GetLangfuseClient()
	var genCtx *GenerationContext
	if lc.IsEnabled() && traceCtx != nil {
		genCtx, _ = traceCtx.StartGeneration(context.Background(), "llm-call-stream", providerCfg.Model, messages)
	}

	startTime := time.Now()
	var reply string
	var err error

	switch providerCfg.Provider {
	case config.ProviderDashScope:
		reply, err = a.doStreamRequestDashScope(ctx, messages, providerCfg, onToken)
	case config.ProviderCoPaw:
		reply, err = a.doStreamRequestCoPaw(ctx, userInput, providerCfg, onToken)
	default:
		reply, err = a.doStreamRequestOpenAICompat(ctx, messages, providerCfg, onToken)
	}

	duration := time.Since(startTime)

	// 更新Generation
	if genCtx != nil {
		inputTokens := CalculateMessagesTokens(messages)
		outputTokens := CalculateTokens(reply)
		genCtx.End(context.Background(), reply, inputTokens, outputTokens)
		log.Printf("[Langfuse] 流式调用完成，耗时: %v, 输入token: %d, 输出token: %d", duration, inputTokens, outputTokens)
	}

	return reply, err
}

// openAIStreamDelta OpenAI SSE 流式响应中的 delta 字段
type openAIStreamDelta struct {
	Content string `json:"content"`
}

// openAIStreamChoice OpenAI SSE 流式响应中的 choice 字段
type openAIStreamChoice struct {
	Delta        openAIStreamDelta `json:"delta"`
	FinishReason string            `json:"finish_reason"`
}

// openAIStreamChunk OpenAI SSE 流式响应的单个数据块
type openAIStreamChunk struct {
	Choices []openAIStreamChoice `json:"choices"`
}

// doStreamRequestOpenAICompat 使用 OpenAI 兼容接口发起流式请求（SSE）
func (a *Agent) doStreamRequestOpenAICompat(ctx context.Context, messages []ChatMessage, providerCfg *config.AIProviderConfig, onToken func(token string)) (string, error) {
	reqBody := openAIRequest{
		Model:    providerCfg.Model,
		Messages: messages,
		Stream:   true,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %w", err)
	}

	requestURL := resolveBaseURL(providerCfg) + "/chat/completions"
	log.Printf("[AI] 发起流式请求 provider=%s url=%s model=%s", providerCfg.Provider, requestURL, providerCfg.Model)

	req, err := http.NewRequestWithContext(ctx, "POST", requestURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+providerCfg.APIKey)
	req.Header.Set("Accept", "text/event-stream")

	httpClient := a.newHTTPClient(providerCfg)
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyContent, _ := io.ReadAll(resp.Body)
		log.Printf("[AI] 流式请求错误响应体: %s", string(bodyContent))
		var errResp openAIResponse
		if jsonErr := json.Unmarshal(bodyContent, &errResp); jsonErr == nil && errResp.Error != nil {
			return "", fmt.Errorf("API 返回错误 (HTTP %d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return "", fmt.Errorf("API 返回非预期状态码 (HTTP %d)，响应: %s", resp.StatusCode, string(bodyContent))
	}

	var fullReply strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk openAIStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			log.Printf("[AI] 解析 SSE chunk 失败: %v, data=%s", err, data)
			continue
		}

		for _, choice := range chunk.Choices {
			token := choice.Delta.Content
			if token == "" {
				continue
			}
			fullReply.WriteString(token)
			onToken(token)
		}
	}

	if err := scanner.Err(); err != nil {
		return fullReply.String(), fmt.Errorf("读取 SSE 流失败: %w", err)
	}

	return strings.TrimSpace(fullReply.String()), nil
}

// dashScopeStreamChunk DashScope SSE 流式响应的单个数据块
type dashScopeStreamChunk struct {
	Output struct {
		Choices []struct {
			Message      ChatMessage `json:"message"`
			FinishReason string      `json:"finish_reason"`
		} `json:"choices"`
		Text         string `json:"text"`
		FinishReason string `json:"finish_reason"`
	} `json:"output"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// doStreamRequestDashScope 使用 DashScope 原生接口发起流式请求（SSE）
func (a *Agent) doStreamRequestDashScope(ctx context.Context, messages []ChatMessage, providerCfg *config.AIProviderConfig, onToken func(token string)) (string, error) {
	var reqBody dashScopeRequest
	reqBody.Model = providerCfg.Model
	reqBody.Input.Messages = messages
	reqBody.Parameters.ResultFormat = "message"

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %w", err)
	}

	requestURL := resolveBaseURL(providerCfg) + "/services/aigc/text-generation/generation"
	log.Printf("[AI] DashScope 流式请求 url=%s model=%s", requestURL, providerCfg.Model)

	req, err := http.NewRequestWithContext(ctx, "POST", requestURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+providerCfg.APIKey)
	// DashScope 流式模式需要设置此 header
	req.Header.Set("X-DashScope-SSE", "enable")
	req.Header.Set("Accept", "text/event-stream")

	httpClient := a.newHTTPClient(providerCfg)
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyContent, _ := io.ReadAll(resp.Body)
		log.Printf("[AI] DashScope 流式请求错误响应体: %s", string(bodyContent))
		return "", fmt.Errorf("DashScope API 返回非预期状态码 (HTTP %d)，响应: %s", resp.StatusCode, string(bodyContent))
	}

	// DashScope SSE 流式返回的是增量内容，需要拼接
	var fullReply strings.Builder
	var lastContent string

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}

		var chunk dashScopeStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			log.Printf("[AI] 解析 DashScope SSE chunk 失败: %v, data=%s", err, data)
			continue
		}

		if chunk.Code != "" {
			return fullReply.String(), fmt.Errorf("DashScope API 错误 (code=%s): %s", chunk.Code, chunk.Message)
		}

		// DashScope 流式返回的是累积内容，需要计算增量
		var currentContent string
		if len(chunk.Output.Choices) > 0 {
			currentContent = chunk.Output.Choices[0].Message.Content
		} else if chunk.Output.Text != "" {
			currentContent = chunk.Output.Text
		}

		if len(currentContent) > len(lastContent) {
			newToken := currentContent[len(lastContent):]
			lastContent = currentContent
			fullReply.WriteString(newToken)
			onToken(newToken)
		}
	}

	if err := scanner.Err(); err != nil {
		return fullReply.String(), fmt.Errorf("读取 DashScope SSE 流失败: %w", err)
	}

	return strings.TrimSpace(fullReply.String()), nil
}

// ChatSync 同步调用 LLM，使用自定义消息列表（不依赖对话记忆），使用指定 provider。
// 适用于定时任务、后台分析等需要同步结果的场景。
// providerName 为空时使用默认 provider。
func (a *Agent) ChatSync(ctx context.Context, messages []ChatMessage, providerName string) (string, error) {
	if !a.IsEnabled() {
		return "", fmt.Errorf("AI 未启用或未配置 API Key")
	}
	providerCfg := a.aiConfig().GetProviderConfig(providerName)
	if providerCfg == nil {
		return "", fmt.Errorf("找不到 provider 配置: %s", providerName)
	}
	return a.doRequest(ctx, messages, providerCfg)
}

// SummarizeMessages 让 AI 总结一批消息（用于紧急消息智能摘要），使用默认 provider
func (a *Agent) SummarizeMessages(messages []string) string {
	return a.SummarizeMessagesWithProvider(messages, "")
}

// SummarizeMessagesWithProvider 让 AI 总结一批消息，使用指定 provider name
func (a *Agent) SummarizeMessagesWithProvider(messages []string, providerName string) string {
	if !a.IsEnabled() || len(messages) == 0 {
		return ""
	}

	providerCfg := a.aiConfig().GetProviderConfig(providerName)
	if providerCfg == nil {
		return ""
	}

	content := strings.Join(messages, "\n")
	prompt := fmt.Sprintf("请用一句话（30字以内）总结以下消息的核心内容，语气活泼简洁：\n%s", content)

	chatMessages := a.buildMessages(prompt)
	timeout := a.aiConfig().GetEffectiveTimeout(providerCfg)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	reply, err := a.doRequest(ctx, chatMessages, providerCfg)
	if err != nil {
		return ""
	}
	return reply
}

// callLLM 调用 LLM 接口（同步，在 goroutine 中调用）
// traceCtx: 可选的 Trace 上下文，如果提供则在此 Trace 下创建 Generation
func (a *Agent) callLLM(userInput string, providerCfg *config.AIProviderConfig, traceCtx *TraceContext) (string, error) {
	messages := a.buildMessages(userInput)
	timeout := a.aiConfig().GetEffectiveTimeout(providerCfg)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	// 创建Langfuse Generation（在传入的 Trace 下）
	lc := GetLangfuseClient()
	var genCtx *GenerationContext
	if lc.IsEnabled() && traceCtx != nil {
		genCtx, _ = traceCtx.StartGeneration(context.Background(), "llm-call", providerCfg.Model, messages)
	}

	startTime := time.Now()
	reply, err := a.doRequest(ctx, messages, providerCfg)
	duration := time.Since(startTime)

	// 更新Generation
	if genCtx != nil {
		inputTokens := CalculateMessagesTokens(messages)
		outputTokens := CalculateTokens(reply)
		genCtx.End(context.Background(), reply, inputTokens, outputTokens)
		log.Printf("[Langfuse] 调用完成，耗时: %v, 输入token: %d, 输出token: %d", duration, inputTokens, outputTokens)
	}

	return reply, err
}

// buildMessages 构建完整的消息列表（系统 prompt + 用户画像上下文 + 历史上下文 + 当前输入）
func (a *Agent) buildMessages(userInput string) []ChatMessage {
	// 拼接 system prompt：从文件动态加载人设 + 用户画像（若存在）
	fullSystemPrompt := loadSystemPrompt()
	if userProfile := loadUserProfileForPrompt(); userProfile != "" {
		fullSystemPrompt += fmt.Sprintf(`

---
以下是主人的个人画像，请在回答时将其作为背景参考，让回答更贴合主人的实际情况：
<主人画像>
%s
</主人画像>`, userProfile)
	}

	messages := []ChatMessage{
		{Role: "system", Content: fullSystemPrompt},
	}

	// 加入历史上下文（短期记忆）
	history := a.memory.GetRecentContext()
	messages = append(messages, history...)

	// 加入当前用户输入
	messages = append(messages, ChatMessage{
		Role:    "user",
		Content: userInput,
	})

	return messages
}

// resolveBaseURL 根据 provider 配置返回最终使用的 base URL（包级函数，供各请求方法复用）
func resolveBaseURL(providerCfg *config.AIProviderConfig) string {
	if providerCfg.BaseURL != "" {
		return strings.TrimRight(providerCfg.BaseURL, "/")
	}
	switch providerCfg.Provider {
	case config.ProviderOpenAI:
		return "https://api.openai.com/v1"
	case config.ProviderBailian:
		return "https://dashscope.aliyuncs.com/compatible-mode/v1"
	case config.ProviderDashScope:
		return "https://dashscope.aliyuncs.com/api/v1"
	case config.ProviderCoPaw:
		return "http://localhost:8088"
	default:
		// 未配置 provider 时默认走 OpenAI 兼容格式
		return "https://api.openai.com/v1"
	}
}

// doRequest 根据 provider 选择对应的接口规范发起请求
// 注意：此方法不再创建 Trace/Generation，Trace 应在业务层创建
func (a *Agent) doRequest(ctx context.Context, messages []ChatMessage, providerCfg *config.AIProviderConfig) (string, error) {
	// 此方法不再创建 Langfuse Trace/Generation
	// Trace 应在业务层（如 Chat、Analyzer）创建

	switch providerCfg.Provider {
	case config.ProviderDashScope:
		return a.doRequestDashScope(ctx, messages, providerCfg)
	case config.ProviderCoPaw:
		// CoPaw 使用 OpenAI 兼容接口
		return a.doRequestOpenAICompat(ctx, messages, providerCfg)
	default:
		return a.doRequestOpenAICompat(ctx, messages, providerCfg)
	}
}

// doRequestOpenAICompat 使用 OpenAI 兼容接口发起请求
// 适用于：OpenAI、阿里云百炼 compatible-mode、DeepSeek、Moonshot 等
// 注意：此方法不再创建 Trace，只处理实际请求
func (a *Agent) doRequestOpenAICompat(ctx context.Context, messages []ChatMessage, providerCfg *config.AIProviderConfig) (string, error) {
	// 此方法不再创建 Langfuse Trace
	// Trace 应在业务层创建

	reqBody := openAIRequest{
		Model:    providerCfg.Model,
		Messages: messages,
		Stream:   false,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %w", err)
	}

	requestURL := resolveBaseURL(providerCfg) + "/chat/completions"
	log.Printf("[AI] 发起请求 provider=%s url=%s model=%s apiKey=%s",
		providerCfg.Provider, requestURL, providerCfg.Model, providerCfg.APIKey)

	req, err := http.NewRequestWithContext(ctx, "POST", requestURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+providerCfg.APIKey)

	httpClient := a.newHTTPClient(providerCfg)
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应体失败: %w", err)
	}

	log.Printf("[AI] 响应状态码: %d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		log.Printf("[AI] 错误响应体: %s", string(respBytes))
		// 尝试解析 error 字段给出更友好的提示
		var errResp openAIResponse
		if jsonErr := json.Unmarshal(respBytes, &errResp); jsonErr == nil && errResp.Error != nil {
			return "", fmt.Errorf("API 返回错误 (HTTP %d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return "", fmt.Errorf("API 返回非预期状态码 (HTTP %d)，响应: %s", resp.StatusCode, string(respBytes))
	}

	var openAIResp openAIResponse
	if err := json.Unmarshal(respBytes, &openAIResp); err != nil {
		log.Printf("[AI] 响应体解析失败，原始内容: %s", string(respBytes))
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if openAIResp.Error != nil {
		return "", fmt.Errorf("API 错误: %s", openAIResp.Error.Message)
	}

	if len(openAIResp.Choices) == 0 {
		log.Printf("[AI] 响应体无 choices，原始内容: %s", string(respBytes))
		return "", fmt.Errorf("API 返回空结果（choices 为空）")
	}

	reply := strings.TrimSpace(openAIResp.Choices[0].Message.Content)

	return reply, nil
}

// dashScopeRequest DashScope 原生接口请求体
type dashScopeRequest struct {
	Model string `json:"model"`
	Input struct {
		Messages []ChatMessage `json:"messages"`
	} `json:"input"`
	Parameters struct {
		ResultFormat string `json:"result_format"`
	} `json:"parameters"`
}

// dashScopeResponse DashScope 原生接口响应体
type dashScopeResponse struct {
	Output struct {
		Choices []struct {
			Message      ChatMessage `json:"message"`
			FinishReason string      `json:"finish_reason"`
		} `json:"choices"`
		Text string `json:"text"`
	} `json:"output"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// visionMessage 视觉接口的消息格式（content 为数组，支持图片）
type visionMessage struct {
	Role    string              `json:"role"`
	Content []visionContentPart `json:"content"`
}

// visionContentPart 视觉消息内容块（文本或图片）
type visionContentPart struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	ImageURL *visionImageURL `json:"image_url,omitempty"`
}

// visionImageURL 图片 URL（支持 base64 data URI）
type visionImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

// visionRequest 视觉接口请求体（content 为数组格式）
type visionRequest struct {
	Model     string          `json:"model"`
	Messages  []visionMessage `json:"messages"`
	Stream    bool            `json:"stream"`
	MaxTokens int             `json:"max_tokens,omitempty"`
}

// AnalyzeImageBase64 使用视觉模型分析 base64 编码的 JPEG 图片（流式模式）
// providerName 为空时使用默认 provider；prompt 为分析指令
// 使用 SSE streaming 接收结果，避免视觉模型推理耗时长导致 HTTP 超时。
// 返回 AI 的文字分析结果，失败时返回空字符串和错误
func (a *Agent) AnalyzeImageBase64(base64JPEG string, prompt string, providerName string) (string, error) {
	if !a.IsEnabled() {
		return "", fmt.Errorf("AI 未启用或未配置 API Key")
	}

	providerCfg := a.aiConfig().GetProviderConfig(providerName)
	if providerCfg == nil {
		return "", fmt.Errorf("找不到 provider 配置: %s", providerName)
	}

	// 构造 data URI
	dataURI := "data:image/jpeg;base64," + base64JPEG

	messages := []visionMessage{
		{
			Role: "user",
			Content: []visionContentPart{
				{
					Type: "image_url",
					ImageURL: &visionImageURL{
						URL:    dataURI,
						Detail: "low", // 用 low 精度节省 token，背景人物检测不需要高精度
					},
				},
				{
					// 在 prompt 末尾追加 /no_think 指令，关闭 Qwen3 系列模型的思考模式。
					// enable_thinking 是 vLLM 服务端参数，无法通过请求体传递；
					// /no_think 是模型层面的软开关，对 OpenAI 兼容接口同样生效。
					Type: "text",
					Text: prompt + "\n/no_think",
				},
			},
		},
	}

	// 注意：视觉分析不再在此方法内创建 Langfuse Trace
	// Trace 应在调用方（如 CameraPatrol）创建

	// 使用流式模式：服务端一旦开始输出 token 就持续发送数据，
	// 避免视觉模型推理耗时长时 HTTP client 因连接空闲而超时。
	reqBody := visionRequest{
		Model:     providerCfg.Model,
		Messages:  messages,
		Stream:    true,
		MaxTokens: 10000,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("序列化视觉请求失败: %w", err)
	}

	requestURL := resolveBaseURL(providerCfg) + "/chat/completions"
	log.Printf("[Camera] 发起视觉分析流式请求 provider=%s url=%s model=%s",
		providerCfg.Provider, requestURL, providerCfg.Model)

	timeout := a.aiConfig().GetEffectiveTimeout(providerCfg)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", requestURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("创建视觉请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+providerCfg.APIKey)
	req.Header.Set("Accept", "text/event-stream")

	httpClient := a.newHTTPClient(providerCfg)
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("视觉 HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyContent, _ := io.ReadAll(resp.Body)
		log.Printf("[Camera] 视觉 API 错误响应: %s", string(bodyContent))
		var errResp openAIResponse
		if jsonErr := json.Unmarshal(bodyContent, &errResp); jsonErr == nil && errResp.Error != nil {
			return "", fmt.Errorf("视觉 API 错误 (HTTP %d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return "", fmt.Errorf("视觉 API 返回非预期状态码 (HTTP %d): %s", resp.StatusCode, string(bodyContent))
	}

	// 逐行读取 SSE 流，拼接完整回复
	var fullReply strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk openAIStreamChunk
		if parseErr := json.Unmarshal([]byte(data), &chunk); parseErr != nil {
			log.Printf("[Camera] 解析视觉 SSE chunk 失败: %v, data=%s", parseErr, data)
			continue
		}

		for _, choice := range chunk.Choices {
			token := choice.Delta.Content
			if token != "" {
				fullReply.WriteString(token)
			}
		}
	}

	if scanErr := scanner.Err(); scanErr != nil {
		return fullReply.String(), fmt.Errorf("读取视觉 SSE 流失败: %w", scanErr)
	}

	content := strings.TrimSpace(fullReply.String())
	log.Printf("[Camera] 视觉流式响应完成，内容: %s", content)

	if content == "" {
		log.Printf("[Camera] 视觉 API 流式响应返回空 content")
		return "", fmt.Errorf("视觉 API 返回空 content")
	}

	return content, nil
}

// ── CoPaw 专用数据结构 ──────────────────────────────────────────────────────

// copawContentPart CoPaw 请求消息内容块（type=text）
type copawContentPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// copawMessage CoPaw 请求消息体
type copawMessage struct {
	Role    string             `json:"role"`
	Content []copawContentPart `json:"content"`
}

// copawRequest CoPaw /api/agent/process 接口请求体
type copawRequest struct {
	SessionID string         `json:"session_id"`
	UserID    string         `json:"user_id"`
	Input     []copawMessage `json:"input"`
}

// copawSSEChunk CoPaw SSE 流式响应的单个数据块
// type 字段可能为：message / reasoning / heartbeat / error
type copawSSEChunk struct {
	Type    string `json:"type"`
	Message *struct {
		Role    string `json:"role"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"message,omitempty"`
	Error string `json:"error,omitempty"`
}

// doStreamRequestCoPaw 调用本地 CoPaw Agent 的流式接口（SSE）
// CoPaw 不使用 OpenAI 格式，而是 agentscope_runtime 的 /api/agent/process 接口。
// 每个 SSE data 块包含 type 字段，type=message 时提取 content[].text 作为 token。
func (a *Agent) doStreamRequestCoPaw(ctx context.Context, userInput string, providerCfg *config.AIProviderConfig, onToken func(token string)) (string, error) {
	baseURL := resolveBaseURL(providerCfg)

	// CoPaw 使用 session_id 维护对话上下文，user_id 标识用户
	// session_id 从 provider 的 Model 字段读取（复用该字段存储 session 名），
	// 若未配置则使用默认值 "luffot"
	sessionID := providerCfg.Model
	if sessionID == "" {
		sessionID = "luffot"
	}
	userID := "luffot-pet"

	reqBody := copawRequest{
		SessionID: sessionID,
		UserID:    userID,
		Input: []copawMessage{
			{
				Role: "user",
				Content: []copawContentPart{
					{Type: "text", Text: userInput},
				},
			},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("序列化 CoPaw 请求失败: %w", err)
	}

	requestURL := baseURL + "/api/agent/process"
	log.Printf("[AI] CoPaw 流式请求 url=%s sessionID=%s", requestURL, sessionID)

	req, err := http.NewRequestWithContext(ctx, "POST", requestURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("创建 CoPaw 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	// CoPaw 本地服务无需鉴权，超时使用全局配置
	timeout := a.aiConfig().GetEffectiveTimeout(providerCfg)
	httpClient := &http.Client{Timeout: time.Duration(timeout) * time.Second}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("CoPaw HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyContent, _ := io.ReadAll(resp.Body)
		log.Printf("[AI] CoPaw 错误响应: %s", string(bodyContent))
		return "", fmt.Errorf("CoPaw API 返回非预期状态码 (HTTP %d): %s", resp.StatusCode, string(bodyContent))
	}

	var fullReply strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	// 扩大 scanner 缓冲区，防止长行截断
	scanner.Buffer(make([]byte, 64*1024), 64*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}

		var chunk copawSSEChunk
		if parseErr := json.Unmarshal([]byte(data), &chunk); parseErr != nil {
			log.Printf("[AI] 解析 CoPaw SSE chunk 失败: %v, data=%s", parseErr, data)
			continue
		}

		switch chunk.Type {
		case "message":
			// 提取 assistant 消息中的文本内容
			if chunk.Message == nil {
				continue
			}
			for _, part := range chunk.Message.Content {
				if part.Type == "text" && part.Text != "" {
					fullReply.WriteString(part.Text)
					onToken(part.Text)
				}
			}
		case "error":
			if chunk.Error != "" {
				return fullReply.String(), fmt.Errorf("CoPaw Agent 返回错误: %s", chunk.Error)
			}
			// heartbeat / reasoning 等类型忽略，不展示给用户
		}
	}

	if scanErr := scanner.Err(); scanErr != nil {
		return fullReply.String(), fmt.Errorf("读取 CoPaw SSE 流失败: %w", scanErr)
	}

	return strings.TrimSpace(fullReply.String()), nil
}

// doRequestDashScope 使用阿里云 DashScope 原生接口发起请求
// 注意：此方法不再创建 Trace，只处理实际请求
func (a *Agent) doRequestDashScope(ctx context.Context, messages []ChatMessage, providerCfg *config.AIProviderConfig) (string, error) {
	// 此方法不再创建 Langfuse Trace
	// Trace 应在业务层创建

	var reqBody dashScopeRequest
	reqBody.Model = providerCfg.Model
	reqBody.Input.Messages = messages
	reqBody.Parameters.ResultFormat = "message"

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %w", err)
	}

	requestURL := resolveBaseURL(providerCfg) + "/services/aigc/text-generation/generation"
	log.Printf("[AI] DashScope 原生请求 url=%s model=%s", requestURL, providerCfg.Model)

	req, err := http.NewRequestWithContext(ctx, "POST", requestURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+providerCfg.APIKey)

	httpClient := a.newHTTPClient(providerCfg)
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应体失败: %w", err)
	}

	log.Printf("[AI] 响应状态码: %d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		log.Printf("[AI] 错误响应体: %s", string(respBytes))
		return "", fmt.Errorf("DashScope API 返回非预期状态码 (HTTP %d)，响应: %s", resp.StatusCode, string(respBytes))
	}

	var dsResp dashScopeResponse
	if err := json.Unmarshal(respBytes, &dsResp); err != nil {
		log.Printf("[AI] 响应体解析失败，原始内容: %s", string(respBytes))
		return "", fmt.Errorf("解析 DashScope 响应失败: %w", err)
	}

	// DashScope 用非空 code 字段表示业务错误（正常时 code 为空）
	if dsResp.Code != "" {
		return "", fmt.Errorf("DashScope API 错误 (code=%s): %s", dsResp.Code, dsResp.Message)
	}

	var reply string
	if len(dsResp.Output.Choices) > 0 {
		reply = strings.TrimSpace(dsResp.Output.Choices[0].Message.Content)
	} else if dsResp.Output.Text != "" {
		// 部分旧版接口直接返回 text 字段
		reply = strings.TrimSpace(dsResp.Output.Text)
	} else {
		log.Printf("[AI] DashScope 响应体无有效内容，原始内容: %s", string(respBytes))
		return "", fmt.Errorf("DashScope API 返回空结果")
	}

	return reply, nil
}
