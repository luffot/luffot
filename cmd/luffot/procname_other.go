//go:build !darwin
// +build !darwin

package luffot

// setProcessName 在非 macOS 平台上设置进程名
func setProcessName(name string) {
	// 非 macOS 平台暂不实现
}
