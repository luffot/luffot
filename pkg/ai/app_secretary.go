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
	}
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
	log.Printf("[AppSecretary-%s] 应用秘书停止", as.appName)
}

// HandleMessage 处理外部传入的消息事件
func (as *AppSecretary) HandleMessage(event *MessageEvent) {
	if as.messageHandler != nil {
		as.messageHandler(event)
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

	// 发布到事件总线
	as.publishAppEvent(appEvent, event)
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
		sec.HandleMessage(event)
	}
}
