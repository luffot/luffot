package ai

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/luffot/luffot/pkg/eventbus"
	"github.com/luffot/luffot/pkg/storage"
)

// AppUsage 应用使用记录
type AppUsage struct {
	AppName        string        `json:"app_name"`         // 应用名称
	StartTime      time.Time     `json:"start_time"`       // 开始使用时间
	Duration       time.Duration `json:"duration"`         // 使用时长
	IsLongTerm     bool          `json:"is_long_term"`     // 是否长期高频使用
	DailyUseCount  int           `json:"daily_use_count"`  // 今日使用次数
	WeeklyUseCount int           `json:"weekly_use_count"` // 本周使用次数
}

// UserHabit 用户习惯数据
type UserHabit struct {
	LongTermApps    []string             `json:"long_term_apps"`    // 长期高频应用列表
	ShortTermApps   []string             `json:"short_term_apps"`   // 短期临时应用列表
	AppUsageStats   map[string]*AppUsage `json:"app_usage_stats"`   // 应用使用统计
	ActiveTimeSlots []TimeSlot           `json:"active_time_slots"` // 活跃时间段
	LastUpdated     time.Time            `json:"last_updated"`      // 最后更新时间
}

// TimeSlot 时间段
type TimeSlot struct {
	StartHour int    `json:"start_hour"` // 开始小时（0-23）
	EndHour   int    `json:"end_hour"`   // 结束小时（0-23）
	Label     string `json:"label"`      // 标签（工作日/周末）
}

// UserTracker 用户习惯记录员
// 职责：长期追踪并分析用户的软件使用模式，构建动态用户画像
type UserTracker struct {
	storage      *storage.Storage
	eventBus     *eventbus.EventBus
	habits       *UserHabit
	currentApp   string
	appStartTime time.Time

	// 应用使用阈值
	longTermThreshold  time.Duration // 长期应用使用时长阈值
	shortTermThreshold time.Duration // 短期应用使用时长阈值

	// 数据持久化定时器
	persistTicker *time.Ticker

	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
}

// NewUserTracker 创建用户习惯跟踪器
func NewUserTracker(st *storage.Storage) *UserTracker {
	ctx, cancel := context.WithCancel(context.Background())
	return &UserTracker{
		storage:  st,
		eventBus: eventbus.GetGlobalEventBus(),
		habits: &UserHabit{
			LongTermApps:    make([]string, 0),
			ShortTermApps:   make([]string, 0),
			AppUsageStats:   make(map[string]*AppUsage),
			ActiveTimeSlots: make([]TimeSlot, 0),
			LastUpdated:     time.Now(),
		},
		longTermThreshold:  30 * time.Minute, // 30分钟以上视为长期使用
		shortTermThreshold: 5 * time.Minute,  // 5分钟以下视为临时使用
		ctx:                ctx,
		cancel:             cancel,
	}
}

// Start 启动用户习惯跟踪器
func (ut *UserTracker) Start() {
	log.Println("[UserTracker] 用户习惯记录员启动")

	// 加载历史习惯数据
	ut.loadHabits()

	// 订阅应用切换事件
	ut.eventBus.Subscribe(eventbus.UserAppSwitched, ut.handleAppSwitched)
	ut.eventBus.Subscribe(eventbus.UserIdleDetected, ut.handleUserIdle)
	ut.eventBus.Subscribe(eventbus.UserActiveDetected, ut.handleUserActive)

	// 启动数据持久化定时器（每5分钟保存一次）
	ut.persistTicker = time.NewTicker(5 * time.Minute)
	go func() {
		for {
			select {
			case <-ut.ctx.Done():
				return
			case <-ut.persistTicker.C:
				ut.persistHabits()
			}
		}
	}()
}

// Stop 停止用户习惯跟踪器
func (ut *UserTracker) Stop() {
	log.Println("[UserTracker] 用户习惯记录员停止")
	ut.cancel()
	if ut.persistTicker != nil {
		ut.persistTicker.Stop()
	}
	// 保存最终数据
	ut.persistHabits()
}

// handleAppSwitched 处理应用切换事件
func (ut *UserTracker) handleAppSwitched(event *eventbus.Event) {
	appName, ok := event.Data["app_name"].(string)
	if !ok || appName == "" {
		return
	}

	ut.mu.Lock()
	defer ut.mu.Unlock()

	// 记录上一个应用的使用时长
	if ut.currentApp != "" && !ut.appStartTime.IsZero() {
		duration := time.Since(ut.appStartTime)
		ut.recordAppUsage(ut.currentApp, duration)
	}

	// 开始记录新应用
	ut.currentApp = appName
	ut.appStartTime = time.Now()

	log.Printf("[UserTracker] 切换到应用: %s", appName)
}

// handleUserIdle 处理用户空闲事件
func (ut *UserTracker) handleUserIdle(event *eventbus.Event) {
	ut.mu.Lock()
	defer ut.mu.Unlock()

	// 记录当前应用的使用时长
	if ut.currentApp != "" && !ut.appStartTime.IsZero() {
		duration := time.Since(ut.appStartTime)
		ut.recordAppUsage(ut.currentApp, duration)
		ut.currentApp = ""
		ut.appStartTime = time.Time{}
	}
}

// handleUserActive 处理用户活跃事件
func (ut *UserTracker) handleUserActive(event *eventbus.Event) {
	// 可以记录用户重新活跃的时间点，用于分析活跃时间段
}

// recordAppUsage 记录应用使用时长
func (ut *UserTracker) recordAppUsage(appName string, duration time.Duration) {
	if appName == "" || duration <= 0 {
		return
	}

	usage, exists := ut.habits.AppUsageStats[appName]
	if !exists {
		usage = &AppUsage{
			AppName:   appName,
			StartTime: ut.appStartTime,
		}
		ut.habits.AppUsageStats[appName] = usage
	}

	usage.Duration += duration
	usage.DailyUseCount++
	usage.WeeklyUseCount++

	// 判断应用类型
	isLongTerm := usage.Duration >= ut.longTermThreshold
	isShortTerm := duration < ut.shortTermThreshold

	// 更新长期应用列表
	if isLongTerm {
		if !ut.containsString(ut.habits.LongTermApps, appName) {
			ut.habits.LongTermApps = append(ut.habits.LongTermApps, appName)
			log.Printf("[UserTracker] %s 被标记为长期高频应用", appName)
		}
		// 从短期列表中移除
		ut.habits.ShortTermApps = ut.removeString(ut.habits.ShortTermApps, appName)
	} else if isShortTerm && !ut.containsString(ut.habits.LongTermApps, appName) {
		if !ut.containsString(ut.habits.ShortTermApps, appName) {
			ut.habits.ShortTermApps = append(ut.habits.ShortTermApps, appName)
		}
	}

	usage.IsLongTerm = isLongTerm
	ut.habits.LastUpdated = time.Now()

	// 发布用户习惯更新事件
	ut.eventBus.Publish(eventbus.NewEvent(
		eventbus.EventType("user.habit_updated"),
		"user_tracker",
		map[string]interface{}{
			"app_name":       appName,
			"duration":       duration.String(),
			"total_duration": usage.Duration.String(),
			"is_long_term":   isLongTerm,
		},
	).WithDescription(fmt.Sprintf("%s 使用 %v", appName, duration)))
}

// GetLongTermApps 获取长期高频应用列表
func (ut *UserTracker) GetLongTermApps() []string {
	ut.mu.RLock()
	defer ut.mu.RUnlock()

	result := make([]string, len(ut.habits.LongTermApps))
	copy(result, ut.habits.LongTermApps)
	return result
}

// GetShortTermApps 获取短期临时应用列表
func (ut *UserTracker) GetShortTermApps() []string {
	ut.mu.RLock()
	defer ut.mu.RUnlock()

	result := make([]string, len(ut.habits.ShortTermApps))
	copy(result, ut.habits.ShortTermApps)
	return result
}

// GetAppUsageStats 获取应用使用统计
func (ut *UserTracker) GetAppUsageStats(appName string) *AppUsage {
	ut.mu.RLock()
	defer ut.mu.RUnlock()

	if usage, exists := ut.habits.AppUsageStats[appName]; exists {
		// 返回副本
		usageCopy := *usage
		return &usageCopy
	}
	return nil
}

// GetAllAppUsageStats 获取所有应用使用统计
func (ut *UserTracker) GetAllAppUsageStats() map[string]*AppUsage {
	ut.mu.RLock()
	defer ut.mu.RUnlock()

	result := make(map[string]*AppUsage)
	for k, v := range ut.habits.AppUsageStats {
		usageCopy := *v
		result[k] = &usageCopy
	}
	return result
}

// GetCurrentActivity 获取当前活动
func (ut *UserTracker) GetCurrentActivity() (appName string, duration time.Duration) {
	ut.mu.RLock()
	defer ut.mu.RUnlock()

	if ut.currentApp != "" && !ut.appStartTime.IsZero() {
		return ut.currentApp, time.Since(ut.appStartTime)
	}
	return "", 0
}

// GetUserHabitsSummary 获取用户习惯摘要（用于AI分析）
func (ut *UserTracker) GetUserHabitsSummary() string {
	ut.mu.RLock()
	defer ut.mu.RUnlock()

	var summary string

	// 长期高频应用
	if len(ut.habits.LongTermApps) > 0 {
		summary += "长期高频应用："
		for _, app := range ut.habits.LongTermApps {
			if usage, ok := ut.habits.AppUsageStats[app]; ok {
				summary += fmt.Sprintf("\n- %s（累计使用 %v）", app, usage.Duration.Round(time.Minute))
			}
		}
		summary += "\n\n"
	}

	// 今日使用统计
	todayTotal := time.Duration(0)
	for _, usage := range ut.habits.AppUsageStats {
		todayTotal += usage.Duration
	}
	if todayTotal > 0 {
		summary += fmt.Sprintf("今日总使用时长：%v\n", todayTotal.Round(time.Minute))
	}

	return summary
}

// loadHabits 从存储加载习惯数据
func (ut *UserTracker) loadHabits() {
	// 这里可以从数据库加载历史习惯数据
	// 简化实现：从内存开始，后续可以添加持久化
}

// persistHabits 持久化习惯数据
func (ut *UserTracker) persistHabits() {
	ut.mu.RLock()
	defer ut.mu.RUnlock()

	// 可以保存到数据库或文件
	log.Printf("[UserTracker] 习惯数据已更新，长期应用：%d，短期应用：%d",
		len(ut.habits.LongTermApps), len(ut.habits.ShortTermApps))
}

// containsString 检查字符串是否在切片中
func (ut *UserTracker) containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// removeString 从切片中移除字符串
func (ut *UserTracker) removeString(slice []string, item string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}

// ResetDailyStats 重置每日统计（应在每天凌晨调用）
func (ut *UserTracker) ResetDailyStats() {
	ut.mu.Lock()
	defer ut.mu.Unlock()

	for _, usage := range ut.habits.AppUsageStats {
		usage.DailyUseCount = 0
		usage.Duration = 0
	}

	log.Println("[UserTracker] 每日统计已重置")
}

// ResetWeeklyStats 重置每周统计（应在每周一凌晨调用）
func (ut *UserTracker) ResetWeeklyStats() {
	ut.mu.Lock()
	defer ut.mu.Unlock()

	for _, usage := range ut.habits.AppUsageStats {
		usage.WeeklyUseCount = 0
	}

	log.Println("[UserTracker] 每周统计已重置")
}
