package settings

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/luffot/luffot/internal/assets"
	"github.com/luffot/luffot/pkg/config"
	"github.com/luffot/luffot/pkg/embedfs"
	"github.com/luffot/luffot/pkg/scheduler"
	"github.com/luffot/luffot/pkg/storage"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

// Run 启动 Wails 设置窗口（在 macOS 上通过 runtime.LockOSThread 确保在主线程运行）
func Run(cfg *config.AppConfig, st *storage.Storage, sched *scheduler.Scheduler) error {
	app := NewApp(cfg, st)
	app.SetScheduler(sched)

	// macOS 需要确保在主线程运行 GUI
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// 获取前端资源文件系统
	// 优先使用项目根目录的 frontend/dist（开发/构建时）
	// 如果不可用，则使用嵌入的资源
	var assetsFS fs.FS
	if rootDist := findRootFrontendDist(); rootDist != "" {
		assetsFS = os.DirFS(rootDist)
	} else {
		assetsFS = assets.WailsAssetsFS()
	}

	err := wails.Run(&options.App{
		Title:     "Luffot Settings",
		Width:     1280,
		Height:    800,
		MinWidth:  1024,
		MinHeight: 600,
		Frameless: false,
		AssetServer: &assetserver.Options{
			Assets: assetsFS,
		},
		BackgroundColour: &options.RGBA{R: 249, G: 250, B: 251, A: 1},
		OnStartup:        app.Startup,
		Bind: []interface{}{
			app,
		},
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: true,
				HideTitle:                  true,
				HideTitleBar:               false,
				FullSizeContent:            true,
				UseToolbar:                 false,
			},
			About: &mac.AboutInfo{
				Title:   "Luffot Settings",
				Message: "Luffot 设置面板 v1.0.0",
				Icon:    embedfs.AppIconPNG(),
			},
			WebviewIsTransparent: true,
			WindowIsTranslucent:  false,
			Appearance:           mac.NSAppearanceNameAqua,
		},
	})

	if err != nil {
		return fmt.Errorf("wails run error: %w", err)
	}
	return nil
}

// findRootFrontendDist 查找项目根目录的 frontend/dist
func findRootFrontendDist() string {
	// 获取当前工作目录
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}

	// 尝试从当前目录向上查找 frontend/dist
	dir := wd
	for i := 0; i < 5; i++ { // 最多向上查找5层
		distPath := filepath.Join(dir, "frontend", "dist")
		if info, err := os.Stat(distPath); err == nil && info.IsDir() {
			// 检查是否有 index.html
			if _, err := os.Stat(filepath.Join(distPath, "index.html")); err == nil {
				return distPath
			}
		}

		// 向上查找
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return ""
}

// RunWithContext 使用上下文启动 Wails 设置窗口（用于与主应用集成）
func RunWithContext(ctx context.Context, cfg *config.AppConfig, st *storage.Storage, sched *scheduler.Scheduler) error {
	app := NewApp(cfg, st)
	app.SetScheduler(sched)
	app.Startup(ctx)
	return nil
}

// RunAsSubprocess 以子进程方式启动 Wails 设置窗口
// 这样可以避免与主应用的 Ebiten 主线程冲突
func RunAsSubprocess(configPath, dataDir string) error {
	// 获取当前可执行文件路径
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("解析可执行文件路径失败: %w", err)
	}

	// 构建子进程命令
	cmd := exec.Command(execPath, "--settings-mode")
	cmd.Args = append(cmd.Args, "--config", configPath)
	cmd.Args = append(cmd.Args, "--data", dataDir)

	// 设置环境变量，标记为设置模式
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// 启动子进程（非阻塞）
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动设置窗口子进程失败: %w", err)
	}

	// 分离子进程，使其在父进程退出后继续运行
	go func() {
		if err := cmd.Wait(); err != nil {
			fmt.Printf("设置窗口进程退出: %v\n", err)
		}
	}()

	return nil
}
