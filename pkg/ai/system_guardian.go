package ai

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"

	"github.com/luffot/luffot/pkg/eventbus"
)

// SystemMetrics 系统指标
type SystemMetrics struct {
	CPUUsage      float64   `json:"cpu_usage"`       // CPU使用率（百分比）
	MemoryUsage   float64   `json:"memory_usage"`    // 内存使用率（百分比）
	MemoryUsedMB  uint64    `json:"memory_used_mb"`  // 已用内存（MB）
	MemoryTotalMB uint64    `json:"memory_total_mb"` // 总内存（MB）
	DiskUsage     float64   `json:"disk_usage"`      // 磁盘使用率（百分比）
	NetworkStatus string    `json:"network_status"`  // 网络状态
	Timestamp     time.Time `json:"timestamp"`       // 采集时间
}

// ProcessInfo 进程信息
type ProcessInfo struct {
	PID         int     `json:"pid"`          // 进程ID
	Name        string  `json:"name"`         // 进程名
	CPUUsage    float64 `json:"cpu_usage"`    // CPU使用率
	MemoryMB    float64 `json:"memory_mb"`    // 内存使用（MB）
	IsAnomaly   bool    `json:"is_anomaly"`   // 是否异常
	AnomalyType string  `json:"anomaly_type"` // 异常类型
}

// SystemGuardian 系统管家智能体
// 职责：持续监控底层系统资源与运行状态，生成系统健康度报告
type SystemGuardian struct {
	eventBus *eventbus.EventBus

	// 监控配置
	checkInterval   time.Duration
	cpuThreshold    float64 // CPU告警阈值（百分比）
	memoryThreshold float64 // 内存告警阈值（百分比）
	diskThreshold   float64 // 磁盘告警阈值（百分比）

	// 状态跟踪
	lastCPUAlert    time.Time
	lastMemoryAlert time.Time
	lastDiskAlert   time.Time
	alertCooldown   time.Duration // 告警冷却时间

	// 异常进程检测
	anomalyProcesses map[int]*ProcessInfo
	processThreshold float64 // 进程CPU/Memory异常阈值

	// 监控状态
	running bool
	mu      sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewSystemGuardian 创建系统管家
func NewSystemGuardian() *SystemGuardian {
	ctx, cancel := context.WithCancel(context.Background())
	return &SystemGuardian{
		eventBus:         eventbus.GetGlobalEventBus(),
		checkInterval:    10 * time.Second,
		cpuThreshold:     80.0, // CPU超过80%告警
		memoryThreshold:  85.0, // 内存超过85%告警
		diskThreshold:    90.0, // 磁盘超过90%告警
		alertCooldown:    5 * time.Minute,
		anomalyProcesses: make(map[int]*ProcessInfo),
		processThreshold: 50.0, // 单个进程超过50%视为异常
		ctx:              ctx,
		cancel:           cancel,
	}
}

// Start 启动系统管家
func (sg *SystemGuardian) Start() {
	sg.mu.Lock()
	if sg.running {
		sg.mu.Unlock()
		return
	}
	sg.running = true
	sg.mu.Unlock()

	log.Println("[SystemGuardian] 系统管家启动")

	// 启动监控循环
	go sg.monitoringLoop()
}

// Stop 停止系统管家
func (sg *SystemGuardian) Stop() {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	if !sg.running {
		return
	}

	sg.running = false
	sg.cancel()
	log.Println("[SystemGuardian] 系统管家停止")
}

// monitoringLoop 监控循环
func (sg *SystemGuardian) monitoringLoop() {
	ticker := time.NewTicker(sg.checkInterval)
	defer ticker.Stop()

	// 立即执行一次检查
	sg.checkSystemHealth()

	for {
		select {
		case <-sg.ctx.Done():
			return
		case <-ticker.C:
			sg.checkSystemHealth()
		}
	}
}

// checkSystemHealth 检查系统健康状态
func (sg *SystemGuardian) checkSystemHealth() {
	metrics := sg.collectMetrics()

	// 检查CPU
	if metrics.CPUUsage > sg.cpuThreshold {
		sg.handleCPUHigh(metrics.CPUUsage)
	}

	// 检查内存
	if metrics.MemoryUsage > sg.memoryThreshold {
		sg.handleMemoryOveruse(metrics.MemoryUsage, metrics.MemoryUsedMB)
	}

	// 检查磁盘
	// 注意：磁盘检查在macOS上需要特殊处理，这里简化实现

	// 检查异常进程
	sg.checkAnomalyProcesses()
}

// collectMetrics 采集系统指标
func (sg *SystemGuardian) collectMetrics() *SystemMetrics {
	metrics := &SystemMetrics{
		Timestamp: time.Now(),
	}

	// 获取内存统计
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	metrics.MemoryUsedMB = m.Sys / 1024 / 1024
	metrics.MemoryTotalMB = sg.getTotalMemory()

	if metrics.MemoryTotalMB > 0 {
		metrics.MemoryUsage = float64(metrics.MemoryUsedMB) / float64(metrics.MemoryTotalMB) * 100
	}

	// CPU使用率简化计算（实际实现可能需要调用系统API）
	metrics.CPUUsage = sg.getCPUUsage()

	return metrics
}

// getTotalMemory 获取系统总内存
func (sg *SystemGuardian) getTotalMemory() uint64 {
	// 简化实现，实际应该调用系统API获取
	// 这里返回一个默认值
	return 16384 // 假设16GB内存
}

// getCPUUsage 获取CPU使用率
func (sg *SystemGuardian) getCPUUsage() float64 {
	// 简化实现，实际应该调用系统API获取
	// 返回一个模拟值
	return 30.0 // 假设30%使用率
}

// handleCPUHigh 处理CPU过高
func (sg *SystemGuardian) handleCPUHigh(usage float64) {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	// 检查冷却时间
	if time.Since(sg.lastCPUAlert) < sg.alertCooldown {
		return
	}

	sg.lastCPUAlert = time.Now()

	// 发布事件
	event := eventbus.NewEvent(
		eventbus.SystemCPUHigh,
		"system_guardian",
		map[string]interface{}{
			"cpu_usage": usage,
			"threshold": sg.cpuThreshold,
			"severity":  sg.calculateSeverity(usage, sg.cpuThreshold),
		},
	).WithPriority(eventbus.PriorityHigh).
		WithDescription(fmt.Sprintf("CPU使用率 %.1f%%，超过阈值 %.1f%%", usage, sg.cpuThreshold))

	sg.eventBus.Publish(event)

	log.Printf("[SystemGuardian] CPU告警：%.1f%%", usage)
}

// handleMemoryOveruse 处理内存过高
func (sg *SystemGuardian) handleMemoryOveruse(usage float64, usedMB uint64) {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	// 检查冷却时间
	if time.Since(sg.lastMemoryAlert) < sg.alertCooldown {
		return
	}

	sg.lastMemoryAlert = time.Now()

	// 发布事件
	event := eventbus.NewEvent(
		eventbus.SystemMemoryOveruse,
		"system_guardian",
		map[string]interface{}{
			"memory_usage":   usage,
			"memory_used_mb": usedMB,
			"threshold":      sg.memoryThreshold,
			"severity":       sg.calculateSeverity(usage, sg.memoryThreshold),
		},
	).WithPriority(eventbus.PriorityHigh).
		WithDescription(fmt.Sprintf("内存使用率 %.1f%%（%d MB），超过阈值 %.1f%%", usage, usedMB, sg.memoryThreshold))

	sg.eventBus.Publish(event)

	log.Printf("[SystemGuardian] 内存告警：%.1f%% (%d MB)", usage, usedMB)
}

// checkAnomalyProcesses 检查异常进程
func (sg *SystemGuardian) checkAnomalyProcesses() {
	// 获取进程列表（简化实现）
	processes := sg.getProcessList()

	for _, proc := range processes {
		if proc.CPUUsage > sg.processThreshold || proc.MemoryMB > 1024 {
			// 发现异常进程
			proc.IsAnomaly = true
			if proc.CPUUsage > sg.processThreshold {
				proc.AnomalyType = "cpu_high"
			} else {
				proc.AnomalyType = "memory_high"
			}

			sg.anomalyProcesses[proc.PID] = proc

			// 发布异常进程事件
			event := eventbus.NewEvent(
				eventbus.SystemProcessAnomaly,
				"system_guardian",
				map[string]interface{}{
					"pid":          proc.PID,
					"process_name": proc.Name,
					"cpu_usage":    proc.CPUUsage,
					"memory_mb":    proc.MemoryMB,
					"anomaly_type": proc.AnomalyType,
				},
			).WithPriority(eventbus.PriorityNormal).
				WithDescription(fmt.Sprintf("发现异常进程：%s (PID:%d, CPU:%.1f%%, MEM:%.0fMB)",
					proc.Name, proc.PID, proc.CPUUsage, proc.MemoryMB))

			sg.eventBus.Publish(event)
		}
	}
}

// getProcessList 获取进程列表（简化实现）
func (sg *SystemGuardian) getProcessList() []*ProcessInfo {
	// 实际实现应该调用系统API获取进程信息
	// 这里返回空列表作为占位
	return []*ProcessInfo{}
}

// calculateSeverity 计算严重等级
func (sg *SystemGuardian) calculateSeverity(current, threshold float64) string {
	ratio := current / threshold
	if ratio > 1.5 {
		return "critical"
	} else if ratio > 1.2 {
		return "high"
	} else if ratio > 1.0 {
		return "medium"
	}
	return "low"
}

// GetSystemHealth 获取系统健康状态
func (sg *SystemGuardian) GetSystemHealth() map[string]interface{} {
	metrics := sg.collectMetrics()

	sg.mu.RLock()
	defer sg.mu.RUnlock()

	return map[string]interface{}{
		"cpu_usage":         metrics.CPUUsage,
		"memory_usage":      metrics.MemoryUsage,
		"memory_used_mb":    metrics.MemoryUsedMB,
		"memory_total_mb":   metrics.MemoryTotalMB,
		"anomaly_processes": len(sg.anomalyProcesses),
		"last_check":        metrics.Timestamp,
	}
}

// SetThresholds 设置告警阈值
func (sg *SystemGuardian) SetThresholds(cpu, memory, disk float64) {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	if cpu > 0 {
		sg.cpuThreshold = cpu
	}
	if memory > 0 {
		sg.memoryThreshold = memory
	}
	if disk > 0 {
		sg.diskThreshold = disk
	}
}

// IsRunning 检查是否运行中
func (sg *SystemGuardian) IsRunning() bool {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	return sg.running
}
