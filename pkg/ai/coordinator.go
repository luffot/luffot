package ai

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/luffot/luffot/pkg/eventbus"
)

// UserActivity 用户当前活动
type UserActivity struct {
	Type        string    `json:"type"`        // 活动类型：working, meeting, idle, etc.
	Description string    `json:"description"` // 活动描述
	StartedAt   time.Time `json:"started_at"`  // 开始时间
	AppName     string    `json:"app_name"`    // 相关应用
}

// UpcomingTask 即将到来的任务
type UpcomingTask struct {
	Title    string    `json:"title"`     // 任务标题
	Time     time.Time `json:"time"`      // 任务时间
	Source   string    `json:"source"`    // 来源（钉钉、日历等）
	Priority string    `json:"priority"`  // 优先级
	IsUrgent bool      `json:"is_urgent"` // 是否紧急
}

// UserState 用户状态视图
type UserState struct {
	CurrentActivities []UserActivity `json:"current_activities"` // 当前进行中的活动
	UpcomingTasks     []UpcomingTask `json:"upcoming_tasks"`     // 即将到来的任务
	LastUpdated       time.Time      `json:"last_updated"`       // 最后更新时间
}

// InsightType 洞察类型
type InsightType string

const (
	// InsightTypeSystemAlert 系统告警
	InsightTypeSystemAlert InsightType = "system_alert"
	// InsightTypeAppNotification 应用通知
	InsightTypeAppNotification InsightType = "app_notification"
	// InsightTypeEnvironmentAlert 环境告警
	InsightTypeEnvironmentAlert InsightType = "environment_alert"
	// InsightTypeActivitySummary 活动摘要
	InsightTypeActivitySummary InsightType = "activity_summary"
	// InsightTypeTaskReminder 任务提醒
	InsightTypeTaskReminder InsightType = "task_reminder"
)

// UserInsight 用户洞察
type UserInsight struct {
	ID        string      `json:"id"`         // 洞察ID
	Type      InsightType `json:"type"`       // 洞察类型
	Title     string      `json:"title"`      // 标题
	Content   string      `json:"content"`    // 内容
	Priority  int         `json:"priority"`   // 优先级（1-10）
	Source    string      `json:"source"`     // 来源智能体
	CreatedAt time.Time   `json:"created_at"` // 创建时间
	ExpiresAt *time.Time  `json:"expires_at"` // 过期时间
}

// OnInsightCallback 洞察回调函数类型
type OnInsightCallback func(insight *UserInsight)

// Coordinator 响应式AI协调器（Luffot秘书长/AI丞相）
// 作为系统中枢，负责：
// 1. 接收所有子智能体的事件
// 2. 聚合事件数据，生成用户状态视图
// 3. 进行事件优先级排序与语义融合
// 4. 向用户主动推送关键信息（通过桌宠汇报）
type Coordinator struct {
	agent     *Agent
	eventBus  *eventbus.EventBus
	userState *UserState
	onInsight OnInsightCallback

	// 事件聚合窗口
	pendingEvents []*eventbus.Event
	eventWindow   time.Duration

	// 洞察生成定时器
	insightTicker *time.Ticker

	// AI 丞相汇报策略
	reportStrategy CoordinatorReportStrategy

	// 汇报冷却控制
	lastReportTime     time.Time
	reportCooldown     time.Duration
	consecutiveReports int

	// 状态锁
	mu sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
}

// CoordinatorReportStrategy AI丞相汇报策略
type CoordinatorReportStrategy struct {
	// 是否启用 AI 总结
	EnableAISummary bool `json:"enable_ai_summary"`
	// 最小汇报间隔（秒）
	MinReportInterval int `json:"min_report_interval"`
	// 最大连续汇报次数（防刷屏）
	MaxConsecutiveReports int `json:"max_consecutive_reports"`
	// 连续汇报冷却时间（秒）
	ConsecutiveCooldown int `json:"consecutive_cooldown"`
	// 高优先级事件立即汇报
	UrgentImmediate bool `json:"urgent_immediate"`
	// 批量事件聚合窗口（秒）
	BatchWindow int `json:"batch_window"`
}

// DefaultCoordinatorReportStrategy 默认汇报策略
func DefaultCoordinatorReportStrategy() CoordinatorReportStrategy {
	return CoordinatorReportStrategy{
		EnableAISummary:       true,
		MinReportInterval:     5,
		MaxConsecutiveReports: 3,
		ConsecutiveCooldown:   60,
		UrgentImmediate:       true,
		BatchWindow:           30,
	}
}

// NewCoordinator 创建响应式AI协调器
func NewCoordinator(agent *Agent, onInsight OnInsightCallback) *Coordinator {
	ctx, cancel := context.WithCancel(context.Background())
	return &Coordinator{
		agent:    agent,
		eventBus: eventbus.GetGlobalEventBus(),
		userState: &UserState{
			CurrentActivities: make([]UserActivity, 0),
			UpcomingTasks:     make([]UpcomingTask, 0),
			LastUpdated:       time.Now(),
		},
		onInsight:      onInsight,
		pendingEvents:  make([]*eventbus.Event, 0),
		eventWindow:    30 * time.Second,
		reportStrategy: DefaultCoordinatorReportStrategy(),
		lastReportTime: time.Now(),
		reportCooldown: 5 * time.Second,
		ctx:            ctx,
		cancel:         cancel,
	}
}

// SetReportStrategy 设置汇报策略
func (c *Coordinator) SetReportStrategy(strategy CoordinatorReportStrategy) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reportStrategy = strategy
}

// Start 启动协调器
func (c *Coordinator) Start() {
	log.Println("[Coordinator] 响应式AI协调器启动")

	// 订阅各类事件
	c.subscribeEvents()

	// 启动洞察生成定时器
	c.insightTicker = time.NewTicker(60 * time.Second)
	go func() {
		for {
			select {
			case <-c.ctx.Done():
				return
			case <-c.insightTicker.C:
				c.generatePeriodicInsights()
			}
		}
	}()
}

// Stop 停止协调器
func (c *Coordinator) Stop() {
	log.Println("[Coordinator] 响应式AI协调器停止")
	c.cancel()
	if c.insightTicker != nil {
		c.insightTicker.Stop()
	}
}

// subscribeEvents 订阅相关事件
func (c *Coordinator) subscribeEvents() {
	// 系统事件
	c.eventBus.Subscribe(eventbus.SystemCPUHigh, c.handleSystemEvent)
	c.eventBus.Subscribe(eventbus.SystemMemoryOveruse, c.handleSystemEvent)
	c.eventBus.Subscribe(eventbus.SystemDiskFull, c.handleSystemEvent)
	c.eventBus.Subscribe(eventbus.SystemProcessAnomaly, c.handleSystemEvent)

	// 应用事件
	c.eventBus.Subscribe(eventbus.AppMessageReceived, c.handleAppEvent)
	c.eventBus.Subscribe(eventbus.AppMessageUrgent, c.handleAppEvent)
	// 订阅应用秘书的汇报事件
	c.eventBus.Subscribe(eventbus.AppMessageBatchReport, c.handleAppBatchReport)
	c.eventBus.Subscribe(eventbus.AppMessageImmediateReport, c.handleAppImmediateReport)
	c.eventBus.Subscribe(eventbus.AppMeetingStarted, c.handleAppEvent)
	c.eventBus.Subscribe(eventbus.AppMeetingEnded, c.handleAppEvent)
	c.eventBus.Subscribe(eventbus.AppTaskReminder, c.handleAppEvent)

	// 环境事件
	c.eventBus.Subscribe(eventbus.EnvPersonDetected, c.handleEnvironmentEvent)
	c.eventBus.Subscribe(eventbus.EnvObjectMoved, c.handleEnvironmentEvent)

	// 用户行为事件
	c.eventBus.Subscribe(eventbus.UserAppSwitched, c.handleUserBehaviorEvent)
	c.eventBus.Subscribe(eventbus.UserIdleDetected, c.handleUserBehaviorEvent)
	c.eventBus.Subscribe(eventbus.UserActiveDetected, c.handleUserBehaviorEvent)
}

// handleAppBatchReport 处理应用批量汇报
func (c *Coordinator) handleAppBatchReport(event *eventbus.Event) {
	c.mu.Lock()
	c.pendingEvents = append(c.pendingEvents, event)
	c.mu.Unlock()

	log.Printf("[Coordinator] 收到批量汇报: %s", event.Description)
}

// handleAppImmediateReport 处理应用立即汇报
func (c *Coordinator) handleAppImmediateReport(event *eventbus.Event) {
	c.mu.Lock()
	c.pendingEvents = append(c.pendingEvents, event)
	c.mu.Unlock()

	log.Printf("[Coordinator] 收到立即汇报: %s", event.Description)

	// 根据策略判断是否立即向用户汇报
	if c.shouldReportToUser(event) {
		c.generateAndSendReport(event)
	}
}

// shouldReportToUser 判断是否应向用户汇报
func (c *Coordinator) shouldReportToUser(event *eventbus.Event) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 检查汇报间隔
	timeSinceLastReport := time.Since(c.lastReportTime)
	minInterval := time.Duration(c.reportStrategy.MinReportInterval) * time.Second
	if timeSinceLastReport < minInterval {
		return false
	}

	// 检查连续汇报次数（防刷屏）
	if c.consecutiveReports >= c.reportStrategy.MaxConsecutiveReports {
		cooldown := time.Duration(c.reportStrategy.ConsecutiveCooldown) * time.Second
		if timeSinceLastReport < cooldown {
			return false
		}
		c.consecutiveReports = 0
	}

	// 高优先级事件立即汇报
	if event.Priority >= eventbus.PriorityHigh && c.reportStrategy.UrgentImmediate {
		return true
	}

	return false
}

// generateAndSendReport 生成并发送汇报
func (c *Coordinator) generateAndSendReport(event *eventbus.Event) {
	// 生成汇报内容
	report := c.generateReport(event)

	// 创建洞察并发送
	insight := &UserInsight{
		ID:        fmt.Sprintf("ins_%d", time.Now().UnixNano()),
		Type:      c.determineInsightType(event),
		Title:     report.Title,
		Content:   report.Content,
		Priority:  c.calculatePriority(event),
		Source:    "coordinator",
		CreatedAt: time.Now(),
	}

	// 更新汇报状态
	c.mu.Lock()
	c.lastReportTime = time.Now()
	c.consecutiveReports++
	c.mu.Unlock()

	// 发送洞察
	if c.onInsight != nil {
		c.onInsight(insight)
	}

	log.Printf("[Coordinator] AI丞相汇报: %s", report.Title)
}

// Report AI丞相生成的汇报
type Report struct {
	Title   string
	Content string
	Source  string
}

// generateReport 生成智能汇报
func (c *Coordinator) generateReport(event *eventbus.Event) *Report {
	appName := c.extractEventDataString(event, "app_name")
	sender := c.extractEventDataString(event, "sender")
	content := c.extractEventDataString(event, "content")
	title := c.extractEventDataString(event, "title")

	// 如果有 AI 智能体，生成智能摘要
	if c.reportStrategy.EnableAISummary && c.agent != nil && c.agent.IsEnabled() {
		summary := c.generateAISummary(event, appName, sender, content)
		if summary != "" {
			return &Report{
				Title:   fmt.Sprintf("[%s] %s", appName, title),
				Content: summary,
				Source:  "ai_summary",
			}
		}
	}

	return &Report{
		Title:   fmt.Sprintf("[%s] %s", appName, title),
		Content: fmt.Sprintf("%s: %s", sender, content),
		Source:  "default",
	}
}

// generateAISummary 使用 AI 生成汇报摘要
func (c *Coordinator) generateAISummary(event *eventbus.Event, appName, sender, content string) string {
	_ = fmt.Sprintf(`作为用户的AI丞相，请根据以下信息生成一个简洁的汇报：

来源应用: %s
发送者: %s
内容: %s
事件类型: %s
优先级: %d

请生成一段30字以内的智能汇报，突出关键信息，语气正式但友好。`,
		appName, sender, content, event.Type, event.Priority)

	// AI 调用是异步的，这里简化处理
	// 实际项目中应该使用同步包装或回调机制
	return ""
}

// determineInsightType 确定洞察类型
func (c *Coordinator) determineInsightType(event *eventbus.Event) InsightType {
	switch event.Type {
	case eventbus.AppMessageUrgent, eventbus.AppMessageImmediateReport:
		return InsightTypeAppNotification
	case eventbus.SystemCPUHigh, eventbus.SystemMemoryOveruse, eventbus.SystemDiskFull:
		return InsightTypeSystemAlert
	case eventbus.EnvPersonDetected:
		return InsightTypeEnvironmentAlert
	default:
		return InsightTypeActivitySummary
	}
}

// calculatePriority 计算优先级
func (c *Coordinator) calculatePriority(event *eventbus.Event) int {
	basePriority := 5
	switch event.Priority {
	case eventbus.PriorityCritical:
		basePriority = 10
	case eventbus.PriorityHigh:
		basePriority = 8
	case eventbus.PriorityNormal:
		basePriority = 5
	case eventbus.PriorityLow:
		basePriority = 3
	}

	// 根据事件类型调整
	switch event.Type {
	case eventbus.AppMessageUrgent:
		basePriority += 2
	case eventbus.SystemDiskFull:
		basePriority += 3
	}

	if basePriority > 10 {
		basePriority = 10
	}
	return basePriority
}

// extractEventDataString 从事件数据中提取字符串
func (c *Coordinator) extractEventDataString(event *eventbus.Event, key string) string {
	if event.Data == nil {
		return ""
	}
	if val, ok := event.Data[key].(string); ok {
		return val
	}
	return ""
}

// handleSystemEvent 处理系统事件
func (c *Coordinator) handleSystemEvent(event *eventbus.Event) {
	c.mu.Lock()
	c.pendingEvents = append(c.pendingEvents, event)
	c.mu.Unlock()

	// 系统事件通常需要立即响应
	c.processUrgentEvent(event)
}

// handleAppEvent 处理应用事件
func (c *Coordinator) handleAppEvent(event *eventbus.Event) {
	c.mu.Lock()
	c.pendingEvents = append(c.pendingEvents, event)
	c.mu.Unlock()

	// 根据优先级决定是否立即处理
	if event.Priority >= eventbus.PriorityHigh {
		c.processUrgentEvent(event)
	}

	// 更新用户状态
	c.updateUserStateFromEvent(event)
}

// handleEnvironmentEvent 处理环境事件
func (c *Coordinator) handleEnvironmentEvent(event *eventbus.Event) {
	c.mu.Lock()
	c.pendingEvents = append(c.pendingEvents, event)
	c.mu.Unlock()

	// 环境事件通常需要立即通知
	c.processUrgentEvent(event)
}

// handleUserBehaviorEvent 处理用户行为事件
func (c *Coordinator) handleUserBehaviorEvent(event *eventbus.Event) {
	c.updateUserStateFromEvent(event)
}

// processUrgentEvent 处理紧急事件
func (c *Coordinator) processUrgentEvent(event *eventbus.Event) {
	insight := c.eventToInsight(event)
	if insight != nil && c.onInsight != nil {
		c.onInsight(insight)
	}
}

// eventToInsight 将事件转换为洞察
func (c *Coordinator) eventToInsight(event *eventbus.Event) *UserInsight {
	insight := &UserInsight{
		ID:        fmt.Sprintf("ins_%d", time.Now().UnixNano()),
		Source:    event.Source,
		CreatedAt: time.Now(),
	}

	switch event.Type {
	case eventbus.SystemCPUHigh:
		insight.Type = InsightTypeSystemAlert
		insight.Title = "系统资源告警"
		insight.Content = "CPU使用率过高，可能影响系统性能"
		insight.Priority = 8

	case eventbus.SystemMemoryOveruse:
		insight.Type = InsightTypeSystemAlert
		insight.Title = "内存不足告警"
		insight.Content = "内存占用过高，建议关闭不必要的应用"
		insight.Priority = 9

	case eventbus.SystemDiskFull:
		insight.Type = InsightTypeSystemAlert
		insight.Title = "磁盘空间不足"
		insight.Content = "磁盘空间即将用完，请清理文件"
		insight.Priority = 10

	case eventbus.AppMessageUrgent:
		insight.Type = InsightTypeAppNotification
		insight.Title = "紧急消息"
		if desc, ok := event.Data["description"].(string); ok {
			insight.Content = desc
		} else {
			insight.Content = "收到紧急消息，请尽快查看"
		}
		insight.Priority = 7

	case eventbus.EnvPersonDetected:
		insight.Type = InsightTypeEnvironmentAlert
		insight.Title = "环境提醒"
		insight.Content = "检测到有人靠近，请注意周围环境"
		insight.Priority = 6

	default:
		return nil
	}

	return insight
}

// updateUserStateFromEvent 根据事件更新用户状态
func (c *Coordinator) updateUserStateFromEvent(event *eventbus.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch event.Type {
	case eventbus.AppMeetingStarted:
		activity := UserActivity{
			Type:        "meeting",
			Description: "正在参加会议",
			StartedAt:   event.Timestamp,
		}
		if appName, ok := event.Data["app_name"].(string); ok {
			activity.AppName = appName
		}
		c.userState.CurrentActivities = append(c.userState.CurrentActivities, activity)

	case eventbus.AppMeetingEnded:
		// 移除会议活动
		newActivities := make([]UserActivity, 0)
		for _, act := range c.userState.CurrentActivities {
			if act.Type != "meeting" {
				newActivities = append(newActivities, act)
			}
		}
		c.userState.CurrentActivities = newActivities

	case eventbus.UserAppSwitched:
		if appName, ok := event.Data["app_name"].(string); ok {
			// 更新或添加当前活动
			found := false
			for i := range c.userState.CurrentActivities {
				if c.userState.CurrentActivities[i].Type == "working" {
					c.userState.CurrentActivities[i].AppName = appName
					c.userState.CurrentActivities[i].StartedAt = event.Timestamp
					found = true
					break
				}
			}
			if !found {
				c.userState.CurrentActivities = append(c.userState.CurrentActivities, UserActivity{
					Type:        "working",
					Description: fmt.Sprintf("正在使用 %s", appName),
					StartedAt:   event.Timestamp,
					AppName:     appName,
				})
			}
		}

	case eventbus.AppTaskReminder:
		task := UpcomingTask{
			Source: "app",
		}
		if title, ok := event.Data["title"].(string); ok {
			task.Title = title
		}
		if priority, ok := event.Data["priority"].(string); ok {
			task.Priority = priority
		}
		c.userState.UpcomingTasks = append(c.userState.UpcomingTasks, task)
	}

	c.userState.LastUpdated = time.Now()
}

// generatePeriodicInsights 生成周期性洞察
func (c *Coordinator) generatePeriodicInsights() {
	c.mu.Lock()
	pendingCount := len(c.pendingEvents)
	c.pendingEvents = make([]*eventbus.Event, 0)
	c.mu.Unlock()

	if pendingCount == 0 {
		return
	}

	log.Printf("[Coordinator] 处理 %d 个待处理事件", pendingCount)

	// 生成活动摘要
	c.generateActivitySummary()
}

// generateActivitySummary 生成活动摘要
func (c *Coordinator) generateActivitySummary() {
	c.mu.RLock()
	state := *c.userState
	c.mu.RUnlock()

	if len(state.CurrentActivities) == 0 && len(state.UpcomingTasks) == 0 {
		return
	}

	var content string

	// 当前活动
	if len(state.CurrentActivities) > 0 {
		content += "当前活动："
		for _, act := range state.CurrentActivities {
			duration := time.Since(act.StartedAt).Round(time.Minute)
			content += fmt.Sprintf("\n• %s（已进行 %v）", act.Description, duration)
		}
	}

	// 即将到来的任务
	if len(state.UpcomingTasks) > 0 {
		content += "\n\n待办事项："
		for _, task := range state.UpcomingTasks {
			content += fmt.Sprintf("\n• %s", task.Title)
			if task.IsUrgent {
				content += " [紧急]"
			}
		}
	}

	insight := &UserInsight{
		ID:        fmt.Sprintf("ins_%d", time.Now().UnixNano()),
		Type:      InsightTypeActivitySummary,
		Title:     "状态更新",
		Content:   content,
		Priority:  3,
		Source:    "coordinator",
		CreatedAt: time.Now(),
	}

	if c.onInsight != nil {
		c.onInsight(insight)
	}
}

// GetUserState 获取当前用户状态（供外部查询）
func (c *Coordinator) GetUserState() *UserState {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 返回副本
	stateCopy := &UserState{
		CurrentActivities: make([]UserActivity, len(c.userState.CurrentActivities)),
		UpcomingTasks:     make([]UpcomingTask, len(c.userState.UpcomingTasks)),
		LastUpdated:       c.userState.LastUpdated,
	}
	copy(stateCopy.CurrentActivities, c.userState.CurrentActivities)
	copy(stateCopy.UpcomingTasks, c.userState.UpcomingTasks)

	return stateCopy
}

// UpdateActivity 更新用户活动（供智能体调用）
func (c *Coordinator) UpdateActivity(activity UserActivity) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 查找并更新现有活动，或添加新活动
	found := false
	for i := range c.userState.CurrentActivities {
		if c.userState.CurrentActivities[i].Type == activity.Type {
			c.userState.CurrentActivities[i] = activity
			found = true
			break
		}
	}
	if !found {
		c.userState.CurrentActivities = append(c.userState.CurrentActivities, activity)
	}

	c.userState.LastUpdated = time.Now()
}

// AddUpcomingTask 添加即将到来的任务
func (c *Coordinator) AddUpcomingTask(task UpcomingTask) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.userState.UpcomingTasks = append(c.userState.UpcomingTasks, task)
	c.userState.LastUpdated = time.Now()
}

// RemoveUpcomingTask 移除任务
func (c *Coordinator) RemoveUpcomingTask(taskTitle string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	newTasks := make([]UpcomingTask, 0)
	for _, task := range c.userState.UpcomingTasks {
		if task.Title != taskTitle {
			newTasks = append(newTasks, task)
		}
	}
	c.userState.UpcomingTasks = newTasks
	c.userState.LastUpdated = time.Now()
}
