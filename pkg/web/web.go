package web

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/luffot/luffot/pkg/config"
	"github.com/luffot/luffot/pkg/pet"
	"github.com/luffot/luffot/pkg/prompt"
	"github.com/luffot/luffot/pkg/scheduler"
	"github.com/luffot/luffot/pkg/storage"
)

// capturesDir 告警截图目录（与 manager.go 保持一致）
var capturesDir = filepath.Join(os.Getenv("HOME"), ".luffot", "captures")

// Server Web 服务器
type Server struct {
	config    *config.AppConfig
	storage   *storage.Storage
	scheduler *scheduler.Scheduler
	server    *http.Server
	mu        sync.RWMutex
	isRunning bool
	staticFS  fs.FS
}

// NewServer 创建 Web 服务器。
// staticFS 是以 web/static 目录为根的文件系统（可来自 embed.FS 或 os.DirFS）。
func NewServer(cfg *config.AppConfig, st *storage.Storage, staticFS fs.FS) *Server {
	s := &Server{
		config:    cfg,
		storage:   st,
		isRunning: false,
		staticFS:  staticFS,
	}
	s.initRoutes()
	return s
}

// SetScheduler 注入调度器实例（在调度器启动后调用，用于任务 API）
func (s *Server) SetScheduler(sched *scheduler.Scheduler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scheduler = sched
}

// initRoutes 初始化路由
func (s *Server) initRoutes() {
	http.HandleFunc("/", s.handleIndex)
	http.HandleFunc("/settings", s.handleSettings)
	http.HandleFunc("/api/messages", s.handleMessages)
	http.HandleFunc("/api/stats", s.handleStats)
	http.HandleFunc("/api/apps", s.handleApps)
	http.HandleFunc("/api/apps/", s.handleApp)
	http.HandleFunc("/api/config", s.handleConfig)
	http.HandleFunc("/api/settings", s.handleSettingsAPI)
	http.HandleFunc("/api/alert-config", s.handleAlertConfigAPI)
	http.HandleFunc("/api/barrage-config", s.handleBarrageConfigAPI)
	http.HandleFunc("/api/search", s.handleSearch)
	http.HandleFunc("/api/tasks", s.handleTasks)
	http.HandleFunc("/api/tasks/", s.handleTaskAction)
	http.HandleFunc("/api/prompts", s.handlePrompts)
	http.HandleFunc("/api/prompts/", s.handlePrompt)
	http.HandleFunc("/api/camera-config", s.handleCameraConfigAPI)
	http.HandleFunc("/api/camera-detections", s.handleCameraDetections)
	http.HandleFunc("/api/skin", s.handleSkinAPI)
	http.HandleFunc("/api/skin/import", s.handleSkinImportAPI)
	http.Handle("/static/", http.StripPrefix("/static/", mimeAwareFSServer(s.staticFS)))
	// 提供告警截图的静态文件访问（/captures/{filename}）
	http.Handle("/captures/", http.StripPrefix("/captures/", http.FileServer(http.Dir(capturesDir))))
}

// Start 启动服务器
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Web.Host, s.config.Web.Port)
	s.server = &http.Server{
		Addr:         addr,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	s.mu.Lock()
	s.isRunning = true
	s.mu.Unlock()

	fmt.Printf("Web UI 启动：http://%s\n", addr)
	return s.server.ListenAndServe()
}

// Stop 停止服务器
func (s *Server) Stop() error {
	s.mu.Lock()
	s.isRunning = false
	s.mu.Unlock()

	if s.server != nil {
		return s.server.Close()
	}
	return nil
}

// IsRunning 是否运行
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isRunning
}

// handleIndex 首页
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	content, err := fs.ReadFile(s.staticFS, "index.html")
	if err != nil {
		http.Error(w, "页面文件不存在", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(content)
}

// handleMessages 获取消息列表
func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)

	if r.Method == "OPTIONS" {
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	app := r.URL.Query().Get("app")
	limit := 50
	offset := 0

	fmt.Sscanf(r.URL.Query().Get("limit"), "%d", &limit)
	fmt.Sscanf(r.URL.Query().Get("offset"), "%d", &offset)

	messages, err := s.storage.GetMessages(app, limit, offset)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"messages": messages,
		"total":    len(messages),
	})
}

// handleStats 获取统计信息
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)

	stats, err := s.storage.GetStats()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, stats)
}

// handleApps 获取/添加应用列表
func (s *Server) handleApps(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)

	switch r.Method {
	case "GET":
		apps := config.Get().Apps
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"apps": apps,
		})

	case "POST":
		var app config.AppConfigItem
		if err := json.NewDecoder(r.Body).Decode(&app); err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "无效的请求体"})
			return
		}

		if err := config.AddApp(app); err != nil {
			respondJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}

		respondJSON(w, http.StatusCreated, app)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleApp 单个应用操作
func (s *Server) handleApp(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)

	// 从路径提取应用名称
	name := r.URL.Path[len("/api/apps/"):]

	switch r.Method {
	case "GET":
		app, err := config.GetApp(name)
		if err != nil {
			respondJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		respondJSON(w, http.StatusOK, app)

	case "PUT":
		var app config.AppConfigItem
		if err := json.NewDecoder(r.Body).Decode(&app); err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "无效的请求体"})
			return
		}

		if err := config.UpdateApp(name, app); err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		respondJSON(w, http.StatusOK, app)

	case "DELETE":
		if err := config.RemoveApp(name); err != nil {
			respondJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		respondJSON(w, http.StatusOK, map[string]string{"message": "已删除"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleConfig 配置操作
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)

	switch r.Method {
	case "GET":
		respondJSON(w, http.StatusOK, config.Get())

	case "PUT":
		var cfg config.AppConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "无效的请求体"})
			return
		}

		if err := config.Save(); err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		respondJSON(w, http.StatusOK, cfg)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSettings 设置页面
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	content, err := fs.ReadFile(s.staticFS, "settings.html")
	if err != nil {
		http.Error(w, "页面文件不存在", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(content)
}

// handleSettingsAPI 设置 API（GET 读取 AI 配置，PUT 保存 AI 配置）
func (s *Server) handleSettingsAPI(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)

	switch r.Method {
	case "GET":
		respondJSON(w, http.StatusOK, config.Get().AI)

	case "PUT":
		var aiCfg config.AIConfig
		if err := json.NewDecoder(r.Body).Decode(&aiCfg); err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "无效的请求体"})
			return
		}
		if err := config.UpdateAIConfig(aiCfg); err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		respondJSON(w, http.StatusOK, map[string]string{"message": "保存成功，立即生效"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleAlertConfigAPI 告警配置 API（GET 读取，PUT 保存）
func (s *Server) handleAlertConfigAPI(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)

	switch r.Method {
	case "GET":
		respondJSON(w, http.StatusOK, config.GetAlertConfig())

	case "PUT":
		var alertCfg config.AlertConfig
		if err := json.NewDecoder(r.Body).Decode(&alertCfg); err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "无效的请求体"})
			return
		}
		if err := config.UpdateAlertConfig(alertCfg); err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		respondJSON(w, http.StatusOK, map[string]string{"message": "保存成功，立即生效"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleBarrageConfigAPI 弹幕配置 API（GET 读取，PUT 保存）
func (s *Server) handleBarrageConfigAPI(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)

	switch r.Method {
	case "GET":
		respondJSON(w, http.StatusOK, config.GetBarrageConfig())

	case "PUT":
		var barrageCfg config.BarrageConfig
		if err := json.NewDecoder(r.Body).Decode(&barrageCfg); err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "无效的请求体"})
			return
		}
		if err := config.UpdateBarrageConfig(barrageCfg); err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		respondJSON(w, http.StatusOK, map[string]string{"message": "保存成功，立即生效"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	keyword := r.URL.Query().Get("q")
	app := r.URL.Query().Get("app")
	limit := 50

	fmt.Sscanf(r.URL.Query().Get("limit"), "%d", &limit)

	if keyword == "" {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"messages": []storage.Message{},
		})
		return
	}

	messages, err := s.storage.SearchMessages(keyword, app, limit)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"messages": messages,
		"total":    len(messages),
	})
}

// handleTasks GET /api/tasks：列出所有已注册的定时任务及其状态
func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)

	if r.Method == "OPTIONS" {
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.RLock()
	sched := s.scheduler
	s.mu.RUnlock()

	if sched == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"tasks":   []interface{}{},
			"message": "调度器未启用",
		})
		return
	}

	tasks := sched.ListTasks()
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"tasks": tasks,
		"total": len(tasks),
	})
}

// handleTaskAction POST /api/tasks/{name}/run：手动触发指定任务立即执行
func (s *Server) handleTaskAction(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)

	if r.Method == "OPTIONS" {
		return
	}

	// 解析路径：/api/tasks/{name}/run
	pathSuffix := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	parts := strings.SplitN(pathSuffix, "/", 2)
	if len(parts) != 2 || parts[1] != "run" {
		http.NotFound(w, r)
		return
	}
	taskName := parts[0]

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.RLock()
	sched := s.scheduler
	s.mu.RUnlock()

	if sched == nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "调度器未启用",
		})
		return
	}

	if err := sched.TriggerTask(taskName); err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{
			"error": err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusAccepted, map[string]string{
		"message": fmt.Sprintf("任务 %q 已触发，正在后台执行", taskName),
		"task":    taskName,
	})
}

// handlePrompts GET /api/prompts：列出所有 prompt 的元信息和内容
func (s *Server) handlePrompts(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)

	if r.Method == "OPTIONS" {
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	prompts, err := prompt.ListAll()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"prompts": prompts,
		"dir":     prompt.GetDir(),
	})
}

// handlePrompt GET /api/prompts/{name}：读取单个 prompt；PUT：保存单个 prompt
func (s *Server) handlePrompt(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)

	if r.Method == "OPTIONS" {
		return
	}

	name := strings.TrimPrefix(r.URL.Path, "/api/prompts/")
	if name == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少 prompt 名称"})
		return
	}

	switch r.Method {
	case "GET":
		content, err := prompt.Load(name)
		if err != nil {
			respondJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		respondJSON(w, http.StatusOK, map[string]string{
			"name":    name,
			"content": content,
		})

	case "PUT":
		var body struct {
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "无效的请求体"})
			return
		}
		if err := prompt.Save(name, body.Content); err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		respondJSON(w, http.StatusOK, map[string]string{"message": "保存成功，立即生效"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleCameraConfigAPI GET /api/camera-config：读取摄像头守卫配置；PUT：保存摄像头守卫配置
func (s *Server) handleCameraConfigAPI(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)

	switch r.Method {
	case "GET":
		respondJSON(w, http.StatusOK, config.GetCameraGuardConfig())

	case "PUT":
		var cameraCfg config.CameraGuardConfig
		if err := json.NewDecoder(r.Body).Decode(&cameraCfg); err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "无效的请求体"})
			return
		}
		if err := config.UpdateCameraGuardConfig(cameraCfg); err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		respondJSON(w, http.StatusOK, map[string]string{"message": "保存成功，立即生效"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSkinAPI GET /api/skin：获取皮肤列表和当前皮肤；PUT：切换皮肤
func (s *Server) handleSkinAPI(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)

	switch r.Method {
	case "GET":
		currentSkin := config.GetPetSkin()
		// 皮肤列表：矢量皮肤固定排在最前面
		skins := []map[string]interface{}{
			{"name": "经典皮肤", "internal": "", "description": "钉三多经典黑，原汁原味的矢量小蜜蜂", "type": "vector"},
			{"name": "星空蓝", "internal": "星空蓝", "description": "深邃星空配色", "type": "vector"},
			{"name": "暗金", "internal": "暗金", "description": "低调奢华暗金风", "type": "vector"},
			{"name": "樱花粉", "internal": "樱花粉", "description": "清新樱花粉嫩风", "type": "vector"},
		}
		// 动态追加所有已注册的图片皮肤（AutoLoadImageSkins 在启动时已扫描）
		for _, imageSkinName := range pet.GetImageSkinNames() {
			skins = append(skins, map[string]interface{}{
				"name":        imageSkinName,
				"internal":    imageSkinName,
				"description": "自定义图片皮肤",
				"type":        "image",
			})
		}
		// 动态追加所有已注册的 Lua 皮肤
		for _, luaSkinName := range pet.GetLuaSkinNames() {
			skins = append(skins, map[string]interface{}{
				"name":        luaSkinName,
				"internal":    luaSkinName,
				"description": "Lua 脚本皮肤（LÖVE2D）",
				"type":        "lua",
			})
		}
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"current_skin": currentSkin,
			"skins":        skins,
		})

	case "PUT":
		var body struct {
			SkinName string `json:"skin_name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "无效的请求体"})
			return
		}
		if err := config.UpdatePetSkin(body.SkinName); err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		respondJSON(w, http.StatusOK, map[string]string{"message": "皮肤已切换，立即生效"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSkinImportAPI POST /api/skin/import：从指定目录路径导入皮肤
// 请求体：{"dir": "/path/to/skin/dir", "name": "皮肤名称（可选，优先读取 skin_meta.json）"}
func (s *Server) handleSkinImportAPI(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)

	if r.Method == "OPTIONS" {
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		Dir  string `json:"dir"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "无效的请求体"})
		return
	}
	if body.Dir == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "皮肤目录路径不能为空"})
		return
	}

	// 检查目录是否存在
	info, err := os.Stat(body.Dir)
	if err != nil || !info.IsDir() {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "目录不存在或路径无效：" + body.Dir})
		return
	}

	// 确定皮肤名称：优先用请求体中的 name，其次读 skin_meta.json，最后用目录名
	skinName := body.Name
	if skinName == "" {
		skinName = filepath.Base(body.Dir)
	}

	// 注册图片皮肤（RegisterImageSkinWithEffect 内部会读取帧文件并验证）
	if err := pet.RegisterImageSkin(skinName, body.Dir); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "导入皮肤失败：" + err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message":   "皮肤导入成功，已可在皮肤列表中选择",
		"skin_name": skinName,
	})
}

// handleCameraDetections GET /api/camera-detections：获取摄像头背后有人检测记录列表（分页）
// 返回记录列表，image_url 字段为可直接访问的图片 URL（/captures/{filename}）
func (s *Server) handleCameraDetections(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)

	if r.Method == "OPTIONS" {
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit := 20
	offset := 0
	fmt.Sscanf(r.URL.Query().Get("limit"), "%d", &limit)
	fmt.Sscanf(r.URL.Query().Get("offset"), "%d", &offset)

	detections, err := s.storage.GetCameraDetections(limit, offset)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	total, err := s.storage.CountCameraDetections()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// 为每条记录附加可访问的图片 URL（前端通过 /captures/{filename} 访问）
	type detectionWithURL struct {
		ID         int64  `json:"id"`
		DetectedAt string `json:"detected_at"`
		ImageURL   string `json:"image_url"`
		AIReason   string `json:"ai_reason"`
	}

	results := make([]detectionWithURL, 0, len(detections))
	for _, detection := range detections {
		imageURL := ""
		if detection.ImagePath != "" {
			imageURL = "/captures/" + filepath.Base(detection.ImagePath)
		}
		results = append(results, detectionWithURL{
			ID:         detection.ID,
			DetectedAt: detection.DetectedAt.Format("2006-01-02 15:04:05"),
			ImageURL:   imageURL,
			AIReason:   detection.AIReason,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"detections": results,
		"total":      total,
		"limit":      limit,
		"offset":     offset,
	})
}

// 工具函数

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

// mimeAwareFSServer 返回一个 http.Handler，基于 fs.FS 提供静态文件服务，并强制设置正确的 MIME 类型。
// macOS 上 Go 标准库有时无法从系统 MIME 数据库中识别 .css/.js 等类型，导致浏览器拒绝加载。
func mimeAwareFSServer(staticFS fs.FS) http.Handler {
	mimeTypes := map[string]string{
		".css":   "text/css; charset=utf-8",
		".js":    "application/javascript; charset=utf-8",
		".html":  "text/html; charset=utf-8",
		".json":  "application/json; charset=utf-8",
		".png":   "image/png",
		".jpg":   "image/jpeg",
		".jpeg":  "image/jpeg",
		".svg":   "image/svg+xml",
		".ico":   "image/x-icon",
		".woff":  "font/woff",
		".woff2": "font/woff2",
		".ttf":   "font/ttf",
	}
	fileServer := http.FileServer(http.FS(staticFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ext := strings.ToLower(filepath.Ext(r.URL.Path))
		if contentType, ok := mimeTypes[ext]; ok {
			w.Header().Set("Content-Type", contentType)
		}
		fileServer.ServeHTTP(w, r)
	})
}
