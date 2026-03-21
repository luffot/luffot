// Package main 是 Luffot Wails GUI 的独立入口
// 此程序由 wails build 构建为独立的 .app bundle
// 主进程通过 open 命令启动此 .app 来打开设置窗口
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/luffot/luffot/pkg/config"
	"github.com/luffot/luffot/pkg/settings"
	"github.com/luffot/luffot/pkg/storage"
)

var (
	configPath string
	dataDir    string
)

func init() {
	homeDir, _ := os.UserHomeDir()
	defaultDataDir := filepath.Join(homeDir, ".luffot")

	flag.StringVar(&configPath, "config", filepath.Join(defaultDataDir, "config.yaml"), "配置文件路径")
	flag.StringVar(&dataDir, "data", defaultDataDir, "数据目录路径")
}

// isWailsBindingGeneration 检测是否是 Wails 绑定生成模式
// Wails 在 build 时会先编译并运行应用来生成前端绑定代码
// 此时需要快速退出，不启动完整的 GUI
func isWailsBindingGeneration() bool {
	execPath, err := os.Executable()
	if err != nil {
		return false
	}

	execName := filepath.Base(execPath)
	execDir := filepath.Dir(execPath)

	// 方式1: Wails 绑定生成时，可执行文件名为 wailsbindings
	if execName == "wailsbindings" {
		return true
	}

	// 方式2: 检查是否在临时目录中运行（Wails 编译绑定到临时目录）
	if strings.Contains(execDir, "/T/") || strings.Contains(execDir, "/tmp") || strings.Contains(execDir, "/temp") {
		return true
	}

	// 方式3: 检查命令行参数中是否包含 wails 相关的绑定标志
	for _, arg := range os.Args {
		if strings.Contains(arg, "generate") || strings.Contains(arg, "bindings") {
			return true
		}
	}

	// 方式4: 检查环境变量（Wails 在绑定生成时可能设置特定环境变量）
	if os.Getenv("WailsGenerateBindings") != "" {
		return true
	}

	return false
}

func main() {
	// Wails build 时会运行应用来生成绑定，需要快速退出
	if isWailsBindingGeneration() {
		return
	}

	flag.Parse()

	// 将相对路径转换为绝对路径
	workDir, _ := os.Getwd()
	if !filepath.IsAbs(configPath) {
		configPath = filepath.Join(workDir, configPath)
	}
	if !filepath.IsAbs(dataDir) {
		dataDir = filepath.Join(workDir, dataDir)
	}

	fmt.Println("[Luffot Settings] 启动中...")

	// 初始化配置
	cfg, err := config.Init(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[Luffot Settings] 加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化存储
	dbPath := filepath.Join(dataDir, "messages.db")
	st, err := storage.NewStorage(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[Luffot Settings] 初始化数据库失败: %v\n", err)
		os.Exit(1)
	}
	defer st.Close()

	// 启动 Wails 设置窗口
	fmt.Println("[Luffot Settings] 正在启动设置窗口...")
	if err := settings.Run(cfg, st, nil); err != nil {
		fmt.Fprintf(os.Stderr, "[Luffot Settings] 设置窗口错误: %v\n", err)
		os.Exit(1)
	}
}
