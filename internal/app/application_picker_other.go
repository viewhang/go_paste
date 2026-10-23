//go:build !darwin

package app

import (
	"github.com/egoist/mygo"

	"gopaste/internal/locales"
)

// chooseApps 通过系统文件选择器选择可执行应用
func chooseApps(parent *mygo.Window) ([]string, error) {
	return mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent: parent, Title: locales.AddExcludedApp(),
		Filters:  []mygo.FileFilter{{Name: locales.AppFilter(), Extensions: []string{"exe"}}},
		Multiple: true,
	})
}
