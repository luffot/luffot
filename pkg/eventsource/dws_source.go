package eventsource

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/luffot/luffot/pkg/config"
)

// DWSMessage DWS 返回的消息结构
type DWSMessage struct {
	// 消息唯一标识
	MessageKey string `json:"messageKey"`
	// 消息类型
	MessageType string `json:"messageType"`
	// 发送者信息
	Sender struct {
		UserID   string `json:"userId"`
		NickName string `json:"nickName"`
		Avatar   string `json:"avatar"`
	} `json:"sender"`
	// 消息内容
	Content struct {
		Text string `json:"text"`
	} `json:"content"`
	// 发送时间（毫秒时间戳）
	CreateTime int64 `json:"createTime"`
	// 会话信息
	Conversation struct {
		OpenConversationID string `json:"openConversationId"`
		Title              string `json:"title"`
		ConversationType   string `json:"conversationType"` // "1"=单聊, "2"=群聊
	} `json:"conversation"`
}

// DWSMessageListResponse DWS 消息列表响应
type DWSMessageListResponse struct {
	// 消息列表
	Messages []DWSMessage `json:"messages"`
	// 是否有更多消息
	HasMore bool `json:"hasMore"`
	// 下一页游标
	NextCursor string `json:"nextCursor"`
}

// DWSEventSource DWS CLI 消息源
// 通过调用 dws chat message list 命令轮询获取钉钉消息
type DWSEventSource struct {
	config          config.DWSConfig
	running         bool
	mu              sync.RWMutex
	cancelFunc      context.CancelFunc
	lastPollTime    time.Time
	seenMessageKeys map[string]time.Time // 已处理的消息 key，用于去重
	messageMu       sync.RWMutex
}

// NewDWSEventSource 创建 DWS 消息源
func NewDWSEventSource(cfg config.DWSConfig) *DWSEventSource {
	// 设置默认值
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 10
	}
	if cfg.MaxResults <= 0 {
		cfg.MaxResults = 50
	}

	return &DWSEventSource{
		config:          cfg,
		seenMessageKeys: make(map[string]time.Time),
	}
}

// Name 返回数据源名称
func (s *DWSEventSource) Name() string {
	return "dws"
}

// Start 启动 DWS 消息源
func (s *DWSEventSource) Start(ctx context.Context, handler MessageEventHandler) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = true
	s.mu.Unlock()

	// 创建可取消的子上下文
	ctx, cancel := context.WithCancel(ctx)
	s.cancelFunc = cancel

	// 启动轮询循环
	go s.pollLoop(ctx, handler)

	return nil
}

// Stop 停止 DWS 消息源
func (s *DWSEventSource) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	if s.cancelFunc != nil {
		s.cancelFunc()
	}

	s.running = false
	return nil
}

// IsRunning 检查是否正在运行
func (s *DWSEventSource) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// pollLoop 轮询循环
func (s *DWSEventSource) pollLoop(ctx context.Context, handler MessageEventHandler) {
	ticker := time.NewTicker(time.Duration(s.config.PollInterval) * time.Second)
	defer ticker.Stop()

	// 立即执行一次
	s.doPoll(ctx, handler)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.doPoll(ctx, handler)
		}
	}
}

// doPoll 执行一次轮询
func (s *DWSEventSource) doPoll(ctx context.Context, handler MessageEventHandler) {
	// 清理过期的消息 key（保留 1 小时）
	s.cleanupSeenKeys()

	// 构建 dws 命令
	args := []string{"chat", "message", "list", "--format", "json", "--limit", fmt.Sprintf("%d", s.config.MaxResults)}

	// 如果指定了特定会话，添加会话过滤
	if len(s.config.Conversations) > 0 {
		// DWS 支持通过 --conversation-id 指定会话
		for _, convID := range s.config.Conversations {
			args = append(args, "--conversation-id", convID)
		}
	}

	// 执行命令
	dwsPath := s.config.DWSBinaryPath
	if dwsPath == "" {
		dwsPath = "dws"
	}

	cmd := exec.CommandContext(ctx, dwsPath, args...)

	// 设置环境变量
	if s.config.ClientID != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("DWS_CLIENT_ID=%s", s.config.ClientID))
	}
	if s.config.ClientSecret != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("DWS_CLIENT_SECRET=%s", s.config.ClientSecret))
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[DWS] poll failed: %v, output: %s", err, string(output))
		return
	}

	// 解析响应
	var response DWSMessageListResponse
	if err := json.Unmarshal(output, &response); err != nil {
		// 尝试解析错误响应
		var errorResp struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		if err2 := json.Unmarshal(output, &errorResp); err2 == nil && errorResp.Error != "" {
			log.Printf("[DWS] API error: %s - %s", errorResp.Error, errorResp.Message)
			return
		}
		log.Printf("[DWS] Failed to parse response: %v, output: %s", err, string(output))
		return
	}

	// 处理消息
	for _, msg := range response.Messages {
		// 检查是否已处理过
		if s.isMessageSeen(msg.MessageKey) {
			continue
		}
		s.markMessageSeen(msg.MessageKey)

		// 转换为 MessageEvent
		event := s.convertToEvent(msg)
		if event != nil {
			go handler(event)
		}
	}

	s.lastPollTime = time.Now()
}

// convertToEvent 将 DWS 消息转换为 MessageEvent
func (s *DWSEventSource) convertToEvent(msg DWSMessage) *MessageEvent {
	// 只处理文本消息
	if msg.MessageType != "text" && msg.MessageType != "" {
		return nil
	}

	// 解析时间戳
	timestamp := time.Unix(msg.CreateTime/1000, (msg.CreateTime%1000)*1000000)

	return &MessageEvent{
		App:       "dingtalk",
		Session:   msg.Conversation.Title,
		Sender:    msg.Sender.NickName,
		Content:   msg.Content.Text,
		RawTime:   timestamp.Format("HH:mm"),
		Timestamp: timestamp,
		AvatarURL: msg.Sender.Avatar,
	}
}

// isMessageSeen 检查消息是否已处理
func (s *DWSEventSource) isMessageSeen(key string) bool {
	s.messageMu.RLock()
	defer s.messageMu.RUnlock()
	_, exists := s.seenMessageKeys[key]
	return exists
}

// markMessageSeen 标记消息已处理
func (s *DWSEventSource) markMessageSeen(key string) {
	s.messageMu.Lock()
	defer s.messageMu.Unlock()
	s.seenMessageKeys[key] = time.Now()
}

// cleanupSeenKeys 清理过期的消息 key
func (s *DWSEventSource) cleanupSeenKeys() {
	s.messageMu.Lock()
	defer s.messageMu.Unlock()

	cutoff := time.Now().Add(-1 * time.Hour)
	for key, ts := range s.seenMessageKeys {
		if ts.Before(cutoff) {
			delete(s.seenMessageKeys, key)
		}
	}
}

// CheckDWSAvailable 检查 DWS 是否可用
func CheckDWSAvailable(binaryPath string) bool {
	path := binaryPath
	if path == "" {
		path = "dws"
	}

	cmd := exec.Command(path, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}

	// 检查输出是否包含版本信息
	return strings.Contains(string(output), "dws") || strings.Contains(string(output), "DingTalk")
}
