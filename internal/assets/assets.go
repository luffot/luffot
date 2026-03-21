// Package assets 提供编译时嵌入的静态资源文件系统
package assets

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// WailsAssets 嵌入 frontend/dist 目录下的所有前端构建产物
// 注意：wails build 时会自动将 frontend/dist 的内容嵌入到二进制中
// 开发时从项目根目录的 frontend/dist 读取
//
//go:embed all:frontend/dist
var wailsAssetsFS embed.FS

// WailsAssetsFS 返回以 frontend/dist 为根的子文件系统，供 Wails 设置窗口使用
// 优先尝试从项目根目录的 frontend/dist 读取（开发模式），
// 如果失败则使用嵌入的资源（生产模式）
func WailsAssetsFS() fs.FS {
	// 首先尝试从项目根目录的 frontend/dist 读取（开发模式或构建前已复制）
	if rootDist := findRootFrontendDist(); rootDist != "" {
		return os.DirFS(rootDist)
	}

	// 使用嵌入的资源
	sub, err := fs.Sub(wailsAssetsFS, "frontend/dist")
	if err != nil {
		panic("无法获取 frontend/dist 子文件系统: " + err.Error())
	}
	return sub
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
		if parent == dir || strings.HasSuffix(parent, ".app") {
			break
		}
		dir = parent
	}

	return ""
}

// HasEmbeddedAssets 检查嵌入的资源是否有效
func HasEmbeddedAssets() bool {
	// 检查嵌入的文件系统中是否有 index.html
	f, err := fs.Sub(wailsAssetsFS, "frontend/dist")
	if err != nil {
		return false
	}
	_, err = fs.Stat(f, "index.html")
	return err == nil
}
