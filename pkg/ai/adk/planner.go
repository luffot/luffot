package adk

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// PlanStep 计划步骤
type PlanStep struct {
	ID           string                 `json:"id"`
	Description  string                 `json:"description"`
	AgentName    string                 `json:"agent_name"`   // 负责执行的Agent
	SkillName    string                 `json:"skill_name"`   // 需要使用的技能
	Dependencies []string               `json:"dependencies"` // 依赖的步骤ID
	Input        map[string]interface{} `json:"input"`
	OutputKey    string                 `json:"output_key"` // 输出存储的key
	Status       StepStatus             `json:"status"`
	Result       map[string]interface{} `json:"result"`
	Error        string                 `json:"error"`
}

// StepStatus 步骤状态
type StepStatus string

const (
	StepStatusPending   StepStatus = "pending"
	StepStatusRunning   StepStatus = "running"
	StepStatusCompleted StepStatus = "completed"
	StepStatusFailed    StepStatus = "failed"
	StepStatusSkipped   StepStatus = "skipped"
)

// Plan 执行计划
type Plan struct {
	ID          string      `json:"id"`
	Goal        string      `json:"goal"`
	Steps       []*PlanStep `json:"steps"`
	CreatedAt   time.Time   `json:"created_at"`
	CompletedAt *time.Time  `json:"completed_at"`
	Status      PlanStatus  `json:"status"`
}

// PlanStatus 计划状态
type PlanStatus string

const (
	PlanStatusDraft     PlanStatus = "draft"
	PlanStatusExecuting PlanStatus = "executing"
	PlanStatusCompleted PlanStatus = "completed"
	PlanStatusFailed    PlanStatus = "failed"
)

// PlannerAgent 智能规划Agent
type PlannerAgent struct {
	*BaseAgent
	llmAgent agent.Agent // LLM Agent 实例
	model    model.LLM   // LLM 模型实例
}

// NewPlannerAgent 创建规划Agent
func NewPlannerAgent(config AgentConfig, llmModel model.LLM) (*PlannerAgent, error) {
	base := NewBaseAgent(config)

	llmConfig := llmagent.Config{
		Name:        config.Name,
		Description: config.Description,
		Model:       llmModel,
		Instruction: config.Instruction,
	}

	llmAgent, err := llmagent.New(llmConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM agent: %w", err)
	}

	return &PlannerAgent{
		BaseAgent: base,
		llmAgent:  llmAgent,
		model:     llmModel,
	}, nil
}

// PlanningRequest 规划请求
type PlanningRequest struct {
	Goal            string                 `json:"goal"`
	Context         map[string]interface{} `json:"context"`
	Constraints     []string               `json:"constraints"`
	AvailableAgents []AgentInfo            `json:"available_agents"`
	AvailableSkills []SkillInfo            `json:"available_skills"`
}

// AgentInfo Agent信息
type AgentInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// SkillInfo 技能信息
type SkillInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// CreatePlan 创建执行计划
func (p *PlannerAgent) CreatePlan(ctx context.Context, req PlanningRequest) (*Plan, error) {
	// 构建规划prompt
	prompt := p.buildPlanningPrompt(req)

	// 调用LLM生成计划
	response, err := p.callLLM(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate plan: %w", err)
	}

	// 解析计划
	plan, err := p.parsePlan(response, req.Goal)
	if err != nil {
		return nil, fmt.Errorf("failed to parse plan: %w", err)
	}

	return plan, nil
}

// buildPlanningPrompt 构建规划prompt
func (p *PlannerAgent) buildPlanningPrompt(req PlanningRequest) string {
	var sb strings.Builder

	sb.WriteString("你是一个智能任务规划助手。请根据以下信息创建一个详细的执行计划。\n\n")
	sb.WriteString(fmt.Sprintf("目标: %s\n\n", req.Goal))

	if len(req.Constraints) > 0 {
		sb.WriteString("约束条件:\n")
		for _, c := range req.Constraints {
			sb.WriteString(fmt.Sprintf("- %s\n", c))
		}
		sb.WriteString("\n")
	}

	if len(req.AvailableAgents) > 0 {
		sb.WriteString("可用的Agent:\n")
		for _, agent := range req.AvailableAgents {
			sb.WriteString(fmt.Sprintf("- %s (%s): %s\n", agent.Name, agent.Type, agent.Description))
		}
		sb.WriteString("\n")
	}

	if len(req.AvailableSkills) > 0 {
		sb.WriteString("可用的技能:\n")
		for _, skill := range req.AvailableSkills {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", skill.Name, skill.Description))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("请按以下JSON格式输出执行计划:\n")
	sb.WriteString(`{
  "steps": [
    {
      "id": "step_1",
      "description": "步骤描述",
      "agent_name": "负责执行的agent名称",
      "skill_name": "需要使用的技能名称(可选)",
      "dependencies": ["依赖的步骤id"],
      "input": {"key": "value"},
      "output_key": "结果存储的key"
    }
  ]
}`)

	return sb.String()
}

// callLLM 调用LLM生成计划
func (p *PlannerAgent) callLLM(ctx context.Context, prompt string) (string, error) {
	// 创建LLM请求
	contents := []*genai.Content{
		genai.NewContentFromText(prompt, genai.RoleUser),
	}

	req := &model.LLMRequest{
		Contents: contents,
		Config: &genai.GenerateContentConfig{
			Temperature: genai.Ptr(float32(0.7)),
		},
	}

	// 调用模型
	var responseText string
	for result, err := range p.model.GenerateContent(ctx, req, false) {
		if err != nil {
			return "", fmt.Errorf("LLM generation failed: %w", err)
		}

		if result.Content != nil && len(result.Content.Parts) > 0 {
			for _, part := range result.Content.Parts {
				if part.Text != "" {
					responseText += part.Text
				}
			}
		}
	}

	if responseText == "" {
		return "", fmt.Errorf("LLM returned empty response")
	}

	return responseText, nil
}

// parsePlan 解析计划
func (p *PlannerAgent) parsePlan(response string, goal string) (*Plan, error) {
	// 提取JSON部分
	jsonStart := strings.Index(response, "{")
	jsonEnd := strings.LastIndex(response, "}")
	if jsonStart == -1 || jsonEnd == -1 || jsonEnd <= jsonStart {
		return nil, fmt.Errorf("no valid JSON found in response")
	}

	jsonStr := response[jsonStart : jsonEnd+1]

	var planData struct {
		Steps []*PlanStep `json:"steps"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &planData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal plan: %w", err)
	}

	// 初始化步骤状态
	for _, step := range planData.Steps {
		step.Status = StepStatusPending
	}

	plan := &Plan{
		ID:        fmt.Sprintf("plan_%d", time.Now().UnixNano()),
		Goal:      goal,
		Steps:     planData.Steps,
		CreatedAt: time.Now(),
		Status:    PlanStatusDraft,
	}

	return plan, nil
}

// RefinePlan 根据执行结果优化计划
func (p *PlannerAgent) RefinePlan(ctx context.Context, plan *Plan, failedStep *PlanStep, errorMsg string) (*Plan, error) {
	// 构建优化prompt
	prompt := fmt.Sprintf(`
当前计划执行失败，请优化计划。

目标: %s
失败步骤: %s
失败原因: %s

当前计划:
%s

请提供优化后的计划，可以:
1. 调整步骤顺序
2. 更换执行Agent
3. 分解复杂步骤
4. 添加错误处理步骤

请按相同JSON格式输出优化后的计划。
`, plan.Goal, failedStep.Description, errorMsg, p.planToJSON(plan))

	response, err := p.callLLM(ctx, prompt)
	if err != nil {
		return nil, err
	}

	return p.parsePlan(response, plan.Goal)
}

// planToJSON 将计划转为JSON字符串
func (p *PlannerAgent) planToJSON(plan *Plan) string {
	data, _ := json.MarshalIndent(plan, "", "  ")
	return string(data)
}

// ValidatePlan 验证计划可行性
func (p *PlannerAgent) ValidatePlan(plan *Plan, availableAgents []string) error {
	agentSet := make(map[string]bool)
	for _, name := range availableAgents {
		agentSet[name] = true
	}

	for _, step := range plan.Steps {
		// 检查Agent是否存在
		if step.AgentName != "" && !agentSet[step.AgentName] {
			return fmt.Errorf("step %s references unknown agent: %s", step.ID, step.AgentName)
		}

		// 检查依赖是否存在
		for _, dep := range step.Dependencies {
			found := false
			for _, s := range plan.Steps {
				if s.ID == dep {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("step %s has unknown dependency: %s", step.ID, dep)
			}
		}
	}

	return nil
}
