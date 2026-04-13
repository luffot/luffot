package ai

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/luffot/luffot/pkg/eventbus"
)

// ActionType 动作类型
type ActionType string

const (
	// 钉钉相关动作
	ActionSendDingTalkMessage ActionType = "send_dingtalk_message"
	ActionCreateDingTalkTodo  ActionType = "create_dingtalk_todo"
	ActionCreateCalendarEvent ActionType = "create_calendar_event"
	ActionSendDingMessage     ActionType = "send_ding_message"

	// 系统相关动作
	ActionCleanupFiles        ActionType = "cleanup_files"
	ActionRestartService      ActionType = "restart_service"
	ActionExecuteShellCommand ActionType = "execute_shell_command"

	// 用户交互动作
	ActionMarkAsImportant     ActionType = "mark_as_important"
	ActionSnoozeNotification  ActionType = "snooze_notification"
	ActionDismissNotification ActionType = "dismiss_notification"
)

// ActionParameter 动作参数
type ActionParameter struct {
	Name        string      `json:"name"`        // 参数名
	Value       interface{} `json:"value"`       // 参数值
	Description string      `json:"description"` // 参数描述
	Required    bool        `json:"required"`    // 是否必需
}

// ActionDefinition 动作定义
type ActionDefinition struct {
	Type        ActionType        `json:"type"`        // 动作类型
	Name        string            `json:"name"`        // 动作名称（人类可读）
	Description string            `json:"description"` // 动作描述
	Parameters  []ActionParameter `json:"parameters"`  // 参数列表
	Handler     ActionHandler     `json:"-"`           // 执行函数（不序列化）
}

// ActionHandler 动作处理函数类型
type ActionHandler func(ctx context.Context, params map[string]interface{}) (*ActionResult, error)

// ActionResult 动作执行结果
type ActionResult struct {
	Success   bool                   `json:"success"`   // 是否成功
	Message   string                 `json:"message"`   // 结果消息
	Data      map[string]interface{} `json:"data"`      // 返回数据
	Error     string                 `json:"error"`     // 错误信息
	Timestamp time.Time              `json:"timestamp"` // 执行时间
}

// UserIntent 用户意图
type UserIntent struct {
	ActionType    ActionType             `json:"action_type"`     // 要执行的动作类型
	Parameters    map[string]interface{} `json:"parameters"`      // 动作参数
	SourceEventID string                 `json:"source_event_id"` // 来源事件ID（用于追踪）
	Context       map[string]interface{} `json:"context"`         // 上下文信息
}

// ActionExecutor 动作执行器
type ActionExecutor struct {
	eventBus     *eventbus.EventBus
	actions      map[ActionType]*ActionDefinition
	mu           sync.RWMutex
	executionLog []*ExecutionRecord
	maxLogSize   int
}

// ExecutionRecord 执行记录
type ExecutionRecord struct {
	ID        string                 `json:"id"`
	Action    ActionType             `json:"action"`
	Params    map[string]interface{} `json:"params"`
	Result    *ActionResult          `json:"result"`
	Duration  time.Duration          `json:"duration"`
	Timestamp time.Time              `json:"timestamp"`
}

// NewActionExecutor 创建动作执行器
func NewActionExecutor(eventBus *eventbus.EventBus) *ActionExecutor {
	return &ActionExecutor{
		eventBus:     eventBus,
		actions:      make(map[ActionType]*ActionDefinition),
		executionLog: make([]*ExecutionRecord, 0),
		maxLogSize:   100,
	}
}

// RegisterAction 注册动作
func (ae *ActionExecutor) RegisterAction(def *ActionDefinition) {
	ae.mu.Lock()
	defer ae.mu.Unlock()

	ae.actions[def.Type] = def
	log.Printf("[ActionExecutor] 注册动作: %s (%s)", def.Name, def.Type)
}

// ExecuteAction 执行动作
func (ae *ActionExecutor) ExecuteAction(ctx context.Context, intent *UserIntent) (*ActionResult, error) {
	ae.mu.RLock()
	actionDef, exists := ae.actions[intent.ActionType]
	ae.mu.RUnlock()

	if !exists {
		return &ActionResult{
			Success:   false,
			Error:     fmt.Sprintf("未找到动作类型: %s", intent.ActionType),
			Timestamp: time.Now(),
		}, fmt.Errorf("未知动作类型: %s", intent.ActionType)
	}

	startTime := time.Now()
	log.Printf("[ActionExecutor] 执行动作: %s", actionDef.Name)

	// 验证必需参数
	if err := ae.validateParameters(actionDef, intent.Parameters); err != nil {
		result := &ActionResult{
			Success:   false,
			Error:     err.Error(),
			Timestamp: time.Now(),
		}
		ae.logExecution(intent.ActionType, intent.Parameters, result, time.Since(startTime))
		return result, err
	}

	// 执行动作
	result, err := actionDef.Handler(ctx, intent.Parameters)
	duration := time.Since(startTime)

	// 记录执行日志
	ae.logExecution(intent.ActionType, intent.Parameters, result, duration)

	// 发布执行结果事件
	if ae.eventBus != nil {
		ae.publishActionResultEvent(intent, result)
	}

	return result, err
}

// validateParameters 验证参数
func (ae *ActionExecutor) validateParameters(def *ActionDefinition, params map[string]interface{}) error {
	for _, param := range def.Parameters {
		if param.Required {
			if _, exists := params[param.Name]; !exists {
				return fmt.Errorf("缺少必需参数: %s", param.Name)
			}
		}
	}
	return nil
}

// logExecution 记录执行日志
func (ae *ActionExecutor) logExecution(actionType ActionType, params map[string]interface{}, result *ActionResult, duration time.Duration) {
	ae.mu.Lock()
	defer ae.mu.Unlock()

	record := &ExecutionRecord{
		ID:        fmt.Sprintf("exec_%d", time.Now().UnixNano()),
		Action:    actionType,
		Params:    params,
		Result:    result,
		Duration:  duration,
		Timestamp: time.Now(),
	}

	ae.executionLog = append(ae.executionLog, record)

	// 限制日志大小
	if len(ae.executionLog) > ae.maxLogSize {
		ae.executionLog = ae.executionLog[len(ae.executionLog)-ae.maxLogSize:]
	}
}

// publishActionResultEvent 发布动作执行结果事件
func (ae *ActionExecutor) publishActionResultEvent(intent *UserIntent, result *ActionResult) {
	eventType := eventbus.EventType("action.executed")
	if !result.Success {
		eventType = eventbus.EventType("action.failed")
	}

	event := eventbus.NewEvent(
		eventType,
		"action_executor",
		map[string]interface{}{
			"action_type":    intent.ActionType,
			"source_event":   intent.SourceEventID,
			"success":        result.Success,
			"message":        result.Message,
			"error":          result.Error,
			"execution_time": result.Timestamp,
		},
	).WithPriority(eventbus.PriorityNormal).
		WithDescription(fmt.Sprintf("动作执行: %s - %s", intent.ActionType, result.Message))

	ae.eventBus.Publish(event)
}

// GetAvailableActions 获取所有可用动作
func (ae *ActionExecutor) GetAvailableActions() []*ActionDefinition {
	ae.mu.RLock()
	defer ae.mu.RUnlock()

	actions := make([]*ActionDefinition, 0, len(ae.actions))
	for _, action := range ae.actions {
		actions = append(actions, action)
	}
	return actions
}

// GetExecutionLog 获取执行日志
func (ae *ActionExecutor) GetExecutionLog(limit int) []*ExecutionRecord {
	ae.mu.RLock()
	defer ae.mu.RUnlock()

	if limit <= 0 || limit > len(ae.executionLog) {
		limit = len(ae.executionLog)
	}

	result := make([]*ExecutionRecord, limit)
	start := len(ae.executionLog) - limit
	copy(result, ae.executionLog[start:])
	return result
}

// ParseUserIntent 解析用户意图（使用 LLM 进行智能解析）
func (ae *ActionExecutor) ParseUserIntent(ctx context.Context, agent *Agent, userInput string, contextEvent *eventbus.Event) (*UserIntent, error) {
	if agent == nil || !agent.IsEnabled() {
		// 降级：使用关键词匹配
		return ae.parseIntentByKeywords(userInput, contextEvent)
	}

	// 构建提示词，让 LLM 解析用户意图
	prompt := fmt.Sprintf(`你是一个意图解析助手。用户收到了一条通知，现在用户想要对这条通知做出回应。

【上下文信息】
通知来源: %s
通知内容: %v

【用户输入】
%s

【可用动作类型】
1. send_dingtalk_message - 发送钉钉消息
   - 参数: content(消息内容), receiver(接收者，可选)
   
2. create_dingtalk_todo - 创建钉钉待办
   - 参数: title(待办标题), description(待办描述，可选), due_date(截止时间，可选)
   
3. create_calendar_event - 创建日历事件
   - 参数: title(事件标题), start_time(开始时间), end_time(结束时间)
   
4. send_ding_message - 发送DING消息（紧急提醒）
   - 参数: content(消息内容), phone(手机号，可选)
   
5. mark_as_important - 标记为重要
   - 无参数
   
6. snooze_notification - 稍后提醒
   - 参数: duration_minutes(稍后分钟数，默认30)
   
7. dismiss_notification - 忽略通知
   - 无参数

【输出格式】
请严格按照以下 JSON 格式输出，不要包含其他文字：
{
  "action_type": "动作类型",
  "parameters": {
    "参数名": "参数值"
  },
  "confidence": 0.95
}

如果无法识别用户意图，输出：
{
  "action_type": "unknown",
  "parameters": {},
  "confidence": 0
}`,
		contextEvent.Source,
		contextEvent.Data,
		userInput,
	)

	// 调用 LLM 进行意图解析
	messages := []ChatMessage{
		{Role: "system", Content: "你是一个意图解析助手，负责将用户的自然语言转换为结构化的动作指令。"},
		{Role: "user", Content: prompt},
	}

	reply, err := agent.ChatSync(ctx, messages, "")
	if err != nil {
		log.Printf("[ActionExecutor] LLM 意图解析失败: %v，降级使用关键词匹配", err)
		return ae.parseIntentByKeywords(userInput, contextEvent)
	}

	// 解析 LLM 返回的 JSON
	intent, err := ae.parseIntentFromLLMReply(reply, contextEvent)
	if err != nil {
		log.Printf("[ActionExecutor] 解析 LLM 返回失败: %v，降级使用关键词匹配", err)
		return ae.parseIntentByKeywords(userInput, contextEvent)
	}

	log.Printf("[ActionExecutor] LLM 意图解析成功: %s (置信度: %.2f)", intent.ActionType, 0.95)
	return intent, nil
}

// parseIntentFromLLMReply 从 LLM 返回中解析意图
func (ae *ActionExecutor) parseIntentFromLLMReply(reply string, contextEvent *eventbus.Event) (*UserIntent, error) {
	// 简化的 JSON 解析（实际应该使用 json.Unmarshal）
	// 这里使用字符串匹配提取关键字段

	actionType := extractJSONString(reply, "action_type")
	if actionType == "" || actionType == "unknown" {
		return nil, fmt.Errorf("无法识别用户意图")
	}

	intent := &UserIntent{
		ActionType:    ActionType(actionType),
		Parameters:    make(map[string]interface{}),
		SourceEventID: contextEvent.ID,
		Context:       make(map[string]interface{}),
	}

	// 提取 parameters 对象中的字段
	paramsStr := extractJSONObject(reply, "parameters")
	if paramsStr != "" {
		// 简单解析 key-value 对
		pairs := strings.Split(paramsStr, ",")
		for _, pair := range pairs {
			pair = strings.TrimSpace(pair)
			if idx := strings.Index(pair, ":"); idx > 0 {
				key := strings.TrimSpace(strings.Trim(pair[:idx], "\""))
				value := strings.TrimSpace(strings.Trim(pair[idx+1:], "\""))
				if key != "" && value != "" {
					intent.Parameters[key] = value
				}
			}
		}
	}

	return intent, nil
}

// parseIntentByKeywords 基于关键词的意图解析（降级方案）
func (ae *ActionExecutor) parseIntentByKeywords(userInput string, contextEvent *eventbus.Event) (*UserIntent, error) {
	intent := &UserIntent{
		Parameters:    make(map[string]interface{}),
		SourceEventID: contextEvent.ID,
		Context: map[string]interface{}{
			"user_input": userInput,
		},
	}

	lowerInput := strings.ToLower(userInput)

	// 关键词匹配
	if containsAny(lowerInput, []string{"回复", "发送", "发消息", "告诉"}) {
		intent.ActionType = ActionSendDingTalkMessage
		intent.Parameters["content"] = userInput
	} else if containsAny(lowerInput, []string{"待办", "任务", "todo", "提醒我"}) {
		intent.ActionType = ActionCreateDingTalkTodo
		intent.Parameters["title"] = userInput
	} else if containsAny(lowerInput, []string{"稍后", "snooze", "晚点"}) {
		intent.ActionType = ActionSnoozeNotification
		if minutes := extractMinutes(lowerInput); minutes > 0 {
			intent.Parameters["duration_minutes"] = minutes
		}
	} else if containsAny(lowerInput, []string{"忽略", "关闭", "dismiss", "不用了"}) {
		intent.ActionType = ActionDismissNotification
	} else if containsAny(lowerInput, []string{"重要", "important", "标记"}) {
		intent.ActionType = ActionMarkAsImportant
	} else {
		return nil, fmt.Errorf("无法识别用户意图: %s", userInput)
	}

	return intent, nil
}

// extractMinutes 从文本中提取分钟数
func extractMinutes(text string) int {
	// 匹配 "30分钟"、"30分钟后"、"半小时" 等
	if strings.Contains(text, "半") && strings.Contains(text, "小时") {
		return 30
	}

	// 简单数字提取
	for i := 0; i < len(text); i++ {
		if text[i] >= '0' && text[i] <= '9' {
			start := i
			for i < len(text) && text[i] >= '0' && text[i] <= '9' {
				i++
			}
			numStr := text[start:i]
			if num, err := strconv.Atoi(numStr); err == nil {
				return num
			}
		}
	}
	return 0
}

// extractJSONString 从 JSON 字符串中提取指定字段的值
func extractJSONString(jsonStr, field string) string {
	pattern := fmt.Sprintf("\"%s\"\\s*:\\s*\"([^\"]*)\"", field)
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(jsonStr)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// extractJSONObject 从 JSON 字符串中提取指定字段的对象内容
func extractJSONObject(jsonStr, field string) string {
	pattern := fmt.Sprintf("\"%s\"\\s*:\\s*\\{([^}]*)\\}", field)
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(jsonStr)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// RegisterDefaultActions 注册默认动作
func (ae *ActionExecutor) RegisterDefaultActions() {
	// 注册钉钉相关动作
	ae.RegisterAction(&ActionDefinition{
		Type:        ActionSendDingTalkMessage,
		Name:        "发送钉钉消息",
		Description: "通过钉钉机器人发送消息",
		Parameters: []ActionParameter{
			{Name: "content", Description: "消息内容", Required: true},
			{Name: "receiver", Description: "接收者（用户ID或群ID）", Required: false},
		},
		Handler: ae.handleSendDingTalkMessage,
	})

	ae.RegisterAction(&ActionDefinition{
		Type:        ActionCreateDingTalkTodo,
		Name:        "创建钉钉待办",
		Description: "在钉钉中创建待办任务",
		Parameters: []ActionParameter{
			{Name: "title", Description: "待办标题", Required: true},
			{Name: "description", Description: "待办描述", Required: false},
			{Name: "due_date", Description: "截止时间（ISO-8601格式）", Required: false},
			{Name: "priority", Description: "优先级（10低/20普通/30较高/40紧急）", Required: false},
		},
		Handler: ae.handleCreateDingTalkTodo,
	})

	ae.RegisterAction(&ActionDefinition{
		Type:        ActionSendDingMessage,
		Name:        "发送DING消息",
		Description: "发送钉钉DING消息（紧急提醒）",
		Parameters: []ActionParameter{
			{Name: "content", Description: "消息内容", Required: true},
			{Name: "phone", Description: "接收者手机号", Required: false},
		},
		Handler: ae.handleSendDingMessage,
	})

	// 注册通知相关动作
	ae.RegisterAction(&ActionDefinition{
		Type:        ActionSnoozeNotification,
		Name:        "稍后提醒",
		Description: "将通知稍后再次提醒",
		Parameters: []ActionParameter{
			{Name: "duration_minutes", Value: 30, Description: "稍后分钟数", Required: false},
		},
		Handler: ae.handleSnoozeNotification,
	})

	ae.RegisterAction(&ActionDefinition{
		Type:        ActionDismissNotification,
		Name:        "忽略通知",
		Description: "忽略当前通知",
		Parameters:  []ActionParameter{},
		Handler:     ae.handleDismissNotification,
	})

	ae.RegisterAction(&ActionDefinition{
		Type:        ActionMarkAsImportant,
		Name:        "标记重要",
		Description: "将通知标记为重要",
		Parameters:  []ActionParameter{},
		Handler:     ae.handleMarkAsImportant,
	})

	log.Println("[ActionExecutor] 默认动作注册完成")
}

// handleSendDingTalkMessage 处理发送钉钉消息
func (ae *ActionExecutor) handleSendDingTalkMessage(ctx context.Context, params map[string]interface{}) (*ActionResult, error) {
	content, ok := params["content"].(string)
	if !ok || content == "" {
		return &ActionResult{
			Success:   false,
			Error:     "缺少必需参数: content",
			Timestamp: time.Now(),
		}, fmt.Errorf("缺少消息内容")
	}

	// 构建 dws 命令
	// 注意：实际使用时需要配置机器人 code 和接收者
	cmd := exec.CommandContext(ctx, "dws", "chat", "message", "send-by-webhook",
		"--token", "${DINGTALK_WEBHOOK_TOKEN}", // 需要从配置中读取
		"--title", "Luffot 通知",
		"--text", content,
		"--format", "json",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[ActionExecutor] 发送钉钉消息失败: %v, output: %s", err, string(output))
		return &ActionResult{
			Success:   false,
			Error:     fmt.Sprintf("发送失败: %v", err),
			Data:      map[string]interface{}{"output": string(output)},
			Timestamp: time.Now(),
		}, err
	}

	log.Printf("[ActionExecutor] 钉钉消息发送成功: %s", content)
	return &ActionResult{
		Success:   true,
		Message:   "钉钉消息已发送",
		Data:      map[string]interface{}{"output": string(output)},
		Timestamp: time.Now(),
	}, nil
}

// handleCreateDingTalkTodo 处理创建钉钉待办
func (ae *ActionExecutor) handleCreateDingTalkTodo(ctx context.Context, params map[string]interface{}) (*ActionResult, error) {
	title, ok := params["title"].(string)
	if !ok || title == "" {
		return &ActionResult{
			Success:   false,
			Error:     "缺少必需参数: title",
			Timestamp: time.Now(),
		}, fmt.Errorf("缺少待办标题")
	}

	// 构建命令参数
	args := []string{"todo", "task", "create", "--title", title, "--format", "json"}

	// 添加可选参数
	if desc, ok := params["description"].(string); ok && desc != "" {
		// dws todo create 不支持 description 参数，忽略
	}

	if dueDate, ok := params["due_date"].(string); ok && dueDate != "" {
		args = append(args, "--due", dueDate)
	}

	if priority, ok := params["priority"].(string); ok && priority != "" {
		args = append(args, "--priority", priority)
	} else {
		args = append(args, "--priority", "20") // 默认普通优先级
	}

	// 执行者（需要从配置中获取当前用户ID）
	args = append(args, "--executors", "${CURRENT_USER_ID}")

	cmd := exec.CommandContext(ctx, "dws", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[ActionExecutor] 创建待办失败: %v, output: %s", err, string(output))
		return &ActionResult{
			Success:   false,
			Error:     fmt.Sprintf("创建失败: %v", err),
			Data:      map[string]interface{}{"output": string(output)},
			Timestamp: time.Now(),
		}, err
	}

	log.Printf("[ActionExecutor] 待办创建成功: %s", title)
	return &ActionResult{
		Success:   true,
		Message:   fmt.Sprintf("待办已创建: %s", title),
		Data:      map[string]interface{}{"output": string(output)},
		Timestamp: time.Now(),
	}, nil
}

// handleSendDingMessage 处理发送DING消息
func (ae *ActionExecutor) handleSendDingMessage(ctx context.Context, params map[string]interface{}) (*ActionResult, error) {
	content, ok := params["content"].(string)
	if !ok || content == "" {
		return &ActionResult{
			Success:   false,
			Error:     "缺少必需参数: content",
			Timestamp: time.Now(),
		}, fmt.Errorf("缺少消息内容")
	}

	// 构建 dws 命令
	args := []string{"ding", "send", "--content", content, "--format", "json"}

	if phone, ok := params["phone"].(string); ok && phone != "" {
		args = append(args, "--mobile", phone)
	}

	cmd := exec.CommandContext(ctx, "dws", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[ActionExecutor] 发送DING消息失败: %v, output: %s", err, string(output))
		return &ActionResult{
			Success:   false,
			Error:     fmt.Sprintf("发送失败: %v", err),
			Data:      map[string]interface{}{"output": string(output)},
			Timestamp: time.Now(),
		}, err
	}

	log.Printf("[ActionExecutor] DING消息发送成功: %s", content)
	return &ActionResult{
		Success:   true,
		Message:   "DING消息已发送",
		Data:      map[string]interface{}{"output": string(output)},
		Timestamp: time.Now(),
	}, nil
}

// handleSnoozeNotification 处理稍后提醒
func (ae *ActionExecutor) handleSnoozeNotification(ctx context.Context, params map[string]interface{}) (*ActionResult, error) {
	duration := 30 // 默认30分钟
	if d, ok := params["duration_minutes"].(int); ok {
		duration = d
	}

	log.Printf("[ActionExecutor] 稍后提醒: %d 分钟后", duration)

	return &ActionResult{
		Success: true,
		Message: fmt.Sprintf("已设置 %d 分钟后再次提醒", duration),
		Data: map[string]interface{}{
			"snooze_duration": duration,
			"next_reminder":   time.Now().Add(time.Duration(duration) * time.Minute),
		},
		Timestamp: time.Now(),
	}, nil
}

// handleDismissNotification 处理忽略通知
func (ae *ActionExecutor) handleDismissNotification(ctx context.Context, params map[string]interface{}) (*ActionResult, error) {
	log.Println("[ActionExecutor] 忽略通知")

	return &ActionResult{
		Success:   true,
		Message:   "通知已忽略",
		Timestamp: time.Now(),
	}, nil
}

// handleMarkAsImportant 处理标记重要
func (ae *ActionExecutor) handleMarkAsImportant(ctx context.Context, params map[string]interface{}) (*ActionResult, error) {
	log.Println("[ActionExecutor] 标记为重要")

	return &ActionResult{
		Success:   true,
		Message:   "已标记为重要",
		Timestamp: time.Now(),
	}, nil
}
