// dingtalk_observer.m
// AXObserver 管理模块，独立编译以便 include _cgo_export.h
// CGo 会自动将同目录下的 .m 文件一起编译

#import <ApplicationServices/ApplicationServices.h>
#import <AppKit/AppKit.h>
#import <stdlib.h>
#import <string.h>

// 引入 CGo 自动生成的导出头文件，包含 goAXChangeCallback 的声明
#include "_cgo_export.h"

// axObserverCallback 是传给 AXObserverCreate 的 C 回调
// 当被监听的 AX 通知触发时，调用 Go 侧导出函数 goAXChangeCallback
static void axObserverCallback(AXObserverRef observer,
                                AXUIElementRef element,
                                CFStringRef notification,
                                void* refcon) {
    // refcon 存储的是 strdup 分配的 appName，转为非 const 以匹配 CGo 导出签名
    char* appName = (char*)refcon;
    goAXChangeCallback(appName);
}

// ObserverEntry 保存单个进程的 AXObserver 信息
typedef struct {
    AXObserverRef  observer;
    AXUIElementRef appElement;
    char*          appName;
} ObserverEntry;

// 全局 observer 列表（最多 2 个进程）
static ObserverEntry gObservers[2];
static int           gObserverCount = 0;

// gObserverRunLoop 保存 observer 线程的 CFRunLoop 引用，用于从外部停止
static CFRunLoopRef gObserverRunLoop = NULL;

// registerAXObserverForPid 为指定进程注册 AXObserver
static void registerAXObserverForPid(pid_t pid, const char* appName) {
    if (gObserverCount >= 2) return;

    AXUIElementRef appElement = AXUIElementCreateApplication(pid);
    if (!appElement) return;

    AXObserverRef observer = NULL;
    AXError err = AXObserverCreate(pid, axObserverCallback, &observer);
    if (err != kAXErrorSuccess || !observer) {
        CFRelease(appElement);
        return;
    }

    char* appNameCopy = strdup(appName);

    // kAXValueChangedNotification：文本内容变化（新消息到来时消息列表内容变化）
    AXObserverAddNotification(observer, appElement, kAXValueChangedNotification, (void*)appNameCopy);
    // kAXFocusedUIElementChangedNotification：焦点变化（切换会话时）
    AXObserverAddNotification(observer, appElement, kAXFocusedUIElementChangedNotification, (void*)appNameCopy);
    // kAXWindowCreatedNotification：新窗口（弹出消息窗口）
    AXObserverAddNotification(observer, appElement, kAXWindowCreatedNotification, (void*)appNameCopy);

    CFRunLoopAddSource(CFRunLoopGetCurrent(),
                       AXObserverGetRunLoopSource(observer),
                       kCFRunLoopDefaultMode);

    gObservers[gObserverCount].observer   = observer;
    gObservers[gObserverCount].appElement = appElement;
    gObservers[gObserverCount].appName    = appNameCopy;
    gObserverCount++;
}

// setupAXObservers 为所有钉钉进程注册 AXObserver，必须在 CFRunLoop 线程中调用
static void setupAXObservers(void) {
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

            if (appName) {
                registerAXObserverForPid([app processIdentifier], appName);
            }
        }
    }
}

// teardownAXObservers 清理所有 AXObserver
static void teardownAXObservers(void) {
    for (int i = 0; i < gObserverCount; i++) {
        if (gObservers[i].observer) {
            CFRunLoopRemoveSource(CFRunLoopGetCurrent(),
                                  AXObserverGetRunLoopSource(gObservers[i].observer),
                                  kCFRunLoopDefaultMode);
            CFRelease(gObservers[i].observer);
            gObservers[i].observer = NULL;
        }
        if (gObservers[i].appElement) {
            CFRelease(gObservers[i].appElement);
            gObservers[i].appElement = NULL;
        }
        if (gObservers[i].appName) {
            free(gObservers[i].appName);
            gObservers[i].appName = NULL;
        }
    }
    gObserverCount = 0;
}

// runObserverLoop 在当前线程启动 CFRunLoop，阻塞直到 stopObserverLoop 被调用
// 必须在独立线程（goroutine）中调用
void runObserverLoop(void) {
    gObserverRunLoop = CFRunLoopGetCurrent();
    CFRetain(gObserverRunLoop);
    setupAXObservers();
    CFRunLoopRun();
    teardownAXObservers();
    CFRelease(gObserverRunLoop);
    gObserverRunLoop = NULL;
}

// stopObserverLoop 从任意线程停止 observer CFRunLoop
void stopObserverLoop(void) {
    if (gObserverRunLoop) {
        CFRunLoopStop(gObserverRunLoop);
    }
}
