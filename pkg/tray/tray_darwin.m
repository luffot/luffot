#import <Cocoa/Cocoa.h>

// Go 回调函数声明（由 Go 侧 export）
extern void goMenuCallback(int tag);

// LuffotMenuDelegate：处理菜单点击的 target
@interface LuffotMenuDelegate : NSObject
@end

@implementation LuffotMenuDelegate
- (void)menuItemClicked:(NSMenuItem *)sender {
    NSLog(@"[LuffotMenuDelegate] menuItemClicked called with tag: %ld", (long)[sender tag]);
    goMenuCallback((int)[sender tag]);
}
@end

// 全局引用（防止被 ARC 释放）
static NSStatusItem *gStatusItem = nil;
static LuffotMenuDelegate *gDelegate = nil;
static NSMenu *gMenu = nil;

// createStatusBar 创建状态栏图标和菜单
// 必须在主线程执行以确保 NSStatusBar 正确显示
void createStatusBar(int webPort, const char **skinNames, const char *activeSkin) {
    // 确保在主线程执行
    if (![NSThread isMainThread]) {
        dispatch_sync(dispatch_get_main_queue(), ^{
            createStatusBar(webPort, skinNames, activeSkin);
        });
        return;
    }
    
    // 创建菜单委托
    gDelegate = [[LuffotMenuDelegate alloc] init];

    // 确保 NSApplication 已初始化
    NSApplication *app = [NSApplication sharedApplication];
    
    // 设置激活策略为Accessory模式（不显示Dock图标，只显示状态栏图标）
    if ([app activationPolicy] != NSApplicationActivationPolicyAccessory) {
        [app setActivationPolicy:NSApplicationActivationPolicyAccessory];
    }
    
    // 完成应用启动（必须在 [NSApp run] 之前调用，否则 macOS 会认为应用"未响应"）
    [app finishLaunching];

    // ── 创建状态栏图标 ──
    NSStatusBar *statusBar = [NSStatusBar systemStatusBar];
    gStatusItem = [statusBar statusItemWithLength:NSSquareStatusItemLength];
    gStatusItem.visible = YES;

    // 绘制"闪电鸟"图标：鸟身轮廓 + 闪电翅膀，18×18pt，模板模式自适应深/浅色
    NSImage *icon = [[NSImage alloc] initWithSize:NSMakeSize(18, 18)];
    [icon lockFocus];

    [[NSColor blackColor] setFill];
    [[NSColor blackColor] setStroke];

    // ── 鸟头（圆形）──
    NSBezierPath *head = [NSBezierPath bezierPathWithOvalInRect:NSMakeRect(11.0, 13.0, 4.5, 4.0)];
    [head fill];

    // ── 鸟嘴（向右的小三角）──
    NSBezierPath *beak = [NSBezierPath bezierPath];
    [beak moveToPoint:NSMakePoint(15.5, 15.5)];
    [beak lineToPoint:NSMakePoint(18.0, 14.8)];
    [beak lineToPoint:NSMakePoint(15.5, 14.0)];
    [beak closePath];
    [beak fill];

    // ── 鸟身（椭圆）──
    NSBezierPath *body = [NSBezierPath bezierPathWithOvalInRect:NSMakeRect(7.5, 9.5, 7.0, 5.0)];
    [body fill];

    // ── 尾巴（向左下方的扇形三角）──
    NSBezierPath *tail = [NSBezierPath bezierPath];
    [tail moveToPoint:NSMakePoint(7.5, 11.5)];
    [tail lineToPoint:NSMakePoint(2.0, 14.0)];
    [tail lineToPoint:NSMakePoint(3.5, 10.5)];
    [tail lineToPoint:NSMakePoint(1.0, 9.0)];
    [tail lineToPoint:NSMakePoint(7.5, 10.0)];
    [tail closePath];
    [tail fill];

    // ── 闪电翅膀（Z 形填充，从鸟身向下展开）──
    NSBezierPath *wing = [NSBezierPath bezierPath];
    [wing moveToPoint:NSMakePoint(8.5, 9.5)];
    [wing lineToPoint:NSMakePoint(13.0, 9.5)];
    [wing lineToPoint:NSMakePoint(10.5, 5.5)];
    [wing lineToPoint:NSMakePoint(13.5, 5.5)];
    [wing lineToPoint:NSMakePoint(9.5, 1.0)];
    [wing lineToPoint:NSMakePoint(7.5, 5.0)];
    [wing lineToPoint:NSMakePoint(5.0, 5.0)];
    [wing closePath];
    [wing fill];

    // ── 眼睛（小白点）──
    [[NSColor whiteColor] setFill];
    NSBezierPath *eye = [NSBezierPath bezierPathWithOvalInRect:NSMakeRect(12.5, 15.2, 1.5, 1.5)];
    [eye fill];

    [icon unlockFocus];

    // 设置为模板图像（macOS 自动适配深色/浅色模式）
    [icon setTemplate:YES];
    gStatusItem.button.image = icon;
    gStatusItem.button.toolTip = @"Luffot 弹幕桌宠";

    // ── 创建菜单 ──
    NSMenu *menu = [[NSMenu alloc] init];
    [menu setAutoenablesItems:NO];

    // 设置（tag=3）
    NSMenuItem *settingsItem = [[NSMenuItem alloc] initWithTitle:@"⚙️ 设置面板"
                                                          action:@selector(menuItemClicked:)
                                                   keyEquivalent:@","];
    [settingsItem setTag:3];
    [settingsItem setTarget:gDelegate];
    [menu addItem:settingsItem];
    NSLog(@"[createStatusBar] 设置菜单项已添加，target=%@", gDelegate);

    // 关于（tag=1）
    NSMenuItem *aboutItem = [[NSMenuItem alloc] initWithTitle:@"关于 Luffot"
                                                       action:@selector(menuItemClicked:)
                                                keyEquivalent:@""];
    [aboutItem setTag:1];
    [aboutItem setTarget:gDelegate];
    [menu addItem:aboutItem];

    [menu addItem:[NSMenuItem separatorItem]];

    // 退出（tag=0）
    NSMenuItem *quitItem = [[NSMenuItem alloc] initWithTitle:@"退出"
                                                      action:@selector(menuItemClicked:)
                                               keyEquivalent:@"q"];
    [quitItem setTag:0];
    [quitItem setTarget:gDelegate];
    [menu addItem:quitItem];
    
    NSLog(@"[createStatusBar] 菜单项总数: %lu", (unsigned long)[menu numberOfItems]);

    gMenu = menu;
    gStatusItem.menu = gMenu;
    
    NSLog(@"[createStatusBar] 状态栏菜单已设置，createStatusBar 返回");
    
    // 注意：不在这里启动事件循环
    // createStatusBar 只负责创建状态栏和菜单，然后返回
    // 事件循环由 Go 侧的 runMainRunLoop() 启动
}

// setAppIcon 使用 PNG 数据设置应用图标
// 此图标会显示在活动监视器、进程管理器中
void setAppIcon(const void *pngData, int pngLen) {
    if (pngData == NULL || pngLen <= 0) return;
    
    NSData *data = [NSData dataWithBytes:pngData length:pngLen];
    NSImage *icon = [[NSImage alloc] initWithData:data];
    if (icon != nil) {
        [[NSApplication sharedApplication] setApplicationIconImage:icon];
    }
}

// hideDockIcon 将应用激活策略重新设为 Accessory 模式，隐藏 Dock 图标
// Ebiten 启动时会将策略重置为 Regular，需要在首帧重新调用
void hideDockIcon() {
    dispatch_async(dispatch_get_main_queue(), ^{
        [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
    });
}

// updateSkinMenu 更新皮肤菜单勾选状态
void updateSkinMenu(const char *activeSkin) {
    NSString *activeStr = activeSkin ? [NSString stringWithUTF8String:activeSkin] : @"";
    if (gStatusItem == nil) return;
    NSMenu *menu = gStatusItem.menu;
    if (menu == nil) return;

    for (NSMenuItem *item in [menu itemArray]) {
        NSString *title = [item title];
        if ([title hasPrefix:@"\u2713 "]) {
            title = [title substringFromIndex:2];
        }
        if ([title isEqualToString:activeStr]) {
            [item setTitle:[NSString stringWithFormat:@"\u2713 %@", title]];
        } else {
            [item setTitle:title];
        }
    }
}
