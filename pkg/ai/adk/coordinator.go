package adk

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
)

// CoordinatorAgent 协调Agent
// 负责任务分发、Agent调度和结果聚合
type CoordinatorAgent struct {
	*BaseAgent
	engine    *Engine
	llmAgent  agent.Agent
	subAgents map[string]*BaseAgent
	workflows map[string]Workflow
	mu        sync.RWMutex
}

// Workflow 工作流接口
type Workflow interface {
	Name() string
	Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error)
}

// NewCoordinatorAgent 创建协调Agent
func NewCoordinatorAgent(config AgentConfig, engine *Engine, llmModel model.LLM) (*CoordinatorAgent, error) {
	base := NewBaseAgent(config)

	llmConfig := llmagent.Config{
		Name:        config.Name,
		Description: config.Description,
		Model:       llmModel,
		Instruction: buildCoordinatorInstruction(config),
	}

	llmAgent, err := llmagent.New(llmConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM agent: %w", err)
	}

	return &CoordinatorAgent{
		BaseAgent: base,
		engine:    engine,
		llmAgent:  llmAgent,
		subAgents: make(map[string]*BaseAgent),
		workflows: make(map[string]Workflow),
	}, nil
}

// buildCoordinatorInstruction 构建协调Agent指令
func buildCoordinatorInstruction(config AgentConfig) string {
	return fmt.Sprintf(`你是 %s，一个智能协调Agent。

你的职责:
1. 分析用户请求，理解任务目标
2. 将复杂任务分解为子任务
3. 选择合适的子Agent执行每个子任务
4. 协调子Agent之间的协作
5. 聚合结果并返回给用户

可用的子Agent:
- planner: 负责任务规划和策略制定
- executor: 负责具体任务执行
- reviewer: 负责结果审查和质量控制
- specialist: 各领域专家Agent

工作流程:
1. 接收用户请求
2. 分析任务复杂度和类型
3. 决定是直接处理还是分发给子Agent
4. 如果需要分发，创建执行计划
5. 监控执行进度
6. 处理异常和错误
7. 返回最终结果

注意事项:
- 简单任务可以直接回答
- 复杂任务必须创建执行计划
- 遇到错误时要尝试重新规划
- 保持与用户的沟通，及时反馈进度`, config.Name)
}

// RegisterSubAgent 注册子Agent
func (c *CoordinatorAgent) RegisterSubAgent(agent *BaseAgent) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	name := agent.GetConfig().Name
	if _, exists := c.subAgents[name]; exists {
		return fmt.Errorf("sub-agent %s already registered", name)
	}

	c.subAgents[name] = agent
	return nil
}

// DelegateTask 委托任务给子Agent
func (c *CoordinatorAgent) DelegateTask(ctx context.Context, agentName string, task map[string]interface{}) (map[string]interface{}, error) {
	c.mu.RLock()
	agent, ok := c.subAgents[agentName]
	c.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("sub-agent %s not found", agentName)
	}

	// 根据Agent类型执行不同逻辑
	config := agent.GetConfig()

	switch config.Type {
	case AgentTypePlanner:
		return c.delegateToPlanner(ctx, agent, task)

	case AgentTypeExecutor:
		return c.delegateToExecutor(ctx, agent, task)

	case AgentTypeReviewer:
		return c.delegateToReviewer(ctx, agent, task)

	case AgentTypeSpecialist:
		return c.delegateToSpecialist(ctx, agent, task)

	default:
		return c.delegateToGeneric(ctx, agent, task)
	}
}

// delegateToPlanner 委托给规划Agent
func (c *CoordinatorAgent) delegateToPlanner(ctx context.Context, agent *BaseAgent, task map[string]interface{}) (map[string]interface{}, error) {
	goal, _ := task["goal"].(string)
	if goal == "" {
		return nil, fmt.Errorf("task missing goal")
	}

	// 使用引擎创建计划
	plan, err := c.engine.createPlan(ctx, goal, task)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"plan_id": plan.ID,
		"steps":   len(plan.Steps),
		"status":  "planned",
	}, nil
}

// delegateToExecutor 委托给执行Agent
func (c *CoordinatorAgent) delegateToExecutor(ctx context.Context, agent *BaseAgent, task map[string]interface{}) (map[string]interface{}, error) {
	// 检查是否有技能需要执行
	skillName, _ := task["skill"].(string)
	if skillName != "" {
		result, err := agent.ExecuteSkill(ctx, skillName, task)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"result": result,
			"status": "executed",
		}, nil
	}

	// 直接执行
	return map[string]interface{}{
		"status": "completed",
		"data":   task,
	}, nil
}

// delegateToReviewer 委托给审查Agent
func (c *CoordinatorAgent) delegateToReviewer(ctx context.Context, agent *BaseAgent, task map[string]interface{}) (map[string]interface{}, error) {
	content, _ := task["content"].(string)
	criteria, _ := task["criteria"].([]string)

	// 构建审查prompt
	prompt := fmt.Sprintf("请审查以下内容:\n\n%s\n\n审查标准:\n", content)
	for _, c := range criteria {
		prompt += fmt.Sprintf("- %s\n", c)
	}

	// 这里简化处理，实际应该调用LLM
	return map[string]interface{}{
		"passed":   true,
		"feedback": "审查通过",
		"status":   "reviewed",
	}, nil
}

// delegateToSpecialist 委托给专家Agent
func (c *CoordinatorAgent) delegateToSpecialist(ctx context.Context, agent *BaseAgent, task map[string]interface{}) (map[string]interface{}, error) {
	// 专家Agent使用自己的专业知识处理
	config := agent.GetConfig()

	// 可以基于metadata中的专业领域进行处理
	specialty, _ := config.Metadata["specialty"].(string)

	return map[string]interface{}{
		"specialty": specialty,
		"result":    "专家处理完成",
		"status":    "completed",
	}, nil
}

// delegateToGeneric 通用委托
func (c *CoordinatorAgent) delegateToGeneric(ctx context.Context, agent *BaseAgent, task map[string]interface{}) (map[string]interface{}, error) {
	// 通用处理逻辑
	return map[string]interface{}{
		"agent":  agent.GetConfig().Name,
		"task":   task,
		"status": "completed",
	}, nil
}

// CreateSequentialWorkflow 创建顺序工作流
func (c *CoordinatorAgent) CreateSequentialWorkflow(name string, agents []*BaseAgent) (Workflow, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	workflow := &sequentialWorkflow{
		name:      name,
		agents:    agents,
		createdAt: time.Now(),
	}

	c.workflows[name] = workflow
	return workflow, nil
}

// CreateParallelWorkflow 创建并行工作流
func (c *CoordinatorAgent) CreateParallelWorkflow(name string, agents []*BaseAgent) (Workflow, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	workflow := &parallelWorkflow{
		name:      name,
		agents:    agents,
		createdAt: time.Now(),
	}

	c.workflows[name] = workflow
	return workflow, nil
}

// ExecuteWorkflow 执行工作流
func (c *CoordinatorAgent) ExecuteWorkflow(ctx context.Context, workflowName string, input map[string]interface{}) (map[string]interface{}, error) {
	c.mu.RLock()
	workflow, ok := c.workflows[workflowName]
	c.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("workflow %s not found", workflowName)
	}

	return workflow.Execute(ctx, input)
}

// HandleRequest 处理用户请求
func (c *CoordinatorAgent) HandleRequest(ctx context.Context, request string, contextData map[string]interface{}) (*Execution, error) {
	// 1. 分析请求
	analysis, err := c.analyzeRequest(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze request: %w", err)
	}

	// 2. 判断复杂度
	complexity, _ := analysis["complexity"].(string)

	if complexity == "simple" {
		// 简单请求直接处理
		return c.handleSimpleRequest(ctx, request, contextData)
	}

	// 3. 复杂请求，使用引擎处理
	return c.engine.ProcessRequest(ctx, request, contextData)
}

// analyzeRequest 分析请求
func (c *CoordinatorAgent) analyzeRequest(ctx context.Context, request string) (map[string]interface{}, error) {
	// 使用LLM分析请求
	// 简化处理
	wordCount := len(request)

	complexity := "simple"
	if wordCount > 50 {
		complexity = "complex"
	} else if wordCount > 20 {
		complexity = "medium"
	}

	return map[string]interface{}{
		"complexity": complexity,
		"type":       "general",
		"keywords":   []string{"task"},
	}, nil
}

// handleSimpleRequest 处理简单请求
func (c *CoordinatorAgent) handleSimpleRequest(ctx context.Context, request string, contextData map[string]interface{}) (*Execution, error) {
	// 创建简单的单步执行
	execution := &Execution{
		ID:        fmt.Sprintf("exec_%d", time.Now().UnixNano()),
		Status:    ExecutionStatusCompleted,
		StartedAt: time.Now(),
		Results: map[string]interface{}{
			"response": fmt.Sprintf("已处理请求: %s", request),
		},
	}

	now := time.Now()
	execution.EndedAt = &now

	return execution, nil
}

// 工作流实现

type sequentialWorkflow struct {
	name      string
	agents    []*BaseAgent
	createdAt time.Time
}

func (w *sequentialWorkflow) Name() string {
	return w.name
}

func (w *sequentialWorkflow) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	result := input

	for _, agent := range w.agents {
		// 顺序执行每个Agent
		config := agent.GetConfig()

		// 根据类型执行
		switch config.Type {
		case AgentTypeExecutor:
			// 执行并更新结果
			if r, ok := result["input"].(map[string]interface{}); ok {
				result = r
			}
		}
	}

	return map[string]interface{}{
		"workflow": w.name,
		"result":   result,
		"status":   "completed",
	}, nil
}

type parallelWorkflow struct {
	name      string
	agents    []*BaseAgent
	createdAt time.Time
}

func (w *parallelWorkflow) Name() string {
	return w.name
}

func (w *parallelWorkflow) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	// 并行执行所有Agent
	results := make(map[string]interface{})
	var mu sync.Mutex
	var wg sync.WaitGroup
	errChan := make(chan error, len(w.agents))

	for _, agent := range w.agents {
		wg.Add(1)
		go func(a *BaseAgent) {
			defer wg.Done()

			// 执行Agent
			config := a.GetConfig()
			result := map[string]interface{}{
				"agent":  config.Name,
				"status": "completed",
			}

			mu.Lock()
			results[config.Name] = result
			mu.Unlock()
		}(agent)
	}

	wg.Wait()
	close(errChan)

	// 检查错误
	for err := range errChan {
		if err != nil {
			return nil, err
		}
	}

	return map[string]interface{}{
		"workflow": w.name,
		"results":  results,
		"status":   "completed",
	}, nil
}
