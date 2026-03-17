package ai

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/luffot/luffot/pkg/eventbus"
)

// MessageEvent 消息事件（避免循环导入，重新定义）
type MessageEvent struct {
	App       string    `json:"app"`       // 应用名称
	Session   string    `json:"session"`   // 会话名称
	Sender    string    `json:"sender"`    // 发送者
	Content   string    `json:"content"`   // 消息内容
	Timestamp time.Time `json:"timestamp"` // 消息时间
}

// AppType 应用类型
type AppType string

const (
	// AppTypeDingTalk 钉钉
	AppTypeDingTalk AppType = "dingtalk"
	// AppTypeWeChat 微信
	AppTypeWeChat AppType = "wechat"
	// AppTypeQQ QQ
	AppTypeQQ AppType = "qq"
	// AppTypeFeishu 飞书
	AppTypeFeishu AppType = "feishu"
	// AppTypeCustom 自定义应用
	AppTypeCustom AppType = "custom"
)

// AppEvent 应用事件
type AppEvent struct {
	Type      string    `json:"type"`      // 事件类型：message, meeting, task
	Title     string    `json:"title"`     // 事件标题
	Content   string    `json:"content"`   // 事件内容
	Sender    string    `json:"sender"`    // 发送者
	Session   string    `json:"session"`   // 会话/群聊名称
	Priority  string    `json:"priority"`  // 优先级：low, normal, high, urgent
	Timestamp time.Time `json:"timestamp"` // 事件时间
	IsUrgent  bool      `json:"is_urgent"` // 是否紧急
}

// AppSecretary 应用秘书智能体
// 职责：实时监听应用内的关键事件，提取事件上下文，将结构化事件上报至EventBus
// 新增：AI 总结能力、汇报决策能力
type AppSecretary struct {
	appType  AppType
	appName  string
	eventBus *eventbus.EventBus

	// 消息处理
	messageHandler func(event *MessageEvent)

	// 紧急关键词
	urgentKeywords []string

	// 去重缓存
	recentEvents map[string]time.Time
	dedupWindow  time.Duration

	// 会议检测
	inMeeting        bool
	meetingStartTime time.Time

	// 状态
	running bool
	mu      sync.RWMutex

	// AI 智能体（用于消息总结）
	agent *Agent

	// 消息聚合窗口（用于批量总结）
	messageBuffer []*MessageEvent
	bufferWindow  time.Duration
	bufferTicker  *time.Ticker
	maxBufferSize int

	// 汇报策略
	reportStrategy ReportStrategy
}

// ReportStrategy 汇报策略
type ReportStrategy struct {
	// 是否启用 AI 总结
	EnableAISummary bool `json:"enable_ai_summary"`
	// 紧急消息立即汇报
	UrgentImmediate bool `json:"urgent_immediate"`
	// 普通消息批量汇报间隔（秒）
	BatchInterval int `json:"batch_interval"`
	// 最小汇报优先级（低于此优先级不汇报）
	MinReportPriority string `json:"min_report_priority"` // "low", "normal", "high", "urgent"
	// 同一发送者消息合并
	MergeSameSender bool `json:"merge_same_sender"`
	// 会话去重（同一会话只汇报最新）
	SessionDeduplicate bool `json:"session_deduplicate"`
}

// DefaultReportStrategy 默认汇报策略
func DefaultReportStrategy() ReportStrategy {
	return ReportStrategy{
		EnableAISummary:    true,
		UrgentImmediate:    true,
		BatchInterval:      30,
		MinReportPriority:  "normal",
		MergeSameSender:    true,
		SessionDeduplicate: false,
	}
}

// NewAppSecretary 创建应用秘书
func NewAppSecretary(appType AppType, appName string) *AppSecretary {
	return &AppSecretary{
		appType:        appType,
		appName:        appName,
		eventBus:       eventbus.GetGlobalEventBus(),
		urgentKeywords: getDefaultUrgentKeywords(),
		recentEvents:   make(map[string]time.Time),
		dedupWindow:    30 * time.Second,
		messageBuffer:  make([]*MessageEvent, 0),
		bufferWindow:   30 * time.Second,
		maxBufferSize:  50,
		reportStrategy: DefaultReportStrategy(),
	}
}

// SetAgent 设置 AI 智能体（用于消息总结）
func (as *AppSecretary) SetAgent(agent *Agent) {
	as.mu.Lock()
	defer as.mu.Unlock()
	as.agent = agent
}

// SetReportStrategy 设置汇报策略
func (as *AppSecretary) SetReportStrategy(strategy ReportStrategy) {
	as.mu.Lock()
	defer as.mu.Unlock()
	as.reportStrategy = strategy
}

// Start 启动应用秘书
func (as *AppSecretary) Start() {
	as.mu.Lock()
	defer as.mu.Unlock()

	if as.running {
		return
	}

	as.running = true

	// 设置消息处理回调
	as.messageHandler = as.handleMessageEvent

	// 启动清理定时器
	go as.cleanupLoop()

	// 启动消息聚合定时器
	if as.reportStrategy.BatchInterval > 0 {
		as.bufferTicker = time.NewTicker(time.Duration(as.reportStrategy.BatchInterval) * time.Second)
		go as.batchProcessLoop()
	}

	log.Printf("[AppSecretary-%s] 应用秘书启动", as.appName)
}

// Stop 停止应用秘书
func (as *AppSecretary) Stop() {
	as.mu.Lock()
	defer as.mu.Unlock()

	if !as.running {
		return
	}

	as.running = false

	if as.bufferTicker != nil {
		as.bufferTicker.Stop()
	}

	log.Printf("[AppSecretary-%s] 应用秘书停止", as.appName)
}

// batchProcessLoop 批量处理循环
func (as *AppSecretary) batchProcessLoop() {
	for {
		select {
		case <-as.bufferTicker.C:
			as.flushBuffer()
		}
	}
}

// flushBuffer 刷新消息缓冲区，生成批量汇报
func (as *AppSecretary) flushBuffer() {
	as.mu.Lock()
	defer as.mu.Unlock()

	if len(as.messageBuffer) == 0 {
		return
	}

	// 复制并清空缓冲区
	messages := make([]*MessageEvent, len(as.messageBuffer))
	copy(messages, as.messageBuffer)
	as.messageBuffer = as.messageBuffer[:0]

	// 生成批量汇报事件
	as.generateBatchReport(messages)
}

// generateBatchReport 生成批量汇报
func (as *AppSecretary) generateBatchReport(messages []*MessageEvent) {
	if len(messages) == 0 {
		return
	}

	// 按发送者分组
	senderGroups := make(map[string][]*MessageEvent)
	for _, msg := range messages {
		senderGroups[msg.Sender] = append(senderGroups[msg.Sender], msg)
	}

	var summaries []string
	for sender, msgs := range senderGroups {
		if len(msgs) == 1 {
			summaries = append(summaries, fmt.Sprintf("%s: %s", sender, msgs[0].Content))
		} else {
			// 同一发送者多条消息
			contents := make([]string, len(msgs))
			for i, m := range msgs {
				contents[i] = m.Content
			}
			summaries = append(summaries, fmt.Sprintf("%s (%d条): %s", sender, len(msgs), strings.Join(contents, "; ")))
		}
	}

	// 构建汇报内容
	content := strings.Join(summaries, "\n")

	// 如果有 AI 智能体，生成智能摘要
	if as.reportStrategy.EnableAISummary && as.agent != nil && as.agent.IsEnabled() {
		summary := as.summarizeMessages(messages)
		if summary != "" {
			content = summary
		}
	}

	// 发布批量汇报事件
	event := eventbus.NewEvent(
		eventbus.AppMessageBatchReport,
		fmt.Sprintf("app_secretary_%s", as.appName),
		map[string]interface{}{
			"app_name":      as.appName,
			"message_count": len(messages),
			"senders":       extractSenders(messages),
			"content":       content,
			"raw_messages":  convertMessagesToMaps(messages),
		},
	).WithPriority(eventbus.PriorityNormal).
		WithDescription(fmt.Sprintf("[%s] 收到 %d 条消息", as.appName, len(messages)))

	as.eventBus.Publish(event)

	log.Printf("[AppSecretary-%s] 批量汇报: %d 条消息", as.appName, len(messages))
}

// summarizeMessages 使用 AI 总结消息
func (as *AppSecretary) summarizeMessages(messages []*MessageEvent) string {
	if as.agent == nil || !as.agent.IsEnabled() {
		return ""
	}

	// 构建消息文本
	var messageTexts []string
	for _, msg := range messages {
		messageTexts = append(messageTexts, fmt.Sprintf("[%s] %s: %s", msg.Session, msg.Sender, msg.Content))
	}
	allText := strings.Join(messageTexts, "\n")

	// 调用 AI 进行总结
	prompt := fmt.Sprintf(`请总结以下 %s 消息，提取关键信息：

%s

请用一句话简要概括这些消息的核心内容，控制在50字以内。`, as.appName, allText)

	// 使用同步方式获取总结（这里使用一个 channel 来模拟同步）
	summaryChan := make(chan string, 1)

	// 设置一个临时的回复回调
	originalOnReply := as.agent.onReply
	as.agent.onReply = func(reply string) {
		summaryChan <- reply
	}

	// 发送请求
	as.agent.ChatWithProvider(prompt, "")

	// 等待回复（带超时）
	select {
	case summary := <-summaryChan:
		// 恢复原始回调
		as.agent.onReply = originalOnReply
		return strings.TrimSpace(summary)
	case <-time.After(10 * time.Second):
		// 恢复原始回调
		as.agent.onReply = originalOnReply
		log.Printf("[AppSecretary-%s] AI 总结超时", as.appName)
		return ""
	}
}

// shouldReportImmediately 判断是否立即汇报
func (as *AppSecretary) shouldReportImmediately(event *AppEvent) bool {
	// 紧急消息立即汇报
	if event.IsUrgent && as.reportStrategy.UrgentImmediate {
		return true
	}

	// 根据最小汇报优先级判断
	minPriority := as.parsePriority(as.reportStrategy.MinReportPriority)
	eventPriority := as.parsePriority(event.Priority)

	return eventPriority >= minPriority
}

// parsePriority 解析优先级
func (as *AppSecretary) parsePriority(p string) int {
	switch p {
	case "urgent":
		return 4
	case "high":
		return 3
	case "normal":
		return 2
	case "low":
		return 1
	default:
		return 2
	}
}

// handleMessageEvent 处理消息事件
func (as *AppSecretary) handleMessageEvent(event *MessageEvent) {
	// 检查是否重复事件
	if as.isDuplicate(event) {
		return
	}

	// 分析消息内容
	appEvent := as.analyzeMessage(event)

	// 判断是否立即汇报
	if as.shouldReportImmediately(appEvent) {
		as.publishImmediateReport(appEvent, event)
	} else {
		// 加入批量处理缓冲区
		as.mu.Lock()
		as.messageBuffer = append(as.messageBuffer, event)
		// 如果缓冲区满了，立即刷新
		if len(as.messageBuffer) >= as.maxBufferSize {
			as.mu.Unlock()
			as.flushBuffer()
			return
		}
		as.mu.Unlock()
	}

	// 发布到事件总线（原始事件）
	as.publishAppEvent(appEvent, event)
}

// publishImmediateReport 发布立即汇报事件
func (as *AppSecretary) publishImmediateReport(appEvent *AppEvent, originalEvent *MessageEvent) {
	// 生成智能摘要
	content := appEvent.Content
	if as.reportStrategy.EnableAISummary && as.agent != nil && as.agent.IsEnabled() {
		summary := as.summarizeSingleMessage(originalEvent)
		if summary != "" {
			content = summary
		}
	}

	// 根据紧急程度选择事件类型和优先级
	var eventType eventbus.EventType
	var priority eventbus.EventPriority

	if appEvent.IsUrgent {
		eventType = eventbus.AppMessageUrgent
		priority = eventbus.PriorityHigh
	} else {
		eventType = eventbus.AppMessageImmediateReport
		priority = eventbus.PriorityNormal
	}

	// 发布立即汇报事件
	event := eventbus.NewEvent(
		eventType,
		fmt.Sprintf("app_secretary_%s", as.appName),
		map[string]interface{}{
			"app_name":    as.appName,
			"event_type":  appEvent.Type,
			"title":       appEvent.Title,
			"content":     content,
			"sender":      appEvent.Sender,
			"session":     appEvent.Session,
			"priority":    appEvent.Priority,
			"is_urgent":   appEvent.IsUrgent,
			"timestamp":   appEvent.Timestamp,
			"raw_content": originalEvent.Content,
		},
	).WithPriority(priority).
		WithDescription(fmt.Sprintf("[%s] %s: %s", as.appName, appEvent.Sender, content))

	as.eventBus.Publish(event)

	log.Printf("[AppSecretary-%s] 立即汇报: %s - %s", as.appName, appEvent.Sender, appEvent.Title)
}

// summarizeSingleMessage 总结单条消息
func (as *AppSecretary) summarizeSingleMessage(event *MessageEvent) string {
	if as.agent == nil || !as.agent.IsEnabled() {
		return ""
	}

	// AI 调用是异步的，暂时返回空
	// TODO: 实现同步 AI 调用或回调机制
	return ""
}

// extractSenders 提取发送者列表
func extractSenders(messages []*MessageEvent) []string {
	senderSet := make(map[string]bool)
	for _, msg := range messages {
		senderSet[msg.Sender] = true
	}
	senders := make([]string, 0, len(senderSet))
	for sender := range senderSet {
		senders = append(senders, sender)
	}
	return senders
}

// convertMessagesToMaps 将消息列表转换为 map 列表
func convertMessagesToMaps(messages []*MessageEvent) []map[string]interface{} {
	result := make([]map[string]interface{}, len(messages))
	for i, msg := range messages {
		result[i] = map[string]interface{}{
			"app":       msg.App,
			"session":   msg.Session,
			"sender":    msg.Sender,
			"content":   msg.Content,
			"timestamp": msg.Timestamp,
		}
	}
	return result
}

// analyzeMessage 分析消息内容
func (as *AppSecretary) analyzeMessage(event *MessageEvent) *AppEvent {
	appEvent := &AppEvent{
		Type:      "message",
		Title:     "新消息",
		Content:   event.Content,
		Sender:    event.Sender,
		Session:   event.Session,
		Timestamp: event.Timestamp,
		Priority:  "normal",
		IsUrgent:  false,
	}

	// 检测紧急程度
	contentLower := strings.ToLower(event.Content)
	for _, keyword := range as.urgentKeywords {
		if strings.Contains(contentLower, strings.ToLower(keyword)) {
			appEvent.IsUrgent = true
			appEvent.Priority = "urgent"
			appEvent.Title = "紧急消息"
			break
		}
	}

	// 检测会议相关
	if as.isMeetingMessage(event.Content) {
		appEvent.Type = "meeting"
		appEvent.Title = "会议通知"
	}

	// 检测任务相关
	if as.isTaskMessage(event.Content) {
		appEvent.Type = "task"
		appEvent.Title = "任务提醒"
	}

	return appEvent
}

// publishAppEvent 发布应用事件到事件总线
func (as *AppSecretary) publishAppEvent(appEvent *AppEvent, originalEvent *MessageEvent) {
	// 构建事件数据
	data := map[string]interface{}{
		"app_type":   string(as.appType),
		"app_name":   as.appName,
		"event_type": appEvent.Type,
		"title":      appEvent.Title,
		"content":    appEvent.Content,
		"sender":     appEvent.Sender,
		"session":    appEvent.Session,
		"priority":   appEvent.Priority,
		"is_urgent":  appEvent.IsUrgent,
		"timestamp":  appEvent.Timestamp,
	}

	// 根据紧急程度选择事件类型和优先级
	var eventType eventbus.EventType
	var priority eventbus.EventPriority

	if appEvent.IsUrgent {
		eventType = eventbus.AppMessageUrgent
		priority = eventbus.PriorityHigh
	} else {
		eventType = eventbus.AppMessageReceived
		priority = eventbus.PriorityNormal
	}

	// 发布事件
	event := eventbus.NewEvent(
		eventType,
		fmt.Sprintf("app_secretary_%s", as.appName),
		data,
	).WithPriority(priority).
		WithDescription(fmt.Sprintf("[%s] %s: %s", as.appName, appEvent.Sender, appEvent.Content))

	as.eventBus.Publish(event)

	// 如果是会议事件，额外发布会议事件
	if appEvent.Type == "meeting" && !as.inMeeting {
		as.startMeetingTracking(appEvent)
	}

	// 如果是任务事件，额外发布任务提醒事件
	if appEvent.Type == "task" {
		taskEvent := eventbus.NewEvent(
			eventbus.AppTaskReminder,
			fmt.Sprintf("app_secretary_%s", as.appName),
			map[string]interface{}{
				"app_name": as.appName,
				"title":    appEvent.Title,
				"content":  appEvent.Content,
				"sender":   appEvent.Sender,
				"priority": appEvent.Priority,
			},
		).WithPriority(eventbus.PriorityNormal)

		as.eventBus.Publish(taskEvent)
	}
}

// startMeetingTracking 开始会议跟踪
func (as *AppSecretary) startMeetingTracking(appEvent *AppEvent) {
	as.mu.Lock()
	defer as.mu.Unlock()

	as.inMeeting = true
	as.meetingStartTime = time.Now()

	// 发布会议开始事件
	event := eventbus.NewEvent(
		eventbus.AppMeetingStarted,
		fmt.Sprintf("app_secretary_%s", as.appName),
		map[string]interface{}{
			"app_name":    as.appName,
			"session":     appEvent.Session,
			"start_time":  as.meetingStartTime,
			"description": appEvent.Content,
		},
	).WithPriority(eventbus.PriorityNormal)

	as.eventBus.Publish(event)

	log.Printf("[AppSecretary-%s] 检测到会议开始: %s", as.appName, appEvent.Session)
}

// EndMeetingTracking 结束会议跟踪（外部调用）
func (as *AppSecretary) EndMeetingTracking() {
	as.mu.Lock()
	defer as.mu.Unlock()

	if !as.inMeeting {
		return
	}

	duration := time.Since(as.meetingStartTime)
	as.inMeeting = false

	// 发布会议结束事件
	event := eventbus.NewEvent(
		eventbus.AppMeetingEnded,
		fmt.Sprintf("app_secretary_%s", as.appName),
		map[string]interface{}{
			"app_name": as.appName,
			"end_time": time.Now(),
			"duration": duration.Minutes(),
		},
	).WithPriority(eventbus.PriorityNormal)

	as.eventBus.Publish(event)

	log.Printf("[AppSecretary-%s] 会议结束，时长: %v", as.appName, duration)
}

// isDuplicate 检查是否重复事件
func (as *AppSecretary) isDuplicate(event *MessageEvent) bool {
	as.mu.Lock()
	defer as.mu.Unlock()

	// 生成事件指纹
	fingerprint := as.generateFingerprint(event)

	// 检查是否最近已处理
	if lastTime, exists := as.recentEvents[fingerprint]; exists {
		if time.Since(lastTime) < as.dedupWindow {
			return true
		}
	}

	// 记录本次事件
	as.recentEvents[fingerprint] = time.Now()
	return false
}

// generateFingerprint 生成事件指纹
func (as *AppSecretary) generateFingerprint(event *MessageEvent) string {
	return fmt.Sprintf("%s:%s:%s:%d", as.appName, event.Sender, event.Content, event.Timestamp.Minute())
}

// cleanupLoop 清理过期事件
func (as *AppSecretary) cleanupLoop() {
	ticker := time.NewTicker(as.dedupWindow)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			as.cleanupRecentEvents()
		}
	}
}

// cleanupRecentEvents 清理过期事件
func (as *AppSecretary) cleanupRecentEvents() {
	as.mu.Lock()
	defer as.mu.Unlock()

	now := time.Now()
	for fingerprint, timestamp := range as.recentEvents {
		if now.Sub(timestamp) > as.dedupWindow {
			delete(as.recentEvents, fingerprint)
		}
	}
}

// isMeetingMessage 检测是否为会议消息
func (as *AppSecretary) isMeetingMessage(content string) bool {
	meetingKeywords := []string{
		"会议", "开会", "视频会议", "语音会议",
		"meeting", "conference", "call",
		"日程", "calendar", "日程邀请",
	}

	contentLower := strings.ToLower(content)
	for _, keyword := range meetingKeywords {
		if strings.Contains(contentLower, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

// isTaskMessage 检测是否为任务消息
func (as *AppSecretary) isTaskMessage(content string) bool {
	taskKeywords := []string{
		"任务", "待办", "todo", "task",
		"截止", "deadline", "ddl",
		"完成", "提交", "交付",
	}

	contentLower := strings.ToLower(content)
	for _, keyword := range taskKeywords {
		if strings.Contains(contentLower, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

// SetUrgentKeywords 设置紧急关键词
func (as *AppSecretary) SetUrgentKeywords(keywords []string) {
	as.mu.Lock()
	defer as.mu.Unlock()
	as.urgentKeywords = keywords
}

// AddUrgentKeyword 添加紧急关键词
func (as *AppSecretary) AddUrgentKeyword(keyword string) {
	as.mu.Lock()
	defer as.mu.Unlock()

	for _, k := range as.urgentKeywords {
		if k == keyword {
			return
		}
	}
	as.urgentKeywords = append(as.urgentKeywords, keyword)
}

// IsInMeeting 检查是否在会议中
func (as *AppSecretary) IsInMeeting() bool {
	as.mu.RLock()
	defer as.mu.RUnlock()
	return as.inMeeting
}

// GetMeetingDuration 获取当前会议时长
func (as *AppSecretary) GetMeetingDuration() time.Duration {
	as.mu.RLock()
	defer as.mu.RUnlock()

	if !as.inMeeting {
		return 0
	}
	return time.Since(as.meetingStartTime)
}

// getDefaultUrgentKeywords 获取默认紧急关键词
func getDefaultUrgentKeywords() []string {
	return []string{
		"紧急", "urgent", "URGENT", "asap", "ASAP",
		"请尽快", "尽快", "马上", "立刻", "立即", "速回", "速看",
		"重要", "important", "IMPORTANT", "关键", "critical", "CRITICAL",
		"报警", "告警", "alert", "alarm", "故障", "异常", "error", "ERROR",
		"宕机", "down", "崩溃", "crash", "线上问题", "线上故障",
		"deadline", "DDL", "ddl", "截止", "今天必须", "今晚必须",
		"等你", "等你回复", "等你确认", "麻烦看一下", "帮忙看看",
		"@我", "@你",
	}
}

// AppSecretaryManager 应用秘书管理器
type AppSecretaryManager struct {
	secretaries map[AppType]*AppSecretary
	mu          sync.RWMutex
}

// NewAppSecretaryManager 创建应用秘书管理器
func NewAppSecretaryManager() *AppSecretaryManager {
	return &AppSecretaryManager{
		secretaries: make(map[AppType]*AppSecretary),
	}
}

// RegisterSecretary 注册应用秘书
func (asm *AppSecretaryManager) RegisterSecretary(appType AppType, appName string) *AppSecretary {
	asm.mu.Lock()
	defer asm.mu.Unlock()

	if sec, exists := asm.secretaries[appType]; exists {
		return sec
	}

	sec := NewAppSecretary(appType, appName)
	asm.secretaries[appType] = sec
	return sec
}

// GetSecretary 获取应用秘书
func (asm *AppSecretaryManager) GetSecretary(appType AppType) *AppSecretary {
	asm.mu.RLock()
	defer asm.mu.RUnlock()
	return asm.secretaries[appType]
}

// StartAll 启动所有应用秘书
func (asm *AppSecretaryManager) StartAll() {
	asm.mu.RLock()
	defer asm.mu.RUnlock()

	for _, sec := range asm.secretaries {
		sec.Start()
	}
}

// StopAll 停止所有应用秘书
func (asm *AppSecretaryManager) StopAll() {
	asm.mu.RLock()
	defer asm.mu.RUnlock()

	for _, sec := range asm.secretaries {
		sec.Stop()
	}
}

// HandleMessage 分发消息到对应的应用秘书
func (asm *AppSecretaryManager) HandleMessage(appType AppType, event *MessageEvent) {
	asm.mu.RLock()
	sec, exists := asm.secretaries[appType]
	asm.mu.RUnlock()

	if exists {
		sec.handleMessageEvent(event)
	}
}
