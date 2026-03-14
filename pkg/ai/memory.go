package ai

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// ConversationTurn 一轮对话（用户输入 + AI 回复）
type ConversationTurn struct {
	UserMessage string
	AIReply     string
	CreatedAt   time.Time
}

// Memory 记忆系统，管理短期上下文和长期 SQLite 存储
type Memory struct {
	mu               sync.RWMutex
	shortTermContext []ConversationTurn // 短期记忆（最近 N 轮）
	maxContextRounds int                // 最大保留轮数
	db               *sql.DB
}

// NewMemory 创建记忆系统
// dbPath 为 SQLite 数据库路径（与主数据库共用同一个文件）
func NewMemory(dbPath string, maxContextRounds int) (*Memory, error) {
	if maxContextRounds <= 0 {
		maxContextRounds = 10
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开记忆数据库失败: %w", err)
	}

	m := &Memory{
		shortTermContext: make([]ConversationTurn, 0, maxContextRounds),
		maxContextRounds: maxContextRounds,
		db:               db,
	}

	if err := m.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("初始化记忆表失败: %w", err)
	}

	return m, nil
}

// initSchema 初始化对话历史表
func (m *Memory) initSchema() error {
	_, err := m.db.Exec(`
		CREATE TABLE IF NOT EXISTS conversations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_message TEXT NOT NULL,
			ai_reply TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_conversations_created_at ON conversations(created_at);
	`)
	return err
}

// AddTurn 添加一轮对话到短期记忆和长期存储
func (m *Memory) AddTurn(userMessage, aiReply string) {
	turn := ConversationTurn{
		UserMessage: userMessage,
		AIReply:     aiReply,
		CreatedAt:   time.Now(),
	}

	m.mu.Lock()
	// 短期记忆：超出最大轮数时移除最旧的一轮
	m.shortTermContext = append(m.shortTermContext, turn)
	if len(m.shortTermContext) > m.maxContextRounds {
		m.shortTermContext = m.shortTermContext[len(m.shortTermContext)-m.maxContextRounds:]
	}
	m.mu.Unlock()

	// 异步写入长期存储，不阻塞调用方
	go func() {
		_, err := m.db.Exec(
			`INSERT INTO conversations (user_message, ai_reply, created_at) VALUES (?, ?, ?)`,
			userMessage, aiReply, turn.CreatedAt,
		)
		if err != nil {
			fmt.Printf("[AI Memory] 保存对话历史失败: %v\n", err)
		}
	}()
}

// GetRecentContext 获取最近 N 轮对话，转换为 OpenAI 消息格式（线程安全）
func (m *Memory) GetRecentContext() []ChatMessage {
	m.mu.RLock()
	defer m.mu.RUnlock()

	messages := make([]ChatMessage, 0, len(m.shortTermContext)*2)
	for _, turn := range m.shortTermContext {
		messages = append(messages,
			ChatMessage{Role: "user", Content: turn.UserMessage},
			ChatMessage{Role: "assistant", Content: turn.AIReply},
		)
	}
	return messages
}

// GetRecentHistory 从数据库获取最近的对话历史（用于展示）
func (m *Memory) GetRecentHistory(limit int) ([]ConversationTurn, error) {
	rows, err := m.db.Query(
		`SELECT user_message, ai_reply, created_at FROM conversations ORDER BY created_at DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("查询对话历史失败: %w", err)
	}
	defer rows.Close()

	turns := make([]ConversationTurn, 0, limit)
	for rows.Next() {
		var turn ConversationTurn
		if err := rows.Scan(&turn.UserMessage, &turn.AIReply, &turn.CreatedAt); err != nil {
			return nil, err
		}
		turns = append(turns, turn)
	}
	return turns, rows.Err()
}

// ClearShortTerm 清空短期记忆（开始新话题时调用）
func (m *Memory) ClearShortTerm() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shortTermContext = m.shortTermContext[:0]
}

// Close 关闭数据库连接
func (m *Memory) Close() error {
	return m.db.Close()
}
