package ai

import (
	"context"
	"fmt"
	"strconv"

	pcTools "github.com/sipeed/picoclaw/pkg/tools"
)

// ── 消息统计工具 ─────────────────────────────────────────────────────────

// MessageStatsTool 消息统计工具，供 PicoClaw Agent 自动调用
type MessageStatsTool struct {
	queryTools *MessageQueryTools
}

func NewMessageStatsTool(queryTools *MessageQueryTools) *MessageStatsTool {
	return &MessageStatsTool{queryTools: queryTools}
}

func (t *MessageStatsTool) Name() string {
	return "get_message_stats"
}

func (t *MessageStatsTool) Description() string {
	return "获取今日消息统计数据，包括各应用的消息数量分布。当用户询问消息数量、统计信息时调用此工具。"
}

func (t *MessageStatsTool) Parameters() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (t *MessageStatsTool) Execute(ctx context.Context, args map[string]any) *pcTools.ToolResult {
	if t.queryTools == nil {
		return pcTools.ErrorResult("消息查询工具未初始化")
	}
	stats := t.queryTools.GetTodayStats()
	return pcTools.NewToolResult(stats)
}

// ── 最近消息查询工具 ─────────────────────────────────────────────────────

// RecentMessagesTool 最近消息查询工具
type RecentMessagesTool struct {
	queryTools *MessageQueryTools
}

func NewRecentMessagesTool(queryTools *MessageQueryTools) *RecentMessagesTool {
	return &RecentMessagesTool{queryTools: queryTools}
}

func (t *RecentMessagesTool) Name() string {
	return "get_recent_messages"
}

func (t *RecentMessagesTool) Description() string {
	return "获取最近的消息列表。当用户询问最近收到了什么消息、谁发了消息时调用此工具。可以按应用名称过滤。"
}

func (t *RecentMessagesTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"limit": map[string]any{
				"type":        "integer",
				"description": "返回消息条数，默认10，最大50",
			},
			"app": map[string]any{
				"type":        "string",
				"description": "按应用名称过滤，如 dingtalk、wechat，为空则返回所有应用的消息",
			},
		},
	}
}

func (t *RecentMessagesTool) Execute(ctx context.Context, args map[string]any) *pcTools.ToolResult {
	if t.queryTools == nil {
		return pcTools.ErrorResult("消息查询工具未初始化")
	}

	limit := 10
	if v, ok := args["limit"]; ok {
		switch val := v.(type) {
		case float64:
			limit = int(val)
		case string:
			if parsed, err := strconv.Atoi(val); err == nil {
				limit = parsed
			}
		}
	}

	app := ""
	if v, ok := args["app"]; ok {
		if s, ok := v.(string); ok {
			app = s
		}
	}

	result := t.queryTools.GetRecentMessages(limit, app)
	return pcTools.NewToolResult(result)
}

// ── 最近一小时消息工具 ───────────────────────────────────────────────────

// HourlyMessagesTool 最近一小时消息查询工具
type HourlyMessagesTool struct {
	queryTools *MessageQueryTools
}

func NewHourlyMessagesTool(queryTools *MessageQueryTools) *HourlyMessagesTool {
	return &HourlyMessagesTool{queryTools: queryTools}
}

func (t *HourlyMessagesTool) Name() string {
	return "get_hourly_messages"
}

func (t *HourlyMessagesTool) Description() string {
	return "获取最近一小时的消息。当用户询问刚才、最近一小时收到了什么消息时调用此工具。"
}

func (t *HourlyMessagesTool) Parameters() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (t *HourlyMessagesTool) Execute(ctx context.Context, args map[string]any) *pcTools.ToolResult {
	if t.queryTools == nil {
		return pcTools.ErrorResult("消息查询工具未初始化")
	}
	result := t.queryTools.GetHourlyMessages()
	return pcTools.NewToolResult(result)
}

// ── 注册所有工具 ─────────────────────────────────────────────────────────

// RegisterLuffotTools 将所有 Luffot 自定义工具注册到 PicoClaw 引擎
func RegisterLuffotTools(engine *PicoClawEngine, queryTools *MessageQueryTools) {
	if queryTools == nil {
		return
	}
	engine.RegisterTool(NewMessageStatsTool(queryTools))
	engine.RegisterTool(NewRecentMessagesTool(queryTools))
	engine.RegisterTool(NewHourlyMessagesTool(queryTools))
	fmt.Println("[PicoClaw] 已注册 Luffot 消息查询工具集")
}
