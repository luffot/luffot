package adk

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// Engine ADK智能引擎
type Engine struct {
	rootAgent     *BaseAgent
	planner       *PlannerAgent
	skillManager  *SkillManager
	agents        map[string]*BaseAgent
	plans         map[string]*Plan
	executions    map[string]*Execution
	mu            sync.RWMutex
	eventHandlers []EventHandler
}

// EventHandler 事件处理器
type EventHandler func(event *Event)

// Event 引擎事件
type Event struct {
	Type      EventType              `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// EventType 事件类型
type EventType string

const (
	EventTypePlanCreated   EventType = "plan_created"
	EventTypePlanStarted   EventType = "plan_started"
	EventTypePlanCompleted EventType = "plan_completed"
	EventTypePlanFailed    EventType = "plan_failed"
	EventTypeStepStarted   EventType = "step_started"
	EventTypeStepCompleted EventType = "step_completed"
	EventTypeStepFailed    EventType = "step_failed"
	EventTypeAgentInvoked  EventType = "agent_invoked"
	EventTypeSkillExecuted EventType = "skill_executed"
)

// Execution 执行实例
type Execution struct {
	ID        string                 `json:"id"`
	PlanID    string                 `json:"plan_id"`
	Status    ExecutionStatus        `json:"status"`
	StartedAt time.Time              `json:"started_at"`
	EndedAt   *time.Time             `json:"ended_at"`
	Results   map[string]interface{} `json:"results"`
	Errors    []ExecutionError       `json:"errors"`
}

// ExecutionStatus 执行状态
type ExecutionStatus string

const (
	ExecutionStatusRunning   ExecutionStatus = "running"
	ExecutionStatusCompleted ExecutionStatus = "completed"
	ExecutionStatusFailed    ExecutionStatus = "failed"
	ExecutionStatusCancelled ExecutionStatus = "cancelled"
)

// ExecutionError 执行错误
type ExecutionError struct {
	StepID    string    `json:"step_id"`
	Error     string    `json:"error"`
	Timestamp time.Time `json:"timestamp"`
}

// NewEngine 创建ADK引擎
func NewEngine(skillsDir string) *Engine {
	return &Engine{
		skillManager: NewSkillManager(skillsDir),
		agents:       make(map[string]*BaseAgent),
		plans:        make(map[string]*Plan),
		executions:   make(map[string]*Execution),
	}
}

// SetPlanner 设置规划Agent
func (e *Engine) SetPlanner(planner *PlannerAgent) {
	e.planner = planner
}

// RegisterAgent 注册Agent
func (e *Engine) RegisterAgent(agent *BaseAgent) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	name := agent.GetConfig().Name
	if _, exists := e.agents[name]; exists {
		return fmt.Errorf("agent %s already registered", name)
	}

	e.agents[name] = agent
	return nil
}

// GetAgent 获取Agent
func (e *Engine) GetAgent(name string) (*BaseAgent, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	agent, ok := e.agents[name]
	return agent, ok
}

// SetRootAgent 设置根Agent
func (e *Engine) SetRootAgent(agent *BaseAgent) {
	e.rootAgent = agent
}

// ProcessRequest 处理用户请求
func (e *Engine) ProcessRequest(ctx context.Context, goal string, contextData map[string]interface{}) (*Execution, error) {
	// 1. 创建执行计划
	plan, err := e.createPlan(ctx, goal, contextData)
	if err != nil {
		return nil, fmt.Errorf("failed to create plan: %w", err)
	}

	// 2. 执行计划
	execution, err := e.executePlan(ctx, plan)
	if err != nil {
		return nil, fmt.Errorf("failed to execute plan: %w", err)
	}

	return execution, nil
}

// createPlan 创建执行计划
func (e *Engine) createPlan(ctx context.Context, goal string, contextData map[string]interface{}) (*Plan, error) {
	if e.planner == nil {
		return nil, fmt.Errorf("planner not set")
	}

	// 收集可用Agent信息
	availableAgents := make([]AgentInfo, 0)
	for _, agent := range e.agents {
		config := agent.GetConfig()
		availableAgents = append(availableAgents, AgentInfo{
			Name:        config.Name,
			Type:        string(config.Type),
			Description: config.Description,
		})
	}

	// 收集可用技能信息
	availableSkills := make([]SkillInfo, 0)
	for _, skill := range e.skillManager.GetRegistry().List() {
		availableSkills = append(availableSkills, SkillInfo{
			Name:        skill.Name(),
			Description: skill.Description(),
		})
	}

	req := PlanningRequest{
		Goal:            goal,
		Context:         contextData,
		AvailableAgents: availableAgents,
		AvailableSkills: availableSkills,
	}

	plan, err := e.planner.CreatePlan(ctx, req)
	if err != nil {
		return nil, err
	}

	// 验证计划
	agentNames := make([]string, 0, len(e.agents))
	for name := range e.agents {
		agentNames = append(agentNames, name)
	}

	if err := e.planner.ValidatePlan(plan, agentNames); err != nil {
		return nil, fmt.Errorf("plan validation failed: %w", err)
	}

	// 存储计划
	e.mu.Lock()
	e.plans[plan.ID] = plan
	e.mu.Unlock()

	e.emitEvent(&Event{
		Type:      EventTypePlanCreated,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"plan_id": plan.ID,
			"goal":    goal,
			"steps":   len(plan.Steps),
		},
	})

	return plan, nil
}

// executePlan 执行计划
func (e *Engine) executePlan(ctx context.Context, plan *Plan) (*Execution, error) {
	execution := &Execution{
		ID:        fmt.Sprintf("exec_%d", time.Now().UnixNano()),
		PlanID:    plan.ID,
		Status:    ExecutionStatusRunning,
		StartedAt: time.Now(),
		Results:   make(map[string]interface{}),
		Errors:    make([]ExecutionError, 0),
	}

	e.mu.Lock()
	e.executions[execution.ID] = execution
	e.mu.Unlock()

	plan.Status = PlanStatusExecuting

	e.emitEvent(&Event{
		Type:      EventTypePlanStarted,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"execution_id": execution.ID,
			"plan_id":      plan.ID,
		},
	})

	// 执行步骤
	for _, step := range plan.Steps {
		if err := e.executeStep(ctx, execution, plan, step); err != nil {
			// 步骤失败，尝试优化计划
			if e.planner != nil {
				newPlan, refineErr := e.planner.RefinePlan(ctx, plan, step, err.Error())
				if refineErr == nil && newPlan != nil {
					// 使用新计划继续执行
					plan = newPlan
					e.mu.Lock()
					e.plans[plan.ID] = plan
					e.mu.Unlock()
					continue
				}
			}

			// 优化失败或无法优化，记录错误
			execution.Status = ExecutionStatusFailed
			execution.Errors = append(execution.Errors, ExecutionError{
				StepID:    step.ID,
				Error:     err.Error(),
				Timestamp: time.Now(),
			})

			e.emitEvent(&Event{
				Type:      EventTypePlanFailed,
				Timestamp: time.Now(),
				Data: map[string]interface{}{
					"execution_id": execution.ID,
					"plan_id":      plan.ID,
					"failed_step":  step.ID,
					"error":        err.Error(),
				},
			})

			return execution, err
		}
	}

	// 计划执行完成
	now := time.Now()
	execution.Status = ExecutionStatusCompleted
	execution.EndedAt = &now
	plan.Status = PlanStatusCompleted
	plan.CompletedAt = &now

	e.emitEvent(&Event{
		Type:      EventTypePlanCompleted,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"execution_id": execution.ID,
			"plan_id":      plan.ID,
			"results":      execution.Results,
		},
	})

	return execution, nil
}

// executeStep 执行单个步骤
func (e *Engine) executeStep(ctx context.Context, execution *Execution, plan *Plan, step *PlanStep) error {
	step.Status = StepStatusRunning

	e.emitEvent(&Event{
		Type:      EventTypeStepStarted,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"execution_id": execution.ID,
			"step_id":      step.ID,
			"agent":        step.AgentName,
		},
	})

	// 等待依赖完成
	for _, depID := range step.Dependencies {
		depStep := e.findStep(plan, depID)
		if depStep == nil {
			continue
		}

		// 检查依赖结果
		if depStep.Status != StepStatusCompleted {
			return fmt.Errorf("dependency %s not completed", depID)
		}

		// 将依赖结果合并到输入
		if depStep.OutputKey != "" && depStep.Result != nil {
			if step.Input == nil {
				step.Input = make(map[string]interface{})
			}
			step.Input[depStep.OutputKey] = depStep.Result
		}
	}

	// 获取执行Agent
	var targetAgent *BaseAgent
	if step.AgentName != "" {
		agent, ok := e.GetAgent(step.AgentName)
		if !ok {
			return fmt.Errorf("agent %s not found", step.AgentName)
		}
		targetAgent = agent
	} else if e.rootAgent != nil {
		targetAgent = e.rootAgent
	} else {
		return fmt.Errorf("no agent available to execute step")
	}

	e.emitEvent(&Event{
		Type:      EventTypeAgentInvoked,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"execution_id": execution.ID,
			"step_id":      step.ID,
			"agent":        targetAgent.GetConfig().Name,
		},
	})

	// 执行技能（如果有）
	var result map[string]interface{}
	var err error

	if step.SkillName != "" {
		result, err = targetAgent.ExecuteSkill(ctx, step.SkillName, step.Input)

		e.emitEvent(&Event{
			Type:      EventTypeSkillExecuted,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"execution_id": execution.ID,
				"step_id":      step.ID,
				"skill":        step.SkillName,
				"success":      err == nil,
			},
		})
	} else {
		// 直接执行Agent逻辑
		result, err = e.executeAgentLogic(ctx, targetAgent, step.Input)
	}

	if err != nil {
		step.Status = StepStatusFailed
		step.Error = err.Error()

		e.emitEvent(&Event{
			Type:      EventTypeStepFailed,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"execution_id": execution.ID,
				"step_id":      step.ID,
				"error":        err.Error(),
			},
		})

		return err
	}

	step.Status = StepStatusCompleted
	step.Result = result

	// 存储结果
	if step.OutputKey != "" {
		execution.Results[step.OutputKey] = result
	}

	e.emitEvent(&Event{
		Type:      EventTypeStepCompleted,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"execution_id": execution.ID,
			"step_id":      step.ID,
			"output_key":   step.OutputKey,
		},
	})

	return nil
}

// executeAgentLogic 执行Agent逻辑
func (e *Engine) executeAgentLogic(ctx context.Context, agent *BaseAgent, input map[string]interface{}) (map[string]interface{}, error) {
	// 这里可以根据Agent类型执行不同的逻辑
	config := agent.GetConfig()

	switch config.Type {
	case AgentTypePlanner:
		// 规划Agent的特殊处理
		return map[string]interface{}{"status": "planning_completed"}, nil

	case AgentTypeExecutor:
		// 执行Agent的特殊处理
		return map[string]interface{}{"status": "execution_completed"}, nil

	default:
		// 默认处理
		return map[string]interface{}{
			"agent":  config.Name,
			"input":  input,
			"status": "completed",
		}, nil
	}
}

// findStep 查找步骤
func (e *Engine) findStep(plan *Plan, stepID string) *PlanStep {
	for _, step := range plan.Steps {
		if step.ID == stepID {
			return step
		}
	}
	return nil
}

// AddEventHandler 添加事件处理器
func (e *Engine) AddEventHandler(handler EventHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.eventHandlers = append(e.eventHandlers, handler)
}

// emitEvent 触发事件
func (e *Engine) emitEvent(event *Event) {
	e.mu.RLock()
	handlers := make([]EventHandler, len(e.eventHandlers))
	copy(handlers, e.eventHandlers)
	e.mu.RUnlock()

	for _, handler := range handlers {
		go handler(event)
	}
}

// GetExecution 获取执行实例
func (e *Engine) GetExecution(id string) (*Execution, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	exec, ok := e.executions[id]
	return exec, ok
}

// CancelExecution 取消执行
func (e *Engine) CancelExecution(id string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	exec, ok := e.executions[id]
	if !ok || exec.Status != ExecutionStatusRunning {
		return false
	}

	exec.Status = ExecutionStatusCancelled
	now := time.Now()
	exec.EndedAt = &now

	return true
}

// GetSkillManager 获取技能管理器
func (e *Engine) GetSkillManager() *SkillManager {
	return e.skillManager
}

// Initialize 初始化引擎
func (e *Engine) Initialize() error {
	// 加载已安装的技能
	if err := e.skillManager.LoadInstalledSkills(); err != nil {
		log.Printf("Failed to load installed skills: %v", err)
	}

	return nil
}
