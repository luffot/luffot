package eventsource

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework ApplicationServices -framework CoreFoundation -framework AppKit

#import <ApplicationServices/ApplicationServices.h>
#import <AppKit/AppKit.h>
#import <stdlib.h>
#import <string.h>

// AXObserver 管理函数在 dingtalk_observer.m 中实现，此处声明以供调用
extern void runObserverLoop(void);
extern void stopObserverLoop(void);

// DingTalkMessage 单条消息结构
typedef struct {
    char* sender;
    char* content;
    char* time_str;
} DingTalkMessage;

// DingTalkResult 单个进程的读取结果
typedef struct {
    char*            app_name;       // 应用标识：dingtalk / alidingding
    char*            session_name;   // 窗口标题（会话名）
    DingTalkMessage* messages;
    int              message_count;
    int              has_error;
    char*            error_msg;
} DingTalkResult;

// DingTalkBatchResult 批量读取结果（支持多进程）
typedef struct {
    DingTalkResult** results;
    int              result_count;
} DingTalkBatchResult;

// freeDingTalkResult 释放单个结果内存
static void freeDingTalkResult(DingTalkResult* result) {
    if (!result) return;
    if (result->app_name)     free(result->app_name);
    if (result->session_name) free(result->session_name);
    if (result->error_msg)    free(result->error_msg);
    if (result->messages) {
        for (int i = 0; i < result->message_count; i++) {
            if (result->messages[i].sender)   free(result->messages[i].sender);
            if (result->messages[i].content)  free(result->messages[i].content);
            if (result->messages[i].time_str) free(result->messages[i].time_str);
        }
        free(result->messages);
    }
    free(result);
}

// freeDingTalkBatchResult 释放批量结果内存
static void freeDingTalkBatchResult(DingTalkBatchResult* batch) {
    if (!batch) return;
    for (int i = 0; i < batch->result_count; i++) {
        freeDingTalkResult(batch->results[i]);
    }
    if (batch->results) free(batch->results);
    free(batch);
}

// copyStr 安全复制 NSString 到 C 字符串
static char* copyStr(NSString* s) {
    if (!s || [s length] == 0) return strdup("");
    return strdup([s UTF8String]);
}

// collectTextFromElement 递归收集 AXUIElement 的文本内容
// maxDepth 控制递归深度，避免遍历过深
static void collectTextFromElement(AXUIElementRef element, NSMutableArray* texts, int depth, int maxDepth) {
    if (depth > maxDepth) return;

    // 读取当前元素的 value（文本内容）
    CFTypeRef valueRef = NULL;
    AXUIElementCopyAttributeValue(element, kAXValueAttribute, &valueRef);
    if (valueRef) {
        if (CFGetTypeID(valueRef) == CFStringGetTypeID()) {
            NSString* value = (NSString*)valueRef;
            if (value && [value length] > 0 && [value length] < 2000) {
                [texts addObject:[[NSString alloc] initWithString:value]];
            }
        }
        CFRelease(valueRef);
    }

    // 读取子元素
    CFTypeRef childrenRef = NULL;
    AXUIElementCopyAttributeValue(element, kAXChildrenAttribute, &childrenRef);
    if (!childrenRef) return;

    NSArray* children = (NSArray*)childrenRef;
    for (id child in children) {
        AXUIElementRef childEl = (AXUIElementRef)child;
        collectTextFromElement(childEl, texts, depth + 1, maxDepth);
    }
    CFRelease(childrenRef);
}

// findMessageContainerAndCollect 在窗口元素树中寻找消息列表容器（AXScrollArea/AXList/AXTable）
// 找到后只读取容器内的文本，避免读取整个窗口（导航栏、输入框等无关内容）
// 如果找不到合适的容器，回退到读取整个窗口
static void findMessageContainerAndCollect(AXUIElementRef element, NSMutableArray* texts, int depth) {
    if (depth > 6) return;

    // 读取 role
    CFTypeRef roleRef = NULL;
    AXUIElementCopyAttributeValue(element, kAXRoleAttribute, &roleRef);
    NSString* role = roleRef ? (NSString*)roleRef : nil;

    // 读取 description / identifier，用于判断是否是消息区域
    CFTypeRef descRef = NULL;
    AXUIElementCopyAttributeValue(element, kAXDescriptionAttribute, &descRef);
    NSString* desc = descRef ? (NSString*)descRef : @"";

    CFTypeRef identRef = NULL;
    AXUIElementCopyAttributeValue(element, kAXIdentifierAttribute, &identRef);
    NSString* ident = identRef ? (NSString*)identRef : @"";

    BOOL isScrollArea = [role isEqualToString:@"AXScrollArea"];
    BOOL isList       = [role isEqualToString:@"AXList"];
    BOOL isTable      = [role isEqualToString:@"AXTable"];

    // 判断是否是消息列表容器：
    // 钉钉消息区域通常是一个较大的 AXScrollArea，description 或 identifier 含有 chat/message/conversation 等关键词
    // 或者是窗口中最大的 AXScrollArea（通过子元素数量判断）
    BOOL looksLikeMessageArea = NO;
    if (isScrollArea || isList || isTable) {
        NSString* combined = [[desc stringByAppendingString:@" "] stringByAppendingString:ident];
        NSString* lower = [combined lowercaseString];
        if ([lower containsString:@"chat"]    ||
            [lower containsString:@"message"] ||
            [lower containsString:@"convers"] ||
            [lower containsString:@"session"] ||
            [lower containsString:@"消息"]    ||
            [lower containsString:@"聊天"]) {
            looksLikeMessageArea = YES;
        }
        // 如果 description/identifier 没有明确关键词，检查子元素数量
        // 消息列表通常有很多子元素（每条消息一个 cell）
        if (!looksLikeMessageArea) {
            CFTypeRef childrenRef2 = NULL;
            AXUIElementCopyAttributeValue(element, kAXChildrenAttribute, &childrenRef2);
            if (childrenRef2) {
                NSArray* ch = (NSArray*)childrenRef2;
                if ([ch count] >= 3) {
                    looksLikeMessageArea = YES;
                }
                CFRelease(childrenRef2);
            }
        }
    }

    if (roleRef) CFRelease(roleRef);
    if (descRef) CFRelease(descRef);
    if (identRef) CFRelease(identRef);

    if (looksLikeMessageArea) {
        // 找到消息容器，递归读取其内部文本（深度限制 10 层）
        collectTextFromElement(element, texts, 0, 10);
        return;
    }

    // 未找到，继续向下搜索
    CFTypeRef childrenRef = NULL;
    AXUIElementCopyAttributeValue(element, kAXChildrenAttribute, &childrenRef);
    if (!childrenRef) return;

    NSArray* children = (NSArray*)childrenRef;
    for (id child in children) {
        AXUIElementRef childEl = (AXUIElementRef)child;
        findMessageContainerAndCollect(childEl, texts, depth + 1);
        // 如果已经收集到文本，说明找到了消息区域，停止搜索
        if ([texts count] > 0) break;
    }
    CFRelease(childrenRef);
}

// readWindowForApp 读取单个应用进程的窗口文本
// appName: 应用标识字符串（dingtalk / alidingding）
// pid: 进程 ID
static DingTalkResult* readWindowForApp(const char* appName, pid_t pid) {
    DingTalkResult* result = (DingTalkResult*)calloc(1, sizeof(DingTalkResult));
    result->app_name = strdup(appName);

    AXUIElementRef appElement = AXUIElementCreateApplication(pid);
    if (!appElement) {
        result->has_error = 1;
        result->error_msg = strdup("Failed to create AXUIElement");
        return result;
    }

    // 获取所有窗口
    CFTypeRef windowsRef = NULL;
    AXError axErr = AXUIElementCopyAttributeValue(appElement, kAXWindowsAttribute, &windowsRef);
    if (axErr != kAXErrorSuccess || !windowsRef) {
        CFRelease(appElement);
        result->has_error = 1;
        result->error_msg = strdup("Cannot get windows - check Accessibility permission");
        return result;
    }

    // 非 ARC 模式：直接桥接，不转移所有权
    NSArray* windows = (NSArray*)windowsRef;
    if ([windows count] == 0) {
        CFRelease(windowsRef);
        CFRelease(appElement);
        result->has_error = 1;
        result->error_msg = strdup("No windows found");
        return result;
    }

    // 取第一个（主）窗口
    AXUIElementRef mainWindow = (AXUIElementRef)[windows objectAtIndex:0];

    // 读取窗口标题作为 session 名称
    CFTypeRef titleRef = NULL;
    AXUIElementCopyAttributeValue(mainWindow, kAXTitleAttribute, &titleRef);
    if (titleRef) {
        result->session_name = copyStr((NSString*)titleRef);
        CFRelease(titleRef);
    } else {
        result->session_name = strdup("");
    }

    // 优先在消息列表容器中收集文本；若找不到容器则回退到全窗口读取
    NSMutableArray* texts = [NSMutableArray array];
    findMessageContainerAndCollect(mainWindow, texts, 0);
    if ([texts count] == 0) {
        // 回退：读取整个窗口（深度 8）
        collectTextFromElement(mainWindow, texts, 0, 8);
    }

    CFRelease(windowsRef);
    CFRelease(appElement);

    // 将文本数组转为 C 结构
    int count = (int)[texts count];
    result->messages = (DingTalkMessage*)calloc(count, sizeof(DingTalkMessage));
    result->message_count = count;
    for (int i = 0; i < count; i++) {
        NSString* text = texts[i];
        result->messages[i].sender   = strdup("");
        result->messages[i].content  = copyStr(text);
        result->messages[i].time_str = strdup("");
    }

    return result;
}

// readAllDingTalkWindows 同时读取普通钉钉和阿里钉两个进程的窗口
// 普通钉钉 Bundle ID: com.alibaba.DingTalkMac
// 阿里钉   Bundle ID: dd.work.exclusive4aliding
// 返回堆分配的 DingTalkBatchResult，调用方负责调用 freeDingTalkBatchResult 释放
static DingTalkBatchResult* readAllDingTalkWindows() {
    DingTalkBatchResult* batch = (DingTalkBatchResult*)calloc(1, sizeof(DingTalkBatchResult));
    // 最多 2 个进程
    batch->results = (DingTalkResult**)calloc(2, sizeof(DingTalkResult*));
    batch->result_count = 0;

    @autoreleasepool {
        NSArray* runningApps = [[NSWorkspace sharedWorkspace] runningApplications];

        for (NSRunningApplication* app in runningApps) {
            NSString* bundleID = [app bundleIdentifier];
            if (!bundleID) continue;

            const char* appName = NULL;
            if ([bundleID isEqualToString:@"com.alibaba.DingTalkMac"]) {
                appName = "dingtalk";
            } else if ([bundleID isEqualToString:@"dd.work.exclusive4aliding"]) {
                appName = "alidingding";
            }

            if (appName != NULL) {
                pid_t pid = [app processIdentifier];
                DingTalkResult* result = readWindowForApp(appName, pid);
                batch->results[batch->result_count] = result;
                batch->result_count++;
                if (batch->result_count >= 2) break;
            }
        }
    }

    return batch;
}

// isAccessibilityGranted 静默检查辅助功能权限（不弹出系统授权对话框）
static int isAccessibilityGranted() {
    return AXIsProcessTrusted() ? 1 : 0;
}

// requestAccessibilityPermission 检查辅助功能权限并在未授权时弹出系统授权对话框
static int requestAccessibilityPermission() {
    NSDictionary* options = @{(__bridge NSString*)kAXTrustedCheckOptionPrompt: @YES};
    return AXIsProcessTrustedWithOptions((__bridge CFDictionaryRef)options) ? 1 : 0;
}
*/
import "C"

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/luffot/luffot/pkg/ai"
	"github.com/luffot/luffot/pkg/config"
	"github.com/luffot/luffot/pkg/prompt"
)

// uiChangeChan 接收来自 C 层 AXObserver 的 UI 变化通知
// key 为 appName，发送时不阻塞（使用 select + default）
var uiChangeChan = make(chan string, 32)

// goAXChangeCallback 由 C 层 axObserverCallback 调用，将 UI 变化事件发送到 Go channel
//
//export goAXChangeCallback
func goAXChangeCallback(appName *C.char) {
	name := C.GoString(appName)
	select {
	case uiChangeChan <- name:
	default:
		// channel 满时丢弃，避免阻塞 CFRunLoop 线程
	}
}

// DingTalkSourceConfig 钉钉监听源配置
type DingTalkSourceConfig struct {
	// CheckInterval 轮询间隔，建议 2-5 秒
	CheckInterval time.Duration
	// MaxCacheSize 去重缓存最大条数
	MaxCacheSize int
	// Agent AI 智能体，用于调用视觉模型分析截图（nil 时回退到 Accessibility API 方式）
	Agent *ai.Agent
}

// DefaultDingTalkSourceConfig 默认配置
var DefaultDingTalkSourceConfig = DingTalkSourceConfig{
	CheckInterval: 3 * time.Second,
	MaxCacheSize:  500,
}

// DingTalkSource 钉钉消息监听源
// 通过 macOS Accessibility API 读取钉钉窗口 UI 元素，提取聊天消息
// 使用 AXObserver 监听 UI 变化事件，变化时立即触发读取，同时保留兜底轮询
type DingTalkSource struct {
	config  DingTalkSourceConfig
	agent   *ai.Agent
	mu      sync.RWMutex
	running bool
	// seenMsgHashes 存储已处理消息的 SHA256 hash（sender+content），用于去重
	seenMsgHashes map[string]bool
	// observerRunLoop 保存 AXObserver 所在线程的 CFRunLoop 引用，用于停止
	observerRunLoop unsafe.Pointer
	// 时间正则，用于识别消息时间戳行
	timeRegexp *regexp.Regexp
}

// NewDingTalkSource 创建钉钉监听源
func NewDingTalkSource(cfg DingTalkSourceConfig) *DingTalkSource {
	if cfg.CheckInterval <= 0 {
		cfg.CheckInterval = DefaultDingTalkSourceConfig.CheckInterval
	}
	if cfg.MaxCacheSize <= 0 {
		cfg.MaxCacheSize = DefaultDingTalkSourceConfig.MaxCacheSize
	}
	return &DingTalkSource{
		config:        cfg,
		agent:         cfg.Agent,
		seenMsgHashes: make(map[string]bool, cfg.MaxCacheSize),
		// 匹配常见时间格式：09:30、9:30、09:30:00
		timeRegexp: regexp.MustCompile(`\b\d{1,2}:\d{2}(:\d{2})?\b`),
	}
}

// randomColors 预定义的随机好看的颜色列表
var randomColors = []string{
	"#FF6B6B", // 珊瑚红
	"#4ECDC4", // 青绿色
	"#45B7D1", // 天蓝色
	"#96CEB4", // 薄荷绿
	"#FFEAA7", // 奶油黄
	"#DDA0DD", // 梅花紫
	"#98D8C8", // 浅青色
	"#F7DC6F", // 金黄色
	"#BB8FCE", // 薰衣草紫
	"#85C1E2", // 浅蓝色
	"#F8B195", // 鲑鱼粉
	"#C06C84", // 玫瑰粉
	"#6C5B7B", // 紫灰色
	"#355C7D", // 深蓝色
	"#A8E6CF", // 薄荷青
	"#DCEDC1", // 柠檬绿
	"#FFD3B6", // 蜜桃色
	"#FFAAA5", // 浅珊瑚
	"#FF8B94", // 粉红色
	"#C7CEEA", // 淡紫色
}

// getRandomColor 返回一个随机的好看的颜色
func getRandomColor() string {
	return randomColors[rand.Intn(len(randomColors))]
}

// Name 返回数据源名称
func (s *DingTalkSource) Name() string {
	return "dingtalk-accessibility"
}

// IsRunning 检查是否运行中
func (s *DingTalkSource) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// Stop 停止监听
func (s *DingTalkSource) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = false
	return nil
}

// appWindowTexts 单个进程的窗口文本快照
type appWindowTexts struct {
	appName     string
	sessionName string
	texts       []string
}

// Start 启动钉钉窗口监听
// 同时使用两种机制触发读取：
//  1. AXObserver：UI 发生变化时立即触发（精准、低延迟）
//  2. 兜底轮询：每 30 秒强制读取一次，防止 AXObserver 漏报
func (s *DingTalkSource) Start(ctx context.Context, handler MessageEventHandler) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = true
	s.mu.Unlock()

	// 检查辅助功能权限：先静默检查，已授权则不弹窗；未授权时才弹出系统授权对话框
	if C.isAccessibilityGranted() == 0 {
		fmt.Println("[DingTalk] ⚠️  未授予辅助功能权限，正在弹出系统授权对话框...")
		C.requestAccessibilityPermission()
		fmt.Println("[DingTalk]    路径：系统设置 → 隐私与安全性 → 辅助功能")
	} else {
		fmt.Println("[DingTalk] ✅ 辅助功能权限已就绪")
	}

	fmt.Println("[DingTalk] 开始监听钉钉/阿里钉窗口消息（AXObserver + 30s 兜底轮询）")

	// 每个进程独立维护上次文本快照，key 为 appName
	lastTextsByApp := make(map[string][]string)

	// processSnapshots 对钉钉进程截图，调用 VL 模型提取最新 IM 消息
	// 若 AI Agent 未配置，则回退到原有的 Accessibility API 读取方式
	processSnapshots := func() {
		if s.agent != nil && s.agent.IsEnabled() {
			s.processSnapshotsViaVLModel(handler)
		} else {
			s.processSnapshotsViaAccessibility(handler, lastTextsByApp)
		}
	}

	// 启动 AXObserver CFRunLoop（独立 goroutine，不阻塞主逻辑）
	// CFRunLoop 必须在同一个 OS 线程上运行，使用 runtime.LockOSThread
	observerDone := make(chan struct{})
	go func() {
		defer close(observerDone)
		// 锁定当前 goroutine 到 OS 线程，满足 CFRunLoop 要求
		// 注意：CGo 调用本身会处理线程切换，runObserverLoop 内部调用 CFRunLoopRun 会阻塞
		C.runObserverLoop()
	}()

	// 兜底轮询定时器（30 秒）
	fallbackTicker := time.NewTicker(30 * time.Second)
	defer fallbackTicker.Stop()

	// 防抖：AXObserver 可能短时间内触发大量通知，使用 100ms 防抖窗口
	debounceTimer := time.NewTimer(0)
	<-debounceTimer.C // 消耗初始触发
	debounceTimer.Stop()
	pendingRead := false

	for {
		select {
		case <-ctx.Done():
			s.mu.Lock()
			s.running = false
			s.mu.Unlock()
			// 停止 CFRunLoop
			C.stopObserverLoop()
			<-observerDone
			return nil

		case <-uiChangeChan:
			// UI 发生变化，启动防抖定时器（100ms 后触发读取）
			if !pendingRead {
				pendingRead = true
				debounceTimer.Reset(100 * time.Millisecond)
			}

		case <-debounceTimer.C:
			// 防抖窗口结束，执行读取
			pendingRead = false
			processSnapshots()

		case <-fallbackTicker.C:
			// 兜底轮询
			processSnapshots()
		}
	}
}

// readAllWindowTexts 调用 CGo 批量读取所有钉钉进程窗口文本
func (s *DingTalkSource) readAllWindowTexts() []appWindowTexts {
	batch := C.readAllDingTalkWindows()
	if batch == nil {
		return nil
	}
	defer C.freeDingTalkBatchResult(batch)

	resultCount := int(batch.result_count)
	if resultCount == 0 {
		return nil
	}

	// 将 C 指针数组转为 Go slice
	resultPtrs := (*[2]*C.DingTalkResult)(unsafe.Pointer(batch.results))[:resultCount:resultCount]

	snapshots := make([]appWindowTexts, 0, resultCount)
	for i := 0; i < resultCount; i++ {
		result := resultPtrs[i]
		if result == nil || result.has_error != 0 {
			continue
		}

		appName := C.GoString(result.app_name)
		sessionName := C.GoString(result.session_name)
		count := int(result.message_count)

		var texts []string
		if count > 0 {
			msgSlice := (*[1 << 20]C.DingTalkMessage)(unsafe.Pointer(result.messages))[:count:count]
			texts = make([]string, 0, count)
			for j := 0; j < count; j++ {
				content := strings.TrimSpace(C.GoString(msgSlice[j].content))
				if content != "" {
					texts = append(texts, content)
				}
			}
		}

		snapshots = append(snapshots, appWindowTexts{
			appName:     appName,
			sessionName: sessionName,
			texts:       texts,
		})
	}

	return snapshots
}

// diffTexts 找出 newTexts 中相比 oldTexts 新增的行
func (s *DingTalkSource) diffTexts(oldTexts, newTexts []string) []string {
	if len(oldTexts) == 0 {
		// 首次读取，不作为"新消息"推送（避免历史消息刷屏）
		return nil
	}

	oldSet := make(map[string]bool, len(oldTexts))
	for _, t := range oldTexts {
		oldSet[t] = true
	}

	var added []string
	for _, t := range newTexts {
		if !oldSet[t] {
			added = append(added, t)
		}
	}
	return added
}

// parseMessages 将原始文本行解析为 MessageEvent 列表
// 钉钉消息 UI 通常呈现为：发送者名 → 时间 → 消息内容 的连续文本块
// appName 区分来源：dingtalk（普通钉钉）或 alidingding（阿里钉）
func (s *DingTalkSource) parseMessages(texts []string, sessionName, appName string) []*MessageEvent {
	var events []*MessageEvent
	now := time.Now()

	// 状态机：追踪当前正在组装的消息的发送者和时间
	currentSender := ""
	currentTime := ""
	// lastSender 记录上一条消息的发送者，用于连续消息场景（同一人连发多条时不重复显示名字）
	lastSender := ""

	for _, text := range texts {
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}

		// 跳过过长的文本（可能是整个聊天区域的拼接，不是单条消息）
		if len([]rune(text)) > 500 {
			continue
		}

		// 检测是否是时间戳行（如 "09:30"、"昨天 09:30"）
		if s.isTimeLine(text) {
			currentTime = text
			continue
		}

		// 检测是否是发送者行（短文本，不含时间，不含换行）
		if s.isSenderLine(text) && currentSender == "" {
			currentSender = text
			continue
		}

		// 其余视为消息内容
		// 先尝试解析内联格式（"发送者 09:30 内容"）
		if parsed := s.tryParseInlineMessage(text, sessionName, appName, now); parsed != nil {
			parsed.Color = getRandomColor()
			events = append(events, parsed)
			lastSender = parsed.Sender
			currentSender = ""
			currentTime = ""
			continue
		}

		// 确定发送者：优先使用已识别的 currentSender，其次从内容中提取，再次使用上一条的发送者
		sender := currentSender
		content := text

		if sender == "" {
			// 尝试从 "发送者: 内容" 格式中提取
			if extracted := s.extractSenderFromContent(text); extracted != "" {
				sender = extracted
				// 从内容中去掉发送者前缀
				for _, sep := range []string{": ", "：", ":"} {
					if idx := strings.Index(text, sep); idx > 0 {
						candidate := strings.TrimSpace(text[:idx])
						if candidate == sender {
							content = strings.TrimSpace(text[idx+len(sep):])
							break
						}
					}
				}
			}
		}

		if sender == "" && lastSender != "" {
			// 连续消息场景：同一人连发多条时钉钉不重复显示名字
			sender = lastSender
		}

		if sender == "" {
			sender = "未知"
		}

		event := &MessageEvent{
			App:       appName,
			Session:   sessionName,
			Sender:    sender,
			Content:   content,
			RawTime:   currentTime,
			Timestamp: now,
			Color:     getRandomColor(),
		}

		events = append(events, event)
		lastSender = sender
		// 消息发出后重置发送者（下一条消息可能是不同人发送）
		currentSender = ""
		currentTime = ""
	}

	return events
}

// isTimeLine 判断是否是时间行
func (s *DingTalkSource) isTimeLine(text string) bool {
	// 纯时间：09:30
	if s.timeRegexp.MatchString(text) && len([]rune(text)) < 30 {
		// 检查文本主体是否就是时间（可能带日期前缀）
		cleaned := s.timeRegexp.ReplaceAllString(text, "")
		cleaned = strings.TrimSpace(cleaned)
		// 剩余部分只有日期词汇
		dateWords := []string{"昨天", "前天", "今天", "星期", "周", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}
		for _, word := range dateWords {
			cleaned = strings.ReplaceAll(cleaned, word, "")
		}
		cleaned = strings.TrimSpace(cleaned)
		if cleaned == "" {
			return true
		}
	}
	return false
}

// isSenderLine 判断是否是发送者行（短文本，不含时间）
func (s *DingTalkSource) isSenderLine(text string) bool {
	runeLen := len([]rune(text))
	if runeLen < 1 || runeLen > 30 {
		return false
	}
	// 不含时间格式
	if s.timeRegexp.MatchString(text) {
		return false
	}
	// 不含换行
	if strings.Contains(text, "\n") {
		return false
	}
	// 不含常见消息内容特征（标点较少）
	punctCount := strings.Count(text, "，") + strings.Count(text, "。") +
		strings.Count(text, "？") + strings.Count(text, "！") +
		strings.Count(text, "、") + strings.Count(text, "…")
	if punctCount > 2 {
		return false
	}
	return true
}

// extractSenderFromContent 从内容中提取发送者（"发送者: 内容" 格式）
func (s *DingTalkSource) extractSenderFromContent(text string) string {
	// 尝试 "发送者: 内容" 格式
	for _, sep := range []string{": ", "：", ":"} {
		if idx := strings.Index(text, sep); idx > 0 && idx < 40 {
			candidate := strings.TrimSpace(text[:idx])
			if len([]rune(candidate)) <= 20 && !s.timeRegexp.MatchString(candidate) {
				return candidate
			}
		}
	}
	return ""
}

// tryParseInlineMessage 尝试解析内联格式消息（"发送者 09:30 内容"）
// appName 区分来源：dingtalk（普通钉钉）或 alidingding（阿里钉）
func (s *DingTalkSource) tryParseInlineMessage(text, sessionName, appName string, ts time.Time) *MessageEvent {
	// 匹配 "发送者 HH:MM 内容" 格式
	inlineRe := regexp.MustCompile(`^(.{1,30}?)\s+(\d{1,2}:\d{2}(?::\d{2})?)\s+(.+)$`)
	matches := inlineRe.FindStringSubmatch(text)
	if len(matches) == 4 {
		sender := strings.TrimSpace(matches[1])
		rawTime := strings.TrimSpace(matches[2])
		content := strings.TrimSpace(matches[3])
		if sender != "" && content != "" && len([]rune(sender)) <= 20 {
			return &MessageEvent{
				App:       appName,
				Session:   sessionName,
				Sender:    sender,
				Content:   content,
				RawTime:   rawTime,
				Timestamp: ts,
			}
		}
	}
	return nil
}

// msgHash 计算消息的 SHA256 去重 hash（基于 sender + content）
// 使用 SHA256 前 16 字节的 hex 字符串，既唯一又紧凑
func msgHash(sender, content string) string {
	h := sha256.Sum256([]byte(sender + "\x00" + content))
	return fmt.Sprintf("%x", h[:16])
}

// processSnapshotsViaAccessibility 使用 Accessibility API 读取窗口文本并处理新消息
// 现在消息会先交给 AppSecretary 进行过滤和汇报决策
func (s *DingTalkSource) processSnapshotsViaAccessibility(handler MessageEventHandler, lastTextsByApp map[string][]string) {
	snapshots := s.readAllWindowTexts()
	for _, snap := range snapshots {
		lastTexts := lastTextsByApp[snap.appName]
		newTexts := s.diffTexts(lastTexts, snap.texts)
		if len(newTexts) > 0 {
			events := s.parseMessages(newTexts, snap.sessionName, snap.appName)
			for _, event := range events {
				if !s.isDuplicate(event) {
					// 同时保留原始 handler 用于弹幕显示
					handler(event)
				}
			}
		}
		lastTextsByApp[snap.appName] = snap.texts
	}
}

// appScreenshot 单个钉钉进程的截图结果
type appScreenshot struct {
	appName     string
	sessionName string
	base64JPEG  string
}

// screenshotDingTalkApps 对所有运行中的钉钉进程截图，返回 base64 JPEG 列表
// 使用 macOS screencapture 命令截取指定进程的窗口
func (s *DingTalkSource) screenshotDingTalkApps() []appScreenshot {
	// 先通过 Accessibility API 获取进程信息（appName、sessionName）
	snapshots := s.readAllWindowTexts()
	if len(snapshots) == 0 {
		return nil
	}

	var screenshots []appScreenshot
	for _, snap := range snapshots {
		// 用 screencapture 截取对应进程名的窗口
		// -l 参数需要 windowID，这里改用 -R 截取全屏后裁剪，或直接截全屏
		// 最简单可靠的方式：截全屏（钉钉通常是前台窗口）
		tmpFile, err := os.CreateTemp("", "dingtalk-screenshot-*.jpg")
		if err != nil {
			fmt.Printf("[DingTalk] 创建临时截图文件失败: %v\n", err)
			continue
		}
		tmpPath := tmpFile.Name()
		tmpFile.Close()

		// screencapture -x 静默截图（不播放声音），-t jpg 输出 JPEG
		cmd := exec.Command("screencapture", "-x", "-t", "jpg", tmpPath)
		if err := cmd.Run(); err != nil {
			fmt.Printf("[DingTalk] screencapture 失败: %v\n", err)
			os.Remove(tmpPath)
			continue
		}

		imageData, err := os.ReadFile(tmpPath)
		os.Remove(tmpPath)
		if err != nil {
			fmt.Printf("[DingTalk] 读取截图文件失败: %v\n", err)
			continue
		}

		screenshots = append(screenshots, appScreenshot{
			appName:     snap.appName,
			sessionName: snap.sessionName,
			base64JPEG:  base64.StdEncoding.EncodeToString(imageData),
		})
	}
	return screenshots
}

// processSnapshotsViaVLModel 截图钉钉窗口，调用 VL 模型提取消息
// 现在消息会先交给 AppSecretary 进行过滤和汇报决策
func (s *DingTalkSource) processSnapshotsViaVLModel(handler MessageEventHandler) {
	screenshots := s.screenshotDingTalkApps()
	if len(screenshots) == 0 {
		return
	}

	// 从配置中获取提示词
	prompt := s.getVLModelPrompt()

	now := time.Now()
	for _, shot := range screenshots {
		rawContent, err := s.agent.AnalyzeImageBase64(shot.base64JPEG, prompt, "vlmodel")
		if err != nil {
			fmt.Printf("[DingTalk] VL 模型调用失败 (app=%s): %v\n", shot.appName, err)
			continue
		}

		rawContent = strings.TrimSpace(rawContent)
		if rawContent == "" || strings.EqualFold(rawContent, "NONE") {
			continue
		}

		for _, line := range strings.Split(rawContent, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.EqualFold(line, "NONE") {
				continue
			}

			sender, rawTime, content := parseVLModelMessageLine(line)
			if content == "" {
				continue
			}

			event := &MessageEvent{
				App:       shot.appName,
				Session:   shot.sessionName,
				Sender:    sender,
				Content:   content,
				RawTime:   rawTime,
				Timestamp: now,
				Color:     getRandomColor(),
			}

			if !s.isDuplicate(event) {
				// 同时保留原始 handler 用于弹幕显示
				handler(event)
			}
		}
	}
}

// getVLModelPrompt 获取 VLModel 识别提示词
// 优先从配置中读取，如果配置为空则使用默认提示词
func (s *DingTalkSource) getVLModelPrompt() string {
	// 尝试从配置中获取提示词名称
	appCfg, err := config.GetApp("dingtalk")
	if err == nil && appCfg.ProcessMonitor.VLModelPrompt != "" {
		// 从提示词管理中加载
		if content, err := prompt.Load(appCfg.ProcessMonitor.VLModelPrompt); err == nil {
			return content
		}
	}

	// 使用默认提示词
	return prompt.DefaultContent("vlmodel_message_extract")
}

// parseVLModelMessageLine 解析 VL 模型输出的单行消息
// 支持两种格式：
//   - "发送者 HH:MM: 消息内容"（带时间）
//   - "发送者: 消息内容"（不带时间）
//
// 返回 sender, rawTime, content
func parseVLModelMessageLine(line string) (sender, rawTime, content string) {
	// 先尝试带时间的格式："发送者 HH:MM: 消息内容"
	timeInlineRe := regexp.MustCompile(`^(.{1,30}?)\s+(\d{1,2}:\d{2}(?::\d{2})?)\s*[:：]\s*(.+)$`)
	matches := timeInlineRe.FindStringSubmatch(line)
	if len(matches) == 4 {
		sender = strings.TrimSpace(matches[1])
		rawTime = strings.TrimSpace(matches[2])
		content = strings.TrimSpace(matches[3])
		if sender != "" && content != "" {
			return sender, rawTime, content
		}
	}

	// 再尝试不带时间的格式："发送者: 消息内容"
	for _, sep := range []string{": ", "："} {
		if idx := strings.Index(line, sep); idx > 0 {
			candidate := strings.TrimSpace(line[:idx])
			body := strings.TrimSpace(line[idx+len(sep):])
			if candidate != "" && body != "" && len([]rune(candidate)) <= 30 {
				return candidate, "", body
			}
		}
	}

	// 无法识别格式，整行作为内容
	trimmed := strings.TrimSpace(line)
	if trimmed != "" {
		return "未知", "", trimmed
	}
	return "", "", ""
}

// isDuplicate 检查消息是否已处理过（基于 SHA256 hash 去重）
// 相同发送人 + 相同内容的消息只展示一次弹幕
func (s *DingTalkSource) isDuplicate(event *MessageEvent) bool {
	hash := msgHash(event.Sender, event.Content)

	s.mu.RLock()
	exists := s.seenMsgHashes[hash]
	s.mu.RUnlock()

	if exists {
		return true
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.seenMsgHashes[hash] = true

	// 缓存超限时清理一半，防止内存无限增长
	if len(s.seenMsgHashes) > s.config.MaxCacheSize {
		newCache := make(map[string]bool, s.config.MaxCacheSize/2)
		count := 0
		for k, v := range s.seenMsgHashes {
			if count >= s.config.MaxCacheSize/2 {
				break
			}
			newCache[k] = v
			count++
		}
		s.seenMsgHashes = newCache
	}

	return false
}
