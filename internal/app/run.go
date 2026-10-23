// Package app 管理剪贴板应用的生命周期、窗口与交互界面
package app

import (
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	updatenative "github.com/egoist/mygo/plugins/updater/native"

	"gopaste/internal/assets"
	"gopaste/internal/locales"
)

// Run 启动应用并注册语言、更新与窗口生命周期
func Run() {
	a := NewApp()

	mygo.App.SetName("GoPaste")
	// No Dock icon and no app switcher entry: the app lives in the menu bar.
	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	// Closing the settings or the guide leaves the app running in the menu
	// bar, its panel hidden rather than closed.
	mygo.App.OnWindowAllClosed(func() {})

	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	mygo.Use(updatenative.New(updater.Options{
		Icon: assets.AppIcon,
		Strings: map[string]updater.Strings{
			"zh-Hans": {
				MenuItem: locales.In("zh-Hans").CheckUpdates(), Checking: locales.In("zh-Hans").CheckingUpdates(),
				Downloading: locales.In("zh-Hans").DownloadingUpdates(), Installing: locales.In("zh-Hans").InstallingUpdates(),
			},
			"en": {
				MenuItem: locales.In("en").CheckUpdates(), Checking: locales.In("en").CheckingUpdates(),
				Downloading: locales.In("en").DownloadingUpdates(), Installing: locales.In("en").InstallingUpdates(),
			},
		},
	}))
	mygo.App.OnLocaleChanged(a.refreshLanguage)
	updater.OnChange(func() {
		if a.setWin != nil {
			a.setWin.Invalidate()
		}
	})
	mygo.App.OnSecondInstance(func([]string, string) { a.show() })
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) { a.shutdown() })
	mygo.App.WhenReady(a.ready)

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
