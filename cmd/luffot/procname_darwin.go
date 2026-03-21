//go:build darwin
// +build darwin

package luffot

// setProcessName 在 macOS 上设置进程名
// 注意：macOS 活动监视器显示的是可执行文件名，无法通过代码修改
// 此函数主要用于设置线程名，方便调试
func setProcessName(name string) {
	// macOS 活动监视器显示进程名来自可执行文件名
	// 无法通过运行时代码修改
	// 此函数保留为空实现，以便将来可能的扩展
}
