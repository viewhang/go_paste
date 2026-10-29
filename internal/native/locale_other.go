//go:build !darwin

package native

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
)

// LocaleOverride 读取 MyGo 保存的应用语言，空值表示跟随系统
func LocaleOverride() string {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return ""
	}
	data, _ := os.ReadFile(filepath.Join(dir, "locale"))
	return strings.TrimSpace(string(data))
}
