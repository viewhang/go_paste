//go:build darwin

package app

import (
	"github.com/egoist/mygo"

	"gopaste/internal/locales"
	"gopaste/internal/native"
)

// chooseApps 在主线程使用应用包类型选择 macOS 应用
func chooseApps(parent *mygo.Window) ([]string, error) {
	var paths []string
	mygo.RunOnMain(func() { paths = native.ChooseApps(locales.AddExcludedApp()) })
	return paths, nil
}
