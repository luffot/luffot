package scheduler

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"sync"
	"time"

	"github.com/luffot/luffot/pkg/ai"
	"github.com/luffot/luffot/pkg/config"
	"github.com/luffot/luffot/pkg/storage"
	"github.com/robfig/cron/v3"
)

// TaskStatus 任务的当前状态
type TaskStatus string

const (
	TaskStatusIdle    TaskStatus = "idle"    // 空闲，等待下次触发
	TaskStatusRunning TaskStatus = "running" // 正在执行
	TaskStatusSuccess TaskStatus = "success" // 上次执行成功
	TaskStatusFailed  TaskStatus = "failed"  // 上次执行失败
)

// TaskInfo 任务运行时信息（供 API 查询）
type TaskInfo struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Cron        string     `json:"cron"`
	Type        string     `json:"type"`
	Enabled     bool       `json:"enabled"`
	Status      TaskStatus `json:"status"`
	LastRunAt   *time.Time `json:"last_run_at,omitempty"`
	LastError   string     `json:"last_error,omitempty"`
	NextRunAt   *time.Time `json:"next_run_at,omitempty"`
}

// taskEntry 内部任务条目，持有运行时状态
type taskEntry struct {
	cfg         config.ScheduledTaskConfig
	cronEntryID cron.EntryID
	mu          sync.Mutex
	status      TaskStatus
	lastRunAt   *time.Time
	lastError   string
}

// Scheduler 定时任务调度器
type Scheduler struct {
	cron    *cron.Cron
	agent   *ai.Agent
	storage *storage.Storage
	entries map[string]*taskEntry // key: task name
	mu      sync.RWMutex
	ctx     context.Context
}

// NewScheduler 创建调度器实例
func NewScheduler(agent *ai.Agent, st *storage.Storage) *Scheduler {
	return &Scheduler{
		cron: cron.New(
			// 使用标准 5 段 cron（分 时 日 月 周），与 Linux crontab 格式一致
			cron.WithParser(cron.NewParser(
				cron.Minute|cron.Hour|cron.Dom|cron.Month|cron.Dow|cron.Descriptor,
			)),
			cron.WithLogger(cron.DefaultLogger),
		),
		agent:   agent,
		storage: st,
		entries: make(map[string]*taskEntry),
	}
}

// Start 启动调度器，注册配置中所有启用的任务
func (s *Scheduler) Start(ctx context.Context) error {
	s.ctx = ctx

	cfg := config.Get().ScheduledTasks
	if !cfg.Enabled {
		log.Println("[Scheduler] 未启用，跳过启动")
		return nil
	}

	for _, taskCfg := range cfg.Tasks {
		if !taskCfg.Enabled {
			log.Printf("[Scheduler] 任务 %q 已禁用，跳过注册", taskCfg.Name)
			continue
		}
		if err := s.registerTask(taskCfg); err != nil {
			log.Printf("[Scheduler] 注册任务 %q 失败: %v", taskCfg.Name, err)
			continue
		}
		log.Printf("[Scheduler] 已注册任务 %q，cron=%q，类型=%s", taskCfg.Name, taskCfg.Cron, taskCfg.Type)
	}

	s.cron.Start()
	log.Printf("[Scheduler] 调度器已启动，共注册 %d 个任务", len(s.entries))

	// 监听 ctx 取消，优雅停止
	go func() {
		<-ctx.Done()
		s.cron.Stop()
		log.Println("[Scheduler] 调度器已停止")
	}()

	return nil
}

// registerTask 注册单个任务到 cron
func (s *Scheduler) registerTask(taskCfg config.ScheduledTaskConfig) error {
	entry := &taskEntry{
		cfg:    taskCfg,
		status: TaskStatusIdle,
	}

	entryID, err := s.cron.AddFunc(taskCfg.Cron, func() {
		s.runTask(entry)
	})
	if err != nil {
		return fmt.Errorf("解析 cron 表达式 %q 失败: %w", taskCfg.Cron, err)
	}

	entry.cronEntryID = entryID

	s.mu.Lock()
	s.entries[taskCfg.Name] = entry
	s.mu.Unlock()

	return nil
}

// TriggerTask 手动触发指定名称的任务（异步执行，立即返回）
// 若任务不存在返回 error；若任务正在运行则跳过本次触发
func (s *Scheduler) TriggerTask(name string) error {
	s.mu.RLock()
	entry, exists := s.entries[name]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("任务 %q 不存在", name)
	}

	go s.runTask(entry)
	return nil
}

// ListTasks 返回所有已注册任务的当前状态信息
func (s *Scheduler) ListTasks() []TaskInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	infos := make([]TaskInfo, 0, len(s.entries))
	for _, entry := range s.entries {
		entry.mu.Lock()
		info := TaskInfo{
			Name:        entry.cfg.Name,
			Description: entry.cfg.Description,
			Cron:        entry.cfg.Cron,
			Type:        string(entry.cfg.Type),
			Enabled:     entry.cfg.Enabled,
			Status:      entry.status,
			LastRunAt:   entry.lastRunAt,
			LastError:   entry.lastError,
		}
		entry.mu.Unlock()

		// 从 cron 引擎获取下次执行时间
		cronEntry := s.cron.Entry(entry.cronEntryID)
		if !cronEntry.Next.IsZero() {
			nextRun := cronEntry.Next
			info.NextRunAt = &nextRun
		}

		infos = append(infos, info)
	}
	return infos
}

// runTask 执行任务（在 goroutine 中调用）
func (s *Scheduler) runTask(entry *taskEntry) {
	entry.mu.Lock()
	if entry.status == TaskStatusRunning {
		entry.mu.Unlock()
		log.Printf("[Scheduler] 任务 %q 正在运行，跳过本次触发", entry.cfg.Name)
		return
	}
	entry.status = TaskStatusRunning
	now := time.Now()
	entry.lastRunAt = &now
	entry.lastError = ""
	entry.mu.Unlock()

	log.Printf("[Scheduler] 开始执行任务 %q（类型=%s）", entry.cfg.Name, entry.cfg.Type)
	startTime := time.Now()

	var runErr error
	switch entry.cfg.Type {
	case config.ScheduledTaskTypeBuiltin:
		runErr = s.runBuiltinTask(entry.cfg)
	case config.ScheduledTaskTypePython:
		runErr = s.runPythonTask(entry.cfg)
	default:
		runErr = fmt.Errorf("未知任务类型: %s", entry.cfg.Type)
	}

	elapsed := time.Since(startTime).Round(time.Millisecond)

	entry.mu.Lock()
	if runErr != nil {
		entry.status = TaskStatusFailed
		entry.lastError = runErr.Error()
		log.Printf("[Scheduler] 任务 %q 执行失败（耗时 %v）: %v", entry.cfg.Name, elapsed, runErr)
	} else {
		entry.status = TaskStatusSuccess
		log.Printf("[Scheduler] 任务 %q 执行成功（耗时 %v）", entry.cfg.Name, elapsed)
	}
	entry.mu.Unlock()
}

// runBuiltinTask 执行内置任务
func (s *Scheduler) runBuiltinTask(taskCfg config.ScheduledTaskConfig) error {
	switch taskCfg.BuiltinName {
	case "daily_profile_report":
		return RunDailyProfileReport(s.ctx, s.agent, s.storage, taskCfg.ProviderName)
	default:
		return fmt.Errorf("未知内置任务名: %s", taskCfg.BuiltinName)
	}
}

// runPythonTask 执行 Python 脚本任务
func (s *Scheduler) runPythonTask(taskCfg config.ScheduledTaskConfig) error {
	if taskCfg.ScriptPath == "" {
		return fmt.Errorf("python 任务未配置 script_path")
	}

	args := append([]string{taskCfg.ScriptPath}, taskCfg.ScriptArgs...)
	cmd := exec.CommandContext(s.ctx, "python3", args...)

	output, err := cmd.CombinedOutput()
	if len(output) > 0 {
		log.Printf("[Scheduler] 任务 %q 脚本输出:\n%s", taskCfg.Name, string(output))
	}
	if err != nil {
		return fmt.Errorf("python 脚本执行失败: %w", err)
	}
	return nil
}
