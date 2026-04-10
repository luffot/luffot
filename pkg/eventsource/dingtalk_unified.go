package eventsource

import (
	"context"
	"log"
	"sync"

	"github.com/luffot/luffot/pkg/ai"
	"github.com/luffot/luffot/pkg/config"
)

// DingTalkUnifiedSource 统一的钉钉消息源
// 根据配置自动选择使用 Accessibility API、VLModel 或 DWS 方式获取消息
type DingTalkUnifiedSource struct {
	mu       sync.RWMutex
	running  bool
	config   config.AppConfigItem
	agent    *ai.Agent
	delegate MessageEventSource
}

// NewDingTalkUnifiedSource 创建统一的钉钉消息源
func NewDingTalkUnifiedSource(cfg config.AppConfigItem, agent *ai.Agent) *DingTalkUnifiedSource {
	return &DingTalkUnifiedSource{
		config: cfg,
		agent:  agent,
	}
}

// Name 返回数据源名称
func (s *DingTalkUnifiedSource) Name() string {
	return "dingtalk-unified"
}

// Start 启动消息源
// 根据配置自动选择实现方式：
//   - dws: 使用 DWS CLI 轮询获取消息
//   - vlmodel: 使用视觉模型识别截图
//   - accessibility: 使用 macOS Accessibility API（默认）
func (s *DingTalkUnifiedSource) Start(ctx context.Context, handler MessageEventHandler) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	// 根据配置选择实现方式
	sourceMode := s.config.DingTalk.SourceMode
	if sourceMode == "" {
		sourceMode = config.DingTalkSourceModeAccessibility
	}

	log.Printf("[DingTalk] 使用消息源模式: %s", sourceMode)

	var delegate MessageEventSource

	switch sourceMode {
	case config.DingTalkSourceModeDWS:
		// 使用 DWS CLI 方式
		delegate = s.createDWSSource()
	case config.DingTalkSourceModeVLModel:
		// 使用视觉模型方式
		delegate = s.createVLModelSource()
	case config.DingTalkSourceModeAccessibility:
		fallthrough
	default:
		// 使用 Accessibility API 方式（默认）
		delegate = s.createAccessibilitySource()
	}

	s.delegate = delegate
	s.running = true

	// 启动 delegate
	go func() {
		if err := delegate.Start(ctx, handler); err != nil {
			log.Printf("[DingTalk] 消息源启动失败: %v", err)
		}
	}()

	return nil
}

// Stop 停止消息源
func (s *DingTalkUnifiedSource) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	if s.delegate != nil {
		if err := s.delegate.Stop(); err != nil {
			return err
		}
	}

	s.running = false
	return nil
}

// IsRunning 检查是否正在运行
func (s *DingTalkUnifiedSource) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// createDWSSource 创建 DWS 消息源
func (s *DingTalkUnifiedSource) createDWSSource() MessageEventSource {
	log.Println("[DingTalk] 初始化 DWS 消息源")
	return NewDWSEventSource(s.config.DingTalk.DWS)
}

// createVLModelSource 创建视觉模型消息源
func (s *DingTalkUnifiedSource) createVLModelSource() MessageEventSource {
	log.Println("[DingTalk] 初始化 VLModel 消息源")
	cfg := DingTalkSourceConfig{
		CheckInterval: 3,
		MaxCacheSize:  500,
		Agent:         s.agent,
	}
	return NewDingTalkSource(cfg)
}

// createAccessibilitySource 创建 Accessibility API 消息源
func (s *DingTalkUnifiedSource) createAccessibilitySource() MessageEventSource {
	log.Println("[DingTalk] 初始化 Accessibility API 消息源")
	cfg := DingTalkSourceConfig{
		CheckInterval: 3,
		MaxCacheSize:  500,
	}
	return NewDingTalkSource(cfg)
}

// CheckDWSAvailable 检查 DWS 是否可用
func (s *DingTalkUnifiedSource) CheckDWSAvailable() bool {
	return CheckDWSAvailable(s.config.DingTalk.DWS.DWSBinaryPath)
}
