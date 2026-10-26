// Package native 提供剪贴板来源、辅助功能权限、模拟粘贴与复制提示音的原生接口
package native

// App 保存剪贴板来源应用的标识、名称与图标路径
type App struct {
	Name string `json:"name"`
	ID   string `json:"id"`
	// Path is the .app bundle (macOS) or executable (Windows), when known.
	Path string `json:"path"`
	Pid  int    `json:"pid"`
	Self bool   `json:"self"`
}
