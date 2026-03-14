package eventsource

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"sync"
	"time"
)

// FileTailSourceConfig 文件监听配置
type FileTailSourceConfig struct {
	FilePath string `yaml:"file_path" json:"file_path"` // 要监听的文件路径
	AppName  string `yaml:"app_name" json:"app_name"`   // 应用名称，默认为文件名
}

// FileTailSource 文件监听数据源
// 使用类似 tail -f 的方式监听文件新增行
type FileTailSource struct {
	config   FileTailSourceConfig
	mu       sync.RWMutex
	running  bool
	cancelFn context.CancelFunc
}

// NewFileTailSource 创建文件监听数据源
func NewFileTailSource(config FileTailSourceConfig) *FileTailSource {
	return &FileTailSource{
		config:  config,
		running: false,
	}
}

// Name 返回数据源名称
func (s *FileTailSource) Name() string {
	return "file:" + s.config.FilePath
}

// Start 启动文件监听
func (s *FileTailSource) Start(ctx context.Context, handler MessageEventHandler) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	s.cancelFn = cancel
	s.running = true
	s.mu.Unlock()

	go s.watchFile(ctx, handler)
	return nil
}

// Stop 停止文件监听
func (s *FileTailSource) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	if s.cancelFn != nil {
		s.cancelFn()
	}
	s.running = false
	return nil
}

// IsRunning 检查是否正在运行
func (s *FileTailSource) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// watchFile 监听文件变化
func (s *FileTailSource) watchFile(ctx context.Context, handler MessageEventHandler) {
	filePath := s.config.FilePath
	appName := s.config.AppName
	if appName == "" {
		// 使用文件名作为应用名称
		appName = "file"
	}

	// 先打开文件，读取到末尾
	file, err := os.Open(filePath)
	if err != nil {
		return
	}

	// 移动到文件末尾
	_, err = file.Seek(0, 2) // 2 = io.SeekEnd
	if err != nil {
		file.Close()
		return
	}

	scanner := bufio.NewScanner(file)

	// 定期检查文件是否有新内容
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			file.Close()
			return
		case <-ticker.C:
			// 检查文件是否有新内容
			currentPos, err := file.Seek(0, 1) // 获取当前位置
			if err != nil {
				continue
			}

			// 获取文件大小
			info, err := os.Stat(filePath)
			if err != nil {
				continue
			}

			fileSize := info.Size()

			// 如果文件被截断（rotate），重新打开
			if fileSize < currentPos {
				file.Close()
				file, err = os.Open(filePath)
				if err != nil {
					continue
				}
				_, _ = file.Seek(0, 2) // 移动到末尾
				continue
			}

			// 读取新内容
			for scanner.Scan() {
				line := scanner.Text()
				if line == "" {
					continue
				}

				event := s.parseLine(line, appName)
				if event != nil {
					handler(event)
				}
			}
		}
	}
}

// parseLine 解析文件行
func (s *FileTailSource) parseLine(line string, appName string) *MessageEvent {
	// 尝试解析 JSON 格式
	var eventData struct {
		App     string `json:"app"`
		Session string `json:"session"`
		Sender  string `json:"sender"`
		Content string `json:"content"`
	}

	if err := json.Unmarshal([]byte(line), &eventData); err == nil {
		// JSON 格式解析成功
		event := &MessageEvent{
			App:       eventData.App,
			Session:   eventData.Session,
			Sender:    eventData.Sender,
			Content:   eventData.Content,
			Timestamp: time.Now(),
		}
		if event.App == "" {
			event.App = appName
		}
		return event
	}

	// 非 JSON 格式，将整行作为消息内容
	return &MessageEvent{
		App:       appName,
		Session:   "default",
		Sender:    "unknown",
		Content:   line,
		Timestamp: time.Now(),
	}
}
