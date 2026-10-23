// Package assets 嵌入应用图标与菜单栏模板图标
//
// 资源编译进可执行文件，运行时不再读取磁盘上的图片，因此交付物只有单个
// 可执行文件：放在项目根的 resources 目录会被 mygo build 视为要随应用
// 附带的资源，复制到可执行文件旁边。
package assets

import _ "embed"

// TrayIcon 保存菜单栏单色描边模板图标
//
//go:embed tray.png
var TrayIcon []byte

// AppIcon 保存应用图标
//
//go:embed icon.png
var AppIcon []byte
