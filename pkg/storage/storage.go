package storage

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Message 消息记录
type Message struct {
	ID        int64     `json:"id"`
	App       string    `json:"app"`       // 应用名称
	Session   string    `json:"session"`   // 会话名称
	Sender    string    `json:"sender"`    // 发送者
	Content   string    `json:"content"`   // 消息内容
	RawTime   string    `json:"raw_time"`  // 消息中的时间戳
	Timestamp time.Time `json:"timestamp"` // 抓取时间
}

// Storage SQLite 存储
type Storage struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewStorage 创建存储实例
func NewStorage(dbPath string) (*Storage, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败：%w", err)
	}

	s := &Storage{db: db}
	if err := s.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("初始化数据库失败：%w", err)
	}

	return s, nil
}

// initSchema 初始化数据库表结构
func (s *Storage) initSchema() error {
	schema := `
	-- 消息表
	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		app VARCHAR(50) NOT NULL,
		session VARCHAR(200) NOT NULL,
		sender VARCHAR(100) NOT NULL,
		content TEXT NOT NULL,
		raw_time VARCHAR(20),
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
	 created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 应用配置表
	CREATE TABLE IF NOT EXISTS app_configs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name VARCHAR(50) UNIQUE NOT NULL,
		display_name VARCHAR(100),
		process_name VARCHAR(100),
		enabled BOOLEAN DEFAULT 1,
		icon_path VARCHAR(255),
		parse_rules TEXT,
		session_config TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 智能分析状态表：记录已分析到的最大消息 ID，避免重复分析
	CREATE TABLE IF NOT EXISTS analysis_state (
		key VARCHAR(50) PRIMARY KEY,
		last_analyzed_message_id INTEGER NOT NULL DEFAULT 0,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 个人画像表：存储由 AI 生成的用户画像（单行 upsert）
	CREATE TABLE IF NOT EXISTS user_profile (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		profile_text TEXT NOT NULL DEFAULT '',
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 摄像头背后有人检测记录表
	CREATE TABLE IF NOT EXISTS camera_detections (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		detected_at DATETIME NOT NULL,
		image_path VARCHAR(500) NOT NULL,
		ai_reason TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 用户记忆表：存储 AI 从消息分析中提炼的结构化长期记忆片段
	-- 每条记忆是一个独立的知识点（如关注话题、社交关系、行为模式等），支持增量更新和过期淘汰
	CREATE TABLE IF NOT EXISTS user_memory (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		category VARCHAR(50) NOT NULL,
		content TEXT NOT NULL,
		source_summary TEXT NOT NULL DEFAULT '',
		importance INTEGER NOT NULL DEFAULT 5,
		access_count INTEGER NOT NULL DEFAULT 0,
		last_accessed_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 索引
	CREATE INDEX IF NOT EXISTS idx_messages_app ON messages(app);
	CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session);
	CREATE INDEX IF NOT EXISTS idx_messages_sender ON messages(sender);
	CREATE INDEX IF NOT EXISTS idx_messages_timestamp ON messages(timestamp);
	CREATE INDEX IF NOT EXISTS idx_messages_app_timestamp ON messages(app, timestamp);
	CREATE INDEX IF NOT EXISTS idx_camera_detections_detected_at ON camera_detections(detected_at);
	CREATE INDEX IF NOT EXISTS idx_user_memory_category ON user_memory(category);
	CREATE INDEX IF NOT EXISTS idx_user_memory_importance ON user_memory(importance);
	CREATE INDEX IF NOT EXISTS idx_messages_session_timestamp ON messages(session, timestamp);
	`

	_, err := s.db.Exec(schema)
	return err
}

// SaveMessage 保存消息
func (s *Storage) SaveMessage(msg *Message) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.Exec(`
		INSERT INTO messages (app, session, sender, content, raw_time, timestamp)
		VALUES (?, ?, ?, ?, ?, ?)
	`, msg.App, msg.Session, msg.Sender, msg.Content, msg.RawTime, msg.Timestamp)

	if err != nil {
		return 0, fmt.Errorf("保存消息失败：%w", err)
	}

	return result.LastInsertId()
}

// SaveMessages 批量保存消息
func (s *Storage) SaveMessages(messages []*Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO messages (app, session, sender, content, raw_time, timestamp)
		VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, msg := range messages {
		_, err := stmt.Exec(msg.App, msg.Session, msg.Sender, msg.Content, msg.RawTime, msg.Timestamp)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetMessage 根据 ID 获取消息
func (s *Storage) GetMessage(id int64) (*Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	msg := &Message{}
	err := s.db.QueryRow(`
		SELECT id, app, session, sender, content, raw_time, timestamp
		FROM messages WHERE id = ?
	`, id).Scan(&msg.ID, &msg.App, &msg.Session, &msg.Sender, &msg.Content, &msg.RawTime, &msg.Timestamp)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return msg, nil
}

// GetMessages 获取消息列表（分页）
func (s *Storage) GetMessages(app string, limit, offset int) ([]*Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT id, app, session, sender, content, raw_time, timestamp
		FROM messages
	`
	args := []interface{}{}

	if app != "" && app != "all" {
		query += " WHERE app = ?"
		args = append(args, app)
	}

	query += " ORDER BY timestamp DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := []*Message{}
	for rows.Next() {
		msg := &Message{}
		err := rows.Scan(&msg.ID, &msg.App, &msg.Session, &msg.Sender, &msg.Content, &msg.RawTime, &msg.Timestamp)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	return messages, rows.Err()
}

// GetMessagesByTimeRange 按时间范围获取消息
func (s *Storage) GetMessagesByTimeRange(app string, start, end time.Time) ([]*Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT id, app, session, sender, content, raw_time, timestamp
		FROM messages
		WHERE timestamp BETWEEN ? AND ?
	`
	args := []interface{}{start, end}

	if app != "" && app != "all" {
		query = `
			SELECT id, app, session, sender, content, raw_time, timestamp
			FROM messages
			WHERE app = ? AND timestamp BETWEEN ? AND ?
		`
		args = []interface{}{app, start, end}
	}

	query += " ORDER BY timestamp ASC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := []*Message{}
	for rows.Next() {
		msg := &Message{}
		err := rows.Scan(&msg.ID, &msg.App, &msg.Session, &msg.Sender, &msg.Content, &msg.RawTime, &msg.Timestamp)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	return messages, rows.Err()
}

// SearchMessages 搜索消息
func (s *Storage) SearchMessages(keyword string, app string, limit int) ([]*Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT id, app, session, sender, content, raw_time, timestamp
		FROM messages
		WHERE content LIKE ? OR sender LIKE ?
	`
	args := []interface{}{"%" + keyword + "%", "%" + keyword + "%"}

	if app != "" && app != "all" {
		query += " AND app = ?"
		args = append(args, app)
	}

	query += " ORDER BY timestamp DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := []*Message{}
	for rows.Next() {
		msg := &Message{}
		err := rows.Scan(&msg.ID, &msg.App, &msg.Session, &msg.Sender, &msg.Content, &msg.RawTime, &msg.Timestamp)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	return messages, rows.Err()
}

// GetStats 获取统计信息
func (s *Storage) GetStats() (map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make(map[string]interface{})

	// 总消息数
	var totalMessages int64
	err := s.db.QueryRow("SELECT COUNT(*) FROM messages").Scan(&totalMessages)
	if err != nil {
		return nil, err
	}
	stats["total_messages"] = totalMessages

	// 各应用消息数
	rows, err := s.db.Query("SELECT app, COUNT(*) as count FROM messages GROUP BY app")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	appCounts := map[string]int64{}
	for rows.Next() {
		var app string
		var count int64
		if err := rows.Scan(&app, &count); err != nil {
			continue
		}
		appCounts[app] = count
	}
	stats["app_counts"] = appCounts

	// 今日消息数
	var todayMessages int64
	err = s.db.QueryRow(`
		SELECT COUNT(*) FROM messages
		WHERE date(timestamp) = date('now')
	`).Scan(&todayMessages)
	if err != nil {
		return nil, err
	}
	stats["today_messages"] = todayMessages

	return stats, nil
}

// CleanupOldMessages 清理过期消息
func (s *Storage) CleanupOldMessages(retentionDays int) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if retentionDays <= 0 {
		retentionDays = 90
	}

	result, err := s.db.Exec(`
		DELETE FROM messages
		WHERE timestamp < datetime('now', '-' || ? || ' days')
	`, retentionDays)

	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// GetSessions 获取所有会话列表
func (s *Storage) GetSessions(app string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := "SELECT DISTINCT session FROM messages"
	args := []interface{}{}

	if app != "" && app != "all" {
		query += " WHERE app = ?"
		args = append(args, app)
	}

	query += " ORDER BY session"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sessions := []string{}
	for rows.Next() {
		var session string
		if err := rows.Scan(&session); err != nil {
			continue
		}
		sessions = append(sessions, session)
	}

	return sessions, rows.Err()
}

// GetSenders 获取所有发送者列表
func (s *Storage) GetSenders(app string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := "SELECT DISTINCT sender FROM messages"
	args := []interface{}{}

	if app != "" && app != "all" {
		query += " WHERE app = ?"
		args = append(args, app)
	}

	query += " ORDER BY sender"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	senders := []string{}
	for rows.Next() {
		var sender string
		if err := rows.Scan(&sender); err != nil {
			continue
		}
		senders = append(senders, sender)
	}

	return senders, rows.Err()
}

// GetLastAnalyzedMessageID 获取已分析到的最大消息 ID（0 表示从未分析过）
func (s *Storage) GetLastAnalyzedMessageID() (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var lastID int64
	err := s.db.QueryRow(`
		SELECT last_analyzed_message_id FROM analysis_state WHERE key = 'default'
	`).Scan(&lastID)

	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return lastID, nil
}

// SaveLastAnalyzedMessageID 保存已分析到的最大消息 ID
func (s *Storage) SaveLastAnalyzedMessageID(lastID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
		INSERT INTO analysis_state (key, last_analyzed_message_id, updated_at)
		VALUES ('default', ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET
			last_analyzed_message_id = excluded.last_analyzed_message_id,
			updated_at = CURRENT_TIMESTAMP
	`, lastID)
	return err
}

// GetUnanalyzedMessages 获取 ID 大于 afterID 的未分析消息，按 ID 升序，最多返回 limit 条
func (s *Storage) GetUnanalyzedMessages(afterID int64, limit int) ([]*Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, app, session, sender, content, raw_time, timestamp
		FROM messages
		WHERE id > ?
		ORDER BY id ASC
		LIMIT ?
	`, afterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := []*Message{}
	for rows.Next() {
		msg := &Message{}
		if err := rows.Scan(&msg.ID, &msg.App, &msg.Session, &msg.Sender, &msg.Content, &msg.RawTime, &msg.Timestamp); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, rows.Err()
}

// GetUserProfile 获取当前个人画像文本，不存在时返回空字符串
func (s *Storage) GetUserProfile() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var profileText string
	err := s.db.QueryRow(`SELECT profile_text FROM user_profile WHERE id = 1`).Scan(&profileText)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return profileText, nil
}

// SaveUserProfile 保存（覆盖）个人画像文本
func (s *Storage) SaveUserProfile(profileText string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
		INSERT INTO user_profile (id, profile_text, updated_at)
		VALUES (1, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			profile_text = excluded.profile_text,
			updated_at = CURRENT_TIMESTAMP
	`, profileText)
	return err
}

// CameraDetection 摄像头背后有人检测记录
type CameraDetection struct {
	ID         int64     `json:"id"`
	DetectedAt time.Time `json:"detected_at"`
	ImagePath  string    `json:"image_path"`
	AIReason   string    `json:"ai_reason"`
	CreatedAt  time.Time `json:"created_at"`
}

// SaveCameraDetection 保存一条摄像头检测记录
func (s *Storage) SaveCameraDetection(detectedAt time.Time, imagePath, aiReason string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.Exec(`
		INSERT INTO camera_detections (detected_at, image_path, ai_reason)
		VALUES (?, ?, ?)
	`, detectedAt, imagePath, aiReason)
	if err != nil {
		return 0, fmt.Errorf("保存检测记录失败：%w", err)
	}
	return result.LastInsertId()
}

// GetCameraDetections 获取检测记录列表，按检测时间倒序，支持分页
func (s *Storage) GetCameraDetections(limit, offset int) ([]*CameraDetection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 20
	}

	rows, err := s.db.Query(`
		SELECT id, detected_at, image_path, ai_reason, created_at
		FROM camera_detections
		ORDER BY detected_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("查询检测记录失败：%w", err)
	}
	defer rows.Close()

	detections := []*CameraDetection{}
	for rows.Next() {
		detection := &CameraDetection{}
		if err := rows.Scan(
			&detection.ID,
			&detection.DetectedAt,
			&detection.ImagePath,
			&detection.AIReason,
			&detection.CreatedAt,
		); err != nil {
			return nil, err
		}
		detections = append(detections, detection)
	}
	return detections, rows.Err()
}

// CountCameraDetections 获取检测记录总数
func (s *Storage) CountCameraDetections() (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int64
	err := s.db.QueryRow(`SELECT COUNT(*) FROM camera_detections`).Scan(&count)
	return count, err
}

// ---- 用户记忆相关 ----

// UserMemoryEntry 用户记忆条目
type UserMemoryEntry struct {
	ID             int64      `json:"id"`
	Category       string     `json:"category"`         // 记忆类别：topic / relationship / behavior / preference / work_context
	Content        string     `json:"content"`          // 记忆内容（自然语言描述）
	SourceSummary  string     `json:"source_summary"`   // 来源摘要（从哪些消息/会话中提炼）
	Importance     int        `json:"importance"`       // 重要性 1-10
	AccessCount    int        `json:"access_count"`     // 被引用次数
	LastAccessedAt *time.Time `json:"last_accessed_at"` // 最后被引用时间
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// SaveUserMemory 保存一条用户记忆
func (s *Storage) SaveUserMemory(entry *UserMemoryEntry) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.Exec(`
		INSERT INTO user_memory (category, content, source_summary, importance)
		VALUES (?, ?, ?, ?)
	`, entry.Category, entry.Content, entry.SourceSummary, entry.Importance)
	if err != nil {
		return 0, fmt.Errorf("保存用户记忆失败：%w", err)
	}
	return result.LastInsertId()
}

// UpdateUserMemory 更新一条用户记忆的内容和重要性
func (s *Storage) UpdateUserMemory(id int64, content string, importance int, sourceSummary string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
		UPDATE user_memory
		SET content = ?, importance = ?, source_summary = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, content, importance, sourceSummary, id)
	return err
}

// TouchUserMemory 标记一条记忆被访问（增加 access_count，更新 last_accessed_at）
func (s *Storage) TouchUserMemory(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
		UPDATE user_memory
		SET access_count = access_count + 1,
		    last_accessed_at = CURRENT_TIMESTAMP,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, id)
	return err
}

// GetUserMemories 按类别获取用户记忆，按重要性降序
// category 为空时返回所有类别
func (s *Storage) GetUserMemories(category string, limit int) ([]*UserMemoryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	var query string
	var args []interface{}

	if category != "" {
		query = `
			SELECT id, category, content, source_summary, importance, access_count, last_accessed_at, created_at, updated_at
			FROM user_memory
			WHERE category = ?
			ORDER BY importance DESC, updated_at DESC
			LIMIT ?
		`
		args = []interface{}{category, limit}
	} else {
		query = `
			SELECT id, category, content, source_summary, importance, access_count, last_accessed_at, created_at, updated_at
			FROM user_memory
			ORDER BY importance DESC, updated_at DESC
			LIMIT ?
		`
		args = []interface{}{limit}
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询用户记忆失败：%w", err)
	}
	defer rows.Close()

	entries := []*UserMemoryEntry{}
	for rows.Next() {
		entry := &UserMemoryEntry{}
		if err := rows.Scan(
			&entry.ID, &entry.Category, &entry.Content, &entry.SourceSummary,
			&entry.Importance, &entry.AccessCount, &entry.LastAccessedAt,
			&entry.CreatedAt, &entry.UpdatedAt,
		); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// SearchUserMemories 在用户记忆中搜索关键词
func (s *Storage) SearchUserMemories(keyword string, limit int) ([]*UserMemoryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 20
	}

	rows, err := s.db.Query(`
		SELECT id, category, content, source_summary, importance, access_count, last_accessed_at, created_at, updated_at
		FROM user_memory
		WHERE content LIKE ?
		ORDER BY importance DESC, updated_at DESC
		LIMIT ?
	`, "%"+keyword+"%", limit)
	if err != nil {
		return nil, fmt.Errorf("搜索用户记忆失败：%w", err)
	}
	defer rows.Close()

	entries := []*UserMemoryEntry{}
	for rows.Next() {
		entry := &UserMemoryEntry{}
		if err := rows.Scan(
			&entry.ID, &entry.Category, &entry.Content, &entry.SourceSummary,
			&entry.Importance, &entry.AccessCount, &entry.LastAccessedAt,
			&entry.CreatedAt, &entry.UpdatedAt,
		); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// DeleteUserMemory 删除一条用户记忆
func (s *Storage) DeleteUserMemory(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM user_memory WHERE id = ?`, id)
	return err
}

// CleanupStaleMemories 清理过期的低重要性记忆
// 删除 importance <= threshold 且超过 retentionDays 天未被访问的记忆
func (s *Storage) CleanupStaleMemories(importanceThreshold int, retentionDays int) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if retentionDays <= 0 {
		retentionDays = 30
	}

	result, err := s.db.Exec(`
		DELETE FROM user_memory
		WHERE importance <= ?
		  AND (last_accessed_at IS NULL OR last_accessed_at < datetime('now', '-' || ? || ' days'))
		  AND updated_at < datetime('now', '-' || ? || ' days')
	`, importanceThreshold, retentionDays, retentionDays)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// ---- 会话参与度统计 ----

// SessionParticipation 会话参与度统计
type SessionParticipation struct {
	App            string `json:"app"`
	Session        string `json:"session"`
	TotalMessages  int    `json:"total_messages"`   // 该会话总消息数
	SelfMessages   int    `json:"self_messages"`    // 用户自己发送的消息数（sender 为 "我" 或配置的用户名）
	OtherMessages  int    `json:"other_messages"`   // 其他人发送的消息数
	UniqueSenders  int    `json:"unique_senders"`   // 不同发送者数量
	IsActive       bool   `json:"is_active"`        // 用户是否在该会话中活跃（有发过消息）
	LastActivityAt string `json:"last_activity_at"` // 最后活动时间
}

// GetSessionParticipation 获取各会话的参与度统计
// selfSenders 是用户自己可能使用的发送者名称列表（如 ["我", "飛神", "renfei"]）
// 用于区分哪些消息是用户自己发的
func (s *Storage) GetSessionParticipation(app string, selfSenders []string, sinceDays int) ([]*SessionParticipation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if sinceDays <= 0 {
		sinceDays = 7
	}

	// 构建 self sender 的 IN 条件
	selfCondition := "0" // 默认无匹配
	selfArgs := []interface{}{}
	if len(selfSenders) > 0 {
		placeholders := make([]string, len(selfSenders))
		for i, name := range selfSenders {
			placeholders[i] = "?"
			selfArgs = append(selfArgs, name)
		}
		selfCondition = "sender IN (" + joinStrings(placeholders, ",") + ")"
	}

	query := fmt.Sprintf(`
		SELECT
			app,
			session,
			COUNT(*) as total_messages,
			SUM(CASE WHEN %s THEN 1 ELSE 0 END) as self_messages,
			SUM(CASE WHEN NOT (%s) THEN 1 ELSE 0 END) as other_messages,
			COUNT(DISTINCT sender) as unique_senders,
			MAX(timestamp) as last_activity_at
		FROM messages
		WHERE timestamp >= datetime('now', '-' || ? || ' days')
	`, selfCondition, selfCondition)

	args := append(selfArgs, selfArgs...)
	args = append(args, sinceDays)

	if app != "" && app != "all" {
		query += " AND app = ?"
		args = append(args, app)
	}

	query += " GROUP BY app, session ORDER BY last_activity_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询会话参与度失败：%w", err)
	}
	defer rows.Close()

	participations := []*SessionParticipation{}
	for rows.Next() {
		participation := &SessionParticipation{}
		if err := rows.Scan(
			&participation.App,
			&participation.Session,
			&participation.TotalMessages,
			&participation.SelfMessages,
			&participation.OtherMessages,
			&participation.UniqueSenders,
			&participation.LastActivityAt,
		); err != nil {
			return nil, err
		}
		participation.IsActive = participation.SelfMessages > 0
		participations = append(participations, participation)
	}
	return participations, rows.Err()
}

// GetMessagesBySessionAndTimeRange 获取指定会话在时间范围内的消息（用于上下文分组）
func (s *Storage) GetMessagesBySessionAndTimeRange(app, session string, start, end time.Time) ([]*Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, app, session, sender, content, raw_time, timestamp
		FROM messages
		WHERE app = ? AND session = ? AND timestamp BETWEEN ? AND ?
		ORDER BY timestamp ASC
	`, app, session, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := []*Message{}
	for rows.Next() {
		msg := &Message{}
		if err := rows.Scan(&msg.ID, &msg.App, &msg.Session, &msg.Sender, &msg.Content, &msg.RawTime, &msg.Timestamp); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, rows.Err()
}

// GetRecentSessionNames 获取最近有消息的会话名称列表
func (s *Storage) GetRecentSessionNames(app string, sinceDays int, limit int) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if sinceDays <= 0 {
		sinceDays = 7
	}
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT session
		FROM messages
		WHERE timestamp >= datetime('now', '-' || ? || ' days')
	`
	args := []interface{}{sinceDays}

	if app != "" && app != "all" {
		query += " AND app = ?"
		args = append(args, app)
	}

	query += " GROUP BY session ORDER BY MAX(timestamp) DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sessions := []string{}
	for rows.Next() {
		var session string
		if err := rows.Scan(&session); err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}

// joinStrings 简单的字符串连接（避免引入 strings 包的额外依赖）
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for _, s := range strs[1:] {
		result += sep + s
	}
	return result
}

// Close 关闭数据库
func (s *Storage) Close() error {
	return s.db.Close()
}
