package eventsource

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// HTTPEventSourceConfig HTTP 接口配置
type HTTPEventSourceConfig struct {
	Host       string `yaml:"host" json:"host"`               // 监听地址
	Port       int    `yaml:"port" json:"port"`               // 监听端口
	PathPrefix string `yaml:"path_prefix" json:"path_prefix"` // 路径前缀，如 /event
	AppName    string `yaml:"app_name" json:"app_name"`       // 默认应用名称
	TokenName  string `yaml:"token_name" json:"token_name"`   // Token 参数名
	TokenValue string `yaml:"token_value" json:"token_value"` // Token 值，用于简单验证
}

// HTTPEventSource HTTP 接口数据源
// 通过 HTTP POST 请求接收消息事件
type HTTPEventSource struct {
	config   HTTPEventSourceConfig
	mu       sync.RWMutex
	running  bool
	server   *http.Server
	basePath string
}

// NewHTTPEventSource 创建 HTTP 接口数据源
func NewHTTPEventSource(config HTTPEventSourceConfig) *HTTPEventSource {
	basePath := config.PathPrefix
	if basePath == "" {
		basePath = "/event"
	}
	return &HTTPEventSource{
		config:   config,
		running:  false,
		basePath: basePath,
	}
}

// Name 返回数据源名称
func (s *HTTPEventSource) Name() string {
	return fmt.Sprintf("http://%s:%d%s", s.config.Host, s.config.Port, s.basePath)
}

// Start 启动 HTTP 服务
func (s *HTTPEventSource) Start(ctx context.Context, handler MessageEventHandler) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = true
	s.mu.Unlock()

	mux := http.NewServeMux()

	// 注册通用处理函数
	mux.HandleFunc(s.basePath+"/", s.makeHandler(handler))

	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	s.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		s.Stop()
	}()

	go func() {
		if err := s.server.ListenAndServe(); err != http.ErrServerClosed {
			// 服务异常停止
			s.mu.Lock()
			s.running = false
			s.mu.Unlock()
		}
	}()

	return nil
}

// Stop 停止 HTTP 服务
func (s *HTTPEventSource) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.server.Shutdown(ctx); err != nil {
			return err
		}
	}

	s.running = false
	return nil
}

// IsRunning 检查是否正在运行
func (s *HTTPEventSource) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// makeHandler 创建 HTTP 处理函数
func (s *HTTPEventSource) makeHandler(handler MessageEventHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 只处理 POST 请求
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// 简单的 token 验证
		if s.config.TokenValue != "" {
			token := r.URL.Query().Get(s.config.TokenName)
			if token == "" {
				token = r.Header.Get("X-Auth-Token")
			}
			if token != s.config.TokenValue {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}

		// 读取请求体
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// 解析消息
		event := s.parseMessage(body, r)
		if event == nil {
			http.Error(w, "Invalid message format", http.StatusBadRequest)
			return
		}

		// 调用处理器
		go handler(event)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
		})
	}
}

// parseMessage 解析消息
func (s *HTTPEventSource) parseMessage(body []byte, r *http.Request) *MessageEvent {
	// 尝试解析 JSON 格式
	var eventData struct {
		App       string `json:"app"`
		Session   string `json:"session"`
		Sender    string `json:"sender"`
		Content   string `json:"content"`
		Timestamp string `json:"timestamp"`
		AvatarURL string `json:"avatar_url"`
		Color     string `json:"color"`
	}

	appName := s.config.AppName
	if appName == "" {
		// 从 URL 路径提取应用名称
		// 例如：/event/dingtalk/on_msg -> dingtalk
		appName = s.extractAppFromPath(r.URL.Path)
	}

	var timestamp time.Time
	if err := json.Unmarshal(body, &eventData); err == nil {
		// JSON 格式解析成功
		if eventData.Timestamp != "" {
			timestamp, _ = time.Parse(time.RFC3339, eventData.Timestamp)
		}
		if timestamp.IsZero() {
			timestamp = time.Now()
		}

		return &MessageEvent{
			App:       stringValue(eventData.App, appName),
			Session:   stringValue(eventData.Session, "default"),
			Sender:    stringValue(eventData.Sender, "unknown"),
			Content:   eventData.Content,
			Timestamp: timestamp,
			AvatarURL: eventData.AvatarURL,
			Color:     eventData.Color,
		}
	}

	// 非 JSON 格式，将 body 作为纯文本内容
	return &MessageEvent{
		App:       appName,
		Session:   "default",
		Sender:    "unknown",
		Content:   string(body),
		Timestamp: time.Now(),
	}
}

// extractAppFromPath 从 URL 路径提取应用名称
func (s *HTTPEventSource) extractAppFromPath(path string) string {
	// 移除 basePath 前缀
	if len(path) > len(s.basePath) {
		remaining := path[len(s.basePath):]
		// 去除前导斜杠
		if len(remaining) > 0 && remaining[0] == '/' {
			remaining = remaining[1:]
		}
		// 获取第一段作为应用名称
		for i, c := range remaining {
			if c == '/' {
				if i > 0 {
					return remaining[:i]
				}
			}
		}
		if len(remaining) > 0 {
			return remaining
		}
	}
	return "unknown"
}

// stringValue 返回第一个非空字符串
func stringValue(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
