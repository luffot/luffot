package ai

import (
	"fmt"
	"strings"
	"time"

	"github.com/luffot/luffot/pkg/storage"
)

// MessageQueryTools 消息查询工具集，供 AI Agent 调用
type MessageQueryTools struct {
	storage *storage.Storage
}

// NewMessageQueryTools 创建消息查询工具集
func NewMessageQueryTools(st *storage.Storage) *MessageQueryTools {
	return &MessageQueryTools{storage: st}
}

// GetTodayStats 获取今日消息统计，返回可直接注入 prompt 的文字描述
func (t *MessageQueryTools) GetTodayStats() string {
	if t.storage == nil {
		return "（暂无统计数据）"
	}

	stats, err := t.storage.GetStats()
	if err != nil {
		return fmt.Sprintf("（获取统计失败: %v）", err)
	}

	todayCount, _ := stats["today_messages"].(int64)
	totalCount, _ := stats["total_messages"].(int64)
	appCounts, _ := stats["app_counts"].(map[string]int64)

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("今日共收到 %d 条消息，累计 %d 条。", todayCount, totalCount))

	if len(appCounts) > 0 {
		builder.WriteString("各应用分布：")
		for app, count := range appCounts {
			builder.WriteString(fmt.Sprintf("%s(%d条) ", app, count))
		}
	}

	return builder.String()
}

// GetRecentMessages 获取最近 N 条消息，返回可直接注入 prompt 的文字描述
func (t *MessageQueryTools) GetRecentMessages(limit int, app string) string {
	if t.storage == nil {
		return "（暂无消息数据）"
	}

	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	messages, err := t.storage.GetMessages(app, limit, 0)
	if err != nil {
		return fmt.Sprintf("（获取消息失败: %v）", err)
	}

	if len(messages) == 0 {
		return "（最近没有收到消息）"
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("最近 %d 条消息：\n", len(messages)))
	for i, msg := range messages {
		timeStr := msg.Timestamp.Format("15:04")
		builder.WriteString(fmt.Sprintf("%d. [%s][%s] %s: %s\n",
			i+1, timeStr, msg.App, msg.Sender, truncateText(msg.Content, 60)))
	}

	return builder.String()
}

// GetHourlyMessages 获取最近一小时的消息，返回可直接注入 prompt 的文字描述
func (t *MessageQueryTools) GetHourlyMessages() string {
	if t.storage == nil {
		return "（暂无消息数据）"
	}

	end := time.Now()
	start := end.Add(-1 * time.Hour)

	messages, err := t.storage.GetMessagesByTimeRange("", start, end)
	if err != nil {
		return fmt.Sprintf("（获取消息失败: %v）", err)
	}

	if len(messages) == 0 {
		return "最近一小时没有收到任何消息。"
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("最近一小时共 %d 条消息：\n", len(messages)))
	for _, msg := range messages {
		timeStr := msg.Timestamp.Format("15:04")
		builder.WriteString(fmt.Sprintf("[%s][%s] %s: %s\n",
			timeStr, msg.App, msg.Sender, truncateText(msg.Content, 80)))
	}

	return builder.String()
}

// BuildContextualPrompt 根据用户输入判断是否需要注入消息上下文，返回增强后的 prompt
// 如果用户问的是消息相关的问题，自动附加相关数据
func (t *MessageQueryTools) BuildContextualPrompt(userInput string) string {
	lowerInput := strings.ToLower(userInput)

	// 检测是否是消息查询类问题
	isMessageQuery := containsAny(lowerInput, []string{
		"消息", "message", "钉钉", "微信", "今天", "最近", "统计", "多少",
		"谁发", "发了", "收到", "汇报", "总结", "摘要",
	})

	if !isMessageQuery {
		return userInput
	}

	// 判断查询类型
	isHourly := containsAny(lowerInput, []string{"一小时", "最近", "刚才", "刚刚"})
	isStats := containsAny(lowerInput, []string{"统计", "多少", "数量", "几条"})

	var contextData string
	if isStats {
		contextData = t.GetTodayStats()
	} else if isHourly {
		contextData = t.GetHourlyMessages()
	} else {
		contextData = t.GetRecentMessages(10, "")
	}

	return fmt.Sprintf("%s\n\n【当前消息数据参考】\n%s", userInput, contextData)
}

// truncateText 截断文字到指定长度
func truncateText(text string, maxLen int) string {
	runes := []rune(text)
	if len(runes) <= maxLen {
		return text
	}
	return string(runes[:maxLen]) + "..."
}

// containsAny 检查字符串是否包含任意一个关键词
func containsAny(s string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(s, keyword) {
			return true
		}
	}
	return false
}
