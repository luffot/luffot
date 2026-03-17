package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/luffot/luffot/pkg/config"
)

// ==================== OTel 数据模型定义 ====================

// OTLPTraceRequest OTel Trace 请求结构
type OTLPTraceRequest struct {
	ResourceSpans []ResourceSpan `json:"resourceSpans"`
}

// ResourceSpan 资源 Span
type ResourceSpan struct {
	Resource   Resource    `json:"resource"`
	ScopeSpans []ScopeSpan `json:"scopeSpans"`
}

// Resource 资源信息
type Resource struct {
	Attributes []Attribute `json:"attributes"`
}

// ScopeSpan 作用域 Span
type ScopeSpan struct {
	Scope Scope  `json:"scope"`
	Spans []Span `json:"spans"`
}

// Scope 作用域信息
type Scope struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Span OTel Span 结构
type Span struct {
	TraceID           string      `json:"traceId"`
	SpanID            string      `json:"spanId"`
	ParentSpanID      string      `json:"parentSpanId,omitempty"`
	Name              string      `json:"name"`
	Kind              int         `json:"kind"` // 1=internal, 2=server, 3=client, 4=producer, 5=consumer
	StartTimeUnixNano int64       `json:"startTimeUnixNano"`
	EndTimeUnixNano   int64       `json:"endTimeUnixNano"`
	Attributes        []Attribute `json:"attributes"`
	Events            []Event     `json:"events,omitempty"`
	Status            *Status     `json:"status,omitempty"`
}

// Attribute 属性
type Attribute struct {
	Key   string         `json:"key"`
	Value AttributeValue `json:"value"`
}

// AttributeValue 属性值
type AttributeValue struct {
	StringValue *string  `json:"stringValue,omitempty"`
	IntValue    *int64   `json:"intValue,omitempty"`
	DoubleValue *float64 `json:"doubleValue,omitempty"`
	BoolValue   *bool    `json:"boolValue,omitempty"`
}

// Event Span 事件
type Event struct {
	Name         string      `json:"name"`
	TimeUnixNano int64       `json:"timeUnixNano"`
	Attributes   []Attribute `json:"attributes,omitempty"`
}

// Status Span 状态
type Status struct {
	Code    int    `json:"code"` // 0=unset, 1=ok, 2=error
	Message string `json:"message,omitempty"`
}

// ==================== 内部数据模型 ====================

// TraceData 追踪数据（内存中缓存，trace 结束后统一上报）
type TraceData struct {
	ID          string
	Name        string
	UserID      string
	StartTime   time.Time
	EndTime     *time.Time
	Metadata    map[string]interface{}
	Spans       []*SpanData
	Generations []*GenerationData
	Events      []*EventData
	mu          sync.RWMutex
	ended       bool
}

// SpanData Span 数据
type SpanData struct {
	ID        string
	Name      string
	StartTime time.Time
	EndTime   *time.Time
	Input     interface{}
	Output    interface{}
	ParentID  string
}

// GenerationData Generation 数据（LLM 调用）
type GenerationData struct {
	ID            string
	Name          string
	Model         string
	StartTime     time.Time
	EndTime       *time.Time
	InputMessages []ChatMessage
	Output        string
	Usage         *UsageData
	ParentSpanID  string
}

// UsageData Token 使用量
type UsageData struct {
	Input  int
	Output int
	Total  int
	Unit   string
}

// EventData 事件数据
type EventData struct {
	Name     string
	Time     time.Time
	Metadata map[string]interface{}
}

// ==================== LangfuseClient 重构 ====================

// LangfuseClient Langfuse客户端封装（基于 OTel 协议）
type LangfuseClient struct {
	enabled    bool
	publicKey  string
	secretKey  string
	baseURL    string
	httpClient *http.Client

	// trace 缓存：traceID -> TraceData
	traces   map[string]*TraceData
	tracesMu sync.RWMutex

	// 等待上报的 trace 队列
	pendingQueue chan string
	shutdown     chan struct{}
	wg           sync.WaitGroup
}

var (
	// 全局Langfuse客户端实例
	globalLangfuse *LangfuseClient
	initOnce       sync.Once
)

// InitLangfuse 初始化Langfuse客户端（从配置文件读取配置）
func InitLangfuse() error {
	var initErr error
	initOnce.Do(func() {
		langfuseCfg := config.GetLangfuseConfig()

		// 如果未启用，创建禁用状态的客户端
		if !langfuseCfg.Enabled {
			log.Println("[Langfuse] 配置中未启用 Langfuse，追踪功能已禁用")
			globalLangfuse = &LangfuseClient{enabled: false}
			return
		}

		// 检查必要配置
		if langfuseCfg.PublicKey == "" || langfuseCfg.SecretKey == "" {
			log.Println("[Langfuse] 配置中缺少 PublicKey 或 SecretKey，Langfuse 追踪已禁用")
			globalLangfuse = &LangfuseClient{enabled: false}
			return
		}

		// 设置默认值
		baseURL := langfuseCfg.BaseURL
		if baseURL == "" {
			baseURL = "https://cloud.langfuse.com"
		}

		globalLangfuse = &LangfuseClient{
			enabled:      true,
			publicKey:    langfuseCfg.PublicKey,
			secretKey:    langfuseCfg.SecretKey,
			baseURL:      baseURL,
			httpClient:   &http.Client{Timeout: 30 * time.Second},
			traces:       make(map[string]*TraceData),
			pendingQueue: make(chan string, 1000),
			shutdown:     make(chan struct{}),
		}

		// 启动后台上报协程
		globalLangfuse.wg.Add(1)
		go globalLangfuse.reportWorker()

		log.Println("[Langfuse] OTel 客户端初始化成功")
	})

	return initErr
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
	if !lc.IsEnabled() {
		return nil
	}

	log.Println("[Langfuse] 正在关闭客户端，等待所有 trace 上报完成...")

	// 关闭队列，通知 worker 退出
	close(lc.shutdown)

	// 等待所有上报完成
	lc.wg.Wait()

	// 上报所有未完成的 trace
	lc.tracesMu.Lock()
	pendingTraces := make([]string, 0, len(lc.traces))
	for traceID := range lc.traces {
		pendingTraces = append(pendingTraces, traceID)
	}
	lc.tracesMu.Unlock()

	for _, traceID := range pendingTraces {
		lc.flushTrace(traceID)
	}

	log.Println("[Langfuse] 客户端已关闭")
	return nil
}

// reportWorker 后台上报工作协程
func (lc *LangfuseClient) reportWorker() {
	defer lc.wg.Done()

	for {
		select {
		case traceID, ok := <-lc.pendingQueue:
			if !ok {
				return
			}
			lc.flushTrace(traceID)
		case <-lc.shutdown:
			// 处理队列中剩余的所有任务
			for {
				select {
				case traceID := <-lc.pendingQueue:
					lc.flushTrace(traceID)
				default:
					return
				}
			}
		}
	}
}

// CreateTrace 创建Trace（仅在内存中创建，不上报）
func (lc *LangfuseClient) CreateTrace(ctx context.Context, name string, userID string, metadata map[string]interface{}) (*TraceData, error) {
	if !lc.IsEnabled() {
		return nil, nil
	}

	traceID := generateID()
	now := time.Now()

	trace := &TraceData{
		ID:          traceID,
		Name:        name,
		UserID:      userID,
		StartTime:   now,
		Metadata:    metadata,
		Spans:       make([]*SpanData, 0),
		Generations: make([]*GenerationData, 0),
		Events:      make([]*EventData, 0),
	}

	lc.tracesMu.Lock()
	lc.traces[traceID] = trace
	lc.tracesMu.Unlock()

	log.Printf("[Langfuse] CreateTrace: traceID=%s, name=%s", traceID, name)
	return trace, nil
}

// EndTrace 结束 Trace 并触发上报
func (lc *LangfuseClient) EndTrace(traceID string, output interface{}) error {
	if !lc.IsEnabled() {
		return nil
	}

	lc.tracesMu.Lock()
	trace, exists := lc.traces[traceID]
	if !exists {
		lc.tracesMu.Unlock()
		return fmt.Errorf("trace not found: %s", traceID)
	}

	// 在持有锁的情况下更新 trace
	now := time.Now()
	trace.EndTime = &now
	trace.ended = true

	// 记录输出到 metadata
	if trace.Metadata == nil {
		trace.Metadata = make(map[string]interface{})
	}
	if output != nil {
		trace.Metadata["output"] = output
		trace.Metadata["duration_ms"] = trace.EndTime.Sub(trace.StartTime).Milliseconds()
	}
	lc.tracesMu.Unlock()

	// 将 trace 加入上报队列
	select {
	case lc.pendingQueue <- traceID:
		log.Printf("[Langfuse] EndTrace: traceID=%s, 已加入上报队列", traceID)
	default:
		log.Printf("[Langfuse] 警告: 上报队列已满，trace %s 将延迟上报", traceID)
		go lc.flushTrace(traceID)
	}

	return nil
}

// CreateSpan 创建Span（在内存中）
func (lc *LangfuseClient) CreateSpan(ctx context.Context, traceID string, name string, input interface{}) (*SpanData, error) {
	if !lc.IsEnabled() {
		return nil, nil
	}

	lc.tracesMu.RLock()
	trace, exists := lc.traces[traceID]
	lc.tracesMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("trace not found: %s", traceID)
	}

	spanID := generateID()
	now := time.Now()

	span := &SpanData{
		ID:        spanID,
		Name:      name,
		StartTime: now,
		Input:     input,
	}

	trace.mu.Lock()
	trace.Spans = append(trace.Spans, span)
	trace.mu.Unlock()

	log.Printf("[Langfuse] CreateSpan: traceID=%s, spanID=%s, name=%s", traceID, spanID, name)
	return span, nil
}

// UpdateSpan 更新Span（在内存中）
func (lc *LangfuseClient) UpdateSpan(ctx context.Context, traceID string, spanID string, output interface{}) (*SpanData, error) {
	if !lc.IsEnabled() {
		return nil, nil
	}

	lc.tracesMu.RLock()
	trace, exists := lc.traces[traceID]
	lc.tracesMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("trace not found: %s", traceID)
	}

	trace.mu.Lock()
	defer trace.mu.Unlock()

	for _, span := range trace.Spans {
		if span.ID == spanID {
			now := time.Now()
			span.EndTime = &now
			span.Output = output
			log.Printf("[Langfuse] UpdateSpan: spanID=%s", spanID)
			return span, nil
		}
	}

	return nil, fmt.Errorf("span not found: %s", spanID)
}

// CreateGeneration 创建Generation（在内存中）
func (lc *LangfuseClient) CreateGeneration(ctx context.Context, traceID string, parentObservationID string, name string, model string, messages []ChatMessage) (*GenerationData, error) {
	if !lc.IsEnabled() {
		return nil, nil
	}

	lc.tracesMu.RLock()
	trace, exists := lc.traces[traceID]
	lc.tracesMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("trace not found: %s", traceID)
	}

	genID := generateID()
	now := time.Now()

	gen := &GenerationData{
		ID:            genID,
		Name:          name,
		Model:         model,
		StartTime:     now,
		InputMessages: messages,
		ParentSpanID:  parentObservationID,
	}

	trace.mu.Lock()
	trace.Generations = append(trace.Generations, gen)
	trace.mu.Unlock()

	log.Printf("[Langfuse] CreateGeneration: traceID=%s, genID=%s, model=%s", traceID, genID, model)
	return gen, nil
}

// UpdateGeneration 更新Generation（在内存中）
func (lc *LangfuseClient) UpdateGeneration(ctx context.Context, traceID string, generationID string, output string, usage *UsageData) (*GenerationData, error) {
	if !lc.IsEnabled() {
		return nil, nil
	}

	lc.tracesMu.RLock()
	trace, exists := lc.traces[traceID]
	lc.tracesMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("trace not found: %s", traceID)
	}

	trace.mu.Lock()
	defer trace.mu.Unlock()

	for _, gen := range trace.Generations {
		if gen.ID == generationID {
			now := time.Now()
			gen.EndTime = &now
			gen.Output = output
			gen.Usage = usage
			log.Printf("[Langfuse] UpdateGeneration: genID=%s", generationID)
			return gen, nil
		}
	}

	return nil, fmt.Errorf("generation not found: %s", generationID)
}

// CreateEvent 创建事件（用于标记特定时间点）
// 在 OTel 模式下，事件会被添加到 trace 中，在 trace 结束时统一上报
func (lc *LangfuseClient) CreateEvent(ctx context.Context, traceID string, name string, metadata map[string]interface{}) error {
	if !lc.IsEnabled() {
		return nil
	}

	lc.tracesMu.RLock()
	trace, exists := lc.traces[traceID]
	lc.tracesMu.RUnlock()

	if !exists {
		// Trace 可能已经结束并上报，记录日志但不报错
		log.Printf("[Langfuse] CreateEvent: trace %s 不存在或已上报", traceID)
		return nil
	}

	event := &EventData{
		Name:     name,
		Time:     time.Now(),
		Metadata: metadata,
	}

	trace.mu.Lock()
	trace.Events = append(trace.Events, event)
	trace.mu.Unlock()

	log.Printf("[Langfuse] CreateEvent: traceID=%s, event=%s", traceID, name)
	return nil
}

// flushTrace 将 trace 转换为 OTel 格式并上报
func (lc *LangfuseClient) flushTrace(traceID string) {
	lc.tracesMu.Lock()
	trace, exists := lc.traces[traceID]
	if !exists {
		lc.tracesMu.Unlock()
		return
	}

	// 从 map 中移除（避免重复上报）
	delete(lc.traces, traceID)
	lc.tracesMu.Unlock()

	// 转换为 OTel 格式并上报
	otlpReq := lc.convertToOTel(trace)
	if err := lc.sendToLangfuse(otlpReq); err != nil {
		log.Printf("[Langfuse] 上报 trace %s 失败: %v", traceID, err)
	} else {
		log.Printf("[Langfuse] 上报 trace %s 成功", traceID)
	}
}

// convertToOTel 将内部 TraceData 转换为 OTel 格式
func (lc *LangfuseClient) convertToOTel(trace *TraceData) *OTLPTraceRequest {
	trace.mu.RLock()
	defer trace.mu.RUnlock()

	// 构建 Resource
	resource := Resource{
		Attributes: []Attribute{
			{Key: "service.name", Value: AttributeValue{StringValue: strPtr("luffot")}},
			{Key: "langfuse.trace.id", Value: AttributeValue{StringValue: &trace.ID}},
			{Key: "langfuse.trace.name", Value: AttributeValue{StringValue: &trace.Name}},
		},
	}

	if trace.UserID != "" {
		resource.Attributes = append(resource.Attributes, Attribute{
			Key:   "user.id",
			Value: AttributeValue{StringValue: &trace.UserID},
		})
	}

	// 构建 Spans
	spans := make([]Span, 0)

	// 1. 创建根 Span（代表整个 Trace）
	rootSpan := Span{
		TraceID:           trace.ID,
		SpanID:            generateSpanID(),
		Name:              trace.Name,
		Kind:              1, // internal
		StartTimeUnixNano: trace.StartTime.UnixNano(),
		Attributes:        lc.convertMetadataToAttributes(trace.Metadata),
	}

	if trace.EndTime != nil {
		rootSpan.EndTimeUnixNano = trace.EndTime.UnixNano()
	} else {
		rootSpan.EndTimeUnixNano = time.Now().UnixNano()
	}

	spans = append(spans, rootSpan)

	// 2. 添加子 Spans
	for _, spanData := range trace.Spans {
		span := Span{
			TraceID:           trace.ID,
			SpanID:            spanData.ID,
			ParentSpanID:      rootSpan.SpanID,
			Name:              spanData.Name,
			Kind:              1, // internal
			StartTimeUnixNano: spanData.StartTime.UnixNano(),
			Attributes:        lc.convertInputOutputToAttributes(spanData.Input, spanData.Output),
		}

		if spanData.EndTime != nil {
			span.EndTimeUnixNano = spanData.EndTime.UnixNano()
		} else {
			span.EndTimeUnixNano = time.Now().UnixNano()
		}

		spans = append(spans, span)
	}

	// 3. 添加 Generations 作为 Spans
	for _, genData := range trace.Generations {
		genSpan := Span{
			TraceID:           trace.ID,
			SpanID:            genData.ID,
			ParentSpanID:      rootSpan.SpanID,
			Name:              genData.Name,
			Kind:              3, // client (LLM 调用)
			StartTimeUnixNano: genData.StartTime.UnixNano(),
			Attributes:        lc.convertGenerationToAttributes(genData),
		}

		if genData.ParentSpanID != "" {
			genSpan.ParentSpanID = genData.ParentSpanID
		}

		if genData.EndTime != nil {
			genSpan.EndTimeUnixNano = genData.EndTime.UnixNano()
		} else {
			genSpan.EndTimeUnixNano = time.Now().UnixNano()
		}

		// 添加 Generation 事件
		if genData.Output != "" {
			genSpan.Events = append(genSpan.Events, Event{
				Name:         "generation.output",
				TimeUnixNano: genSpan.EndTimeUnixNano,
				Attributes: []Attribute{
					{Key: "output", Value: AttributeValue{StringValue: &genData.Output}},
				},
			})
		}

		spans = append(spans, genSpan)
	}

	// 4. 添加 Events
	for _, eventData := range trace.Events {
		event := Event{
			Name:         eventData.Name,
			TimeUnixNano: eventData.Time.UnixNano(),
			Attributes:   lc.convertMetadataToAttributes(eventData.Metadata),
		}
		// 将事件附加到根 span
		rootSpan.Events = append(rootSpan.Events, event)
	}

	// 更新根 span
	spans[0] = rootSpan

	// 构建 ScopeSpan
	scopeSpan := ScopeSpan{
		Scope: Scope{
			Name:    "luffot.langfuse",
			Version: "1.0.0",
		},
		Spans: spans,
	}

	// 构建 ResourceSpan
	resourceSpan := ResourceSpan{
		Resource:   resource,
		ScopeSpans: []ScopeSpan{scopeSpan},
	}

	return &OTLPTraceRequest{
		ResourceSpans: []ResourceSpan{resourceSpan},
	}
}

// convertMetadataToAttributes 将 metadata 转换为 OTel attributes
func (lc *LangfuseClient) convertMetadataToAttributes(metadata map[string]interface{}) []Attribute {
	attrs := make([]Attribute, 0)

	for key, value := range metadata {
		attr := lc.convertToAttribute("langfuse.metadata."+key, value)
		if attr != nil {
			attrs = append(attrs, *attr)
		}
	}

	return attrs
}

// convertInputOutputToAttributes 将 input/output 转换为 attributes
func (lc *LangfuseClient) convertInputOutputToAttributes(input, output interface{}) []Attribute {
	attrs := make([]Attribute, 0)

	if input != nil {
		if attr := lc.convertToAttribute("input", input); attr != nil {
			attrs = append(attrs, *attr)
		}
	}

	if output != nil {
		if attr := lc.convertToAttribute("output", output); attr != nil {
			attrs = append(attrs, *attr)
		}
	}

	return attrs
}

// convertGenerationToAttributes 将 Generation 数据转换为 attributes
func (lc *LangfuseClient) convertGenerationToAttributes(gen *GenerationData) []Attribute {
	attrs := make([]Attribute, 0)

	// 模型信息
	if gen.Model != "" {
		attrs = append(attrs, Attribute{
			Key:   "gen_ai.system",
			Value: AttributeValue{StringValue: &gen.Model},
		})
	}

	// Token 使用量
	if gen.Usage != nil {
		inputTokens := int64(gen.Usage.Input)
		outputTokens := int64(gen.Usage.Output)
		totalTokens := int64(gen.Usage.Total)

		attrs = append(attrs,
			Attribute{Key: "gen_ai.usage.input_tokens", Value: AttributeValue{IntValue: &inputTokens}},
			Attribute{Key: "gen_ai.usage.output_tokens", Value: AttributeValue{IntValue: &outputTokens}},
			Attribute{Key: "gen_ai.usage.total_tokens", Value: AttributeValue{IntValue: &totalTokens}},
		)
	}

	// 输入消息
	if len(gen.InputMessages) > 0 {
		messagesJSON, _ := json.Marshal(gen.InputMessages)
		messagesStr := string(messagesJSON)
		attrs = append(attrs, Attribute{
			Key:   "gen_ai.prompt",
			Value: AttributeValue{StringValue: &messagesStr},
		})
	}

	return attrs
}

// convertToAttribute 将任意值转换为 Attribute
func (lc *LangfuseClient) convertToAttribute(key string, value interface{}) *Attribute {
	switch v := value.(type) {
	case string:
		return &Attribute{Key: key, Value: AttributeValue{StringValue: &v}}
	case int:
		intVal := int64(v)
		return &Attribute{Key: key, Value: AttributeValue{IntValue: &intVal}}
	case int64:
		return &Attribute{Key: key, Value: AttributeValue{IntValue: &v}}
	case float64:
		return &Attribute{Key: key, Value: AttributeValue{DoubleValue: &v}}
	case bool:
		return &Attribute{Key: key, Value: AttributeValue{BoolValue: &v}}
	default:
		// 其他类型序列化为 JSON
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		jsonStr := string(jsonBytes)
		return &Attribute{Key: key, Value: AttributeValue{StringValue: &jsonStr}}
	}
}

// sendToLangfuse 发送 OTel 数据到 Langfuse
func (lc *LangfuseClient) sendToLangfuse(req *OTLPTraceRequest) error {
	// 序列化请求
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal otel request failed: %w", err)
	}

	// 构建请求
	url := lc.baseURL + "/api/public/otel/v1/traces"
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}

	// 设置 Headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Basic "+lc.encodeAuth())

	// 发送请求
	resp, err := lc.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("send request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// encodeAuth 编码认证信息
func (lc *LangfuseClient) encodeAuth() string {
	auth := lc.publicKey + ":" + lc.secretKey
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

// generateID 生成 ID（用于 trace/span/generation）
func generateID() string {
	return uuid.New().String()
}

// generateSpanID 生成 Span ID（OTel 使用 8 字节十六进制）
func generateSpanID() string {
	return uuid.New().String()[:16]
}

// strPtr 返回字符串指针
func strPtr(s string) *string {
	return &s
}

// Score 创建评分（用于评估LLM输出质量）
// 注意：OTel 协议暂不支持评分功能，此方法保留接口但返回 nil
func (lc *LangfuseClient) Score(ctx context.Context, traceID string, name string, value float64, comment string) error {
	if !lc.IsEnabled() {
		return nil
	}
	// OTel 协议暂不支持评分，记录日志即可
	log.Printf("[Langfuse] Score 暂不支持: traceID=%s, name=%s, value=%f", traceID, name, value)
	return nil
}

// TraceContext Langfuse追踪上下文
type TraceContext struct {
	TraceID      string
	SpanID       string
	GenerationID string
	Name         string
	StartTime    time.Time
	Input        interface{} // 记录输入
}

// StartTrace 开始一个新的Trace会话（业务功能级别）
// input: 业务功能的输入，会记录在 Trace 的 metadata 中
func StartTrace(ctx context.Context, name string, userID string, input interface{}, metadata map[string]interface{}) (*TraceContext, error) {
	lc := GetLangfuseClient()
	log.Printf("[Langfuse] StartTrace 被调用: name=%s, clientEnabled=%v", name, lc.IsEnabled())

	if !lc.IsEnabled() {
		log.Printf("[Langfuse] StartTrace 返回 nil，客户端未启用")
		return nil, nil
	}

	// 将 input 放入 metadata
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	if input != nil {
		metadata["input"] = input
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
		Input:     input,
	}, nil
}

// End 结束Trace，触发上报
func (tc *TraceContext) End(ctx context.Context, output interface{}) error {
	if tc == nil {
		return nil
	}
	lc := GetLangfuseClient()
	log.Printf("[Langfuse] Trace.End: traceID=%s", tc.TraceID)
	return lc.EndTrace(tc.TraceID, output)
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
	if sc == nil {
		return nil
	}
	lc := GetLangfuseClient()
	_, err := lc.UpdateSpan(ctx, sc.TraceID, sc.SpanID, output)
	return err
}

// StartGeneration 在指定Trace下开始一个Generation（LLM调用）
// 注意：这是 Trace 下的子调用，用于追踪大模型请求
func (tc *TraceContext) StartGeneration(ctx context.Context, name string, model string, messages []ChatMessage) (*GenerationContext, error) {
	return tc.StartGenerationWithParent(ctx, "", name, model, messages)
}

// StartGenerationWithParent 在指定Trace和Parent下开始一个Generation（LLM调用）
// parentObservationID: 父观察ID（如 Span ID），可为空
func (tc *TraceContext) StartGenerationWithParent(ctx context.Context, parentObservationID string, name string, model string, messages []ChatMessage) (*GenerationContext, error) {
	lc := GetLangfuseClient()
	log.Printf("[Langfuse] TraceContext.StartGenerationWithParent 被调用: name=%s, model=%s, traceID=%s, parentObservationID=%s, clientEnabled=%v, tcIsNil=%v",
		name, model, tc.TraceID, parentObservationID, lc.IsEnabled(), tc == nil)

	if !lc.IsEnabled() || tc == nil {
		log.Printf("[Langfuse] StartGenerationWithParent 返回 nil，clientEnabled=%v, tcIsNil=%v", lc.IsEnabled(), tc == nil)
		return nil, nil
	}

	generation, err := lc.CreateGeneration(ctx, tc.TraceID, parentObservationID, name, model, messages)
	if err != nil {
		log.Printf("[Langfuse] TraceContext.StartGenerationWithParent 失败: %v", err)
		return nil, err
	}
	if generation == nil {
		log.Printf("[Langfuse] TraceContext.StartGenerationWithParent 返回 nil generation")
		return nil, nil
	}

	log.Printf("[Langfuse] TraceContext.StartGenerationWithParent 成功: generationID=%s", generation.ID)
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

	usage := &UsageData{
		Input:  inputTokens,
		Output: outputTokens,
		Total:  inputTokens + outputTokens,
		Unit:   "TOKENS",
	}

	_, err := lc.UpdateGeneration(ctx, gc.TraceID, gc.GenerationID, output, usage)
	if err != nil {
		log.Printf("[Langfuse] GenerationContext.End 失败: %v", err)
		return err
	}
	log.Printf("[Langfuse] GenerationContext.End 成功: generationID=%s", gc.GenerationID)
	return nil
}

// EndWithUsage 结束Generation，使用预计算的Usage
func (gc *GenerationContext) EndWithUsage(ctx context.Context, output string, usage *UsageData) error {
	lc := GetLangfuseClient()
	if !lc.IsEnabled() || gc == nil {
		return nil
	}
	_, err := lc.UpdateGeneration(ctx, gc.TraceID, gc.GenerationID, output, usage)
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
