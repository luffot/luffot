package ai

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/luffot/luffot/pkg/config"
	"github.com/luffot/luffot/pkg/prompt"
	"github.com/luffot/luffot/pkg/storage"
)

// AlertCallback 触发桌宠气泡通知的回调函数类型
type AlertCallback func(message string)

// IntelliAnalyzer 智能消息分析器
// 定时从数据库拉取未分析的消息，用大模型判断重要性并推送气泡通知，
// 同时维护一份基于消息内容的个人画像和结构化用户记忆供后续分析使用。
type IntelliAnalyzer struct {
	agent         *Agent
	storage       *storage.Storage
	memoryManager *UserMemoryManager
	onAlert       AlertCallback
	batchCount    int // 已完成的批次数，用于控制画像更新频率
}

// NewIntelliAnalyzer 创建智能消息分析器
// agent：AI 智能体（用于调用 LLM）
// st：存储层（读取消息、读写分析状态和画像）
// onAlert：有重要消息时触发的气泡通知回调
func NewIntelliAnalyzer(agent *Agent, st *storage.Storage, onAlert AlertCallback) *IntelliAnalyzer {
	return &IntelliAnalyzer{
		agent:         agent,
		storage:       st,
		memoryManager: NewUserMemoryManager(st, []string{"我"}),
		onAlert:       onAlert,
	}
}

// SetSelfSenders 设置用户自己的发送者名称列表（用于会话参与度分析）
func (a *IntelliAnalyzer) SetSelfSenders(senders []string) {
	if a.memoryManager != nil {
		a.memoryManager.SetSelfSenders(senders)
	}
}

// GetMemoryManager 获取用户记忆管理器（供外部访问）
func (a *IntelliAnalyzer) GetMemoryManager() *UserMemoryManager {
	return a.memoryManager
}

// Start 在后台 goroutine 中启动定时分析循环，非阻塞
func (a *IntelliAnalyzer) Start(ctx context.Context) {
	cfg := config.GetIntelliAnalyzerConfig()
	if !cfg.Enabled {
		log.Println("[IntelliAnalyzer] 未启用，跳过启动")
		return
	}

	if a.agent == nil || !a.agent.IsEnabled() {
		log.Println("[IntelliAnalyzer] AI 未启用，无法启动智能分析器")
		return
	}

	intervalSeconds := cfg.IntervalSeconds
	if intervalSeconds <= 0 {
		intervalSeconds = 120
	}

	log.Printf("[IntelliAnalyzer] 启动成功，分析间隔=%ds，批次大小=%d", intervalSeconds, cfg.BatchSize)

	go func() {
		// 启动后稍作延迟，等待其他模块初始化完成
		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Second):
		}

		ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Println("[IntelliAnalyzer] 收到退出信号，停止分析")
				return
			case <-ticker.C:
				a.runOneBatch(ctx)
			}
		}
	}()
}

// runOneBatch 执行一次分析批次：拉取未分析消息 → LLM 判断重要性 → 推送通知 → 更新画像
func (a *IntelliAnalyzer) runOneBatch(ctx context.Context) {
	// 重新读取配置，支持运行时热更新
	cfg := config.GetIntelliAnalyzerConfig()
	if !cfg.Enabled {
		return
	}

	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = 50
	}
	profileUpdateEvery := cfg.ProfileUpdateEveryBatches
	if profileUpdateEvery <= 0 {
		profileUpdateEvery = 5
	}

	// 创建 Trace 追踪整个批处理流程
	traceCtx, err := StartTrace(ctx, "intelli-analyzer-batch", "", nil, map[string]interface{}{
		"batch_size":           batchSize,
		"profile_update_every": profileUpdateEvery,
	})
	if err != nil {
		log.Printf("[IntelliAnalyzer] 创建Trace失败: %v", err)
	}

	// 确保 Trace 在函数结束时被结束
	if traceCtx != nil {
		defer func() {
			_ = traceCtx.End(ctx, map[string]interface{}{
				"status": "completed",
			})
		}()
	}

	// 创建 Span 追踪批处理流程
	var spanCtx *SpanContext
	if traceCtx != nil {
		spanCtx, err = traceCtx.StartSpan(ctx, "batch_processing", map[string]interface{}{
			"batch_size": batchSize,
		})
		if err != nil {
			log.Printf("[IntelliAnalyzer] 创建Span失败: %v", err)
		}
	}

	if spanCtx != nil {
		defer func() {
			_ = spanCtx.End(ctx, map[string]interface{}{
				"status": "completed",
			})
		}()
	}

	// 获取上次分析到的消息 ID
	lastID, err := a.storage.GetLastAnalyzedMessageID()
	if err != nil {
		log.Printf("[IntelliAnalyzer] 读取分析状态失败: %v", err)
		if spanCtx != nil {
			_ = spanCtx.End(ctx, map[string]interface{}{
				"status": "failed",
				"error":  err.Error(),
			})
		}
		return
	}

	// 拉取未分析的消息
	messages, err := a.storage.GetUnanalyzedMessages(lastID, batchSize)
	if err != nil {
		log.Printf("[IntelliAnalyzer] 读取未分析消息失败: %v", err)
		if spanCtx != nil {
			_ = spanCtx.End(ctx, map[string]interface{}{
				"status": "failed",
				"error":  err.Error(),
			})
		}
		return
	}

	if len(messages) == 0 {
		log.Println("[IntelliAnalyzer] 暂无新消息需要分析")
		if spanCtx != nil {
			_ = spanCtx.End(ctx, map[string]interface{}{
				"status": "no_messages",
			})
		}
		return
	}

	log.Printf("[IntelliAnalyzer] 本批次待分析消息数: %d（起始 ID > %d）", len(messages), lastID)

	// 将消息按会话和时间间隔分组为讨论上下文
	contexts := a.memoryManager.GroupMessagesIntoContexts(messages)
	log.Printf("[IntelliAnalyzer] 消息分组为 %d 个讨论上下文", len(contexts))

	// 读取当前个人画像，作为分析上下文
	userProfile, err := a.storage.GetUserProfile()
	if err != nil {
		log.Printf("[IntelliAnalyzer] 读取个人画像失败（将忽略）: %v", err)
		userProfile = ""
	}

	// 读取用户记忆，作为长期分析上下文
	memoryText, memErr := a.memoryManager.FormatMemoriesForPrompt(30)
	if memErr != nil {
		log.Printf("[IntelliAnalyzer] 读取用户记忆失败（将忽略）: %v", memErr)
		memoryText = ""
	}

	// 读取会话参与度统计
	participationText, partErr := a.memoryManager.FormatParticipationForPrompt("", 7)
	if partErr != nil {
		log.Printf("[IntelliAnalyzer] 读取会话参与度失败（将忽略）: %v", partErr)
		participationText = ""
	}

	// 构建结构化的消息文本（使用上下文分组格式）
	contextualMessagesText := a.memoryManager.FormatContextsForPrompt(contexts)

	// 调用 LLM 分析重要性
	importantNotices, err := a.analyzeImportance(ctx, messages, userProfile, memoryText, participationText, contextualMessagesText, cfg.ProviderName)
	if err != nil {
		log.Printf("[IntelliAnalyzer] 重要性分析失败: %v", err)
		// 分析失败时仍然推进 lastID，避免卡死在同一批消息上
		// 更新 Trace 状态为失败
		if traceCtx != nil {
			_ = traceCtx.End(ctx, map[string]interface{}{
				"status":       "failed",
				"error":        err.Error(),
				"notice_count": 0,
			})
			// 防止 defer 中再次结束
			traceCtx = nil
		}
		return
	}

	// 推送气泡通知
	for _, notice := range importantNotices {
		log.Printf("[IntelliAnalyzer] 推送重要通知: %s", notice)
		if a.onAlert != nil {
			a.onAlert(notice)
		}
	}

	// 更新已分析到的最大消息 ID
	newLastID := messages[len(messages)-1].ID
	if saveErr := a.storage.SaveLastAnalyzedMessageID(newLastID); saveErr != nil {
		log.Printf("[IntelliAnalyzer] 保存分析状态失败: %v", saveErr)
	}

	a.batchCount++

	// 每隔 profileUpdateEvery 批次更新一次个人画像和用户记忆
	if a.batchCount%profileUpdateEvery == 0 {
		a.updateUserProfileAndMemory(ctx, messages, contexts, userProfile, memoryText, participationText, contextualMessagesText, cfg.ProviderName)

		// 定期清理过期的低重要性记忆
		if cleaned, cleanErr := a.memoryManager.CleanupStaleMemories(); cleanErr != nil {
			log.Printf("[IntelliAnalyzer] 清理过期记忆失败: %v", cleanErr)
		} else if cleaned > 0 {
			log.Printf("[IntelliAnalyzer] 已清理 %d 条过期记忆", cleaned)
		}
	}
}

// analyzeImportance 调用 LLM 判断一批消息中哪些值得通知用户
// 返回需要推送的通知文本列表（每条对应一个重要事项）
// memoryText: 用户长期记忆的格式化文本
// participationText: 会话参与度统计的格式化文本
// contextualMessagesText: 按上下文分组的结构化消息文本
func (a *IntelliAnalyzer) analyzeImportance(
	ctx context.Context,
	messages []*storage.Message,
	userProfile string,
	memoryText string,
	participationText string,
	contextualMessagesText string,
	providerName string,
) ([]string, error) {
	// 使用上下文分组的结构化消息文本（如果有），否则回退到扁平格式
	messagesText := contextualMessagesText
	if messagesText == "" {
		var msgLines []string
		for _, msg := range messages {
			line := fmt.Sprintf("[%s][%s] %s: %s",
				msg.Timestamp.Format("01-02 15:04"),
				msg.Session,
				msg.Sender,
				msg.Content,
			)
			msgLines = append(msgLines, line)
		}
		messagesText = strings.Join(msgLines, "\n")
	}

	// 创建 Trace 追踪消息重要性分析
	traceCtx, err := StartTrace(ctx, "intelli-analyzer-importance", "", messagesText, map[string]interface{}{
		"message_count":     len(messages),
		"has_profile":       userProfile != "",
		"has_memory":        memoryText != "",
		"has_participation": participationText != "",
	})
	if err != nil {
		log.Printf("[IntelliAnalyzer] 创建Trace失败: %v", err)
	}

	providerCfg := a.agent.aiConfig().GetProviderConfig(providerName)
	if providerCfg == nil {
		if traceCtx != nil {
			_ = traceCtx.End(ctx, map[string]interface{}{
				"status": "failed",
				"error":  fmt.Sprintf("provider not found: %s", providerName),
			})
		}
		return nil, fmt.Errorf("找不到 provider 配置: %s", providerName)
	}

	// 构建 profile 注入段（有画像时插入，无画像时为空字符串）
	profileSection := ""
	if userProfile != "" {
		profileSection = fmt.Sprintf(`
以下是用户的个人画像，请结合画像判断消息对该用户的重要程度：
<用户画像>
%s
</用户画像>
`, userProfile)
	}

	// 构建用户记忆注入段
	memorySection := ""
	if memoryText != "" {
		memorySection = fmt.Sprintf(`
以下是用户的长期记忆（从历史消息中提炼的结构化知识），请结合记忆判断消息的重要性：
<用户记忆>
%s
</用户记忆>
`, memoryText)
	}

	// 构建会话参与度注入段
	participationSection := ""
	if participationText != "" {
		participationSection = fmt.Sprintf(`
以下是用户的会话参与度统计，用户从未发言的会话优先级较低：
<会话参与度>
%s
</会话参与度>
`, participationText)
	}

	// 合并所有上下文注入段
	fullContextSection := profileSection + memorySection + participationSection

	// 从文件加载 prompt 模板，用占位符替换实际内容
	importanceSystemPrompt, err := prompt.Load("analyzer_importance_system")
	if err != nil {
		log.Printf("[IntelliAnalyzer] 加载 analyzer_importance_system prompt 失败，使用默认值: %v", err)
		importanceSystemPrompt = prompt.DefaultContent("analyzer_importance_system")
	}
	importanceUserTemplate, err := prompt.Load("analyzer_importance_user")
	if err != nil {
		log.Printf("[IntelliAnalyzer] 加载 analyzer_importance_user prompt 失败，使用默认值: %v", err)
		importanceUserTemplate = prompt.DefaultContent("analyzer_importance_user")
	}
	importanceUserPrompt := strings.ReplaceAll(importanceUserTemplate, "{{profile}}", fullContextSection)
	importanceUserPrompt = strings.ReplaceAll(importanceUserPrompt, "{{messages}}", messagesText)

	chatMessages := []ChatMessage{
		{Role: "system", Content: importanceSystemPrompt},
		{Role: "user", Content: importanceUserPrompt},
	}

	// 创建 Generation 追踪 LLM 调用
	var genCtx *GenerationContext
	if traceCtx != nil {
		genCtx, err = traceCtx.StartGeneration(ctx, "analyze_importance_llm", providerCfg.Model, chatMessages)
		if err != nil {
			log.Printf("[IntelliAnalyzer] 创建Generation失败: %v", err)
		}
	}

	timeout := a.agent.aiConfig().GetEffectiveTimeout(providerCfg)
	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	reply, err := a.agent.doRequest(reqCtx, chatMessages, providerCfg)
	if err != nil {
		if genCtx != nil {
			// 使用 background context 避免 reqCtx 被取消导致上报失败
			_ = genCtx.End(context.Background(), "", 0, 0)
		}
		if traceCtx != nil {
			_ = traceCtx.End(ctx, map[string]interface{}{
				"status": "failed",
				"error":  err.Error(),
			})
		}
		return nil, fmt.Errorf("LLM 请求失败: %w", err)
	}

	reply = strings.TrimSpace(reply)
	if reply == "" || strings.EqualFold(reply, "NONE") {
		if genCtx != nil {
			// 使用 background context 避免 reqCtx 被取消导致上报失败
			_ = genCtx.End(context.Background(), reply, CalculateMessagesTokens(chatMessages), CalculateTokens(reply))
		}
		if traceCtx != nil {
			_ = traceCtx.End(ctx, map[string]interface{}{
				"status": "no_notices",
			})
		}
		return nil, nil
	}

	// 解析输出：每行一条通知，过滤掉 NONE 和空行
	var notices []string
	for _, line := range strings.Split(reply, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.EqualFold(line, "NONE") {
			continue
		}
		notices = append(notices, line)
	}

	// 结束 Generation，记录输出和 token 消耗
	if genCtx != nil {
		// 使用 background context 避免 reqCtx 被取消导致上报失败
		_ = genCtx.End(context.Background(), reply, CalculateMessagesTokens(chatMessages), CalculateTokens(reply))
	}

	// 创建结束事件（在 trace 结束前）
	if traceCtx != nil {
		if err := GetLangfuseClient().CreateEvent(ctx, traceCtx.TraceID, "analyze_importance_end", map[string]interface{}{
			"status":       "completed",
			"notice_count": len(notices),
		}); err != nil {
			log.Printf("[IntelliAnalyzer] 创建Event失败: %v", err)
		}
		// 结束 Trace，触发上报
		if err := traceCtx.End(ctx, notices); err != nil {
			log.Printf("[IntelliAnalyzer] 结束Trace失败: %v", err)
		}
	}

	return notices, nil
}

// updateUserProfileAndMemory 根据本批消息、上下文分组和旧画像，调用 LLM 同时更新个人画像和用户记忆
// LLM 输出格式约定：
//
//	画像文本在 <profile>...</profile> 标签内
//	记忆更新指令在 <memory_updates>...</memory_updates> 标签内
func (a *IntelliAnalyzer) updateUserProfileAndMemory(
	ctx context.Context,
	messages []*storage.Message,
	contexts []*ConversationContext,
	currentProfile string,
	memoryText string,
	participationText string,
	contextualMessagesText string,
	providerName string,
) {
	// 使用上下文分组的结构化消息文本（如果有），否则回退到扁平格式
	messagesText := contextualMessagesText
	if messagesText == "" {
		var msgLines []string
		for _, msg := range messages {
			line := fmt.Sprintf("[%s][%s] %s: %s",
				msg.Timestamp.Format("01-02 15:04"),
				msg.Session,
				msg.Sender,
				msg.Content,
			)
			msgLines = append(msgLines, line)
		}
		messagesText = strings.Join(msgLines, "\n")
	}

	// 构建旧画像注入段
	oldProfileSection := ""
	if currentProfile != "" {
		oldProfileSection = fmt.Sprintf(`
当前已有的用户画像（请在此基础上更新，保留准确信息，修正或补充新信息）：
<旧画像>
%s
</旧画像>
`, currentProfile)
	}

	// 构建现有记忆注入段
	existingMemorySection := ""
	if memoryText != "" {
		existingMemorySection = fmt.Sprintf(`
当前已有的用户记忆（请在此基础上增量更新，避免重复添加已有记忆）：
<现有记忆>
%s
</现有记忆>
`, memoryText)
	}

	// 构建会话参与度注入段
	participationSection := ""
	if participationText != "" {
		participationSection = fmt.Sprintf(`
用户的会话参与度统计（用户从未发言的会话说明关注度低）：
<会话参与度>
%s
</会话参与度>
`, participationText)
	}

	// 从文件加载 prompt 模板，用占位符替换实际内容
	profileSystemPrompt, err := prompt.Load("analyzer_profile_system")
	if err != nil {
		log.Printf("[IntelliAnalyzer] 加载 analyzer_profile_system prompt 失败，使用默认值: %v", err)
		profileSystemPrompt = prompt.DefaultContent("analyzer_profile_system")
	}
	profileUserTemplate, err := prompt.Load("analyzer_profile_user")
	if err != nil {
		log.Printf("[IntelliAnalyzer] 加载 analyzer_profile_user prompt 失败，使用默认值: %v", err)
		profileUserTemplate = prompt.DefaultContent("analyzer_profile_user")
	}
	profileUserPrompt := strings.ReplaceAll(profileUserTemplate, "{{old_profile}}", oldProfileSection)
	profileUserPrompt = strings.ReplaceAll(profileUserPrompt, "{{messages}}", messagesText)
	profileUserPrompt = strings.ReplaceAll(profileUserPrompt, "{{existing_memory}}", existingMemorySection)
	profileUserPrompt = strings.ReplaceAll(profileUserPrompt, "{{participation}}", participationSection)

	// 创建 Trace 追踪画像更新
	traceCtx, err := StartTrace(ctx, "intelli-analyzer-profile-memory", "", profileUserPrompt, map[string]interface{}{
		"message_count":     len(messages),
		"context_count":     len(contexts),
		"has_old_profile":   currentProfile != "",
		"has_memory":        memoryText != "",
		"has_participation": participationText != "",
	})
	if err != nil {
		log.Printf("[IntelliAnalyzer] 创建Trace失败: %v", err)
	}

	providerCfg := a.agent.aiConfig().GetProviderConfig(providerName)
	if providerCfg == nil {
		log.Printf("[IntelliAnalyzer] 更新画像失败：找不到 provider 配置: %s", providerName)
		if traceCtx != nil {
			if err := GetLangfuseClient().CreateEvent(ctx, traceCtx.TraceID, "provider_not_found", map[string]interface{}{
				"provider": providerName,
			}); err != nil {
				log.Printf("[IntelliAnalyzer] 创建Event失败: %v", err)
			}
			_ = traceCtx.End(ctx, map[string]interface{}{
				"status": "failed",
				"error":  fmt.Sprintf("provider not found: %s", providerName),
			})
		}
		return
	}

	chatMessages := []ChatMessage{
		{Role: "system", Content: profileSystemPrompt},
		{Role: "user", Content: profileUserPrompt},
	}

	// 创建 Generation 追踪 LLM 调用
	var genCtx *GenerationContext
	if traceCtx != nil {
		genCtx, err = traceCtx.StartGeneration(ctx, "update_profile_memory_llm", providerCfg.Model, chatMessages)
		if err != nil {
			log.Printf("[IntelliAnalyzer] 创建Generation失败: %v", err)
		}
	}

	timeout := a.agent.aiConfig().GetEffectiveTimeout(providerCfg)
	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	llmReply, err := a.agent.doRequest(reqCtx, chatMessages, providerCfg)
	if err != nil {
		log.Printf("[IntelliAnalyzer] 生成画像和记忆失败: %v", err)
		if genCtx != nil {
			_ = genCtx.End(context.Background(), "", 0, 0)
		}
		if traceCtx != nil {
			_ = traceCtx.End(ctx, map[string]interface{}{
				"status": "failed",
				"error":  err.Error(),
			})
		}
		return
	}

	llmReply = strings.TrimSpace(llmReply)
	if llmReply == "" {
		if genCtx != nil {
			_ = genCtx.End(context.Background(), llmReply, CalculateMessagesTokens(chatMessages), CalculateTokens(llmReply))
		}
		if traceCtx != nil {
			_ = traceCtx.End(ctx, map[string]interface{}{
				"status": "empty_result",
			})
		}
		return
	}

	// 结束 Generation，记录输出和 token 消耗
	if genCtx != nil {
		_ = genCtx.End(context.Background(), llmReply, CalculateMessagesTokens(chatMessages), CalculateTokens(llmReply))
	}

	// 解析 LLM 输出：提取画像和记忆更新指令
	newProfile, memoryInstructions := a.parseLLMProfileAndMemory(llmReply)

	// 保存画像
	if newProfile != "" {
		if saveErr := a.storage.SaveUserProfile(newProfile); saveErr != nil {
			log.Printf("[IntelliAnalyzer] 保存个人画像失败: %v", saveErr)
			if traceCtx != nil {
				_ = GetLangfuseClient().CreateEvent(ctx, traceCtx.TraceID, "update_profile_failed", map[string]interface{}{
					"status": "save_failed",
					"error":  saveErr.Error(),
				})
			}
		} else {
			log.Printf("[IntelliAnalyzer] 个人画像已更新（%d 字）", len([]rune(newProfile)))
		}
	}

	// 执行记忆更新指令
	if memoryInstructions != "" {
		appliedCount, applyErr := a.memoryManager.ApplyMemoryInstructions(memoryInstructions)
		if applyErr != nil {
			log.Printf("[IntelliAnalyzer] 执行记忆更新指令失败: %v", applyErr)
		} else {
			log.Printf("[IntelliAnalyzer] 已执行 %d 条记忆更新指令", appliedCount)
		}
	}

	// 创建结束事件并结束 trace
	if traceCtx != nil {
		if err := GetLangfuseClient().CreateEvent(ctx, traceCtx.TraceID, "update_profile_memory_end", map[string]interface{}{
			"status":             "completed",
			"profile_length":     len([]rune(newProfile)),
			"has_memory_updates": memoryInstructions != "",
		}); err != nil {
			log.Printf("[IntelliAnalyzer] 创建Event失败: %v", err)
		}
		if err := traceCtx.End(ctx, map[string]interface{}{
			"profile_length":     len([]rune(newProfile)),
			"has_memory_updates": memoryInstructions != "",
		}); err != nil {
			log.Printf("[IntelliAnalyzer] 结束Trace失败: %v", err)
		}
	}
}

// parseLLMProfileAndMemory 从 LLM 输出中解析画像文本和记忆更新指令
// 输出格式约定：
//
//	<profile>画像文本</profile>
//	<memory_updates>记忆更新指令</memory_updates>
//
// 如果没有标签包裹，则整个输出视为画像文本（向后兼容）
func (a *IntelliAnalyzer) parseLLMProfileAndMemory(llmReply string) (profile string, memoryInstructions string) {
	// 提取 <profile>...</profile>
	profileStart := strings.Index(llmReply, "<profile>")
	profileEnd := strings.Index(llmReply, "</profile>")
	if profileStart >= 0 && profileEnd > profileStart {
		profile = strings.TrimSpace(llmReply[profileStart+len("<profile>") : profileEnd])
	}

	// 提取 <memory_updates>...</memory_updates>
	memStart := strings.Index(llmReply, "<memory_updates>")
	memEnd := strings.Index(llmReply, "</memory_updates>")
	if memStart >= 0 && memEnd > memStart {
		memoryInstructions = strings.TrimSpace(llmReply[memStart+len("<memory_updates>") : memEnd])
	}

	// 向后兼容：如果没有 <profile> 标签，整个输出视为画像文本
	if profile == "" && profileStart < 0 {
		profile = strings.TrimSpace(llmReply)
		// 如果有 memory_updates 标签但没有 profile 标签，去掉 memory_updates 部分
		if memStart >= 0 && memEnd > memStart {
			beforeMem := strings.TrimSpace(llmReply[:memStart])
			afterMem := ""
			if memEnd+len("</memory_updates>") < len(llmReply) {
				afterMem = strings.TrimSpace(llmReply[memEnd+len("</memory_updates>"):])
			}
			profile = strings.TrimSpace(beforeMem + "\n" + afterMem)
		}
	}

	return profile, memoryInstructions
}
