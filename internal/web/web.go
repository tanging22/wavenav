package web

import (
	"embed"
	"io/fs"
)

// dist 里放前端构建产物；未构建时只有占位 index.html，服务仍能起来（提示去构建）。
//
//go:embed all:dist
var embedded embed.FS

// FS 返回以 dist 为根的文件系统。
func FS() fs.FS {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		return embedded
	}
	return sub
}
