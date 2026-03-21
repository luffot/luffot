//go:build darwin
// +build darwin

package luffot

// runMainRunLoop 在 macOS 上不再需要独立的事件循环。
// Ebiten 的 RunGame() 内部会启动自己的 macOS RunLoop，
// 该 RunLoop 同时能驱动 NSStatusBar 菜单点击等事件。
// 此函数保留为空实现，仅在非 Ebiten 模式下作为 fallback。
func runMainRunLoop() {
	// Ebiten RunGame 已经接管了主线程事件循环，
	// 如果走到这里说明 Ebiten 未启动，用 select{} 阻塞主线程
	select {}
}
