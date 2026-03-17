package ai

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/luffot/luffot/pkg/barrage"
	"github.com/luffot/luffot/pkg/config"
	"github.com/luffot/luffot/pkg/eventbus"
	"github.com/luffot/luffot/pkg/storage"
)

// ReactiveAIChain 响应式AI链路
// 整合所有智能体，提供统一的启动、停止和管理接口
type ReactiveAIChain struct {
	// 核心组件
	agent       *Agent
	coordinator *Coordinator
	userTracker *UserTracker
	eventBus    *eventbus.EventBus

	// 智能体
	systemGuardian  *SystemGuardian
	appSecretaryMgr *AppSecretaryManager
	cameraPatrol    *CameraPatrol

	// 输出
	barrageDisplay *barrage.BarrageDisplay
	onInsight      OnInsightCallback

	// 配置
	config *config.ReactiveAIConfig

	// 状态
	running bool
	mu      sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
}

// ReactiveAIConfig 响应式AI配置（需要在config包中添加）
type ReactiveAIConfig struct {
	// 是否启用响应式AI链路
	Enabled bool `yaml:"enabled" json:"enabled"`

	// 各智能体开关
	SystemGuardianEnabled bool `yaml:"system_guardian_enabled" json:"system_guardian_enabled"`
	UserTrackerEnabled    bool `yaml:"user_tracker_enabled" json:"user_tracker_enabled"`
	CameraPatrolEnabled   bool `yaml:"camera_patrol_enabled" json:"camera_patrol_enabled"`

	// 协调器配置
	CoordinatorInterval int `yaml:"coordinator_interval" json:"coordinator_interval"` // 洞察生成间隔（秒）
}

// NewReactiveAIChain 创建响应式AI链路
func NewReactiveAIChain(
	agent *Agent,
	storage *storage.Storage,
	barrageDisplay *barrage.BarrageDisplay,
	cfg config.ReactiveAIConfig,
) *ReactiveAIChain {
	ctx, cancel := context.WithCancel(context.Background())

	chain := &ReactiveAIChain{
		agent:          agent,
		eventBus:       eventbus.GetGlobalEventBus(),
		barrageDisplay: barrageDisplay,
		config:         &cfg,
		ctx:            ctx,
		cancel:         cancel,
	}

	// 设置洞察回调
	chain.onInsight = chain.handleInsight

	// 初始化协调器
	chain.coordinator = NewCoordinator(agent, chain.onInsight)

	// 初始化用户跟踪器
	chain.userTracker = NewUserTracker(storage)

	// 初始化系统管家
	chain.systemGuardian = NewSystemGuardian()

	// 初始化应用秘书管理器
	chain.appSecretaryMgr = NewAppSecretaryManager()

	// 初始化摄像头巡查员
	chain.cameraPatrol = NewCameraPatrol(agent)

	return chain
}

// Start 启动响应式AI链路
func (rac *ReactiveAIChain) Start() error {
	rac.mu.Lock()
	defer rac.mu.Unlock()

	if rac.running {
		return nil
	}

	if !rac.config.Enabled {
		log.Println("[ReactiveAIChain] 响应式AI链路未启用")
		return nil
	}

	log.Println("[ReactiveAIChain] 启动响应式AI链路...")

	// 启动事件总线
	rac.eventBus.Start()

	// 启动协调器
	rac.coordinator.Start()

	// 启动用户跟踪器
	if rac.config.UserTrackerEnabled {
		rac.userTracker.Start()
		log.Println("[ReactiveAIChain] 用户习惯记录员已启动")
	}

	// 启动系统管家
	if rac.config.SystemGuardianEnabled {
		rac.systemGuardian.Start()
		log.Println("[ReactiveAIChain] 系统管家已启动")
	}

	// 启动应用秘书
	rac.appSecretaryMgr.StartAll()
	log.Println("[ReactiveAIChain] 应用秘书已启动")

	// 启动摄像头巡查员（如果启用）
	if rac.config.CameraPatrolEnabled {
		rac.cameraPatrol.Start()
		rac.cameraPatrol.Enable()
		log.Println("[ReactiveAIChain] 摄像头巡查员已启动")
	}

	rac.running = true
	log.Println("[ReactiveAIChain] 响应式AI链路启动完成")

	return nil
}

// Stop 停止响应式AI链路
func (rac *ReactiveAIChain) Stop() {
	rac.mu.Lock()
	defer rac.mu.Unlock()

	if !rac.running {
		return
	}

	log.Println("[ReactiveAIChain] 停止响应式AI链路...")

	rac.cancel()

	// 停止各个组件
	rac.coordinator.Stop()
	rac.userTracker.Stop()
	rac.systemGuardian.Stop()
	rac.appSecretaryMgr.StopAll()
	rac.cameraPatrol.Stop()

	rac.running = false
	log.Println("[ReactiveAIChain] 响应式AI链路已停止")
}

// handleInsight 处理洞察
func (rac *ReactiveAIChain) handleInsight(insight *UserInsight) {
	// 根据洞察类型和优先级决定是否展示
	if !rac.shouldShowInsight(insight) {
		return
	}

	// 格式化洞察内容
	message := rac.formatInsightMessage(insight)

	// 通过弹幕展示
	if rac.barrageDisplay != nil {
		rac.barrageDisplay.ShowAlert(message)
	}

	log.Printf("[ReactiveAIChain] 洞察透出: %s", insight.Title)
}

// shouldShowInsight 判断是否应展示洞察
func (rac *ReactiveAIChain) shouldShowInsight(insight *UserInsight) bool {
	// 根据优先级过滤
	if insight.Priority < 3 {
		// 低优先级洞察不直接展示，只记录
		return false
	}

	// 根据类型过滤
	switch insight.Type {
	case InsightTypeSystemAlert:
		// 系统告警总是展示
		return true
	case InsightTypeAppNotification:
		// 应用通知根据优先级
		return insight.Priority >= 5
	case InsightTypeEnvironmentAlert:
		// 环境告警总是展示
		return true
	case InsightTypeActivitySummary:
		// 活动摘要不频繁展示
		return insight.Priority >= 7
	case InsightTypeTaskReminder:
		// 任务提醒总是展示
		return true
	default:
		return false
	}
}

// formatInsightMessage 格式化洞察消息
func (rac *ReactiveAIChain) formatInsightMessage(insight *UserInsight) string {
	var prefix string

	switch insight.Type {
	case InsightTypeSystemAlert:
		prefix = "⚠️ "
	case InsightTypeAppNotification:
		if insight.Priority >= 7 {
			prefix = "🔔 "
		} else {
			prefix = "📱 "
		}
	case InsightTypeEnvironmentAlert:
		prefix = "👁️ "
	case InsightTypeActivitySummary:
		prefix = "📊 "
	case InsightTypeTaskReminder:
		prefix = "⏰ "
	default:
		prefix = "💡 "
	}

	return fmt.Sprintf("%s%s\n%s", prefix, insight.Title, insight.Content)
}

// HandleMessageEvent 处理消息事件（供外部调用）
func (rac *ReactiveAIChain) HandleMessageEvent(appType string, event interface{}) {
	// 将消息事件转发给应用秘书
	// 这里需要根据appType分发到对应的应用秘书
}

// GetUserState 获取用户状态
func (rac *ReactiveAIChain) GetUserState() *UserState {
	if rac.coordinator != nil {
		return rac.coordinator.GetUserState()
	}
	return nil
}

// GetUserHabits 获取用户习惯
func (rac *ReactiveAIChain) GetUserHabits() map[string]interface{} {
	rac.mu.RLock()
	defer rac.mu.RUnlock()

	if rac.userTracker == nil {
		return nil
	}

	currentApp, duration := rac.userTracker.GetCurrentActivity()
	return map[string]interface{}{
		"long_term_apps":   rac.userTracker.GetLongTermApps(),
		"short_term_apps":  rac.userTracker.GetShortTermApps(),
		"current_app":      currentApp,
		"current_duration": duration.String(),
	}
}

// GetSystemHealth 获取系统健康状态
func (rac *ReactiveAIChain) GetSystemHealth() map[string]interface{} {
	if rac.systemGuardian != nil {
		return rac.systemGuardian.GetSystemHealth()
	}
	return nil
}

// EnableCameraPatrol 启用摄像头巡查
func (rac *ReactiveAIChain) EnableCameraPatrol() {
	if rac.cameraPatrol != nil {
		rac.cameraPatrol.Enable()
	}
}

// DisableCameraPatrol 禁用摄像头巡查
func (rac *ReactiveAIChain) DisableCameraPatrol() {
	if rac.cameraPatrol != nil {
		rac.cameraPatrol.Disable()
	}
}

// IsCameraPatrolEnabled 检查摄像头巡查是否启用
func (rac *ReactiveAIChain) IsCameraPatrolEnabled() bool {
	if rac.cameraPatrol != nil {
		return rac.cameraPatrol.IsEnabled()
	}
	return false
}

// RegisterAppSecretary 注册应用秘书
func (rac *ReactiveAIChain) RegisterAppSecretary(appType AppType, appName string) *AppSecretary {
	sec := rac.appSecretaryMgr.RegisterSecretary(appType, appName)
	// 注入 AI 智能体
	if rac.agent != nil {
		sec.SetAgent(rac.agent)
	}
	return sec
}

// RegisterDingTalkSecretary 注册钉钉消息秘书（专用方法）
func (rac *ReactiveAIChain) RegisterDingTalkSecretary() *AppSecretary {
	return rac.RegisterAppSecretary(AppTypeDingTalk, "钉钉")
}

// GetAppSecretaryManager 获取应用秘书管理器
func (rac *ReactiveAIChain) GetAppSecretaryManager() *AppSecretaryManager {
	return rac.appSecretaryMgr
}

// IsRunning 检查是否运行中
func (rac *ReactiveAIChain) IsRunning() bool {
	rac.mu.RLock()
	defer rac.mu.RUnlock()
	return rac.running
}

// GetCoordinator 获取协调器（供外部访问）
func (rac *ReactiveAIChain) GetCoordinator() *Coordinator {
	return rac.coordinator
}

// SetCoordinatorReportStrategy 设置 AI 丞相汇报策略
func (rac *ReactiveAIChain) SetCoordinatorReportStrategy(strategy CoordinatorReportStrategy) {
	if rac.coordinator != nil {
		rac.coordinator.SetReportStrategy(strategy)
	}
}

// SetAppSecretaryReportStrategy 设置应用秘书汇报策略
func (rac *ReactiveAIChain) SetAppSecretaryReportStrategy(appType AppType, strategy ReportStrategy) {
	sec := rac.appSecretaryMgr.GetSecretary(appType)
	if sec != nil {
		sec.SetReportStrategy(strategy)
	}
}

// GetUserTracker 获取用户跟踪器（供外部访问）
func (rac *ReactiveAIChain) GetUserTracker() *UserTracker {
	return rac.userTracker
}

// GetSystemGuardian 获取系统管家（供外部访问）
func (rac *ReactiveAIChain) GetSystemGuardian() *SystemGuardian {
	return rac.systemGuardian
}

// GetCameraPatrol 获取摄像头巡查员（供外部访问）
func (rac *ReactiveAIChain) GetCameraPatrol() *CameraPatrol {
	return rac.cameraPatrol
}
