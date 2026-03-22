package tray

/*
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>

// 只声明 C 函数原型，ObjC 实现在 tray_darwin.m 中
extern void createStatusBar(int webPort, const char **skinNames, const char *activeSkin);
extern void updateSkinMenu(const char *activeSkin);
extern void hideDockIcon();

// Go 回调函数的前向声明（由 Go 侧 export）
extern void goMenuCallback(int tag);
*/
import "C"

import (
	"fmt"
	"time"
	"unsafe"

	"github.com/luffot/luffot/pkg/pet"
)

// HideDockIcon 隐藏 macOS Dock 图标
// Ebiten 的 RunGame 会将激活策略重置为 Regular，导致 Dock 图标出现
// 需要在 Ebiten 启动后的首帧重新强制设置为 Accessory 模式
func HideDockIcon() {
	C.hideDockIcon()
}

// gWebPort 保存 Web 端口，供 goMenuCallback 使用
var gWebPort int

// gOnQuit 保存退出回调
var gOnQuit func()

// gOnOpenSettings 保存打开设置窗口回调
var gOnOpenSettings func()

// startNSStatusBar 初始化 NSStatusBar
func startNSStatusBar(webPort int, webEnabled bool, onQuit func()) {
	gWebPort = webPort
	gOnQuit = onQuit

	// 构建皮肤名称 C 字符串数组（以 NULL 结尾）
	skinNames := pet.SkinNames
	cSkinNames := make([]*C.char, len(skinNames)+1)
	for i, name := range skinNames {
		cSkinNames[i] = C.CString(name)
	}
	cSkinNames[len(skinNames)] = nil // NULL 终止符

	activeSkin := C.CString(pet.GetActiveSkinName())

	displayPort := 0
	if webEnabled {
		displayPort = webPort
	}

	C.createStatusBar(C.int(displayPort), (**C.char)(unsafe.Pointer(&cSkinNames[0])), activeSkin)

	// 释放 C 字符串
	go func() {
		time.Sleep(500 * time.Millisecond)
		for i := range skinNames {
			C.free(unsafe.Pointer(cSkinNames[i]))
		}
		C.free(unsafe.Pointer(activeSkin))
	}()
}

// goMenuCallback 由 ObjC 菜单点击时回调（在主线程调用）
//
//export goMenuCallback
func goMenuCallback(tag C.int) {
	tagInt := int(tag)
	fmt.Printf("[状态栏] 菜单点击回调触发, tag=%d\n", tagInt)

	switch {
	case tagInt == 0:
		// 退出：由 onQuit 回调负责清理子进程和服务后退出，不在此处直接 os.Exit
		if gOnQuit != nil {
			gOnQuit()
		}

	case tagInt == 1:
		// 关于
		go showAboutDialog()

	case tagInt == 2:
		// 打开 Web 管理界面
		go openBrowser(fmt.Sprintf("http://localhost:%d", gWebPort))

	case tagInt == 3:
		// 打开 Wails 设置窗口
		if gOnOpenSettings != nil {
			go gOnOpenSettings()
		}

	case tagInt >= 100:
		// 皮肤切换：tag=100 对应 SkinNames[0]，以此类推
		skinIndex := tagInt - 100
		if skinIndex >= 0 && skinIndex < len(pet.SkinNames) {
			skinName := pet.SkinNames[skinIndex]
			pet.SetActiveSkin(skinName)
			// 更新菜单勾选状态
			cActive := C.CString(skinName)
			C.updateSkinMenu(cActive)
			C.free(unsafe.Pointer(cActive))
		}
	}
}
