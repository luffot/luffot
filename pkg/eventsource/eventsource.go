package eventsource

import (
	"context"
	"time"

	"github.com/luffot/luffot/pkg/storage"
)

// MessageEvent 消息事件
type MessageEvent struct {
	App       string    `json:"app"`        // 应用名称
	Session   string    `json:"session"`    // 会话名称
	Sender    string    `json:"sender"`     // 发送者
	Content   string    `json:"content"`    // 消息内容
	RawTime   string    `json:"raw_time"`   // 消息中的原始时间戳（如 "09:30"、"昨天 14:20"）
	Timestamp time.Time `json:"timestamp"`  // 消息抓取时间
	AvatarURL string    `json:"avatar_url"` // 发送者头像 URL（可选）
	Color     string    `json:"color"`      // 弹幕颜色，十六进制如 #FF5500（可选，默认蓝色）
}

// ToStorageMessage 转换为存储消息
func (e *MessageEvent) ToStorageMessage() *storage.Message {
	return &storage.Message{
		App:       e.App,
		Session:   e.Session,
		Sender:    e.Sender,
		Content:   e.Content,
		RawTime:   e.RawTime,
		Timestamp: e.Timestamp,
	}
}

// MessageEventHandler 消息事件处理器
type MessageEventHandler func(event *MessageEvent)

// MessageEventSource 消息事件源接口
// 实现该接口的数据源可以将消息推送到系统中
type MessageEventSource interface {
	// Name 返回数据源名称
	Name() string

	// Start 启动数据源监听
	// 当有消息时，通过 handler 回调通知
	Start(ctx context.Context, handler MessageEventHandler) error

	// Stop 停止数据源监听
	Stop() error

	// IsRunning 检查数据源是否正在运行
	IsRunning() bool
}

// SourceManager 数据源管理器
type SourceManager struct {
	sources  []MessageEventSource
	handlers []MessageEventHandler
}

// NewSourceManager 创建数据源管理器
func NewSourceManager() *SourceManager {
	return &SourceManager{
		sources:  make([]MessageEventSource, 0),
		handlers: make([]MessageEventHandler, 0),
	}
}

// AddSource 添加数据源
func (m *SourceManager) AddSource(source MessageEventSource) {
	m.sources = append(m.sources, source)
}

// AddHandler 添加消息处理器
func (m *SourceManager) AddHandler(handler MessageEventHandler) {
	m.handlers = append(m.handlers, handler)
}

// StartAll 启动所有数据源
func (m *SourceManager) StartAll(ctx context.Context) error {
	for _, source := range m.sources {
		for _, handler := range m.handlers {
			// 每个数据源使用独立的 handler 包装，保留 source 信息
			go func(s MessageEventSource, h MessageEventHandler) {
				_ = s.Start(ctx, func(event *MessageEvent) {
					// 可以在这里添加额外的处理逻辑
					h(event)
				})
			}(source, handler)
		}
	}
	return nil
}

// StopAll 停止所有数据源
func (m *SourceManager) StopAll() error {
	for _, source := range m.sources {
		if err := source.Stop(); err != nil {
			return err
		}
	}
	return nil
}
