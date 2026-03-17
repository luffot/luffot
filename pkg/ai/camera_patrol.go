package ai

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/luffot/luffot/pkg/camera"
	"github.com/luffot/luffot/pkg/eventbus"
)

// DetectionResult 检测结果
type DetectionResult struct {
	HasPerson   bool      `json:"has_person"`   // 是否检测到人
	HasMovement bool      `json:"has_movement"` // 是否检测到移动
	PersonCount int       `json:"person_count"` // 人数
	Description string    `json:"description"`  // 描述
	Confidence  float64   `json:"confidence"`   // 置信度
	Timestamp   time.Time `json:"timestamp"`    // 检测时间
}

// CameraPatrol 摄像头巡查员智能体
// 职责：在用户授权下，通过摄像头采集环境图像，辅助用户理解当前所处空间
type CameraPatrol struct {
	agent    *Agent
	eventBus *eventbus.EventBus

	// 配置
	enabled           bool
	checkInterval     time.Duration
	movementThreshold float64 // 移动检测阈值

	// 状态跟踪
	lastFrameHash         string
	lastDetection         *DetectionResult
	consecutiveDetections int // 连续检测计数

	// 历史记录
	detectionHistory []*DetectionResult
	maxHistorySize   int

	// 运行状态
	running  bool
	mu       sync.RWMutex
	stopChan chan struct{}
}

// NewCameraPatrol 创建摄像头巡查员
func NewCameraPatrol(agent *Agent) *CameraPatrol {
	return &CameraPatrol{
		agent:             agent,
		eventBus:          eventbus.GetGlobalEventBus(),
		enabled:           false,
		checkInterval:     5 * time.Second,
		movementThreshold: 0.3,
		lastFrameHash:     "",
		detectionHistory:  make([]*DetectionResult, 0),
		maxHistorySize:    100,
		stopChan:          make(chan struct{}),
	}
}

// Start 启动摄像头巡查员
func (cp *CameraPatrol) Start() {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if cp.running {
		return
	}

	cp.running = true
	cp.stopChan = make(chan struct{})

	go cp.patrolLoop()

	log.Println("[CameraPatrol] 摄像头巡查员启动")
}

// Stop 停止摄像头巡查员
func (cp *CameraPatrol) Stop() {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if !cp.running {
		return
	}

	cp.running = false
	close(cp.stopChan)

	log.Println("[CameraPatrol] 摄像头巡查员停止")
}

// Enable 启用摄像头巡查
func (cp *CameraPatrol) Enable() {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if !cp.enabled {
		cp.enabled = true
		log.Println("[CameraPatrol] 摄像头巡查已启用")
	}
}

// Disable 禁用摄像头巡查
func (cp *CameraPatrol) Disable() {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if cp.enabled {
		cp.enabled = false
		log.Println("[CameraPatrol] 摄像头巡查已禁用")
	}
}

// IsEnabled 检查是否启用
func (cp *CameraPatrol) IsEnabled() bool {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	return cp.enabled
}

// patrolLoop 巡查循环
func (cp *CameraPatrol) patrolLoop() {
	ticker := time.NewTicker(cp.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-cp.stopChan:
			return
		case <-ticker.C:
			if cp.IsEnabled() {
				cp.performDetection()
			}
		}
	}
}

// performDetection 执行检测
func (cp *CameraPatrol) performDetection() {
	ctx := context.Background()

	// 开始Trace追踪整个检测流程
	traceCtx, err := StartTrace(ctx, "camera-patrol-analysis", "", "camera_frame", map[string]interface{}{
		"component": "camera_patrol",
	})
	if err != nil {
		log.Printf("[CameraPatrol] 创建Langfuse Trace失败: %v", err)
	}
	// 确保Trace结束时记录输出
	defer func() {
		if traceCtx != nil {
			if err := traceCtx.End(ctx, "detection_completed"); err != nil {
				log.Printf("[CameraPatrol] 结束Trace失败: %v", err)
			}
		}
	}()

	// 开始Span追踪检测流程
	var spanCtx *SpanContext
	if traceCtx != nil {
		spanCtx, err = traceCtx.StartSpan(ctx, "detection-flow", map[string]interface{}{
			"action": "perform_detection",
		})
		if err != nil {
			log.Printf("[CameraPatrol] 创建Langfuse Span失败: %v", err)
		}
	}

	defer func() {
		if spanCtx != nil {
			if err := spanCtx.End(ctx, map[string]interface{}{
				"status": "completed",
			}); err != nil {
				log.Printf("[CameraPatrol] 结束Span失败: %v", err)
			}
		}
	}()

	// 检查摄像头权限
	if !camera.HasPermission() {
		log.Println("[CameraPatrol] 无摄像头权限，跳过检测")
		return
	}

	// 抓取帧
	base64JPEG, err := camera.CaptureFrame()
	if err != nil {
		log.Printf("[CameraPatrol] 抓帧失败: %v", err)
		return
	}

	// 计算帧哈希（简化实现，实际应该用图像哈希算法）
	frameHash := cp.calculateFrameHash(base64JPEG)

	cp.mu.Lock()
	isDuplicate := frameHash == cp.lastFrameHash
	cp.lastFrameHash = frameHash
	cp.mu.Unlock()

	// 如果帧没有变化，跳过分析
	if isDuplicate {
		return
	}

	// 使用AI分析图像
	if cp.agent != nil && cp.agent.IsEnabled() {
		result := cp.analyzeWithAI(ctx, traceCtx, spanCtx, base64JPEG)
		cp.processDetectionResult(result)
	}
}

// analyzeWithAI 使用AI分析图像
func (cp *CameraPatrol) analyzeWithAI(ctx context.Context, traceCtx *TraceContext, spanCtx *SpanContext, base64JPEG string) *DetectionResult {
	prompt := `分析这张图片，回答以下问题：
1. 图片中是否有人？（YES/NO）
2. 有几个人？
3. 是否检测到物品移动或环境变化？（YES/NO）
4. 简要描述场景

请按以下格式回答：
HAS_PERSON: YES/NO
PERSON_COUNT: 数字
HAS_MOVEMENT: YES/NO
DESCRIPTION: 场景描述`

	// 获取模型信息
	modelName := "unknown"
	if cp.agent != nil {
		providerCfg := cp.agent.aiConfig().GetProviderConfig("")
		if providerCfg != nil {
			modelName = providerCfg.Model
		}
	}

	// 构建包含图片的完整消息（用于记录到Generation）
	// 注意：这里记录的是图片的元信息，而不是完整的base64数据（避免数据过大）
	messages := []ChatMessage{
		{
			Role:    "user",
			Content: prompt + "\n[图片数据: " + fmt.Sprintf("%d bytes", len(base64JPEG)) + "]",
		},
	}

	// 开始Generation追踪LLM调用
	var genCtx *GenerationContext
	if traceCtx != nil {
		var err error
		// 如果存在 Span，将 Generation 关联到 Span
		parentID := ""
		if spanCtx != nil {
			parentID = spanCtx.SpanID
			log.Printf("[CameraPatrol] 将 Generation 关联到 Span: %s", parentID)
		}
		genCtx, err = traceCtx.StartGenerationWithParent(ctx, parentID, "image-analysis", modelName, messages)
		if err != nil {
			log.Printf("[CameraPatrol] 创建Langfuse Generation失败: %v", err)
		}
	}

	// 记录输入信息
	inputTokens := CalculateTokens(prompt)
	imageSize := len(base64JPEG)

	startTime := time.Now()

	result, err := cp.agent.AnalyzeImageBase64(base64JPEG, prompt, "")

	duration := time.Since(startTime)

	if err != nil {
		log.Printf("[CameraPatrol] AI分析失败: %v", err)
		if genCtx != nil {
			// 使用 background context 避免 AnalyzeImageBase64 的 context 被取消导致上报失败
			if err := genCtx.End(context.Background(), "", inputTokens, 0); err != nil {
				log.Printf("[CameraPatrol] Generation.End 失败: %v", err)
			}
		}
		return nil
	}

	// 结束Generation记录输出
	outputTokens := CalculateTokens(result)
	if genCtx != nil {
		// 使用 background context 避免 AnalyzeImageBase64 的 context 被取消导致上报失败
		if err := genCtx.End(context.Background(), result, inputTokens, outputTokens); err != nil {
			log.Printf("[CameraPatrol] Generation.End 失败: %v", err)
		}
	}

	// 记录到Trace的metadata中
	log.Printf("[CameraPatrol] 图像分析完成 - 模型: %s, 图像大小: %d bytes, 耗时: %v, 输入tokens: %d, 输出tokens: %d",
		modelName, imageSize, duration, inputTokens, outputTokens)

	return cp.parseAIResult(result)
}

// parseAIResult 解析AI结果
func (cp *CameraPatrol) parseAIResult(result string) *DetectionResult {
	detection := &DetectionResult{
		Timestamp: time.Now(),
	}

	// 简化解析，实际应该用正则表达式
	lines := splitLines(result)
	for _, line := range lines {
		if contains(line, "HAS_PERSON: YES") {
			detection.HasPerson = true
		} else if contains(line, "PERSON_COUNT:") {
			detection.PersonCount = parseInt(extractValue(line, "PERSON_COUNT:"))
		} else if contains(line, "HAS_MOVEMENT: YES") {
			detection.HasMovement = true
		} else if contains(line, "DESCRIPTION:") {
			detection.Description = extractValue(line, "DESCRIPTION:")
		}
	}

	return detection
}

// processDetectionResult 处理检测结果
func (cp *CameraPatrol) processDetectionResult(result *DetectionResult) {
	if result == nil {
		return
	}

	cp.mu.Lock()
	cp.lastDetection = result
	cp.detectionHistory = append(cp.detectionHistory, result)

	// 限制历史记录大小
	if len(cp.detectionHistory) > cp.maxHistorySize {
		cp.detectionHistory = cp.detectionHistory[len(cp.detectionHistory)-cp.maxHistorySize:]
	}
	cp.mu.Unlock()

	// 检测到人
	if result.HasPerson {
		cp.consecutiveDetections++

		// 连续检测到人达到一定次数才上报（避免误报）
		if cp.consecutiveDetections >= 2 {
			cp.publishPersonDetectedEvent(result)
		}
	} else {
		// 未检测到人，重置计数
		if cp.consecutiveDetections > 0 {
			cp.publishPersonLeftEvent()
		}
		cp.consecutiveDetections = 0
	}

	// 检测到移动
	if result.HasMovement && !result.HasPerson {
		cp.publishMovementEvent(result)
	}
}

// publishPersonDetectedEvent 发布人员检测事件
func (cp *CameraPatrol) publishPersonDetectedEvent(result *DetectionResult) {
	desc := "检测到有人进入房间"
	if result.PersonCount > 1 {
		desc = fmt.Sprintf("检测到%d人进入房间", result.PersonCount)
	}
	if result.Description != "" {
		desc += "：" + result.Description
	}

	event := eventbus.NewEvent(
		eventbus.EnvPersonDetected,
		"camera_patrol",
		map[string]interface{}{
			"person_count": result.PersonCount,
			"description":  result.Description,
			"confidence":   result.Confidence,
		},
	).WithPriority(eventbus.PriorityNormal).
		WithDescription(desc)

	cp.eventBus.Publish(event)

	log.Printf("[CameraPatrol] %s", desc)
}

// publishPersonLeftEvent 发布人员离开事件
func (cp *CameraPatrol) publishPersonLeftEvent() {
	event := eventbus.NewEvent(
		eventbus.EnvPersonLeft,
		"camera_patrol",
		map[string]interface{}{
			"timestamp": time.Now(),
		},
	).WithPriority(eventbus.PriorityLow).
		WithDescription("人员已离开")

	cp.eventBus.Publish(event)
}

// publishMovementEvent 发布移动事件
func (cp *CameraPatrol) publishMovementEvent(result *DetectionResult) {
	event := eventbus.NewEvent(
		eventbus.EnvObjectMoved,
		"camera_patrol",
		map[string]interface{}{
			"description": result.Description,
			"timestamp":   time.Now(),
		},
	).WithPriority(eventbus.PriorityLow).
		WithDescription("检测到物品移动：" + result.Description)

	cp.eventBus.Publish(event)
}

// calculateFrameHash 计算帧哈希（简化实现）
func (cp *CameraPatrol) calculateFrameHash(base64JPEG string) string {
	// 简化实现：取base64字符串的前100个字符作为哈希
	if len(base64JPEG) > 100 {
		return base64JPEG[:100]
	}
	return base64JPEG
}

// GetLastDetection 获取最后一次检测结果
func (cp *CameraPatrol) GetLastDetection() *DetectionResult {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	if cp.lastDetection == nil {
		return nil
	}

	// 返回副本
	result := *cp.lastDetection
	return &result
}

// GetDetectionHistory 获取检测历史
func (cp *CameraPatrol) GetDetectionHistory(limit int) []*DetectionResult {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	if limit <= 0 || limit > len(cp.detectionHistory) {
		limit = len(cp.detectionHistory)
	}

	// 返回副本
	result := make([]*DetectionResult, limit)
	start := len(cp.detectionHistory) - limit
	for i := 0; i < limit; i++ {
		detection := *cp.detectionHistory[start+i]
		result[i] = &detection
	}

	return result
}

// SetCheckInterval 设置检测间隔
func (cp *CameraPatrol) SetCheckInterval(interval time.Duration) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	cp.checkInterval = interval
}

// IsRunning 检查是否运行中
func (cp *CameraPatrol) IsRunning() bool {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	return cp.running
}

// Helper functions
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func extractValue(line, prefix string) string {
	if idx := findIndex(line, prefix); idx >= 0 {
		value := line[idx+len(prefix):]
		return trimSpace(value)
	}
	return ""
}

func findIndex(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func parseInt(s string) int {
	result := 0
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			result = result*10 + int(s[i]-'0')
		}
	}
	return result
}
