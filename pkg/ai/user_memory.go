package ai

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/luffot/luffot/pkg/storage"
)

// MemoryCategory 记忆类别常量
const (
	MemoryCategoryTopic        = "topic"        // 关注话题
	MemoryCategoryRelationship = "relationship" // 社交关系
	MemoryCategoryBehavior     = "behavior"     // 行为模式
	MemoryCategoryPreference   = "preference"   // 偏好习惯
	MemoryCategoryWorkContext  = "work_context" // 工作上下文
)

// ConversationContext 会话上下文分组
// 同一会话中时间间隔较近的消息被归为同一个讨论上下文
type ConversationContext struct {
	SessionName string             `json:"session_name"`
	App         string             `json:"app"`
	StartTime   time.Time          `json:"start_time"`
	EndTime     time.Time          `json:"end_time"`
	Messages    []*storage.Message `json:"messages"`
	Senders     []string           `json:"senders"`      // 参与者列表
	HasSelfMsg  bool               `json:"has_self_msg"` // 用户自己是否参与了讨论
}

// UserMemoryManager 用户记忆管理器
// 负责从消息中提炼结构化记忆、管理记忆生命周期、为 AI 分析提供上下文
type UserMemoryManager struct {
	storage     *storage.Storage
	selfSenders []string // 用户自己可能使用的发送者名称

	// 上下文分组的时间间隔阈值：同一会话中两条消息间隔超过此值则视为不同讨论上下文
	contextGapThreshold time.Duration

	mu sync.RWMutex
}

// NewUserMemoryManager 创建用户记忆管理器
// selfSenders: 用户自己可能使用的发送者名称列表（如 ["我", "飛神"]）
func NewUserMemoryManager(st *storage.Storage, selfSenders []string) *UserMemoryManager {
	if len(selfSenders) == 0 {
		selfSenders = []string{"我"}
	}
	return &UserMemoryManager{
		storage:             st,
		selfSenders:         selfSenders,
		contextGapThreshold: 10 * time.Minute,
	}
}

// SetSelfSenders 更新用户自己的发送者名称列表
func (umm *UserMemoryManager) SetSelfSenders(senders []string) {
	umm.mu.Lock()
	defer umm.mu.Unlock()
	umm.selfSenders = senders
}

// GetSelfSenders 获取用户自己的发送者名称列表
func (umm *UserMemoryManager) GetSelfSenders() []string {
	umm.mu.RLock()
	defer umm.mu.RUnlock()
	result := make([]string, len(umm.selfSenders))
	copy(result, umm.selfSenders)
	return result
}

// isSelfSender 判断发送者是否是用户自己
func (umm *UserMemoryManager) isSelfSender(sender string) bool {
	for _, self := range umm.selfSenders {
		if strings.EqualFold(sender, self) {
			return true
		}
	}
	return false
}

// ---- 记忆 CRUD 封装 ----

// SaveMemory 保存一条新记忆
func (umm *UserMemoryManager) SaveMemory(category, content, sourceSummary string, importance int) (int64, error) {
	entry := &storage.UserMemoryEntry{
		Category:      category,
		Content:       content,
		SourceSummary: sourceSummary,
		Importance:    importance,
	}
	return umm.storage.SaveUserMemory(entry)
}

// UpdateMemory 更新已有记忆
func (umm *UserMemoryManager) UpdateMemory(id int64, content string, importance int, sourceSummary string) error {
	return umm.storage.UpdateUserMemory(id, content, importance, sourceSummary)
}

// GetAllMemories 获取所有记忆（按重要性排序）
func (umm *UserMemoryManager) GetAllMemories(limit int) ([]*storage.UserMemoryEntry, error) {
	return umm.storage.GetUserMemories("", limit)
}

// GetMemoriesByCategory 按类别获取记忆
func (umm *UserMemoryManager) GetMemoriesByCategory(category string, limit int) ([]*storage.UserMemoryEntry, error) {
	return umm.storage.GetUserMemories(category, limit)
}

// SearchMemories 搜索记忆
func (umm *UserMemoryManager) SearchMemories(keyword string, limit int) ([]*storage.UserMemoryEntry, error) {
	return umm.storage.SearchUserMemories(keyword, limit)
}

// CleanupStaleMemories 清理过期低重要性记忆
func (umm *UserMemoryManager) CleanupStaleMemories() (int64, error) {
	return umm.storage.CleanupStaleMemories(3, 30)
}

// ---- 会话参与度分析 ----

// GetSessionParticipation 获取会话参与度统计
func (umm *UserMemoryManager) GetSessionParticipation(app string, sinceDays int) ([]*storage.SessionParticipation, error) {
	umm.mu.RLock()
	selfSenders := make([]string, len(umm.selfSenders))
	copy(selfSenders, umm.selfSenders)
	umm.mu.RUnlock()

	return umm.storage.GetSessionParticipation(app, selfSenders, sinceDays)
}

// ---- 上下文分组 ----

// GroupMessagesIntoContexts 将一批消息按会话和时间间隔分组为讨论上下文
// 同一会话中，两条消息间隔超过 contextGapThreshold 则视为不同讨论上下文
func (umm *UserMemoryManager) GroupMessagesIntoContexts(messages []*storage.Message) []*ConversationContext {
	if len(messages) == 0 {
		return nil
	}

	// 按 session 分组
	sessionMessages := make(map[string][]*storage.Message)
	sessionApp := make(map[string]string)
	for _, msg := range messages {
		key := msg.App + "|" + msg.Session
		sessionMessages[key] = append(sessionMessages[key], msg)
		sessionApp[key] = msg.App
	}

	var contexts []*ConversationContext

	for key, msgs := range sessionMessages {
		app := sessionApp[key]
		// 按时间间隔切分为多个上下文
		sessionContexts := umm.splitIntoContexts(msgs, app)
		contexts = append(contexts, sessionContexts...)
	}

	return contexts
}

// splitIntoContexts 将同一会话的消息按时间间隔切分为多个上下文
func (umm *UserMemoryManager) splitIntoContexts(messages []*storage.Message, app string) []*ConversationContext {
	if len(messages) == 0 {
		return nil
	}

	var contexts []*ConversationContext
	currentContext := &ConversationContext{
		SessionName: messages[0].Session,
		App:         app,
		StartTime:   messages[0].Timestamp,
		Messages:    []*storage.Message{messages[0]},
	}

	senderSet := map[string]bool{messages[0].Sender: true}
	if umm.isSelfSender(messages[0].Sender) {
		currentContext.HasSelfMsg = true
	}

	for i := 1; i < len(messages); i++ {
		msg := messages[i]
		gap := msg.Timestamp.Sub(messages[i-1].Timestamp)

		if gap > umm.contextGapThreshold {
			// 超过间隔阈值，结束当前上下文，开始新的
			currentContext.EndTime = messages[i-1].Timestamp
			currentContext.Senders = mapKeysToSlice(senderSet)
			contexts = append(contexts, currentContext)

			// 开始新上下文
			currentContext = &ConversationContext{
				SessionName: msg.Session,
				App:         app,
				StartTime:   msg.Timestamp,
				Messages:    []*storage.Message{msg},
			}
			senderSet = map[string]bool{msg.Sender: true}
			if umm.isSelfSender(msg.Sender) {
				currentContext.HasSelfMsg = true
			}
		} else {
			// 同一上下文
			currentContext.Messages = append(currentContext.Messages, msg)
			senderSet[msg.Sender] = true
			if umm.isSelfSender(msg.Sender) {
				currentContext.HasSelfMsg = true
			}
		}
	}

	// 收尾最后一个上下文
	currentContext.EndTime = messages[len(messages)-1].Timestamp
	currentContext.Senders = mapKeysToSlice(senderSet)
	contexts = append(contexts, currentContext)

	return contexts
}

// ---- 格式化输出（供 AI 分析使用）----

// FormatMemoriesForPrompt 将记忆格式化为 prompt 注入段
// 按类别分组展示，供 AI 分析时作为上下文参考
func (umm *UserMemoryManager) FormatMemoriesForPrompt(limit int) (string, error) {
	memories, err := umm.GetAllMemories(limit)
	if err != nil {
		return "", err
	}
	if len(memories) == 0 {
		return "", nil
	}

	// 按类别分组
	categoryMemories := make(map[string][]*storage.UserMemoryEntry)
	for _, mem := range memories {
		categoryMemories[mem.Category] = append(categoryMemories[mem.Category], mem)
	}

	categoryLabels := map[string]string{
		MemoryCategoryTopic:        "关注话题",
		MemoryCategoryRelationship: "社交关系",
		MemoryCategoryBehavior:     "行为模式",
		MemoryCategoryPreference:   "偏好习惯",
		MemoryCategoryWorkContext:  "工作上下文",
	}

	var builder strings.Builder
	for category, mems := range categoryMemories {
		label := categoryLabels[category]
		if label == "" {
			label = category
		}
		builder.WriteString(fmt.Sprintf("【%s】\n", label))
		for _, mem := range mems {
			builder.WriteString(fmt.Sprintf("- %s（重要性:%d）\n", mem.Content, mem.Importance))
		}
		builder.WriteString("\n")
	}

	return strings.TrimSpace(builder.String()), nil
}

// FormatParticipationForPrompt 将会话参与度格式化为 prompt 注入段
func (umm *UserMemoryManager) FormatParticipationForPrompt(app string, sinceDays int) (string, error) {
	participations, err := umm.GetSessionParticipation(app, sinceDays)
	if err != nil {
		return "", err
	}
	if len(participations) == 0 {
		return "", nil
	}

	var builder strings.Builder
	builder.WriteString("会话参与度统计：\n")

	activeCount := 0
	passiveCount := 0

	for _, participation := range participations {
		status := "仅旁观"
		if participation.IsActive {
			status = "有参与"
			activeCount++
		} else {
			passiveCount++
		}
		builder.WriteString(fmt.Sprintf("- [%s] %s：总消息%d条，用户发送%d条，%d位参与者，%s\n",
			participation.App,
			participation.Session,
			participation.TotalMessages,
			participation.SelfMessages,
			participation.UniqueSenders,
			status,
		))
	}

	builder.WriteString(fmt.Sprintf("\n总计：%d个活跃会话（有参与），%d个旁观会话（从未发言）\n",
		activeCount, passiveCount))

	return builder.String(), nil
}

// FormatContextsForPrompt 将上下文分组格式化为 prompt 注入段
func (umm *UserMemoryManager) FormatContextsForPrompt(contexts []*ConversationContext) string {
	if len(contexts) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("消息上下文分组（同一讨论主题的消息被归为一组）：\n\n")

	for i, ctx := range contexts {
		participationLabel := "旁观"
		if ctx.HasSelfMsg {
			participationLabel = "参与"
		}

		builder.WriteString(fmt.Sprintf("--- 上下文 #%d [%s][%s] %s~%s（%s，%d条消息，参与者：%s）---\n",
			i+1,
			ctx.App,
			ctx.SessionName,
			ctx.StartTime.Format("01-02 15:04"),
			ctx.EndTime.Format("15:04"),
			participationLabel,
			len(ctx.Messages),
			strings.Join(ctx.Senders, "、"),
		))

		for _, msg := range ctx.Messages {
			selfMark := ""
			if umm.isSelfSender(msg.Sender) {
				selfMark = "【我】"
			}
			builder.WriteString(fmt.Sprintf("  [%s] %s%s: %s\n",
				msg.Timestamp.Format("15:04"),
				selfMark,
				msg.Sender,
				msg.Content,
			))
		}
		builder.WriteString("\n")
	}

	return builder.String()
}

// ---- 记忆解析（从 AI 输出中提取结构化记忆）----

// ApplyMemoryInstructions 解析并执行 AI 返回的记忆更新指令
// 这是 ParseMemoryUpdates 的便捷封装，自动生成 sourceSummary
// 返回执行的操作数量和可能的错误
func (umm *UserMemoryManager) ApplyMemoryInstructions(instructions string) (int, error) {
	if strings.TrimSpace(instructions) == "" {
		return 0, nil
	}
	sourceSummary := fmt.Sprintf("画像分析 %s", time.Now().Format("2006-01-02 15:04"))
	count := umm.ParseMemoryUpdates(instructions, sourceSummary)
	return count, nil
}

// ParseMemoryUpdates 解析 AI 返回的记忆更新指令
// AI 输出格式约定：
//
//	[ADD:category:importance] 记忆内容
//	[UPDATE:id:importance] 更新后的记忆内容
//	[DELETE:id] 删除原因
//
// 返回执行的操作数量
func (umm *UserMemoryManager) ParseMemoryUpdates(aiOutput string, sourceSummary string) int {
	lines := strings.Split(aiOutput, "\n")
	operationCount := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "[ADD:") {
			umm.parseAddInstruction(line, sourceSummary)
			operationCount++
		} else if strings.HasPrefix(line, "[UPDATE:") {
			umm.parseUpdateInstruction(line, sourceSummary)
			operationCount++
		} else if strings.HasPrefix(line, "[DELETE:") {
			umm.parseDeleteInstruction(line)
			operationCount++
		}
	}

	return operationCount
}

// parseAddInstruction 解析 ADD 指令：[ADD:category:importance] 内容
func (umm *UserMemoryManager) parseAddInstruction(line string, sourceSummary string) {
	// 提取 [] 内的指令部分
	closeBracket := strings.Index(line, "]")
	if closeBracket < 0 {
		return
	}

	instruction := line[1:closeBracket] // "ADD:category:importance"
	content := strings.TrimSpace(line[closeBracket+1:])
	if content == "" {
		return
	}

	parts := strings.SplitN(instruction, ":", 3)
	if len(parts) < 3 {
		return
	}

	category := strings.TrimSpace(parts[1])
	importance := parseImportanceValue(parts[2])

	if category == "" {
		category = MemoryCategoryTopic
	}

	id, err := umm.SaveMemory(category, content, sourceSummary, importance)
	if err != nil {
		log.Printf("[UserMemory] 保存记忆失败: %v", err)
		return
	}
	log.Printf("[UserMemory] 新增记忆 #%d [%s] %s", id, category, truncateString(content, 50))
}

// parseUpdateInstruction 解析 UPDATE 指令：[UPDATE:id:importance] 内容
func (umm *UserMemoryManager) parseUpdateInstruction(line string, sourceSummary string) {
	closeBracket := strings.Index(line, "]")
	if closeBracket < 0 {
		return
	}

	instruction := line[1:closeBracket]
	content := strings.TrimSpace(line[closeBracket+1:])
	if content == "" {
		return
	}

	parts := strings.SplitN(instruction, ":", 3)
	if len(parts) < 3 {
		return
	}

	id := parseIDValue(parts[1])
	importance := parseImportanceValue(parts[2])

	if id <= 0 {
		return
	}

	if err := umm.UpdateMemory(id, content, importance, sourceSummary); err != nil {
		log.Printf("[UserMemory] 更新记忆 #%d 失败: %v", id, err)
		return
	}
	log.Printf("[UserMemory] 更新记忆 #%d %s", id, truncateString(content, 50))
}

// parseDeleteInstruction 解析 DELETE 指令：[DELETE:id] 原因
func (umm *UserMemoryManager) parseDeleteInstruction(line string) {
	closeBracket := strings.Index(line, "]")
	if closeBracket < 0 {
		return
	}

	instruction := line[1:closeBracket]
	parts := strings.SplitN(instruction, ":", 2)
	if len(parts) < 2 {
		return
	}

	id := parseIDValue(parts[1])
	if id <= 0 {
		return
	}

	if err := umm.storage.DeleteUserMemory(id); err != nil {
		log.Printf("[UserMemory] 删除记忆 #%d 失败: %v", id, err)
		return
	}
	log.Printf("[UserMemory] 删除记忆 #%d", id)
}

// ---- 辅助函数 ----

// parseImportanceValue 解析重要性数值字符串，返回 1-10 的整数
func parseImportanceValue(s string) int {
	s = strings.TrimSpace(s)
	value := 5
	for _, c := range s {
		if c >= '0' && c <= '9' {
			value = int(c - '0')
			break
		}
	}
	if value < 1 {
		value = 1
	}
	if value > 10 {
		value = 10
	}
	return value
}

// parseIDValue 解析 ID 数值字符串
func parseIDValue(s string) int64 {
	s = strings.TrimSpace(s)
	var id int64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			id = id*10 + int64(c-'0')
		}
	}
	return id
}

// mapKeysToSlice 将 map 的 key 转为 slice
func mapKeysToSlice(m map[string]bool) []string {
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}

// truncateString 截断字符串
func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
