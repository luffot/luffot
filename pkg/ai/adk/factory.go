package adk

import (
	"fmt"

	"google.golang.org/adk/model"
)

// AgentFactory Agent工厂
type AgentFactory struct {
	engine        *Engine
	model         model.LLM
	defaultConfig AgentConfig
}

// NewAgentFactory 创建Agent工厂
func NewAgentFactory(engine *Engine, llm model.LLM) *AgentFactory {
	return &AgentFactory{
		engine:        engine,
		model:         llm,
		defaultConfig: AgentConfig{},
	}
}

// SetDefaultModel 设置默认模型
func (f *AgentFactory) SetDefaultModel(llm model.LLM) {
	f.model = llm
}

// CreatePlanner 创建规划Agent
func (f *AgentFactory) CreatePlanner(name string, instruction string) (*PlannerAgent, error) {
	config := AgentConfig{
		Name:        name,
		Type:        AgentTypePlanner,
		Description: "负责任务规划和策略制定",
		Instruction: instruction,
	}

	planner, err := NewPlannerAgent(config, f.model)
	if err != nil {
		return nil, fmt.Errorf("failed to create planner: %w", err)
	}

	if err := f.engine.RegisterAgent(planner.BaseAgent); err != nil {
		return nil, err
	}

	return planner, nil
}

// CreateExecutor 创建执行Agent
func (f *AgentFactory) CreateExecutor(name string, skills []string) (*BaseAgent, error) {
	config := AgentConfig{
		Name:        name,
		Type:        AgentTypeExecutor,
		Description: "负责任务执行",
		Instruction: "你是一个执行Agent，负责高效、准确地完成分配给你的任务。",
		Skills:      skills,
	}

	agent := NewBaseAgent(config)

	// 注册技能
	for _, skillName := range skills {
		if skill, ok := f.engine.GetSkillManager().GetRegistry().Get(skillName); ok {
			agent.RegisterSkill(skill)
		}
	}

	if err := f.engine.RegisterAgent(agent); err != nil {
		return nil, err
	}

	return agent, nil
}

// CreateReviewer 创建审查Agent
func (f *AgentFactory) CreateReviewer(name string) (*BaseAgent, error) {
	config := AgentConfig{
		Name:        name,
		Type:        AgentTypeReviewer,
		Description: "负责结果审查和质量控制",
		Instruction: `你是一个审查Agent，负责审查工作成果的质量。

审查标准:
1. 准确性: 内容是否正确无误
2. 完整性: 是否涵盖了所有要求
3. 一致性: 内部逻辑是否一致
4. 合规性: 是否符合规范和要求

输出格式:
{
  "passed": true/false,
  "score": 0-100,
  "feedback": "详细反馈",
  "issues": ["问题1", "问题2"]
}`,
	}

	agent := NewBaseAgent(config)

	if err := f.engine.RegisterAgent(agent); err != nil {
		return nil, err
	}

	return agent, nil
}

// CreateSpecialist 创建专家Agent
func (f *AgentFactory) CreateSpecialist(name string, specialty string, instruction string) (*BaseAgent, error) {
	config := AgentConfig{
		Name:        name,
		Type:        AgentTypeSpecialist,
		Description: fmt.Sprintf("%s领域专家", specialty),
		Instruction: instruction,
		Metadata: map[string]interface{}{
			"specialty": specialty,
		},
	}

	agent := NewBaseAgent(config)

	if err := f.engine.RegisterAgent(agent); err != nil {
		return nil, err
	}

	return agent, nil
}

// CreateCoordinator 创建协调Agent
func (f *AgentFactory) CreateCoordinator(name string) (*CoordinatorAgent, error) {
	config := AgentConfig{
		Name:        name,
		Type:        AgentTypeCoordinator,
		Description: "负责任务协调和Agent调度",
		Instruction: "",
	}

	coordinator, err := NewCoordinatorAgent(config, f.engine, f.model)
	if err != nil {
		return nil, fmt.Errorf("failed to create coordinator: %w", err)
	}

	if err := f.engine.RegisterAgent(coordinator.BaseAgent); err != nil {
		return nil, err
	}

	// 设置为根Agent
	f.engine.SetRootAgent(coordinator.BaseAgent)

	return coordinator, nil
}

// CreateDefaultTeam 创建默认Agent团队
func (f *AgentFactory) CreateDefaultTeam() error {
	// 1. 创建协调Agent
	coordinator, err := f.CreateCoordinator("coordinator")
	if err != nil {
		return fmt.Errorf("failed to create coordinator: %w", err)
	}

	// 2. 创建规划Agent
	planner, err := f.CreatePlanner("planner", `你是一个智能规划Agent。

你的职责:
1. 分析复杂任务，理解目标和约束
2. 将任务分解为可执行的步骤
3. 为每个步骤分配合适的Agent和技能
4. 识别步骤间的依赖关系
5. 生成详细的执行计划

输出格式:
{
  "steps": [
    {
      "id": "step_1",
      "description": "步骤描述",
      "agent_name": "执行Agent名称",
      "skill_name": "技能名称(可选)",
      "dependencies": ["依赖步骤id"],
      "input": {"参数": "值"},
      "output_key": "结果存储key"
    }
  ]
}`)
	if err != nil {
		return fmt.Errorf("failed to create planner: %w", err)
	}

	f.engine.SetPlanner(planner)

	// 3. 创建执行Agent
	executor, err := f.CreateExecutor("executor", []string{})
	if err != nil {
		return fmt.Errorf("failed to create executor: %w", err)
	}

	// 4. 创建审查Agent
	reviewer, err := f.CreateReviewer("reviewer")
	if err != nil {
		return fmt.Errorf("failed to create reviewer: %w", err)
	}

	// 5. 注册为协调Agent的子Agent
	coordinator.RegisterSubAgent(planner.BaseAgent)
	coordinator.RegisterSubAgent(executor)
	coordinator.RegisterSubAgent(reviewer)

	return nil
}

// CreateCustomAgent 创建自定义Agent
func (f *AgentFactory) CreateCustomAgent(config AgentConfig) (*BaseAgent, error) {
	agent := NewBaseAgent(config)

	if err := f.engine.RegisterAgent(agent); err != nil {
		return nil, err
	}

	return agent, nil
}
