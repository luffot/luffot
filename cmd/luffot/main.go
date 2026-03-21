package luffot

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/luffot/luffot/pkg/ai"
	"github.com/luffot/luffot/pkg/barrage"
	"github.com/luffot/luffot/pkg/config"
	"github.com/luffot/luffot/pkg/embedfs"
	"github.com/luffot/luffot/pkg/eventsource"
	"github.com/luffot/luffot/pkg/logger"
	"github.com/luffot/luffot/pkg/manager"
	"github.com/luffot/luffot/pkg/pet"
	"github.com/luffot/luffot/pkg/scheduler"
	"github.com/luffot/luffot/pkg/storage"
	"github.com/luffot/luffot/pkg/tray"
	"github.com/luffot/luffot/pkg/web"
	"runtime"
)

var (
	configPath    string
	dataDir       string
	showVersion   bool
	enableBarrage bool
	barrageWidth  int
	barrageHeight int
	logFile       string
	httpPort      int
)

func init() {
	// 初始化滚动日志
	logDir := filepath.Join(os.Getenv("HOME"), ".luffot", "log")
	if err := logger.InitLogger(logDir); err != nil {
		fmt.Fprintf(os.Stderr, "初始化日志失败: %v\n", err)
		os.Exit(1)
	}

	defaultConfigPath := filepath.Join(os.Getenv("HOME"), ".luffot", "config.yaml")
	defaultDataDir := filepath.Join(os.Getenv("HOME"), ".luffot", "data")
	flag.StringVar(&configPath, "config", defaultConfigPath, "配置文件路径")
	flag.StringVar(&dataDir, "data", defaultDataDir, "数据目录")
	flag.BoolVar(&showVersion, "version", false, "显示版本号")
	flag.BoolVar(&enableBarrage, "barrage", true, "是否启用弹幕显示")
	flag.IntVar(&barrageWidth, "barrage-width", 800, "弹幕窗口宽度")
	flag.IntVar(&barrageHeight, "barrage-height", 200, "弹幕窗口高度")
	flag.StringVar(&logFile, "log-file", "", "监听日志文件路径 (tail 模式)")
	flag.IntVar(&httpPort, "http-port", 8766, "HTTP 事件接收端口")
}

// Run 应用入口（单进程架构）
// Ebiten 弹幕/桌宠 + NSStatusBar 状态栏 + 业务逻辑 在同一进程内运行。
// Wails 设置面板作为独立 .app，通过 open 命令启动。
func Run() {
	flag.Parse()

	if showVersion {
		fmt.Println("陪伴钉 v1.0.0")
		return
	}

	// macOS: 锁定主线程以确保 NSStatusBar 和 Ebiten 正常工作
	runtime.LockOSThread()

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║              陪伴钉 (AI Dingtalk) v1.0.0                ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Println()

	// 将相对路径转换为绝对路径
	workDir, _ := os.Getwd()
	if !filepath.IsAbs(configPath) {
		configPath = filepath.Join(workDir, configPath)
	}
	if !filepath.IsAbs(dataDir) {
		dataDir = filepath.Join(workDir, dataDir)
	}

	// 初始化配置
	fmt.Print("正在加载配置... ")
	cfg, err := config.Init(configPath)
	if err != nil {
		fmt.Printf("失败：%v\n", err)
		os.Exit(1)
	}
	fmt.Println("完成")

	// 初始化 Langfuse
	if err := ai.InitLangfuse(); err != nil {
		log.Printf("[Langfuse] 初始化失败: %v", err)
	}

	// 确保数据目录存在
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		fmt.Printf("创建数据目录失败：%v\n", err)
		os.Exit(1)
	}

	// 初始化存储
	dbPath := filepath.Join(dataDir, "messages.db")
	fmt.Printf("正在初始化数据库 (%s)... ", dbPath)
	st, err := storage.NewStorage(dbPath)
	if err != nil {
		fmt.Printf("失败：%v\n", err)
		os.Exit(1)
	}
	fmt.Println("完成")
	defer st.Close()

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 初始化 AI 模块
	var aiAgent *ai.Agent
	var aiTools *ai.MessageQueryTools
	if cfg.AI.Enabled {
		fmt.Print("正在初始化 AI 智能体... ")
		aiMemory, memErr := ai.NewMemory(dbPath, cfg.AI.MaxContextRounds)
		if memErr != nil {
			fmt.Printf("记忆模块初始化失败（将跳过 AI 功能）：%v\n", memErr)
		} else {
			aiTools = ai.NewMessageQueryTools(st)
			aiAgent = ai.NewAgent(aiMemory, nil, nil)

			if !aiAgent.IsEnabled() {
				fmt.Println("完成（注意：providers 中未配置有效的 api_key，AI 对话功能不可用）")
			} else {
				fmt.Println("完成")
			}
		}
	}

	// 加载皮肤
	pet.AutoLoadImageSkinsFromFS(embedfs.SkinsFS())

	// 创建弹幕显示器（Ebiten 在主进程内运行）
	var barrageDisplay *barrage.BarrageDisplay
	if enableBarrage {
		screenWidth, screenHeight := barrage.GetScreenSize()
		barrageAreaHeight := screenHeight * 20 / 100
		barrageDisplay = barrage.NewBarrageDisplay(barrage.BarrageDisplayConfig{
			ScreenWidth:  screenWidth,
			ScreenHeight: barrageAreaHeight,
			FontSize:     28,
			TrackHeight:  44,
			MaxTracks:    barrageAreaHeight / 44,
		})
	}

	// 创建消息管理器（传入弹幕显示器）
	msgManager := manager.NewManager(barrageDisplay, st)

	if aiAgent != nil {
		msgManager.SetAIAgent(aiAgent, aiTools)
	}

	// 添加事件源
	if logFile != "" {
		fmt.Printf("正在启动文件监听 (%s)... ", logFile)
		fileSource := eventsource.NewFileTailSource(eventsource.FileTailSourceConfig{
			FilePath: logFile,
			AppName:  "logfile",
		})
		msgManager.AddEventSource(fileSource)
		fmt.Println("完成")
	}

	// 钉钉 Accessibility 监听源
	for _, appCfg := range cfg.Apps {
		if appCfg.Name == "dingtalk" && appCfg.Enabled {
			checkInterval := cfg.GetCheckInterval()
			useVLModel := appCfg.DingTalk.SourceMode == config.DingTalkSourceModeVLModel
			var dingAgent *ai.Agent
			if useVLModel && aiAgent != nil && aiAgent.IsEnabled() {
				dingAgent = aiAgent
				fmt.Printf("正在启动钉钉窗口监听（vlmodel 截图识别模式，轮询间隔：%v）... ", checkInterval)
			} else {
				if useVLModel {
					fmt.Println("[警告] source_mode=vlmodel 但 AI 未启用，回退到 accessibility 模式")
				}
				fmt.Printf("正在启动钉钉窗口监听（accessibility 模式，轮询间隔：%v）... ", checkInterval)
			}

			dingSourceConfig := eventsource.DingTalkSourceConfig{
				CheckInterval: checkInterval,
				MaxCacheSize:  500,
				Agent:         dingAgent,
			}

			dingSource := eventsource.NewDingTalkSource(dingSourceConfig)
			msgManager.AddEventSource(dingSource)
			fmt.Println("完成")
			break
		}
	}

	// HTTP 事件源
	eventServer, _ := web.StartEventServer(ctx, "127.0.0.1", httpPort, msgManager)

	// 启动消息管理器
	if err := msgManager.Start(ctx); err != nil {
		fmt.Printf("启动消息管理器失败：%v\n", err)
	}

	// 摄像头背后守卫
	if cfg.CameraGuard.Enabled {
		fmt.Print("正在启动摄像头背后守卫... ")
		msgManager.StartCameraGuard(ctx)
		fmt.Println("完成")
	}

	// 智能消息分析器
	if cfg.IntelliAnalyzer.Enabled {
		fmt.Print("正在启动智能消息分析器... ")
		msgManager.StartIntelliAnalyzer(ctx)
		fmt.Println("完成")
	}

	// 响应式AI链路
	var reactiveAIChain *ai.ReactiveAIChain
	if cfg.ReactiveAI.Enabled {
		fmt.Print("正在启动响应式AI链路... ")
		reactiveAIChain = ai.NewReactiveAIChain(aiAgent, st, nil, cfg.ReactiveAI)
		if err := reactiveAIChain.Start(); err != nil {
			fmt.Printf("失败：%v\n", err)
		} else {
			fmt.Println("完成")
			coordinatorStrategy := ai.DefaultCoordinatorReportStrategy()
			if cfg.ReactiveAI.CoordinatorStrategy != nil {
				configStrategy := cfg.ReactiveAI.CoordinatorStrategy
				coordinatorStrategy = ai.CoordinatorReportStrategy{
					EnableAISummary:       configStrategy.EnableAISummary,
					MinReportInterval:     configStrategy.MinReportInterval,
					MaxConsecutiveReports: configStrategy.MaxConsecutiveReports,
					ConsecutiveCooldown:   configStrategy.ConsecutiveCooldown,
					UrgentImmediate:       configStrategy.UrgentImmediate,
					BatchWindow:           configStrategy.BatchWindow,
				}
			}
			reactiveAIChain.SetCoordinatorReportStrategy(coordinatorStrategy)

			for _, appCfg := range cfg.Apps {
				if appCfg.Enabled && appCfg.Name != "dingtalk" {
					appType := ai.AppType(appCfg.Name)
					reactiveAIChain.RegisterAppSecretary(appType, appCfg.DisplayName)
				}
			}
		}
	}

	// 启动 Web 服务器
	var webServer *web.Server
	if cfg.Web.Enabled {
		fmt.Printf("正在启动 Web 服务 (http://%s:%d)... ", cfg.Web.Host, cfg.Web.Port)
		webServer = web.NewServer(cfg, st, embedfs.WebStaticFS())
		go func() {
			if err := webServer.Start(); err != nil {
				fmt.Printf("Web 服务启动失败：%v\n", err)
			}
		}()
		time.Sleep(500 * time.Millisecond)
		fmt.Println("完成")
	}

	// 启动定时任务调度器
	var sched *scheduler.Scheduler
	if cfg.ScheduledTasks.Enabled {
		fmt.Print("正在启动定时任务调度器... ")
		sched = msgManager.StartScheduler(ctx)
		if sched != nil && webServer != nil {
			webServer.SetScheduler(sched)
		}
		fmt.Println("完成")
	}

	// 启动状态栏（在 Ebiten RunGame 之前创建，Ebiten 的 RunLoop 会驱动状态栏事件）
	fmt.Print("正在启动状态栏... ")
	trayInstance := tray.NewTray(cfg,
		func() {
			fmt.Println("退出请求")
			cancel()
		},
		func() {
			fmt.Println("切换监听状态")
		},
		func() {
			fmt.Println("打开 Web 管理界面...")
		},
		func() {
			// 通过 open 命令启动独立的 Wails 设置窗口 .app
			fmt.Println("正在打开 Wails 设置窗口...")
			if err := openWailsSettingsApp(); err != nil {
				log.Printf("启动 Wails 设置窗口失败: %v", err)
			}
		},
	)
	trayInstance.Start()
	fmt.Println("完成")

	fmt.Println()
	fmt.Println("═ 服务已就绪 ═")
	if cfg.Web.Enabled {
		fmt.Printf("Web 管理界面：http://%s:%d\n", cfg.Web.Host, cfg.Web.Port)
	}
	fmt.Printf("HTTP 事件接口：http://127.0.0.1:%d/event/{app}/on_msg\n", httpPort)
	if logFile != "" {
		fmt.Printf("日志监听文件：%s\n", logFile)
	}
	fmt.Println("按 Ctrl+C 退出")
	fmt.Println()

	// 发送通知
	trayInstance.ShowNotification("消息监听器", "服务已启动")

	// 在 goroutine 中等待退出信号
	go func() {
		<-sigChan
		fmt.Println("\n正在关闭服务...")
		cancel()
		msgManager.Stop()
		eventServer.Stop()
		if webServer != nil {
			webServer.Stop()
		}
		if reactiveAIChain != nil {
			reactiveAIChain.Stop()
		}

		fmt.Println("已退出")
		os.Exit(0)
	}()

	// Ebiten 主循环（阻塞主线程）
	// Ebiten 内部会启动 macOS RunLoop，同时驱动 NSStatusBar 菜单点击等事件
	if enableBarrage && barrageDisplay != nil {
		fmt.Println("正在启动弹幕和桌宠显示...")
		if err := barrage.RunBarrage(barrageDisplay); err != nil {
			fmt.Printf("弹幕显示异常：%v\n", err)
			os.Exit(1)
		}
	} else {
		// 未启用弹幕时，用 RunLoop 保持状态栏响应
		runMainRunLoop()
	}
}

// openWailsSettingsApp 通过 macOS open 命令启动独立的 Wails 设置窗口 .app
func openWailsSettingsApp() error {
	wailsAppPath := findWailsApp()
	if wailsAppPath == "" {
		return fmt.Errorf("未找到 Luffot Settings.app，请确保已构建 Wails 设置窗口")
	}

	cmd := exec.Command("open", "-a", wailsAppPath,
		"--args",
		"--config", configPath,
		"--data", dataDir,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 Wails 设置窗口失败: %w", err)
	}

	go func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("Wails 设置窗口进程退出: %v", err)
		}
	}()

	return nil
}

// findWailsApp 查找 Luffot Settings.app 的路径
// 搜索顺序：
// 1. 主进程所在 .app bundle 的同级目录
// 2. 主进程可执行文件的同级目录
// 3. build/bin 目录（开发时）
// 4. /Applications 目录
func findWailsApp() string {
	// 兼容不同的构建产物名称：
	// wails build 可能输出 "Luffot Settings.app"（outputfilename）或 "luffot-settings.app"（name）
	wailsAppNames := []string{"Luffot Settings.app", "luffot-settings.app"}

	execPath, err := os.Executable()
	if err != nil {
		return ""
	}
	execPath, _ = filepath.EvalSymlinks(execPath)
	execDir := filepath.Dir(execPath)

	searchDirs := make([]string, 0, 4)

	// 1. 如果主进程在 .app bundle 内，查找同级目录
	if strings.Contains(execDir, ".app/Contents/MacOS") {
		bundleDir := filepath.Dir(filepath.Dir(filepath.Dir(execDir)))
		searchDirs = append(searchDirs, bundleDir)
	}

	// 2. 可执行文件同级目录
	searchDirs = append(searchDirs, execDir)

	// 3. build/bin 目录（开发时）
	workDir, _ := os.Getwd()
	searchDirs = append(searchDirs, filepath.Join(workDir, "build", "bin"))

	// 4. /Applications 目录
	searchDirs = append(searchDirs, "/Applications")

	for _, dir := range searchDirs {
		for _, appName := range wailsAppNames {
			candidate := filepath.Join(dir, appName)
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}

	return ""
}
