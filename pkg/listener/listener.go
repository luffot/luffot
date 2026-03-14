package listener

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/luffot/luffot/pkg/config"
	"github.com/luffot/luffot/pkg/storage"
)

// AppListener 应用监听器
type AppListener struct {
	appConfig     config.AppConfigItem
	storage       *storage.Storage
	seenMessages  map[string]bool
	mu            sync.RWMutex
	maxCache      int
	checkInterval time.Duration
}

// NewAppListener 创建应用监听器
func NewAppListener(appConfig config.AppConfigItem, storage *storage.Storage, checkInterval time.Duration) *AppListener {
	return &AppListener{
		appConfig:     appConfig,
		storage:       storage,
		seenMessages:  make(map[string]bool, 500),
		maxCache:      1000,
		checkInterval: checkInterval,
	}
}

// IsRunning 检查应用是否运行
func (l *AppListener) IsRunning() bool {
	cmd := exec.Command("pgrep", "-xi", l.appConfig.ProcessName)
	if cmd.Run() == nil {
		return true
	}
	// 也尝试中文名称
	cmd = exec.Command("pgrep", "-fi", l.appConfig.DisplayName)
	return cmd.Run() == nil
}

// GetChatMessages 获取聊天消息
func (l *AppListener) GetChatMessages() (session string, content string, err error) {
	script := l.buildAppleScript()
	output, err := runAppleScript(script)
	if err != nil {
		return "", "", err
	}

	session, content = parseScriptOutput(output)
	return session, content, nil
}

// buildAppleScript 构建 AppleScript
func (l *AppListener) buildAppleScript() string {
	processName := l.appConfig.ProcessName
	displayName := l.appConfig.DisplayName

	return fmt.Sprintf(`
tell application "System Events"
	try
		set proc to first process whose name contains "%s" or name contains "%s"

		set session_name to ""
		try
			set session_name to title of first window of proc
		end try

		set all_text to {}
		try
			set win to first window of proc
			set ui_elements to UI elements of win

			repeat with el in ui_elements
				try
					if role of el is "AXStaticText" or role of el is "AXTextField" then
						set val to value of el
						if val is not "" and val is not missing value then
							set end of all_text to val as text
						end if
					end if
				end try

				try
					set children to UI elements of el
					repeat with child in children
						try
							if role of child is "AXStaticText" or role of child is "AXTextField" then
								set val to value of child
								if val is not "" and val is not missing value then
									set end of all_text to val as text
								end if
							end if
						end try

						try
							set grandchildren to UI elements of child
							repeat with gc in grandchildren
								try
									if role of gc is "AXStaticText" or role of gc is "AXTextField" then
										set val to value of gc
										if val is not "" and val is not missing value then
											set end of all_text to val as text
										end if
									end if
								end try
							end repeat
						end try
					end repeat
				end try
			end repeat
		end try

		set chat_text to ""
		repeat with t in all_text
			if chat_text is "" then
				set chat_text to t
			else
				set chat_text to chat_text & linefeed & t
			end if
		end repeat

		return {session:session_name, content:chat_text}
	on error
		return {session:"", content:""}
	end try
end tell
`, processName, displayName)
}

// Start 开始监听
func (l *AppListener) Start(ctx context.Context, messageChan chan<- *storage.Message) {
	ticker := time.NewTicker(l.checkInterval)
	defer ticker.Stop()

	var lastContent string
	var lastSession string

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !l.IsRunning() {
				continue
			}

			session, content, err := l.GetChatMessages()
			if err != nil {
				continue
			}

			if content == "" {
				continue
			}

			// 检测变化
			if content != lastContent || session != lastSession {
				newLines := l.getNewLines(lastContent, content)
				timestamp := time.Now()

				for _, line := range newLines {
					line = strings.TrimSpace(line)
					if line == "" || len(line) > 5000 {
						continue
					}

					msg := l.parseMessage(line, session, timestamp)
					if msg != nil && !l.isDuplicate(msg) {
						messageChan <- msg
					}
				}

				lastContent = content
				lastSession = session
			}
		}
	}
}

// getNewLines 获取新增的行
func (l *AppListener) getNewLines(old, new string) []string {
	if old == "" {
		return []string{new}
	}

	oldSet := make(map[string]bool)
	for _, line := range strings.Split(old, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			oldSet[line] = true
		}
	}

	var newLines []string
	for _, line := range strings.Split(new, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !oldSet[line] {
			newLines = append(newLines, line)
		}
	}
	return newLines
}

// parseMessage 解析消息
func (l *AppListener) parseMessage(line string, session string, timestamp time.Time) *storage.Message {
	msg := &storage.Message{
		App:       l.appConfig.Name,
		Session:   session,
		Content:   line,
		Sender:    "未知",
		Timestamp: timestamp,
	}

	rules := l.appConfig.ParseRules

	// 使用配置的时间模式解析
	timePattern := rules.TimePattern
	if timePattern == "" {
		timePattern = `\d{1,2}:\d{2}`
	}

	timeRe := regexp.MustCompile(timePattern)
	timeMatch := timeRe.FindStringSubmatchIndex(line)

	if timeMatch != nil {
		timeStr := line[timeMatch[0]:timeMatch[1]]
		msg.RawTime = timeStr

		// 根据配置模式提取发送者和内容
		if rules.ContentMode == "after_time" {
			beforeTime := strings.TrimSpace(line[:timeMatch[0]])
			afterTime := strings.TrimSpace(line[timeMatch[1]:])

			beforeTime = strings.TrimRight(beforeTime, "::")
			if beforeTime != "" && len(beforeTime) < 50 {
				msg.Sender = beforeTime
			}
			if afterTime != "" {
				msg.Content = afterTime
			}
		}
	}

	// 尝试发送者模式
	if msg.Sender == "未知" && rules.SenderPattern != "" {
		senderRe := regexp.MustCompile(rules.SenderPattern)
		matches := senderRe.FindStringSubmatch(line)
		if len(matches) > 1 {
			msg.Sender = matches[1]
		}
	}

	// 尝试冒号分隔
	if msg.Sender == "未知" && strings.Contains(line, ":") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			sender := strings.TrimSpace(parts[0])
			if sender != "" && len(sender) < 50 {
				msg.Sender = sender
				msg.Content = strings.TrimSpace(parts[1])
			}
		}
	}

	return msg
}

// isDuplicate 检查是否重复
func (l *AppListener) isDuplicate(msg *storage.Message) bool {
	key := fmt.Sprintf("%s|%s|%s|%s", msg.App, msg.Sender, msg.Session, msg.Content)

	l.mu.RLock()
	exists := l.seenMessages[key]
	l.mu.RUnlock()

	if exists {
		return true
	}

	l.mu.Lock()
	l.seenMessages[key] = true

	// 清理缓存
	if len(l.seenMessages) > l.maxCache {
		newCache := make(map[string]bool, l.maxCache/2)
		count := 0
		for k, v := range l.seenMessages {
			if count >= l.maxCache/2 {
				newCache[k] = v
			}
			count++
		}
		l.seenMessages = newCache
	}
	l.mu.Unlock()

	return false
}

// runAppleScript 执行 AppleScript
func runAppleScript(script string) (string, error) {
	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, string(output))
	}
	return string(output), nil
}

// parseScriptOutput 解析 AppleScript 输出
func parseScriptOutput(output string) (session string, content string) {
	sessionRe := regexp.MustCompile(`session:\s*"([^"]*)"`)
	contentRe := regexp.MustCompile(`content:\s*"([^"]*)"`)

	if matches := sessionRe.FindStringSubmatch(output); len(matches) > 1 {
		session = matches[1]
	}
	if matches := contentRe.FindStringSubmatch(output); len(matches) > 1 {
		content = matches[1]
	}

	return session, content
}
