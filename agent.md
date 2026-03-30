## Luffot AI Agent 计算流程说明

本文档详细描述 Luffot 中 AI Agent 的计算触发时机、计算内容、内容监听方式、用户透出时机和透出形式。

---

### 整体架构

Luffot 的 AI 系统由以下核心模块组成：

```
┌─────────────────────────────────────────────────────────────────┐
│                        用户交互层                                │
│  桌宠对话 / 弹幕气泡 / 秘书汇报气泡 / 系统通知                    │
└──────────────┬──────────────────────────────────┬───────────────┘
               │                                  │
┌──────────────▼──────────────┐  ┌────────────────▼───────────────┐
│     Agent (核心智能体)       │  │   ReactiveAIChain (响应式链路)  │
│  PicoClaw Engine (LLM引擎)  │  │  Coordinator / AppSecretary    │
│  Memory (对话记忆)           │  │  SystemGuardian / CameraPatrol │
│  MessageQueryTools (工具集)  │  │  UserTracker / IntelliAnalyzer │
└──────────────┬──────────────┘  └────────────────┬───────────────┘
               │                                  │
┌──────────────▼──────────────────────────────────▼───────────────┐
│                     EventBus (全局事件总线)                       │
└──────────────┬──────────────────────────────────┬───────────────┘
               │                                  │
┌──────────────▼──────────────┐  ┌────────────────▼───────────────┐
│   EventSource (事件源)       │  │     Storage (SQLite 存储)      │
│  DingTalk / FileTail / HTTP  │  │  消息 / 画像 / 记忆 / 统计     │
└─────────────────────────────┘  └────────────────────────────────┘
```

---

### 一、Agent 核心对话（用户主动触发）

#### 触发时机

用户通过桌宠界面主动输入文字时触发。调用入口为 `Agent.Chat(userInput)` 或 `Agent.ChatWithProvider(userInput, providerName)`。

#### 计算内容

1. **检查引擎状态**：确认 PicoClaw 引擎已初始化且 API Key 已配置
2. **防重入**：通过 `isThinking` 互斥锁防止并发请求，正在思考时忽略新输入
3. **调用 PicoClaw 引擎**：`engine.ChatDirect(ctx, userInput)` → PicoClaw AgentLoop 处理
   - PicoClaw 内部管理 session 上下文（`luffot-chat` session）
   - PicoClaw 可自动调用已注册的 Tool（消息统计、最近消息查询等）
4. **记录对话**：`memory.AddTurn(userInput, reply)` 写入短期记忆 + SQLite 长期存储
5. **更新状态**：设置 `latestReply` 供桌宠读取

#### 内容监听方式

- 用户输入由桌宠 UI 层捕获，直接调用 `Agent.Chat()` 方法
- 整个过程在 **goroutine** 中异步执行，不阻塞 UI 线程

#### 透出时机与形式

- **透出时机**：LLM 返回完整回复后立即透出
- **透出形式**：通过 `onReply` 回调函数通知桌宠 PetSprite，桌宠以**对话气泡**形式展示

---

### 二、同步对话（后台任务触发）

#### 触发时机

由定时任务、后台分析等模块调用 `Agent.ChatSync(ctx, messages, providerName)`。

#### 计算内容

将自定义消息列表的最后一条消息作为输入，通过 PicoClaw 引擎同步调用 LLM，阻塞等待结果返回。

#### 透出时机与形式

- 不直接透出给用户，结果返回给调用方（如 IntelliAnalyzer、Coordinator）
- 由调用方决定是否以弹幕、气泡等形式展示

---

### 三、消息摘要（紧急消息触发）

#### 触发时机

`Manager.handleMessage()` 检测到**紧急消息**时触发。紧急消息判定逻辑在 `isUrgentMessage()` 中，基于关键词匹配（如 @提及、紧急、线上问题等）。

#### 计算内容

调用 `Agent.SummarizeMessages(messages)` → PicoClaw 引擎生成 30 字以内的活泼简洁摘要。

#### 内容监听方式

```
EventSource (钉钉/微信/文件/HTTP)
  → SourceManager.handleMessage()
    → Manager.handleMessage()
      → isUrgentMessage() 检测
        → Agent.SummarizeMessages()
```

消息来源包括：
- **钉钉 Accessibility 监听**：通过 macOS Accessibility API 或 VL 模型截图识别，定时轮询钉钉窗口
- **文件 Tail 监听**：监听指定日志文件的新增行
- **HTTP 事件接口**：外部系统通过 `POST /event/{app}/on_msg` 推送消息

#### 透出时机与形式

- **透出时机**：AI 摘要生成后立即透出；AI 失败时回退到原始消息格式
- **透出形式**：通过 `barrageDisplay.ShowAlert(alertText)` 以**桌宠秘书汇报气泡**形式展示（带 ⚡ 前缀）

---

### 四、智能消息分析器 IntelliAnalyzer（定时批量触发）

#### 触发时机

定时器驱动，按配置的 `AnalysisInterval`（默认 5 分钟）周期性触发 `analyzeBatch()`。

#### 计算内容

1. **读取未分析消息**：从 SQLite 中读取上次分析后的新消息（按 `lastAnalyzedMessageID` 追踪）
2. **消息分组**：按会话和时间间隔将消息分组为讨论上下文（`GroupMessagesIntoContexts`）
3. **加载上下文**：读取用户画像、用户记忆、会话参与度统计
4. **重要性分析**：调用 LLM 判断哪些消息值得通知用户（使用 `analyzer_importance_system/user` prompt 模板）
5. **画像与记忆更新**（每 N 批次一次）：调用 LLM 同时更新用户画像和结构化记忆

#### 内容监听方式

- 不直接监听消息流，而是从 **SQLite 数据库**中批量读取已存储的消息
- 通过 `lastAnalyzedMessageID` 实现增量分析，避免重复处理

#### 透出时机与形式

- **透出时机**：LLM 分析完成后，对每条重要通知立即透出
- **透出形式**：通过 `onAlert` 回调 → `barrageDisplay.ShowAlert()` 以**弹幕气泡**形式展示
- 画像和记忆更新不直接透出，静默写入 SQLite 供后续分析使用

---

### 五、响应式 AI 链路 ReactiveAIChain（事件驱动触发）

ReactiveAIChain 是一个多智能体协同系统，包含以下子智能体：

#### 5.1 Coordinator（AI 丞相 / 协调器）

**触发时机**：
- **事件驱动**：订阅 EventBus 上的系统事件、应用事件、环境事件、用户行为事件
- **定时驱动**：每 60 秒执行一次 `generatePeriodicInsights()` 处理累积事件

**计算内容**：
- 接收子智能体上报的事件，聚合到 `pendingEvents` 窗口
- 维护 `UserState`（当前活动、待办任务）
- 根据事件优先级和汇报策略决定是否向用户汇报
- 高优先级事件可触发 AI 摘要生成（`generateAISummary`）

**汇报策略**（防刷屏）：
- `MinReportInterval`：最小汇报间隔（默认 5 秒）
- `MaxConsecutiveReports`：最大连续汇报次数（默认 3 次）
- `ConsecutiveCooldown`：连续汇报冷却时间（默认 60 秒）
- `UrgentImmediate`：高优先级事件立即汇报

**透出形式**：生成 `UserInsight` → `onInsight` 回调 → `barrageDisplay.ShowAlert()` 以**弹幕气泡**展示，带有类型前缀（⚠️ 系统告警 / 🔔 应用通知 / 👁️ 环境提醒 / 📊 活动摘要 / ⏰ 任务提醒）

#### 5.2 AppSecretary（应用秘书）

**触发时机**：
- 收到应用消息事件时（通过 EventBus 的 `AppMessageReceived` 事件）
- 消息聚合窗口到期时（批量汇报）

**计算内容**：
- 紧急关键词检测（@提及、紧急、线上等）
- 消息聚合：在 `bufferWindow` 时间窗口内收集消息，批量处理
- 可选 AI 总结：调用 `Agent.SummarizeMessages()` 生成摘要

**透出形式**：
- 紧急消息：立即通过 EventBus 发布 `AppMessageImmediateReport` → Coordinator 处理
- 普通消息：聚合后通过 `AppMessageBatchReport` 批量上报 → Coordinator 决定是否展示

#### 5.3 SystemGuardian（系统管家）

**触发时机**：每 10 秒执行一次 `monitoringLoop()`。

**计算内容**：
- 采集系统指标：CPU 使用率、内存使用率、磁盘使用率
- 阈值检测：CPU > 80%、内存 > 85%、磁盘 > 90% 时触发告警
- 异常进程检测：单进程 CPU/内存超过 50% 视为异常
- 告警冷却：同类告警 5 分钟内不重复发送

**透出形式**：通过 EventBus 发布系统事件（`SystemCPUHigh` / `SystemMemoryOveruse` / `SystemDiskFull`）→ Coordinator 接收并以弹幕气泡展示

#### 5.4 CameraPatrol（摄像头巡查员）

**触发时机**：每 5 秒执行一次 `patrolLoop()`（需用户授权启用）。

**计算内容**：
1. 通过 macOS 摄像头 API 采集一帧图像
2. 计算帧哈希，与上一帧对比检测是否有变化
3. 调用 `Agent.AnalyzeImageBase64()` → 视觉模型（VL Model）分析图像
   - 使用 SSE 流式模式接收结果，避免推理超时
   - 检测是否有人靠近、人数、移动等

**透出形式**：通过 EventBus 发布 `EnvPersonDetected` 事件 → Coordinator 以 👁️ 前缀弹幕气泡展示

#### 5.5 UserTracker（用户习惯记录员）

**触发时机**：持续监控用户的应用切换行为。

**计算内容**：
- 记录用户使用各应用的时长和频率
- 维护长期/短期应用使用统计
- 检测用户空闲/活跃状态

**透出形式**：不直接透出给用户，数据供 Coordinator 生成活动摘要时使用

---

### 六、PicoClaw 引擎层

PicoClaw 是底层 AI 引擎，以 SDK 形式嵌入应用，负责所有 LLM 调用。

#### 初始化流程

```
main.go
  → NewPicoClawEngine(&cfg.AI)
    → convertToModelConfig()     // Luffot config → PicoClaw ModelConfig
    → CreateProviderFromConfig() // 创建 LLM Provider
    → buildPicoClawConfig()      // 构建完整 PicoClaw Config
    → NewAgentLoop()             // 创建 AgentLoop
  → RegisterLuffotTools()        // 注册消息查询工具
  → engine.Start()               // 启动 AgentLoop 后台运行
  → NewAgent(memory, engine, onReply, onToken)
```

#### 注册的 Tool

PicoClaw AgentLoop 可在对话过程中自动调用以下工具：

| 工具名 | 功能 | 触发条件 |
|--------|------|----------|
| `get_message_stats` | 获取今日消息统计 | 用户询问消息数量、统计信息 |
| `get_recent_messages` | 获取最近消息列表 | 用户询问最近收到了什么消息 |
| `get_hourly_messages` | 获取最近一小时消息 | 用户询问刚才收到了什么 |

#### 会话管理

- PicoClaw 内部管理对话 session（默认 key: `luffot-chat`）
- 当消息数超过 `SummarizeMessageThreshold` 时自动触发上下文摘要压缩
- Luffot 的 Memory 系统独立运行，负责长期对话存储到 SQLite

---

### 七、数据流总结

```
用户输入 ──→ Agent.Chat() ──→ PicoClaw Engine ──→ LLM API ──→ 桌宠对话气泡
                                    ↑
                              Tool 自动调用
                           (消息统计/查询等)

消息事件 ──→ Manager.handleMessage()
              ├──→ 弹幕显示（所有消息）
              ├──→ 紧急检测 → Agent.SummarizeMessages() → 秘书汇报气泡
              ├──→ Storage 存储
              └──→ EventBus → AppSecretary → Coordinator → 弹幕气泡

定时任务 ──→ IntelliAnalyzer.analyzeBatch()
              ├──→ 从 SQLite 读取未分析消息
              ├──→ LLM 重要性分析 → 弹幕气泡通知
              └──→ LLM 画像/记忆更新 → SQLite 静默写入

系统监控 ──→ SystemGuardian.monitoringLoop()
              └──→ 阈值告警 → EventBus → Coordinator → 弹幕气泡

摄像头   ──→ CameraPatrol.patrolLoop()
              └──→ 视觉模型分析 → EventBus → Coordinator → 弹幕气泡
```

---

### 八、透出形式汇总

| 场景 | 触发方式 | 透出形式 | 前缀/样式 |
|------|----------|----------|-----------|
| 用户主动对话 | 用户输入 | 桌宠对话气泡 | 无 |
| 紧急消息摘要 | 关键词检测 | 秘书汇报气泡 | ⚡ |
| 智能消息分析 | 定时批量 | 弹幕气泡 | 由 LLM 生成 |
| 系统资源告警 | 阈值检测 | 弹幕气泡 | ⚠️ |
| 应用紧急通知 | 关键词/优先级 | 弹幕气泡 | 🔔 |
| 环境人物检测 | 摄像头巡查 | 弹幕气泡 | 👁️ |
| 活动摘要 | 定时聚合 | 弹幕气泡 | 📊 |
| 任务提醒 | 事件驱动 | 弹幕气泡 | ⏰ |
