//go:build !darwin
// +build !darwin

package luffot

import "os"

// runMainRunLoop 在非 macOS 平台上等待退出
func runMainRunLoop() {
	// 在其他平台上，简单地阻塞等待
	// 由于 os.Exit 会在 goroutine 中调用，这里只需要一个无限循环
	select {}
}
