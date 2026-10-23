package app

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"gopaste/internal/locales"
)

// settingsLanguage 显示应用语言选择并通过 MyGo 保存选择或恢复跟随系统
func (a *App) settingsLanguage(c *ui.Context, p palette) {
	settingRow(c, p, locales.Language(), "", func() {
		languages := mygo.App.Languages()
		choices := []string{locales.FollowSystem()}
		for _, tag := range languages {
			choices = append(choices, i18n.Autonym(tag))
		}
		current := i18n.Autonym(i18n.Locale())
		selected := current
		ui.Select(c, &selected, choices).Label(locales.Language())
		if selected != current {
			tag := ""
			for i, choice := range choices[1:] {
				if selected == choice {
					tag = languages[i]
					break
				}
			}
			mygo.App.SetLocale(tag)
		}
	})
}

// refreshLanguage 刷新托盘菜单与已打开窗口的本地化标题
func (a *App) refreshLanguage(string) {
	a.refreshTray()
	if a.setWin != nil {
		a.setWin.SetTitle(locales.SettingsTitle())
	}
	if a.obWin != nil {
		a.obWin.SetTitle(locales.WelcomeTitle())
	}
}
