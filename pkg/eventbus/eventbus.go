package eventbus

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// EventType 事件类型
type EventType string

// 系统级事件类型
const (
	// SystemCPUHigh CPU使用率过高
	SystemCPUHigh EventType = "system.cpu_high"
	// SystemMemoryOveruse 内存占用过高
	SystemMemoryOveruse EventType = "system.memory_overuse"
	// SystemDiskFull 磁盘空间不足
	SystemDiskFull EventType = "system.disk_full"
	// SystemNetworkError 网络异常
	SystemNetworkError EventType = "system.network_error"
	// SystemProcessAnomaly 进程异常
	SystemProcessAnomaly EventType = "system.process_anomaly"
)

// 应用级事件类型
const (
	// AppMessageReceived 收到应用消息
	AppMessageReceived EventType = "app.message_received"
	// AppMessageUrgent 收到紧急消息
	AppMessageUrgent EventType = "app.message_urgent"
	// AppMessageBatchReport 批量消息汇报
	AppMessageBatchReport EventType = "app.message_batch_report"
	// AppMessageImmediateReport 立即消息汇报
	AppMessageImmediateReport EventType = "app.message_immediate_report"
	// AppMeetingStarted 会议开始
	AppMeetingStarted EventType = "app.meeting_started"
	// AppMeetingEnded 会议结束
	AppMeetingEnded EventType = "app.meeting_ended"
	// AppTaskReminder 任务提醒
	AppTaskReminder EventType = "app.task_reminder"
)

// 环境级事件类型
const (
	// EnvPersonDetected 检测到人员
	EnvPersonDetected EventType = "env.person_detected"
	// EnvPersonLeft 人员离开
	EnvPersonLeft EventType = "env.person_left"
	// EnvObjectMoved 物品移动
	EnvObjectMoved EventType = "env.object_moved"
	// EnvNoiseDetected 检测到噪音
	EnvNoiseDetected EventType = "env.noise_detected"
)

// 语音助手事件类型
const (
	// VoiceWakeWordDetected 检测到唤醒词
	VoiceWakeWordDetected EventType = "voice.wake_word_detected"
	// VoiceCommandReceived 收到语音指令
	VoiceCommandReceived EventType = "voice.command_received"
)

// 用户行为事件类型
const (
	// UserAppSwitched 用户切换应用
	UserAppSwitched EventType = "user.app_switched"
	// UserIdleDetected 检测到用户空闲
	UserIdleDetected EventType = "user.idle_detected"
	// UserActiveDetected 检测到用户活跃
	UserActiveDetected EventType = "user.active_detected"
)

// EventPriority 事件优先级
type EventPriority int

const (
	// PriorityLow 低优先级
	PriorityLow EventPriority = iota
	// PriorityNormal 普通优先级
	PriorityNormal
	// PriorityHigh 高优先级
	PriorityHigh
	// PriorityCritical 紧急优先级
	PriorityCritical
)

// Event 标准化事件对象
type Event struct {
	ID          string                 `json:"id"`          // 事件唯一ID
	Type        EventType              `json:"type"`        // 事件类型
	Priority    EventPriority          `json:"priority"`    // 事件优先级
	Source      string                 `json:"source"`      // 事件来源（智能体名称）
	Timestamp   time.Time              `json:"timestamp"`   // 事件发生时间
	Data        map[string]interface{} `json:"data"`        // 事件数据
	Description string                 `json:"description"` // 事件描述
}

// NewEvent 创建新事件
func NewEvent(eventType EventType, source string, data map[string]interface{}) *Event {
	return &Event{
		ID:        generateEventID(),
		Type:      eventType,
		Source:    source,
		Timestamp: time.Now(),
		Data:      data,
		Priority:  PriorityNormal,
	}
}

// WithPriority 设置事件优先级
func (e *Event) WithPriority(priority EventPriority) *Event {
	e.Priority = priority
	return e
}

// WithDescription 设置事件描述
func (e *Event) WithDescription(desc string) *Event {
	e.Description = desc
	return e
}

// generateEventID 生成事件ID
func generateEventID() string {
	return fmt.Sprintf("evt_%d_%d", time.Now().UnixNano(), time.Now().Unix())
}

// EventHandler 事件处理器函数类型
type EventHandler func(event *Event)

// EventBus 事件总线
type EventBus struct {
	subscribers map[EventType][]EventHandler
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	eventChan   chan *Event
}

// NewEventBus 创建事件总线
func NewEventBus(bufferSize int) *EventBus {
	ctx, cancel := context.WithCancel(context.Background())
	if bufferSize <= 0 {
		bufferSize = 100
	}
	return &EventBus{
		subscribers: make(map[EventType][]EventHandler),
		ctx:         ctx,
		cancel:      cancel,
		eventChan:   make(chan *Event, bufferSize),
	}
}

// Subscribe 订阅指定类型的事件
func (eb *EventBus) Subscribe(eventType EventType, handler EventHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.subscribers[eventType] = append(eb.subscribers[eventType], handler)
}

// SubscribeMultiple 订阅多个事件类型
func (eb *EventBus) SubscribeMultiple(eventTypes []EventType, handler EventHandler) {
	for _, et := range eventTypes {
		eb.Subscribe(et, handler)
	}
}

// Unsubscribe 取消订阅（移除指定类型的所有处理器）
func (eb *EventBus) Unsubscribe(eventType EventType) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	delete(eb.subscribers, eventType)
}

// Publish 发布事件（异步）
func (eb *EventBus) Publish(event *Event) {
	select {
	case eb.eventChan <- event:
		// 成功放入channel
	default:
		// channel已满，丢弃事件（避免阻塞）
	}
}

// PublishSync 同步发布事件（立即处理）
func (eb *EventBus) PublishSync(event *Event) {
	eb.dispatch(event)
}

// dispatch 分发事件到订阅者
func (eb *EventBus) dispatch(event *Event) {
	eb.mu.RLock()
	handlers := eb.subscribers[event.Type]
	eb.mu.RUnlock()

	// 异步调用所有处理器，避免阻塞
	for _, handler := range handlers {
		go handler(event)
	}
}

// Start 启动事件总线处理循环
func (eb *EventBus) Start() {
	go func() {
		for {
			select {
			case <-eb.ctx.Done():
				return
			case event := <-eb.eventChan:
				eb.dispatch(event)
			}
		}
	}()
}

// Stop 停止事件总线
func (eb *EventBus) Stop() {
	eb.cancel()
	close(eb.eventChan)
}

// GetSubscriberCount 获取指定事件类型的订阅者数量
func (eb *EventBus) GetSubscriberCount(eventType EventType) int {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	return len(eb.subscribers[eventType])
}

// GlobalEventBus 全局事件总线实例
var globalEventBus *EventBus
var globalEventBusOnce sync.Once

// GetGlobalEventBus 获取全局事件总线（单例）
func GetGlobalEventBus() *EventBus {
	globalEventBusOnce.Do(func() {
		globalEventBus = NewEventBus(1000)
		globalEventBus.Start()
	})
	return globalEventBus
}

// ResetGlobalEventBus 重置全局事件总线（主要用于测试）
func ResetGlobalEventBus() {
	if globalEventBus != nil {
		globalEventBus.Stop()
	}
	globalEventBus = NewEventBus(1000)
	globalEventBus.Start()
}
