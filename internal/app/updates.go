package app

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/ui"

	"gopaste/internal/locales"
)

// settingsUpdates 显示当前版本、更新检查与自动下载安装偏好
func (a *App) settingsUpdates(c *ui.Context, p palette) {
	ui.Column(c).Gap(8).Children(func() {
		a.card(c, p).Children(func() {
			version := mygo.App.Version()
			if version == "" {
				version = locales.DevelopmentVersion()
			}
			settingRow(c, p, locales.CurrentVersion(), version, func() {
				if ui.Button(c, locales.CheckUpdates()).Clicked() {
					updater.CheckForUpdates()
				}
			})
			if !a.updatesEnabled {
				return
			}
			ui.Divider(c)
			automatic := updater.AutomaticChecks()
			settingRow(c, p, locales.AutoCheckUpdates(), "", func() {
				ui.Switch(c, &automatic).Label(locales.AutoCheckUpdates())
			})
			if automatic != updater.AutomaticChecks() {
				updater.SetAutomaticChecks(automatic)
			}
			ui.Divider(c)
			download := updater.AutomaticDownloads()
			settingRow(c, p, locales.AutoInstallUpdates(), "", func() {
				ui.Switch(c, &download).Label(locales.AutoInstallUpdates()).Disabled(!automatic)
			})
			if download != updater.AutomaticDownloads() {
				updater.SetAutomaticDownloads(download)
			}
			if last := updater.LastCheck(); !last.IsZero() {
				ui.Text(c, locales.LastUpdateCheck(last.Local())).FontSize(11).TextColor(p.muted)
			}
		})
	})
}
