package tray

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/luffot/luffot/pkg/config"
)

// Tray 系统状态栏
type Tray struct {
	config     *config.AppConfig
	onQuit     func()
	onToggle   func()
	onOpenWeb  func()
	webPort    int
	webEnabled bool
}

// NewTray 创建状态栏实例
func NewTray(cfg *config.AppConfig, onQuit, onToggle, onOpenWeb func()) *Tray {
	return &Tray{
		config:     cfg,
		onQuit:     onQuit,
		onToggle:   onToggle,
		onOpenWeb:  onOpenWeb,
		webPort:    cfg.Web.Port,
		webEnabled: cfg.Web.Enabled,
	}
}

// Start 启动系统托盘（非阻塞，在 goroutine 中初始化 NSStatusBar）
func (t *Tray) Start() {
	startNSStatusBar(t.webPort, t.webEnabled, t.onQuit)
}

// ShowNotification 显示系统通知
func (t *Tray) ShowNotification(title, message string) {
	safeTitle := strings.ReplaceAll(title, `"`, `\"`)
	safeMsg := strings.ReplaceAll(message, `"`, `\"`)
	script := fmt.Sprintf(`display notification "%s" with title "%s"`, safeMsg, safeTitle)
	cmd := exec.Command("osascript", "-e", script)
	_ = cmd.Start()
}

// openBrowser 使用默认浏览器打开 URL
func openBrowser(url string) {
	cmd := exec.Command("open", url)
	_ = cmd.Start()
}

// showAboutDialog 用 AppleScript 弹出关于对话框
func showAboutDialog() {
	script := `display dialog "Luffot 弹幕桌宠` + "\n\n" +
		`• 实时接收消息并以弹幕形式展示` + "\n" +
		`• 桌宠钉三多陪伴你工作` + "\n" +
		`• 支持多种皮肤切换` + "\n" +
		`• 快捷键 Shift+Alt+T 切换弹幕显示` + "\n\n" +
		`版本：v1.0.0" ` +
		`buttons {"确定"} default button 1 with title "关于 Luffot" with icon note`
	cmd := exec.Command("osascript", "-e", script)
	_ = cmd.Start()
}
