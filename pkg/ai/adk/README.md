# Luffot ADK - Agent Development Kit

基于 Google ADK 的多Agent智能系统，为 Luffot 桌面宠物提供强大的AI能力。

## 架构概览

```
┌─────────────────────────────────────────────────────────┐
│                    Coordinator Agent                     │
│              (任务协调 & Agent调度)                       │
└─────────────┬───────────────────────────┬───────────────┘
              │                           │
    ┌─────────▼─────────┐       ┌─────────▼─────────┐
    │   Planner Agent   │       │  Reviewer Agent   │
    │   (任务规划)       │       │   (结果审查)       │
    └─────────┬─────────┘       └───────────────────┘
              │
    ┌─────────▼─────────┐
    │  Executor Agent   │
    │   (任务执行)       │
    │  + Skill System   │
    └───────────────────┘
```

## 核心组件

### 1. Agent 类型

| Agent类型 | 职责 | 说明 |
|-----------|------|------|
| **Coordinator** | 任务协调 | 接收用户请求，分发给子Agent |
| **Planner** | 任务规划 | 将复杂任务分解为执行步骤 |
| **Executor** | 任务执行 | 执行具体任务，调用技能 |
| **Reviewer** | 结果审查 | 审查输出质量 |
| **Specialist** | 领域专家 | 特定领域的专业Agent |

### 2. 技能系统 (Skills)

支持多种技能类型：

- **Builtin**: Go代码实现的内置技能
- **HTTP**: 调用外部HTTP API
- **Python**: 执行Python脚本
- **Lua**: 执行Lua脚本 (使用 gopher-lua)

#### 技能安装

```go
// 从URL安装
manager.InstallSkill("https://example.com/skills/my-skill.zip")

// 从本地目录安装
manager.InstallSkill("/path/to/skill")

// 从压缩包安装
manager.InstallSkill("/path/to/skill.tar.gz")
```

#### 技能配置 (skill.yaml)

```yaml
name: my-skill
version: 1.0.0
description: 技能描述
type: lua  # builtin, http, python, lua
entry: main.lua

parameters:
  - name: input
    type: string
    required: true

returns:
  - name: result
    type: string
```

### 3. 执行引擎 (Engine)

负责任务的完整生命周期管理：

1. **接收请求** → 2. **创建计划** → 3. **执行步骤** → 4. **审查结果** → 5. **返回输出**

## 快速开始

### 初始化 ADK 系统

```go
package main

import (
    "context"
    "github.com/luffot/luffot/pkg/ai/adk"
)

func main() {
    // 1. 创建引擎
    engine := adk.NewEngine(adk.EngineConfig{
        ModelName: "gemini-2.0-flash",
    })
    
    // 2. 创建Agent工厂
    factory := adk.NewAgentFactory(engine, engine.GetModel())
    
    // 3. 创建默认Agent团队
    if err := factory.CreateDefaultTeam(); err != nil {
        panic(err)
    }
    
    // 4. 处理请求
    ctx := context.Background()
    execution, err := engine.ProcessRequest(ctx, "帮我规划今天的任务", nil)
    if err != nil {
        panic(err)
    }
    
    // 5. 获取结果
    fmt.Printf("执行状态: %s\n", execution.Status)
    fmt.Printf("结果: %v\n", execution.Results)
}
```

### 创建自定义Agent

```go
// 使用工厂创建
config := adk.AgentConfig{
    Name:        "my-agent",
    Type:        adk.AgentTypeSpecialist,
    Description: "我的自定义Agent",
    Instruction: "你是专业的...",
    Model:       "gemini-2.0-flash",
}

agent, err := factory.CreateCustomAgent(config)
```

### 注册技能

```go
// 内置技能
skill := &adk.BuiltinSkill{
    Name: "hello",
    ExecuteFunc: func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
        return map[string]interface{}{
            "message": "Hello, World!",
        }, nil
    },
}

agent.RegisterSkill(skill)
```

## 配置说明

### 环境变量

| 变量名 | 说明 | 必需 |
|--------|------|------|
| `GEMINI_API_KEY` | Google Gemini API密钥 | 是 |
| `ADK_LOG_LEVEL` | 日志级别 (debug/info/warn/error) | 否 |
| `ADK_SKILLS_DIR` | 技能目录路径 | 否 |

### 配置文件

参考 `example/agent_config.yaml` 了解完整配置选项。

## 示例工作流

### 代码审查流程

```go
// 创建顺序工作流
workflow, err := coordinator.CreateSequentialWorkflow("code-review", []*adk.BaseAgent{
    planner.BaseAgent,
    codeExpert,
    reviewer,
})

// 执行工作流
result, err := coordinator.ExecuteWorkflow(ctx, "code-review", map[string]interface{}{
    "code": sourceCode,
    "language": "go",
})
```

### 并行任务处理

```go
// 创建并行工作流
workflow, err := coordinator.CreateParallelWorkflow("parallel-tasks", []*adk.BaseAgent{
    agent1,
    agent2,
    agent3,
})
```

## 开发指南

### 添加新技能类型

1. 实现 `Skill` 接口
2. 在 `SkillManager` 中添加加载逻辑
3. 更新配置解析

### 扩展Agent功能

1. 继承 `BaseAgent`
2. 重写需要自定义的方法
3. 注册到引擎

## API 参考

### Engine

- `NewEngine(config EngineConfig) *Engine` - 创建引擎
- `ProcessRequest(ctx, request, context) (*Execution, error)` - 处理请求
- `RegisterAgent(agent *BaseAgent) error` - 注册Agent
- `GetSkillManager() *SkillManager` - 获取技能管理器

### AgentFactory

- `NewAgentFactory(engine, model) *AgentFactory` - 创建工厂
- `CreatePlanner(name, instruction) (*PlannerAgent, error)` - 创建规划Agent
- `CreateExecutor(name, skills) (*BaseAgent, error)` - 创建执行Agent
- `CreateReviewer(name) (*BaseAgent, error)` - 创建审查Agent
- `CreateCoordinator(name) (*CoordinatorAgent, error)` - 创建协调Agent
- `CreateDefaultTeam() error` - 创建默认团队

### SkillManager

- `InstallSkill(source string) error` - 安装技能
- `UninstallSkill(name string) error` - 卸载技能
- `LoadInstalledSkills() error` - 加载已安装技能
- `GetRegistry() *SkillRegistry` - 获取注册表

## 注意事项

1. **API密钥安全**: 不要将API密钥硬编码，使用环境变量
2. **错误处理**: 所有Agent方法都可能返回错误，需要正确处理
3. **上下文管理**: 使用 `context.Context` 控制超时和取消
4. **资源清理**: 程序退出前调用引擎的清理方法

## 许可证

MIT License - 详见项目根目录 LICENSE 文件
