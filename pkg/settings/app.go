package settings

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/luffot/luffot/pkg/config"
	"github.com/luffot/luffot/pkg/embedfs"
	"github.com/luffot/luffot/pkg/pet"
	"github.com/luffot/luffot/pkg/prompt"
	"github.com/luffot/luffot/pkg/scheduler"
	"github.com/luffot/luffot/pkg/storage"
	"gopkg.in/yaml.v3"
)

// App Wails 应用结构
type App struct {
	ctx       context.Context
	config    *config.AppConfig
	storage   *storage.Storage
	scheduler *scheduler.Scheduler
	mu        sync.RWMutex
}

// NewApp 创建新的 Wails 应用实例
func NewApp(cfg *config.AppConfig, st *storage.Storage) *App {
	return &App{
		config:  cfg,
		storage: st,
	}
}

// Startup 在应用启动时调用
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
}

// SetScheduler 设置调度器实例
func (a *App) SetScheduler(sched *scheduler.Scheduler) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.scheduler = sched
}

// ==================== 消息相关 API ====================

// GetMessages 获取消息列表
func (a *App) GetMessages(app string, limit int, offset int) (map[string]interface{}, error) {
	if limit <= 0 {
		limit = 50
	}
	messages, err := a.storage.GetMessages(app, limit, offset)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"messages": messages,
		"total":    len(messages),
	}, nil
}

// GetMessageStats 获取消息统计
func (a *App) GetMessageStats() (map[string]interface{}, error) {
	return a.storage.GetStats()
}

// SearchMessages 搜索消息
func (a *App) SearchMessages(keyword string, app string, limit int) (map[string]interface{}, error) {
	if limit <= 0 {
		limit = 50
	}
	messages, err := a.storage.SearchMessages(keyword, app, limit)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"messages": messages,
		"total":    len(messages),
	}, nil
}

// GetApps 获取应用列表
func (a *App) GetApps() (map[string]interface{}, error) {
	apps := config.Get().Apps
	return map[string]interface{}{
		"apps": apps,
	}, nil
}

// ==================== AI 配置 API ====================

// GetAIConfig 获取 AI 配置
func (a *App) GetAIConfig() config.AIConfig {
	return config.Get().AI
}

// SaveAIConfig 保存 AI 配置
func (a *App) SaveAIConfig(cfg config.AIConfig) error {
	return config.UpdateAIConfig(cfg)
}

// ==================== 告警配置 API ====================

// GetAlertConfig 获取告警配置
func (a *App) GetAlertConfig() config.AlertConfig {
	return config.GetAlertConfig()
}

// SaveAlertConfig 保存告警配置
func (a *App) SaveAlertConfig(cfg config.AlertConfig) error {
	return config.UpdateAlertConfig(cfg)
}

// ==================== 弹幕配置 API ====================

// GetBarrageConfig 获取弹幕配置
func (a *App) GetBarrageConfig() config.BarrageConfig {
	return config.GetBarrageConfig()
}

// SaveBarrageConfig 保存弹幕配置
func (a *App) SaveBarrageConfig(cfg config.BarrageConfig) error {
	return config.UpdateBarrageConfig(cfg)
}

// ==================== 摄像头守卫 API ====================

// GetCameraGuardConfig 获取摄像头守卫配置
func (a *App) GetCameraGuardConfig() config.CameraGuardConfig {
	return config.GetCameraGuardConfig()
}

// SaveCameraGuardConfig 保存摄像头守卫配置
func (a *App) SaveCameraGuardConfig(cfg config.CameraGuardConfig) error {
	return config.UpdateCameraGuardConfig(cfg)
}

// GetCameraDetections 获取摄像头检测记录
func (a *App) GetCameraDetections(limit int, offset int) (map[string]interface{}, error) {
	if limit <= 0 {
		limit = 20
	}
	detections, err := a.storage.GetCameraDetections(limit, offset)
	if err != nil {
		return nil, err
	}
	total, err := a.storage.CountCameraDetections()
	if err != nil {
		return nil, err
	}

	// 转换为带 URL 的格式
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
			// Wails 应用中无法使用相对路径访问本地文件，需要将图片转为 base64 数据 URL
			if imgData, err := os.ReadFile(detection.ImagePath); err == nil {
				imageURL = "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(imgData)
			}
		}
		results = append(results, detectionWithURL{
			ID:         detection.ID,
			DetectedAt: detection.DetectedAt.Format("2006-01-02 15:04:05"),
			ImageURL:   imageURL,
			AIReason:   detection.AIReason,
		})
	}

	return map[string]interface{}{
		"detections": results,
		"total":      total,
		"limit":      limit,
		"offset":     offset,
	}, nil
}

// ==================== 皮肤配置 API ====================

// GetSkins 获取皮肤列表
func (a *App) GetSkins() (map[string]interface{}, error) {
	currentSkin := config.GetPetSkin()

	// 确保图片皮肤和 Lua 皮肤已加载（Wails 独立窗口模式下可能尚未初始化）
	pet.AutoLoadImageSkinsFromFS(embedfs.SkinsFS())
	pet.InitLuaSkinSystem()

	// 皮肤列表：矢量皮肤固定排在最前面
	skins := []map[string]interface{}{
		{"name": "经典皮肤", "internal": "", "description": "钉三多经典黑，原汁原味的矢量小蜜蜂", "type": "vector"},
		{"name": "星空蓝", "internal": "星空蓝", "description": "深邃星空配色", "type": "vector"},
		{"name": "暗金", "internal": "暗金", "description": "低调奢华暗金风", "type": "vector"},
		{"name": "樱花粉", "internal": "樱花粉", "description": "清新樱花粉嫩风", "type": "vector"},
	}

	// 动态追加所有已注册的图片皮肤
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

	return map[string]interface{}{
		"current_skin": currentSkin,
		"skins":        skins,
	}, nil
}

// SetSkin 设置当前皮肤
func (a *App) SetSkin(skinName string) error {
	return config.UpdatePetSkin(skinName)
}

// ImportSkin 导入皮肤
func (a *App) ImportSkin(dir string, name string) (map[string]string, error) {
	if dir == "" {
		return nil, fmt.Errorf("皮肤目录路径不能为空")
	}

	// 检查目录是否存在
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("目录不存在或路径无效：%s", dir)
	}

	// 确定皮肤名称
	skinName := name
	if skinName == "" {
		skinName = filepath.Base(dir)
	}

	// 注册图片皮肤
	if err := pet.RegisterImageSkin(skinName, dir); err != nil {
		return nil, fmt.Errorf("导入皮肤失败：%w", err)
	}

	return map[string]string{
		"message":   "皮肤导入成功，已可在皮肤列表中选择",
		"skin_name": skinName,
	}, nil
}

// ==================== 用户画像 API ====================

// GetUserProfile 获取用户个人基础信息（~/.luffot/.my_profile）
func (a *App) GetUserProfile() (map[string]string, error) {
	profilePath := filepath.Join(os.Getenv("HOME"), ".luffot", ".my_profile")
	data, err := os.ReadFile(profilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{
				"content": "",
			}, nil
		}
		return nil, fmt.Errorf("读取用户画像失败: %w", err)
	}
	return map[string]string{
		"content": string(data),
	}, nil
}

// SaveUserProfile 保存用户个人基础信息（~/.luffot/.my_profile）
func (a *App) SaveUserProfile(content string) error {
	profileDir := filepath.Join(os.Getenv("HOME"), ".luffot")
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	profilePath := filepath.Join(profileDir, ".my_profile")
	if err := os.WriteFile(profilePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("保存用户画像失败: %w", err)
	}
	return nil
}

// ==================== 提示词管理 API ====================

// GetPrompts 获取所有提示词
func (a *App) GetPrompts() (map[string]interface{}, error) {
	prompts, err := prompt.ListAll()
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"prompts": prompts,
		"dir":     prompt.GetDir(),
	}, nil
}

// GetPrompt 获取单个提示词内容
func (a *App) GetPrompt(name string) (map[string]string, error) {
	content, err := prompt.Load(name)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"name":    name,
		"content": content,
	}, nil
}

// SavePrompt 保存提示词
func (a *App) SavePrompt(name string, content string) error {
	return prompt.Save(name, content)
}

// ==================== 定时任务 API ====================

// GetTasks 获取定时任务列表
func (a *App) GetTasks() (map[string]interface{}, error) {
	a.mu.RLock()
	sched := a.scheduler
	a.mu.RUnlock()

	if sched == nil {
		return map[string]interface{}{
			"tasks":   []interface{}{},
			"message": "调度器未启用",
		}, nil
	}

	tasks := sched.ListTasks()
	return map[string]interface{}{
		"tasks": tasks,
		"total": len(tasks),
	}, nil
}

// TriggerTask 手动触发任务
func (a *App) TriggerTask(name string) (map[string]string, error) {
	a.mu.RLock()
	sched := a.scheduler
	a.mu.RUnlock()

	if sched == nil {
		return nil, fmt.Errorf("调度器未启用")
	}

	if err := sched.TriggerTask(name); err != nil {
		return nil, err
	}

	return map[string]string{
		"message": fmt.Sprintf("任务 %q 已触发，正在后台执行", name),
		"task":    name,
	}, nil
}

// ==================== Langfuse 配置 API ====================

// GetLangfuseConfig 获取 Langfuse 配置
func (a *App) GetLangfuseConfig() config.LangfuseConfig {
	return config.GetLangfuseConfig()
}

// SaveLangfuseConfig 保存 Langfuse 配置
func (a *App) SaveLangfuseConfig(cfg config.LangfuseConfig) error {
	return config.UpdateLangfuseConfig(cfg)
}

// ==================== ADK 配置 API ====================

// GetADKConfig 获取 ADK 配置
func (a *App) GetADKConfig() (map[string]interface{}, error) {
	adkConfigPath := filepath.Join(os.Getenv("HOME"), ".luffot", "adk", "agent_config.yaml")
	data, err := os.ReadFile(adkConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]interface{}{
				"enabled": false,
				"message": "ADK 配置不存在，请先初始化",
			}, nil
		}
		return nil, err
	}

	var cfg map[string]interface{}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// SaveADKConfig 保存 ADK 配置
func (a *App) SaveADKConfig(cfg map[string]interface{}) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	adkDir := filepath.Join(os.Getenv("HOME"), ".luffot", "adk")
	if err := os.MkdirAll(adkDir, 0755); err != nil {
		return err
	}

	adkConfigPath := filepath.Join(adkDir, "agent_config.yaml")
	return os.WriteFile(adkConfigPath, data, 0644)
}

// ==================== 应用配置 API ====================

// GetAppConfig 获取单个应用配置
func (a *App) GetAppConfig(name string) (*config.AppConfigItem, error) {
	return config.GetApp(name)
}

// SaveAppConfig 保存应用配置
func (a *App) SaveAppConfig(name string, app config.AppConfigItem) error {
	return config.UpdateApp(name, app)
}

// AddAppConfig 添加应用配置
func (a *App) AddAppConfig(app config.AppConfigItem) error {
	return config.AddApp(app)
}

// RemoveAppConfig 删除应用配置
func (a *App) RemoveAppConfig(name string) error {
	return config.RemoveApp(name)
}

// ==================== 通用配置 API ====================

// GetFullConfig 获取完整配置（用于导出）
func (a *App) GetFullConfig() (map[string]interface{}, error) {
	cfg := config.Get()
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ReloadConfig 重新加载配置
func (a *App) ReloadConfig() error {
	return config.Load()
}
