package app

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"

	"gopaste/internal/assets"
	"gopaste/internal/locales"
	"gopaste/internal/types"
)

// setUpTray 创建菜单栏或通知区域图标并绑定常用设置菜单
func (a *App) setUpTray() {
	tray, err := mygo.NewTray(mygo.TrayOptions{
		ID:   "gopaste",
		Icon: assets.TrayIcon,
		// The glyph is one colour, so macOS tints it with the bar.
		IconIsTemplate: true,
		ToolTip:        "GoPaste",
		Title:          "",
	})
	if err != nil {
		log.Print("no tray icon: ", err)
		return
	}
	a.tray = tray
	a.refreshTray()
}

// refreshTray 更新面板、记录偏好与软件更新菜单
func (a *App) refreshTray() {
	if a.tray == nil {
		return
	}
	menu := mygo.NewMenu([]*mygo.MenuItem{
		{Label: locales.OpenGoPaste(), Accelerator: a.hotkeyAccel(), Click: func(*mygo.MenuItem, *mygo.Window) {
			a.openPanel()
		}},
		{Label: locales.Settings(), Click: func(*mygo.MenuItem, *mygo.Window) { a.openSettings() }},
		mygo.Separator(),
		{Label: locales.BottomBar(), Type: mygo.MenuItemRadio, Checked: a.form == formBar,
			Click: func(*mygo.MenuItem, *mygo.Window) { a.setForm(formBar) }},
		{Label: locales.PointerPanel(), Type: mygo.MenuItemRadio, Checked: a.form == formPanel,
			Click: func(*mygo.MenuItem, *mygo.Window) { a.setForm(formPanel) }},
		mygo.Separator(),
		{Label: locales.PauseRecording(), Type: mygo.MenuItemCheckbox, Checked: a.settings.Paused,
			Click: func(it *mygo.MenuItem, _ *mygo.Window) {
				paused := it.Checked
				a.setSetting(func(s *types.Settings) { s.Paused = paused })
				if !paused && a.watch != nil {
					a.watch.Resync()
				}
			}},
		mygo.Separator(),
		{Label: locales.ClearHistory(), Click: func(*mygo.MenuItem, *mygo.Window) {
			res, err := mygo.Dialog.Message(mygo.MessageOptions{
				Type:          mygo.MessageWarning,
				Title:         locales.ClearHistoryTitle(),
				Message:       locales.ClearHistoryMessage(),
				Buttons:       []string{locales.ClearAction(), locales.Cancel()},
				DefaultButton: 1,
				CancelButton:  1,
			})
			if err != nil || res.Button != 0 {
				return
			}
			if a.st != nil {
				if err := a.st.Clear(); err != nil {
					a.storageError(err)
					return
				}
			}
			a.reload()
			if a.win != nil {
				a.win.Invalidate()
			}
		}},
		mygo.Separator(),
		updater.MenuItem(),
		{Role: mygo.RoleQuit},
	})
	a.tray.SetMenu(menu)
}
