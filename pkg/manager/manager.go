package manager

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/luffot/luffot/pkg/ai"
	"github.com/luffot/luffot/pkg/barrage"
	"github.com/luffot/luffot/pkg/camera"
	"github.com/luffot/luffot/pkg/config"
	"github.com/luffot/luffot/pkg/eventbus"
	"github.com/luffot/luffot/pkg/eventsource"
	"github.com/luffot/luffot/pkg/prompt"
	"github.com/luffot/luffot/pkg/scheduler"
	"github.com/luffot/luffot/pkg/storage"
)

// intelliAnalyzerInstance 全局智能分析器实例（由 StartIntelliAnalyzer 初始化）
var intelliAnalyzerInstance *ai.IntelliAnalyzer

// capturesDir 告警截图保存目录
var capturesDir = filepath.Join(os.Getenv("HOME"), ".luffot", "captures")

// saveEvidenceFrame 将 base64 JPEG 解码后保存到 capturesDir，文件名含时间戳
// 返回保存的文件路径，失败时记录日志并返回空字符串
func saveEvidenceFrame(base64JPEG string, detectedAt time.Time) string {
	if err := os.MkdirAll(capturesDir, 0755); err != nil {
		log.Printf("[CameraGuard] 创建截图目录失败: %v", err)
		return ""
	}

	jpegBytes, err := base64.StdEncoding.DecodeString(base64JPEG)
	if err != nil {
		log.Printf("[CameraGuard] base64 解码失败: %v", err)
		return ""
	}

	fileName := fmt.Sprintf("intruder_%s.jpg", detectedAt.Format("20060102_150405"))
	filePath := filepath.Join(capturesDir, fileName)

	if err := os.WriteFile(filePath, jpegBytes, 0644); err != nil {
		log.Printf("[CameraGuard] 保存截图失败: %v", err)
		return ""
	}

	log.Printf("[CameraGuard] 证据截图已保存: %s", filePath)
	return filePath
}

// cameraGuardPromptName 摄像头守卫检测 prompt 在 prompt 管理系统中的名称
const cameraGuardPromptName = "camera_guard"

// MessageType 消息类型
type MessageType int

const (
	// MessageTypeBarrage 弹幕显示
	MessageTypeBarrage MessageType = iota
	// MessageTypeStorage 存储到数据库
	MessageTypeStorage
)

// Manager 消息管理器
// 负责协调事件源、弹幕显示和存储
type Manager struct {
	sourceManager   *eventsource.SourceManager
	barrageDisplay  *barrage.BarrageDisplay
	storage         *storage.Storage
	agent           *ai.Agent
	tools           *ai.MessageQueryTools
	scheduler       *scheduler.Scheduler
	reactiveAIChain *ai.ReactiveAIChain // 响应式AI链路
	mu              sync.RWMutex
	running         bool
}

// NewManager 创建消息管理器
func NewManager(barrageDisplay *barrage.BarrageDisplay, storage *storage.Storage) *Manager {
	return &Manager{
		sourceManager:  eventsource.NewSourceManager(),
		barrageDisplay: barrageDisplay,
		storage:        storage,
		running:        false,
	}
}

// SetAIAgent 注入 AI 智能体（在 main.go 中初始化后调用）
func (m *Manager) SetAIAgent(agent *ai.Agent, tools *ai.MessageQueryTools) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agent = agent
	m.tools = tools
}

// SetReactiveAIChain 注入响应式AI链路
func (m *Manager) SetReactiveAIChain(chain *ai.ReactiveAIChain) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reactiveAIChain = chain
}

// AddEventSource 添加事件源
func (m *Manager) AddEventSource(source eventsource.MessageEventSource) {
	m.sourceManager.AddSource(source)
}

// Start 启动管理器
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return nil
	}
	m.running = true
	m.mu.Unlock()

	// 添加消息处理器
	m.sourceManager.AddHandler(m.handleMessage)

	// 启动所有数据源
	return m.sourceManager.StartAll(ctx)
}

// Stop 停止管理器
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	m.running = false
	return m.sourceManager.StopAll()
}

// handleMessage 处理消息
func (m *Manager) handleMessage(event *eventsource.MessageEvent) {
	fmt.Printf("  [弹幕] [%s] %s: %s\n", event.App, event.Sender, truncate(event.Content, 50))

	// 显示弹幕（传递头像 URL 和颜色）
	if m.barrageDisplay != nil {
		m.barrageDisplay.AddMessage(event.Content, event.Sender, event.App, event.AvatarURL, event.Color)
	}

	// 检测重要消息，触发桌宠秘书汇报气泡
	if m.barrageDisplay != nil && isUrgentMessage(event.Content) {
		m.mu.RLock()
		agent := m.agent
		m.mu.RUnlock()

		var alertText string
		if agent != nil && agent.IsEnabled() {
			// 有 AI 时：用 AI 生成智能摘要
			summary := agent.SummarizeMessages([]string{
				fmt.Sprintf("[%s] %s: %s", event.App, event.Sender, event.Content),
			})
			if summary != "" {
				alertText = fmt.Sprintf("⚡ %s", summary)
			}
		}
		// AI 摘要失败或未启用时，回退到原始格式
		if alertText == "" {
			alertText = fmt.Sprintf("【%s】%s", event.Sender, truncate(event.Content, 60))
		}

		m.barrageDisplay.ShowAlert(alertText)
		fmt.Printf("  [秘书汇报] 重要消息来自 %s\n", event.Sender)
	}

	// 存储消息
	if m.storage != nil {
		m.storage.SaveMessage(event.ToStorageMessage())
	}

	// 转发到响应式AI链路的应用秘书
	m.mu.RLock()
	chain := m.reactiveAIChain
	m.mu.RUnlock()

	if chain != nil {
		// 将eventsource.MessageEvent转换为ai.MessageEvent
		aiEvent := &ai.MessageEvent{
			App:       event.App,
			Session:   event.Session,
			Sender:    event.Sender,
			Content:   event.Content,
			Timestamp: event.Timestamp,
		}

		appType := ai.AppType(event.App)
		eventBus := eventbus.GetGlobalEventBus()
		eventBus.Publish(eventbus.NewEvent(
			eventbus.AppMessageReceived,
			"manager",
			map[string]interface{}{
				"app_type": string(appType),
				"sender":   event.Sender,
				"content":  event.Content,
				"session":  event.Session,
			},
		))

		// 触发用户习惯更新
		eventBus.Publish(eventbus.NewEvent(
			eventbus.UserAppSwitched,
			"manager",
			map[string]interface{}{
				"app_name": event.App,
			},
		))

		_ = aiEvent
		_ = appType
	}
}

// isUrgentMessage 判断消息是否需要触发告警，需要桌宠主动汇报。
// 先检查过滤词：命中任意过滤词则直接跳过，不触发告警。
// 再检查告警词：命中任意告警词才触发。
// 关键词列表从配置文件动态读取，支持运行时热更新。
func isUrgentMessage(content string) bool {
	alertCfg := config.GetAlertConfig()
	if !alertCfg.Enabled {
		return false
	}

	lowerContent := strings.ToLower(content)

	// 过滤词优先：命中任意过滤词则不触发告警
	for _, filterWord := range alertCfg.FilterKeywords {
		if filterWord != "" && strings.Contains(lowerContent, strings.ToLower(filterWord)) {
			return false
		}
	}

	// 匹配告警关键词
	for _, keyword := range alertCfg.Keywords {
		if strings.Contains(lowerContent, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

// IsRunning 检查是否运行中
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// GetBarrageDisplay 获取弹幕显示器
func (m *Manager) GetBarrageDisplay() *barrage.BarrageDisplay {
	return m.barrageDisplay
}

// GetSourceManager 获取数据源管理器
func (m *Manager) GetSourceManager() *eventsource.SourceManager {
	return m.sourceManager
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// StartScheduler 启动定时任务调度器，非阻塞
// 返回调度器实例，供 Web 层注入（用于任务 API）
func (m *Manager) StartScheduler(ctx context.Context) *scheduler.Scheduler {
	cfg := config.Get().ScheduledTasks
	if !cfg.Enabled {
		log.Println("[Scheduler] 未启用，跳过启动")
		return nil
	}

	m.mu.RLock()
	agent := m.agent
	m.mu.RUnlock()

	sched := scheduler.NewScheduler(agent, m.storage)
	if err := sched.Start(ctx); err != nil {
		log.Printf("[Scheduler] 启动失败: %v", err)
		return nil
	}

	m.mu.Lock()
	m.scheduler = sched
	m.mu.Unlock()

	log.Println("[Scheduler] 定时任务调度器已启动")
	return sched
}

// StartIntelliAnalyzer 启动智能消息分析器（定时扫描未分析消息，重要消息推送气泡通知）
// 该方法完全非阻塞，分析循环在独立 goroutine 中运行
func (m *Manager) StartIntelliAnalyzer(ctx context.Context) {
	cfg := config.GetIntelliAnalyzerConfig()
	if !cfg.Enabled {
		log.Println("[IntelliAnalyzer] 未启用，跳过启动")
		return
	}

	m.mu.RLock()
	agent := m.agent
	m.mu.RUnlock()

	if agent == nil || !agent.IsEnabled() {
		log.Println("[IntelliAnalyzer] AI 未启用，无法启动智能分析器")
		return
	}

	// 使用 EventBus 发布重要通知事件（不再直接调用气泡回调）
	intelliAnalyzerInstance = ai.NewIntelliAnalyzer(agent, m.storage, eventbus.GetGlobalEventBus())
	intelliAnalyzerInstance.Start(ctx)
	log.Println("[IntelliAnalyzer] 智能消息分析器已启动")
}

// GetScheduler 获取调度器实例（供 Web 层注入使用）
func (m *Manager) GetScheduler() *scheduler.Scheduler {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.scheduler
}

// StartCameraGuard 启动摄像头背后守卫，在后台定期抓帧并用视觉 AI 检测是否有人在背后
// 检测到人时通过 barrageDisplay.ShowAlert 发出告警
// 该方法完全非阻塞：权限请求和检测循环均在独立 goroutine 中运行，不影响主流程
func (m *Manager) StartCameraGuard(ctx context.Context) {
	cfg := config.GetCameraGuardConfig()
	if !cfg.Enabled {
		log.Println("[CameraGuard] 未启用，跳过启动")
		return
	}

	m.mu.RLock()
	agent := m.agent
	m.mu.RUnlock()

	if agent == nil || !agent.IsEnabled() {
		log.Println("[CameraGuard] AI 未启用，无法启动摄像头守卫")
		return
	}

	// 所有操作（含同步权限请求弹窗）均在独立 goroutine 中执行，不阻塞主流程
	go func() {
		log.Println("[CameraGuard] 后台守卫 goroutine 已启动，开始检查摄像头权限...")

		// 请求摄像头权限（同步等待用户在系统弹窗中点击允许/拒绝）
		if !camera.HasPermission() {
			log.Println("[CameraGuard] 当前无摄像头权限，触发系统授权弹窗，请在弹窗中点击「允许」...")
			granted := camera.RequestPermission()
			if !granted {
				log.Println("[CameraGuard] 用户拒绝了摄像头权限，守卫启动失败")
				return
			}
			log.Println("[CameraGuard] 摄像头权限已获得，继续启动")
		} else {
			log.Println("[CameraGuard] 摄像头权限已就绪，无需重新授权")
		}

		intervalSeconds := cfg.IntervalSeconds
		if intervalSeconds <= 0 {
			intervalSeconds = 10
		}

		log.Printf("[CameraGuard] 守卫正式启动 @ %s，检测间隔=%ds",
			time.Now().Format("15:04:05"), intervalSeconds)

		ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
		defer ticker.Stop()

		// 连续检测到人的计数（需达到 confirmCount 才触发告警，避免误报）
		consecutiveDetections := 0
		// 上次告警时间（用于冷却控制）
		var lastAlertTime time.Time

		for {
			select {
			case <-ctx.Done():
				log.Println("[CameraGuard] 收到退出信号，停止检测")
				return
			case <-ticker.C:
				// 重新读取配置（支持运行时热更新）
				latestCfg := config.GetCameraGuardConfig()
				if !latestCfg.Enabled {
					log.Println("[CameraGuard] 配置已禁用，停止检测")
					return
				}

				// 每次循环重新读取 agent（支持运行时热更新）
				m.mu.RLock()
				currentAgent := m.agent
				m.mu.RUnlock()

				if currentAgent == nil || !currentAgent.IsEnabled() {
					log.Println("[CameraGuard] AI 未启用，跳过本次检测")
					consecutiveDetections = 0
					continue
				}

				// 从最新配置读取参数（支持热更新）
				confirmCount := latestCfg.ConfirmCount
				if confirmCount <= 0 {
					confirmCount = 2
				}
				cooldownSeconds := latestCfg.CooldownSeconds
				if cooldownSeconds <= 0 {
					cooldownSeconds = 60
				}

				sampleTime := time.Now().Format("15:04:05")
				log.Printf("[CameraGuard] [%s] 开始采样抓帧...", sampleTime)

				// 抓取摄像头帧
				base64JPEG, err := camera.CaptureFrame()
				if err != nil {
					log.Printf("[CameraGuard] [%s] 抓帧失败: %v", sampleTime, err)
					consecutiveDetections = 0
					continue
				}
				log.Printf("[CameraGuard] [%s] 抓帧成功，发送给视觉 AI 分析...", sampleTime)

				// 动态加载检测 prompt（支持运行时热更新，用户可在设置页面修改）
				guardPrompt, promptErr := prompt.Load(cameraGuardPromptName)
				if promptErr != nil {
					log.Printf("[CameraGuard] [%s] 加载检测 prompt 失败，使用默认值: %v", sampleTime, promptErr)
					guardPrompt = prompt.DefaultContent(cameraGuardPromptName)
				}

				// 调用视觉 AI 分析
				result, err := currentAgent.AnalyzeImageBase64(base64JPEG, guardPrompt, latestCfg.ProviderName)
				if err != nil {
					log.Printf("[CameraGuard] [%s] 视觉分析失败: %v", sampleTime, err)
					consecutiveDetections = 0
					continue
				}

				log.Printf("[CameraGuard] [%s] 视觉分析结果: %s", sampleTime, result)

				// 判断是否检测到人（第一行包含 YES 即视为检测到）
				// AI 返回格式：第一行 YES/NO，第二行起为理由（仅 YES 时有）
				resultTrimmed := strings.TrimSpace(result)
				firstLine := resultTrimmed
				aiReason := ""
				if newlineIdx := strings.IndexByte(resultTrimmed, '\n'); newlineIdx >= 0 {
					firstLine = resultTrimmed[:newlineIdx]
					aiReason = strings.TrimSpace(resultTrimmed[newlineIdx+1:])
				}
				upperFirstLine := strings.ToUpper(strings.TrimSpace(firstLine))
				personDetected := strings.Contains(upperFirstLine, "YES")

				if personDetected {
					consecutiveDetections++
					log.Printf("[CameraGuard] 检测到背后有人（连续第 %d 次），理由: %s", consecutiveDetections, aiReason)

					// 达到确认次数且冷却时间已过，才触发告警
					cooldownPassed := time.Since(lastAlertTime) >= time.Duration(cooldownSeconds)*time.Second
					if consecutiveDetections >= confirmCount && cooldownPassed {
						detectedAt := time.Now()

						// 保存证据截图到 ~/.luffot/captures/
						savedPath := saveEvidenceFrame(base64JPEG, detectedAt)

						// 将检测记录（图片路径 + AI 理由）持久化到数据库
						if m.storage != nil && savedPath != "" {
							if _, saveErr := m.storage.SaveCameraDetection(detectedAt, savedPath, aiReason); saveErr != nil {
								log.Printf("[CameraGuard] 保存检测记录失败: %v", saveErr)
							} else {
								log.Printf("[CameraGuard] 检测记录已保存到数据库，图片: %s", filepath.Base(savedPath))
							}
						}

						alertMsg := "⚠️ 注意！背后有人靠近，请留意周围环境！"
						if savedPath != "" {
							alertMsg += fmt.Sprintf("\n📸 证据已保存：%s", filepath.Base(savedPath))
						}
						log.Printf("[CameraGuard] 触发告警: %s", alertMsg)

						if m.barrageDisplay != nil {
							m.barrageDisplay.ShowAlert(alertMsg)
						}

						lastAlertTime = detectedAt
						consecutiveDetections = 0 // 告警后重置计数
					}
				} else {
					// 未检测到人，重置连续计数
					if consecutiveDetections > 0 {
						log.Println("[CameraGuard] 背后无人，重置检测计数")
					}
					consecutiveDetections = 0
				}
			}
		}
	}()
}
