package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/luffot/luffot/pkg/ai"
	"github.com/luffot/luffot/pkg/barrage"
	"github.com/luffot/luffot/pkg/config"
	"github.com/luffot/luffot/pkg/embedfs"
	"github.com/luffot/luffot/pkg/eventsource"
	"github.com/luffot/luffot/pkg/logger"
	"github.com/luffot/luffot/pkg/manager"
	"github.com/luffot/luffot/pkg/pet"
	"github.com/luffot/luffot/pkg/storage"
	"github.com/luffot/luffot/pkg/tray"
	"github.com/luffot/luffot/pkg/web"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

// 注意：eventsource 包仍被 FileTailSource 使用，保留 import

var (
	configPath    string
	dataDir       string
	showVersion   bool
	enableBarrage bool   // 是否启用弹幕显示
	barrageWidth  int    // 弹幕窗口宽度
	barrageHeight int    // 弹幕窗口高度
	logFile       string // 监听日志文件路径
	httpPort      int    // HTTP 事件接收端口
)

func init() {
	// 初始化滚动日志（输出到 stdout 和 ~/.luffot/log/app.log）
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

func main() {
	flag.Parse()

	if showVersion {
		fmt.Println("陪伴钉 v1.0.0")
		return
	}

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║              陪伴钉 (AI Dingtalk)						 ║")
	fmt.Println("║                      v1.0.0                              ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Println()

	// 将相对路径转换为绝对路径（绝对路径直接使用，不拼接工作目录）
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

	// 初始化 Langfuse（从配置文件读取配置）
	if err := ai.InitLangfuse(); err != nil {
		log.Printf("[Langfuse] 初始化失败: %v", err)
		// Langfuse 初始化失败不影响主程序运行
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

	// 从嵌入 FS 加载图片皮肤（embed 模式，无需外部 assets/skins 目录）
	pet.AutoLoadImageSkinsFromFS(embedfs.SkinsFS())

	// 获取屏幕尺寸并创建弹幕显示器（只占屏幕上方 20% 高度）
	var barrageDisplay *barrage.BarrageDisplay
	if enableBarrage {
		screenWidth, screenHeight := barrage.GetScreenSize()
		barrageAreaHeight := screenHeight * 20 / 100
		fmt.Printf("正在初始化弹幕显示 (宽=%d 高=%d，上方20%%区域)... ", screenWidth, barrageAreaHeight)
		barrageDisplay = barrage.NewBarrageDisplay(barrage.BarrageDisplayConfig{
			ScreenWidth:  screenWidth,
			ScreenHeight: barrageAreaHeight,
			FontSize:     28,
			TrackHeight:  44,
			MaxTracks:    barrageAreaHeight / 44,
		})
		fmt.Println("完成")
	}

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

			// AI 回复完成回调：流式结束后将完整回复写入消息列表
			onAIReply := func(reply string) {
				if barrageDisplay != nil {
					barrageDisplay.ShowChatReply(reply)
				}
			}

			// AI 流式 token 回调：每收到一个 token 片段就实时追加到对话框
			onAIToken := func(token string) {
				if barrageDisplay != nil {
					barrageDisplay.AppendStreamToken(token)
				}
			}

			aiAgent = ai.NewAgent(aiMemory, onAIReply, onAIToken)

			// 将 AI 对话回调注入到桌宠（用户输入时触发）
			if barrageDisplay != nil {
				barrageDisplay.SetPetChatCallback(func(userInput string) {
					// 注入消息上下文（如果问题涉及消息查询）
					enhancedInput := aiTools.BuildContextualPrompt(userInput)
					aiAgent.Chat(enhancedInput)
					// 立即设置思考状态（Chat 是异步的）
					if barrageDisplay != nil {
						barrageDisplay.SetPetThinking(true)
					}
				})
			}

			if !aiAgent.IsEnabled() {
				fmt.Println("完成（注意：providers 中未配置有效的 api_key，AI 对话功能不可用）")
			} else {
				fmt.Println("完成")
			}
		}
	}

	// 创建消息管理器
	msgManager := manager.NewManager(barrageDisplay, st)

	// 注入 AI Agent 到消息管理器（用于紧急消息智能摘要）
	if aiAgent != nil {
		msgManager.SetAIAgent(aiAgent, aiTools)
	}

	// 添加事件源
	// 1. 文件监听源
	if logFile != "" {
		fmt.Printf("正在启动文件监听 (%s)... ", logFile)
		fileSource := eventsource.NewFileTailSource(eventsource.FileTailSourceConfig{
			FilePath: logFile,
			AppName:  "logfile",
		})
		msgManager.AddEventSource(fileSource)
		fmt.Println("完成")
	}

	// 2. 钉钉 Accessibility 监听源
	for _, appCfg := range cfg.Apps {
		if appCfg.Name == "dingtalk" && appCfg.Enabled {
			checkInterval := cfg.GetCheckInterval()

			// 根据 source_mode 决定读取方式：
			// - vlmodel：截图后调用视觉模型识别（需配置 vlmodel provider 和 AI 启用）
			// - accessibility（默认）：通过 Accessibility API 读取窗口文本
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

			// 创建钉钉消息源配置
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

	// 2. HTTP 事件源（通过 web.EventServer 封装）
	eventServer, _ := web.StartEventServer(ctx, "127.0.0.1", httpPort, msgManager)

	// 启动消息管理器
	if err := msgManager.Start(ctx); err != nil {
		fmt.Printf("启动消息管理器失败：%v\n", err)
	}

	// 启动摄像头背后守卫（需要 AI 已初始化且配置已启用）
	cameraGuardCfg := cfg.CameraGuard
	if cameraGuardCfg.Enabled {
		fmt.Print("正在启动摄像头背后守卫... ")
		msgManager.StartCameraGuard(ctx)
		fmt.Println("完成")
	}

	// 启动智能消息分析器（定时扫描未分析消息，重要消息推送桌宠气泡通知）
	if cfg.IntelliAnalyzer.Enabled {
		fmt.Print("正在启动智能消息分析器... ")
		msgManager.StartIntelliAnalyzer(ctx)
		fmt.Println("完成")
	}

	// 启动响应式AI链路（必须在钉钉消息源之前启动，以便注入秘书）
	var reactiveAIChain *ai.ReactiveAIChain
	if cfg.ReactiveAI.Enabled {
		fmt.Print("正在启动响应式AI链路... ")
		reactiveAIChain = ai.NewReactiveAIChain(aiAgent, st, barrageDisplay, cfg.ReactiveAI)
		if err := reactiveAIChain.Start(); err != nil {
			fmt.Printf("失败：%v\n", err)
		} else {
			fmt.Println("完成")

			// 配置 AI 丞相汇报策略
			coordinatorStrategy := ai.DefaultCoordinatorReportStrategy()
			if cfg.ReactiveAI.CoordinatorStrategy != nil {
				// 从 config 转换为 ai 包的类型
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

			// 注册应用秘书（钉钉秘书会在钉钉消息源初始化时注册）
			for _, appCfg := range cfg.Apps {
				if appCfg.Enabled && appCfg.Name != "dingtalk" {
					appType := ai.AppType(appCfg.Name)
					reactiveAIChain.RegisterAppSecretary(appType, appCfg.DisplayName)
				}
			}
		}
	}

	// 消息通道
	messageChan := make(chan *storage.Message, 100)

	// messageChan 保留但不再使用（消息由 Manager 直接处理）
	close(messageChan)

	// 应用屏幕监听器已关闭（通过 Accessibility API 截图监听进程消息的功能暂不启用）
	_ = cfg // 避免未使用变量报错

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

	// 启动定时任务调度器（在 Web 服务器之后，以便注入 scheduler 到 webServer）
	if cfg.ScheduledTasks.Enabled {
		fmt.Print("正在启动定时任务调度器... ")
		sched := msgManager.StartScheduler(ctx)
		if sched != nil && webServer != nil {
			webServer.SetScheduler(sched)
		}
		fmt.Println("完成")
	}

	// 启动状态栏（简化版）
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
			fmt.Println("打开 Web UI")
		},
	)
	trayInstance.Start()
	fmt.Println("完成")

	fmt.Println()
	fmt.Println("═ 服务已就绪 ═")
	fmt.Printf("Web 管理界面：http://%s:%d\n", cfg.Web.Host, cfg.Web.Port)
	fmt.Printf("HTTP 事件接口：http://127.0.0.1:%d/event/{app}/on_msg\n", httpPort)
	if logFile != "" {
		fmt.Printf("日志监听文件：%s\n", logFile)
	}
	fmt.Println("按 Ctrl+C 退出")
	fmt.Println()

	// 发送通知
	trayInstance.ShowNotification("消息监听器", "服务已启动")

	// 监听退出信号，在 goroutine 中处理，主线程留给 ebiten
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

	// ebiten 要求在主 goroutine 中运行，RunBarrage 会阻塞直到窗口关闭
	if barrageDisplay != nil {
		if err := barrage.RunBarrage(barrageDisplay); err != nil {
			fmt.Printf("弹幕显示异常：%v\n", err)
		}
	} else {
		// 未启用弹幕时，主线程等待退出信号
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
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
