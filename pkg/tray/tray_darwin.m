#import <Cocoa/Cocoa.h>

// Go 回调函数声明（由 Go 侧 export）
extern void goMenuCallback(int tag);

// LuffotMenuDelegate：处理菜单点击的 target
@interface LuffotMenuDelegate : NSObject
@end

@implementation LuffotMenuDelegate
- (void)menuItemClicked:(NSMenuItem *)sender {
    goMenuCallback((int)[sender tag]);
}
@end

// 全局引用（防止被 ARC 释放）
static NSStatusItem *gStatusItem = nil;
static LuffotMenuDelegate *gDelegate = nil;
static NSMenu *gMenu = nil;

// createStatusBar 创建状态栏图标和菜单（由 Go 在主线程直接调用，不使用 dispatch_async）
// 注意：本函数必须在主线程调用，且必须在 ebiten/RunBarrage 占用主线程之前执行。
// 由于 ebiten 接管了主线程的 RunLoop，dispatch_async 派发的 block 无法被执行，
// 因此这里直接同步创建，调用方（Go 侧）需保证在主线程且 RunLoop 启动前调用。
void createStatusBar(int webPort, const char **skinNames, const char *activeSkin) {
    NSMutableArray<NSString *> *skinNamesCopy = [NSMutableArray array];
    for (int i = 0; skinNames[i] != NULL; i++) {
        [skinNamesCopy addObject:[NSString stringWithUTF8String:skinNames[i]]];
    }
    NSString *activeSkinCopy = activeSkin ? [NSString stringWithUTF8String:activeSkin] : @"";
    int webPortCopy = webPort;

    // 如果当前已在主线程，直接执行；否则通过 dispatch_sync 切换到主线程同步执行
    void (^createBlock)(void) = ^{
        gDelegate = [[LuffotMenuDelegate alloc] init];

        // 确保 NSApplication 已初始化（ebiten 不走标准 NSApp run，需要手动激活）
        NSApplication *app = [NSApplication sharedApplication];
        [app setActivationPolicy:NSApplicationActivationPolicyAccessory];
        [app activateIgnoringOtherApps:YES];

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
        [wing moveToPoint:NSMakePoint(8.5, 9.5)];   // 翅膀根部（鸟身底部左侧）
        [wing lineToPoint:NSMakePoint(13.0, 9.5)];  // 翅膀根部右侧
        [wing lineToPoint:NSMakePoint(10.5, 5.5)];  // 中折点（向下）
        [wing lineToPoint:NSMakePoint(13.5, 5.5)];  // 中折点右侧
        [wing lineToPoint:NSMakePoint(9.5, 1.0)];   // 翅尖（最低点）
        [wing lineToPoint:NSMakePoint(7.5, 5.0)];   // 回折点
        [wing lineToPoint:NSMakePoint(5.0, 5.0)];   // 回折点左侧
        [wing closePath];
        [wing fill];

        // ── 眼睛（小白点，用白色覆盖在鸟头上）──
        [[NSColor whiteColor] setFill];
        NSBezierPath *eye = [NSBezierPath bezierPathWithOvalInRect:NSMakeRect(12.5, 15.2, 1.5, 1.5)];
        [eye fill];

        [icon unlockFocus];

        // 设置为模板图像（macOS 自动适配深色/浅色模式，图标颜色跟随系统）
        [icon setTemplate:YES];
        gStatusItem.button.image = icon;
        gStatusItem.button.toolTip = @"Luffot 弹幕桌宠";

        // ── 创建菜单 ──
        // 顺序：皮肤配置 → Web管理 → 分隔线 → 设置 → 关于 → 分隔线 → 退出
        NSMenu *menu = [[NSMenu alloc] init];
        [menu setAutoenablesItems:NO];

        // 皮肤子菜单（tag 从 100 开始）
        NSMenuItem *skinParent = [[NSMenuItem alloc] initWithTitle:@"🎨 皮肤配置"
                                                            action:nil
                                                     keyEquivalent:@""];
        NSMenu *skinMenu = [[NSMenu alloc] init];
        for (NSUInteger i = 0; i < [skinNamesCopy count]; i++) {
            NSString *name = skinNamesCopy[i];
            NSString *title = [name isEqualToString:activeSkinCopy]
                ? [NSString stringWithFormat:@"\u2713 %@", name]
                : name;
            NSMenuItem *skinItem = [[NSMenuItem alloc] initWithTitle:title
                                                              action:@selector(menuItemClicked:)
                                                       keyEquivalent:@""];
            [skinItem setTag:(NSInteger)(100 + i)];
            [skinItem setTarget:gDelegate];
            [skinMenu addItem:skinItem];
        }
        [skinParent setSubmenu:skinMenu];
        [menu addItem:skinParent];

        // Web 管理界面（tag=2，仅 webPort>0 时显示）
        if (webPortCopy > 0) {
            NSString *webTitle = [NSString stringWithFormat:@"🌐 Web 管理界面 (:%d)", webPortCopy];
            NSMenuItem *webItem = [[NSMenuItem alloc] initWithTitle:webTitle
                                                             action:@selector(menuItemClicked:)
                                                      keyEquivalent:@""];
            [webItem setTag:2];
            [webItem setTarget:gDelegate];
            [menu addItem:webItem];
        }

        [menu addItem:[NSMenuItem separatorItem]];

        // 设置（tag=3）
        NSMenuItem *settingsItem = [[NSMenuItem alloc] initWithTitle:@"⚙️ 设置面板"
                                                              action:@selector(menuItemClicked:)
                                                       keyEquivalent:@","];
        [settingsItem setTag:3];
        [settingsItem setTarget:gDelegate];
        [menu addItem:settingsItem];

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

        gMenu = menu;
        gStatusItem.menu = gMenu;
    };

    // 必须在主线程操作 NSStatusItem；若已在主线程则直接执行，否则同步切换到主线程
    if ([NSThread isMainThread]) {
        createBlock();
    } else {
        dispatch_sync(dispatch_get_main_queue(), createBlock);
    }
}

// updateSkinMenu 更新皮肤菜单勾选状态（由 Go 调用）
void updateSkinMenu(const char *activeSkin) {
    NSString *activeStr = activeSkin ? [NSString stringWithUTF8String:activeSkin] : @"";
    dispatch_async(dispatch_get_main_queue(), ^{
        if (gStatusItem == nil) return;
        NSMenu *menu = gStatusItem.menu;

        // 皮肤子菜单是第 3 个菜单项（index=2，前面是"关于"和分隔线）
        if ([menu numberOfItems] < 3) return;
        NSMenuItem *skinParent = [menu itemAtIndex:2];
        NSMenu *skinMenu = [skinParent submenu];
        if (skinMenu == nil) return;

        for (NSMenuItem *item in [skinMenu itemArray]) {
            NSString *title = [item title];
            // 去掉已有的勾选前缀（"✓ "，Unicode U+2713 + 空格）
            if ([title hasPrefix:@"\u2713 "]) {
                title = [title substringFromIndex:2];
            }
            if ([title isEqualToString:activeStr]) {
                [item setTitle:[NSString stringWithFormat:@"\u2713 %@", title]];
            } else {
                [item setTitle:title];
            }
        }
    });
}
