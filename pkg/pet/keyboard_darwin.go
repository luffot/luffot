package pet

import (
	"fmt"
	"sync/atomic"
	"time"

	hook "github.com/robotn/gohook"
)

// keyHeld 当前是否有按键被按下（1=有，0=无）
var keyHeld atomic.Int32

// lastKeyUpTime 最后一次 KeyUp 事件的时间（纳秒 Unix 时间戳）
var lastKeyUpNano atomic.Int64

// toggleBarrage Shift+Alt+T 被按下时置 1，Go 层读取后清零
var toggleBarrage atomic.Int32

// globalKeyListenerStarted 防止重复启动
var globalKeyListenerStarted atomic.Bool

// startGlobalKeyboardListener 使用 gohook 启动全局键盘事件监听
// 任意按键按下时将桌宠切换为打字状态，同时检测 Shift+Alt+L 组合键
func startGlobalKeyboardListener() {
	if !globalKeyListenerStarted.CompareAndSwap(false, true) {
		return
	}
	fmt.Println("[PET] startGlobalKeyboardListener: launching gohook goroutine...")
	go func() {
		evChan := hook.Start()
		defer hook.End()

		// 追踪修饰键状态，用于检测 Shift+Alt+T 组合键
		shiftPressed := false
		altPressed := false

		for ev := range evChan {
			switch ev.Kind {
			case hook.KeyDown, hook.KeyHold:
				keyHeld.Store(1)

				// 追踪修饰键
				keychar := string(ev.Keychar)
				rawcode := ev.Rawcode
				// shift rawcode: 56(左), 60(右)；alt rawcode: 58(左), 61(右)
				if rawcode == 56 || rawcode == 60 {
					shiftPressed = true
				}
				if rawcode == 58 || rawcode == 61 {
					altPressed = true
				}

				// 检测 Shift+Alt+L 组合键（L 的 macOS rawcode 为 37）
				if rawcode == 37 && shiftPressed && altPressed {
					toggleBarrage.Store(1)
					fmt.Println("[PET] Shift+Alt+L detected! toggle barrage")
				}
				_ = keychar

			case hook.KeyUp:
				// 记录最后一次 KeyUp 时间，不立即清零 keyHeld
				// isGlobalKeyHeld() 会根据时间差判断是否仍处于按键状态
				lastKeyUpNano.Store(time.Now().UnixNano())
				keyHeld.Store(0)

				// 修饰键抬起时清除状态
				rawcode := ev.Rawcode
				if rawcode == 56 || rawcode == 60 {
					shiftPressed = false
				}
				if rawcode == 58 || rawcode == 61 {
					altPressed = false
				}
			}
		}
	}()
}

// isGlobalKeyHeld 检测当前是否有按键被按下。
// keyHeld 为 1 时直接返回 true；
// keyHeld 为 0 时，若距最后一次 KeyUp 不足 100ms，也视为仍在按键（避免连击间隙导致状态闪烁）。
func isGlobalKeyHeld() bool {
	if keyHeld.Load() == 1 {
		return true
	}
	lastUp := lastKeyUpNano.Load()
	if lastUp == 0 {
		return false
	}
	return time.Since(time.Unix(0, lastUp)) < 100*time.Millisecond
}

// ConsumeToggleBarrage 检测 Shift+Alt+T 是否被按下（读取并清零，供 barrage.go 调用）
func ConsumeToggleBarrage() bool {
	return toggleBarrage.Swap(0) == 1
}
