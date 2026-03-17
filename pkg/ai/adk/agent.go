package adk

import (
	"context"
	"fmt"
	"sync"

	"google.golang.org/adk/agent"
)

// Skill 技能接口，每个Agent可以拥有的能力
type Skill interface {
	Name() string
	Description() string
	Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error)
}

// AgentType Agent类型
type AgentType string

const (
	AgentTypePlanner     AgentType = "planner"     // 规划Agent
	AgentTypeExecutor    AgentType = "executor"    // 执行Agent
	AgentTypeReviewer    AgentType = "reviewer"    // 审查Agent
	AgentTypeSpecialist  AgentType = "specialist"  // 专家Agent
	AgentTypeCoordinator AgentType = "coordinator" // 协调Agent
)

// AgentConfig Agent配置
type AgentConfig struct {
	Name        string                 `json:"name"`
	Type        AgentType              `json:"type"`
	Description string                 `json:"description"`
	Model       string                 `json:"model"`
	Instruction string                 `json:"instruction"`
	Skills      []string               `json:"skills"` // 技能ID列表
	Metadata    map[string]interface{} `json:"metadata"`
}

// BaseAgent 基础Agent结构
type BaseAgent struct {
	config    AgentConfig
	skills    map[string]Skill
	adkAgent  agent.Agent
	parent    *BaseAgent
	subAgents []*BaseAgent
	mu        sync.RWMutex
	state     map[string]interface{}
}

// NewBaseAgent 创建基础Agent
func NewBaseAgent(config AgentConfig) *BaseAgent {
	return &BaseAgent{
		config:    config,
		skills:    make(map[string]Skill),
		subAgents: make([]*BaseAgent, 0),
		state:     make(map[string]interface{}),
	}
}

// RegisterSkill 注册技能
func (a *BaseAgent) RegisterSkill(skill Skill) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.skills[skill.Name()] = skill
}

// GetSkill 获取技能
func (a *BaseAgent) GetSkill(name string) (Skill, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	skill, ok := a.skills[name]
	return skill, ok
}

// AddSubAgent 添加子Agent
func (a *BaseAgent) AddSubAgent(sub *BaseAgent) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// 检查是否已存在
	for _, existing := range a.subAgents {
		if existing.config.Name == sub.config.Name {
			return fmt.Errorf("sub-agent %s already exists", sub.config.Name)
		}
	}

	sub.parent = a
	a.subAgents = append(a.subAgents, sub)
	return nil
}

// RemoveSubAgent 移除子Agent
func (a *BaseAgent) RemoveSubAgent(name string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	for i, sub := range a.subAgents {
		if sub.config.Name == name {
			a.subAgents = append(a.subAgents[:i], a.subAgents[i+1:]...)
			return true
		}
	}
	return false
}

// FindAgent 递归查找Agent
func (a *BaseAgent) FindAgent(name string) *BaseAgent {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.config.Name == name {
		return a
	}

	for _, sub := range a.subAgents {
		if found := sub.FindAgent(name); found != nil {
			return found
		}
	}
	return nil
}

// GetConfig 获取配置
func (a *BaseAgent) GetConfig() AgentConfig {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.config
}

// GetSubAgents 获取所有子Agent
func (a *BaseAgent) GetSubAgents() []*BaseAgent {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make([]*BaseAgent, len(a.subAgents))
	copy(result, a.subAgents)
	return result
}

// SetState 设置状态
func (a *BaseAgent) SetState(key string, value interface{}) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state[key] = value
}

// GetState 获取状态
func (a *BaseAgent) GetState(key string) (interface{}, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	val, ok := a.state[key]
	return val, ok
}

// ExecuteSkill 执行技能
func (a *BaseAgent) ExecuteSkill(ctx context.Context, skillName string, input map[string]interface{}) (map[string]interface{}, error) {
	skill, ok := a.GetSkill(skillName)
	if !ok {
		return nil, fmt.Errorf("skill %s not found", skillName)
	}
	return skill.Execute(ctx, input)
}
