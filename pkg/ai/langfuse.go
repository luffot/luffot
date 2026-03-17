package ai

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/AEKurt/langfuse-go"
	"github.com/luffot/luffot/pkg/config"
)

// LangfuseClient Langfuse客户端封装
type LangfuseClient struct {
	client      *langfuse.Client
	asyncClient *langfuse.AsyncClient
	enabled     bool
	publicKey   string
	secretKey   string
	baseURL     string
}

var (
	// 全局Langfuse客户端实例
	globalLangfuse *LangfuseClient
)

// InitLangfuse 初始化Langfuse客户端（从配置文件读取配置）
func InitLangfuse() error {
	langfuseCfg := config.GetLangfuseConfig()

	// 如果未启用，创建禁用状态的客户端
	if !langfuseCfg.Enabled {
		log.Println("[Langfuse] 配置中未启用 Langfuse，追踪功能已禁用")
		globalLangfuse = &LangfuseClient{enabled: false}
		return nil
	}

	// 检查必要配置
	if langfuseCfg.PublicKey == "" || langfuseCfg.SecretKey == "" {
		log.Println("[Langfuse] 配置中缺少 PublicKey 或 SecretKey，Langfuse 追踪已禁用")
		globalLangfuse = &LangfuseClient{enabled: false}
		return nil
	}

	// 设置默认值
	baseURL := langfuseCfg.BaseURL
	if baseURL == "" {
		baseURL = "https://cloud.langfuse.com"
	}

	// 设置批量配置默认值
	batchSize := langfuseCfg.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}
	flushInterval := langfuseCfg.FlushInterval
	if flushInterval <= 0 {
		flushInterval = 5
	}

	client, err := langfuse.NewClient(langfuse.Config{
		PublicKey: langfuseCfg.PublicKey,
		SecretKey: langfuseCfg.SecretKey,
		BaseURL:   baseURL,
	})
	if err != nil {
		return fmt.Errorf("初始化Langfuse客户端失败: %w", err)
	}

	// 创建异步客户端用于高性能批量处理
	asyncClient, err := langfuse.NewAsyncClient(
		langfuse.Config{
			PublicKey: langfuseCfg.PublicKey,
			SecretKey: langfuseCfg.SecretKey,
			BaseURL:   baseURL,
		},
		langfuse.BatchConfig{
			MaxBatchSize:    batchSize,
			FlushInterval:   time.Duration(flushInterval) * time.Second,
			MaxRetries:      3,
			ShutdownTimeout: 30 * time.Second,
			OnError: func(err error, events []langfuse.BatchEvent) {
				log.Printf("[Langfuse] 批量发送失败 %d 个事件: %v", len(events), err)
			},
		},
	)
	if err != nil {
		return fmt.Errorf("初始化Langfuse异步客户端失败: %w", err)
	}

	globalLangfuse = &LangfuseClient{
		client:      client,
		asyncClient: asyncClient,
		enabled:     true,
		publicKey:   langfuseCfg.PublicKey,
		secretKey:   langfuseCfg.SecretKey,
		baseURL:     baseURL,
	}

	log.Println("[Langfuse] 客户端初始化成功")
	return nil
}

// GetLangfuseClient 获取全局Langfuse客户端
func GetLangfuseClient() *LangfuseClient {
	if globalLangfuse == nil {
		globalLangfuse = &LangfuseClient{enabled: false}
	}
	return globalLangfuse
}

// IsEnabled 检查Langfuse是否启用
func (lc *LangfuseClient) IsEnabled() bool {
	return lc != nil && lc.enabled
}

// Shutdown 关闭Langfuse客户端
func (lc *LangfuseClient) Shutdown() error {
	if !lc.IsEnabled() || lc.asyncClient == nil {
		return nil
	}
	return lc.asyncClient.Shutdown()
}

// CreateTrace 创建Trace（追踪根节点）
func (lc *LangfuseClient) CreateTrace(ctx context.Context, name string, userID string, metadata map[string]interface{}) (*langfuse.TraceResponse, error) {
	if !lc.IsEnabled() {
		log.Printf("[Langfuse] CreateTrace 被调用但客户端未启用")
		return nil, nil
	}

	log.Printf("[Langfuse] CreateTrace 开始: name=%s, userID=%s", name, userID)

	now := time.Now()
	trace, err := lc.client.CreateTrace(ctx, langfuse.Trace{
		Name:      name,
		UserID:    userID,
		Metadata:  metadata,
		Timestamp: &now,
	})
	if err != nil {
		log.Printf("[Langfuse] CreateTrace 失败: %v", err)
		return nil, fmt.Errorf("创建Trace失败: %w", err)
	}
	log.Printf("[Langfuse] CreateTrace 成功: traceID=%s", trace.ID)
	return trace, nil
}

// CreateTraceAsync 异步创建Trace
func (lc *LangfuseClient) CreateTraceAsync(name string, userID string, metadata map[string]interface{}) string {
	if !lc.IsEnabled() {
		return ""
	}

	traceID, _ := lc.asyncClient.CreateTraceAsync(langfuse.Trace{
		Name:     name,
		UserID:   userID,
		Metadata: metadata,
	})
	return traceID
}

// CreateSpan 创建Span（子操作）
func (lc *LangfuseClient) CreateSpan(ctx context.Context, traceID string, name string, input interface{}) (*langfuse.SpanResponse, error) {
	if !lc.IsEnabled() {
		return nil, nil
	}

	now := time.Now()
	span, err := lc.client.CreateSpan(ctx, langfuse.Span{
		TraceID:   traceID,
		Name:      name,
		Input:     input,
		StartTime: &now,
	})
	if err != nil {
		return nil, fmt.Errorf("创建Span失败: %w", err)
	}
	return span, nil
}

// UpdateSpan 更新Span（记录输出和结束时间）
func (lc *LangfuseClient) UpdateSpan(ctx context.Context, spanID string, output interface{}) (*langfuse.SpanResponse, error) {
	if !lc.IsEnabled() {
		return nil, nil
	}

	now := time.Now()
	span, err := lc.client.UpdateSpan(ctx, spanID, langfuse.SpanUpdate{
		EndTime: &now,
		Output:  output,
	})
	if err != nil {
		return nil, fmt.Errorf("更新Span失败: %w", err)
	}
	return span, nil
}

// CreateGeneration 创建Generation（LLM调用）
func (lc *LangfuseClient) CreateGeneration(ctx context.Context, traceID string, name string, model string, messages []ChatMessage) (*langfuse.GenerationResponse, error) {
	if !lc.IsEnabled() {
		log.Printf("[Langfuse] CreateGeneration 被调用但客户端未启用")
		return nil, nil
	}

	log.Printf("[Langfuse] CreateGeneration 开始: traceID=%s, name=%s, model=%s, messagesCount=%d", traceID, name, model, len(messages))

	// 转换消息格式
	inputMessages := make([]map[string]interface{}, len(messages))
	for i, msg := range messages {
		inputMessages[i] = map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content,
		}
	}

	now := time.Now()
	generation, err := lc.client.CreateGeneration(ctx, langfuse.Generation{
		TraceID:   traceID,
		Name:      name,
		Model:     model,
		StartTime: &now,
		Input: map[string]interface{}{
			"messages": inputMessages,
		},
	})
	if err != nil {
		log.Printf("[Langfuse] CreateGeneration 失败: %v", err)
		return nil, fmt.Errorf("创建Generation失败: %w", err)
	}
	log.Printf("[Langfuse] CreateGeneration 成功: generationID=%s", generation.ID)
	return generation, nil
}

// UpdateGeneration 更新Generation（记录输出、token消耗等）
func (lc *LangfuseClient) UpdateGeneration(ctx context.Context, generationID string, output string, usage *langfuse.Usage) (*langfuse.GenerationResponse, error) {
	if !lc.IsEnabled() {
		log.Printf("[Langfuse] UpdateGeneration 被调用但客户端未启用")
		return nil, nil
	}

	log.Printf("[Langfuse] UpdateGeneration 开始: generationID=%s, outputLen=%d", generationID, len(output))

	now := time.Now()

	// 构建输出消息
	outputData := map[string]interface{}{
		"messages": []map[string]interface{}{
			{
				"role":    "assistant",
				"content": output,
			},
		},
	}

	update := langfuse.GenerationUpdate{
		EndTime: &now,
		Output:  outputData,
	}

	if usage != nil {
		update.Usage = usage
		log.Printf("[Langfuse] UpdateGeneration usage: input=%d, output=%d, total=%d", usage.Input, usage.Output, usage.Total)
	}

	generation, err := lc.client.UpdateGeneration(ctx, generationID, update)
	if err != nil {
		log.Printf("[Langfuse] UpdateGeneration 失败: %v", err)
		return nil, fmt.Errorf("更新Generation失败: %w", err)
	}
	log.Printf("[Langfuse] UpdateGeneration 成功: generationID=%s", generationID)
	return generation, nil
}

// CreateGenerationAsync 异步创建Generation（用于高性能场景）
func (lc *LangfuseClient) CreateGenerationAsync(traceID string, name string, model string, messages []ChatMessage) string {
	if !lc.IsEnabled() {
		return ""
	}

	inputMessages := make([]map[string]interface{}, len(messages))
	for i, msg := range messages {
		inputMessages[i] = map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content,
		}
	}

	generationID, _ := lc.asyncClient.CreateGenerationAsync(langfuse.Generation{
		TraceID: traceID,
		Name:    name,
		Model:   model,
		Input: map[string]interface{}{
			"messages": inputMessages,
		},
	})
	return generationID
}

// UpdateGenerationAsync 异步更新Generation
func (lc *LangfuseClient) UpdateGenerationAsync(generationID string, output string, usage *langfuse.Usage) {
	if !lc.IsEnabled() {
		return
	}

	outputData := map[string]interface{}{
		"messages": []map[string]interface{}{
			{
				"role":    "assistant",
				"content": output,
			},
		},
	}

	update := langfuse.GenerationUpdate{
		Output: outputData,
	}
	if usage != nil {
		update.Usage = usage
	}

	lc.asyncClient.UpdateGenerationAsync(generationID, update)
}

// CreateEvent 创建Event（记录特定事件）
func (lc *LangfuseClient) CreateEvent(ctx context.Context, traceID string, name string, metadata map[string]interface{}) (*langfuse.EventResponse, error) {
	if !lc.IsEnabled() {
		return nil, nil
	}

	event, err := lc.client.CreateEvent(ctx, langfuse.Event{
		TraceID:  traceID,
		Name:     name,
		Metadata: metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("创建Event失败: %w", err)
	}
	return event, nil
}

// Score 创建评分（用于评估LLM输出质量）
func (lc *LangfuseClient) Score(ctx context.Context, traceID string, name string, value float64, comment string) (*langfuse.ScoreResponse, error) {
	if !lc.IsEnabled() {
		return nil, nil
	}

	score, err := lc.client.Score(ctx, langfuse.Score{
		TraceID: traceID,
		Name:    name,
		Value:   value,
		Comment: comment,
	})
	if err != nil {
		return nil, fmt.Errorf("创建Score失败: %w", err)
	}
	return score, nil
}

// TraceContext Langfuse追踪上下文
type TraceContext struct {
	TraceID      string
	SpanID       string
	GenerationID string
	Name         string
	StartTime    time.Time
}

// StartTrace 开始一个新的Trace会话
func StartTrace(ctx context.Context, name string, userID string, metadata map[string]interface{}) (*TraceContext, error) {
	lc := GetLangfuseClient()
	log.Printf("[Langfuse] StartTrace 被调用: name=%s, clientEnabled=%v", name, lc.IsEnabled())

	if !lc.IsEnabled() {
		log.Printf("[Langfuse] StartTrace 返回 nil，客户端未启用")
		return nil, nil
	}

	trace, err := lc.CreateTrace(ctx, name, userID, metadata)
	if err != nil {
		log.Printf("[Langfuse] StartTrace 失败: %v", err)
		return nil, err
	}
	if trace == nil {
		log.Printf("[Langfuse] StartTrace 返回 nil trace")
		return nil, nil
	}

	log.Printf("[Langfuse] StartTrace 成功: traceID=%s", trace.ID)
	return &TraceContext{
		TraceID:   trace.ID,
		Name:      name,
		StartTime: time.Now(),
	}, nil
}

// StartSpan 在指定Trace下开始一个Span
func (tc *TraceContext) StartSpan(ctx context.Context, name string, input interface{}) (*SpanContext, error) {
	lc := GetLangfuseClient()
	if !lc.IsEnabled() || tc == nil {
		return nil, nil
	}

	span, err := lc.CreateSpan(ctx, tc.TraceID, name, input)
	if err != nil {
		return nil, err
	}

	return &SpanContext{
		TraceID:   tc.TraceID,
		SpanID:    span.ID,
		Name:      name,
		StartTime: time.Now(),
		Input:     input,
	}, nil
}

// SpanContext Span上下文
type SpanContext struct {
	TraceID   string
	SpanID    string
	Name      string
	StartTime time.Time
	Input     interface{}
}

// End 结束Span
func (sc *SpanContext) End(ctx context.Context, output interface{}) error {
	lc := GetLangfuseClient()
	if !lc.IsEnabled() || sc == nil {
		return nil
	}

	_, err := lc.UpdateSpan(ctx, sc.SpanID, output)
	return err
}

// StartGeneration 在指定Trace下开始一个Generation（LLM调用）
func (tc *TraceContext) StartGeneration(ctx context.Context, name string, model string, messages []ChatMessage) (*GenerationContext, error) {
	lc := GetLangfuseClient()
	log.Printf("[Langfuse] TraceContext.StartGeneration 被调用: name=%s, model=%s, traceID=%s, clientEnabled=%v, tcIsNil=%v",
		name, model, tc.TraceID, lc.IsEnabled(), tc == nil)

	if !lc.IsEnabled() || tc == nil {
		log.Printf("[Langfuse] StartGeneration 返回 nil，clientEnabled=%v, tcIsNil=%v", lc.IsEnabled(), tc == nil)
		return nil, nil
	}

	generation, err := lc.CreateGeneration(ctx, tc.TraceID, name, model, messages)
	if err != nil {
		log.Printf("[Langfuse] TraceContext.StartGeneration 失败: %v", err)
		return nil, err
	}
	if generation == nil {
		log.Printf("[Langfuse] TraceContext.StartGeneration 返回 nil generation")
		return nil, nil
	}

	log.Printf("[Langfuse] TraceContext.StartGeneration 成功: generationID=%s", generation.ID)
	return &GenerationContext{
		TraceID:      tc.TraceID,
		GenerationID: generation.ID,
		Name:         name,
		Model:        model,
		StartTime:    time.Now(),
		Messages:     messages,
	}, nil
}

// GenerationContext Generation上下文
type GenerationContext struct {
	TraceID      string
	GenerationID string
	Name         string
	Model        string
	StartTime    time.Time
	Messages     []ChatMessage
}

// End 结束Generation，记录输出和token消耗
func (gc *GenerationContext) End(ctx context.Context, output string, inputTokens, outputTokens int) error {
	lc := GetLangfuseClient()
	log.Printf("[Langfuse] GenerationContext.End 被调用: generationID=%s, clientEnabled=%v, gcIsNil=%v",
		gc.GenerationID, lc.IsEnabled(), gc == nil)

	if !lc.IsEnabled() || gc == nil {
		log.Printf("[Langfuse] GenerationContext.End 直接返回，clientEnabled=%v, gcIsNil=%v", lc.IsEnabled(), gc == nil)
		return nil
	}

	usage := &langfuse.Usage{
		Input:  inputTokens,
		Output: outputTokens,
		Total:  inputTokens + outputTokens,
		Unit:   "TOKENS",
	}

	_, err := lc.UpdateGeneration(ctx, gc.GenerationID, output, usage)
	if err != nil {
		log.Printf("[Langfuse] GenerationContext.End 失败: %v", err)
		return err
	}
	log.Printf("[Langfuse] GenerationContext.End 成功: generationID=%s", gc.GenerationID)
	return nil
}

// EndWithUsage 结束Generation，使用预计算的Usage
func (gc *GenerationContext) EndWithUsage(ctx context.Context, output string, usage *langfuse.Usage) error {
	lc := GetLangfuseClient()
	if !lc.IsEnabled() || gc == nil {
		return nil
	}

	_, err := lc.UpdateGeneration(ctx, gc.GenerationID, output, usage)
	return err
}

// CalculateTokens 简单估算token数量（实际应该用tiktoken等库）
func CalculateTokens(text string) int {
	// 粗略估算：中文字符按1个token，英文单词按1个token
	// 实际生产环境应该使用更精确的token计算库
	runes := []rune(text)
	tokenCount := 0
	for _, r := range runes {
		if r > 127 {
			// 非ASCII字符（主要是中文）
			tokenCount++
		} else if r == ' ' || r == '\n' || r == '\t' {
			// 空白字符
			continue
		} else {
			tokenCount++
		}
	}
	// 英文按空格分词
	words := 0
	inWord := false
	for _, r := range runes {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
			if !inWord {
				words++
				inWord = true
			}
		} else {
			inWord = false
		}
	}

	// 取较大值
	if words > tokenCount {
		return words
	}
	return tokenCount
}

// CalculateMessagesTokens 计算消息列表的token数
func CalculateMessagesTokens(messages []ChatMessage) int {
	total := 0
	for _, msg := range messages {
		total += CalculateTokens(msg.Content)
		if msg.ReasoningContent != "" {
			total += CalculateTokens(msg.ReasoningContent)
		}
	}
	return total
}
