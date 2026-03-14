// Package embedfs 提供编译时嵌入的静态资源文件系统。
// 此包位于 pkg/embedfs/，通过 //go:embed 指令引用项目根目录的 static/ 目录。
// 使用方式：import "github.com/luffot/luffot/pkg/embedfs"，然后调用 embedfs.WebStaticFS() 和 embedfs.SkinsFS()。
package embedfs

import (
	"embed"
	"io/fs"
)

// webStaticEmbedFS 嵌入 static/web 目录下的所有静态文件（HTML、JS、CSS 等）
//
//go:embed all:static/web
var webStaticEmbedFS embed.FS

// WebStaticFS 返回以 static/web 为根的子文件系统，供 web.Server 使用
func WebStaticFS() fs.FS {
	sub, err := fs.Sub(webStaticEmbedFS, "static/web")
	if err != nil {
		panic("无法获取 static/web 子文件系统: " + err.Error())
	}
	return sub
}

// skinsEmbedFS 嵌入 static/skins 目录下的所有皮肤资源（GIF、PNG、JSON 等）
//
//go:embed all:static/skins
var skinsEmbedFS embed.FS

// SkinsFS 返回以 static/skins 为根的子文件系统，供 pet.AutoLoadImageSkinsFromFS 使用
func SkinsFS() fs.FS {
	sub, err := fs.Sub(skinsEmbedFS, "static/skins")
	if err != nil {
		panic("无法获取 static/skins 子文件系统: " + err.Error())
	}
	return sub
}
