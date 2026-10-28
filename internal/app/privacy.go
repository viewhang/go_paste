package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"gopaste/internal/locales"
	"gopaste/internal/native"
	"gopaste/internal/types"
)

// settingsPrivacy 显示排除应用及机密内容、空白文本与图片大小规则
func (a *App) settingsPrivacy(c *ui.Context, p palette) {
	ui.Column(c).Gap(20).Children(func() {
		a.card(c, p).Children(func() {
			settingRow(c, p, locales.ExcludedApps(), "", func() {
				a.themeButton(c, locales.AddExcludedApp()).Disabled(a.choosingExcludedApps).OnClick(a.chooseExcludedApps)
			})
			remove := -1
			for i, app := range a.settings.ExcludedApps {
				path := excludedAppPath(app, a.installedApps)
				if path == "" {
					continue
				}
				ui.Row(c).Key(app.ID).Gap(10).AlignItems(ui.Center).Children(func() {
					if icon := a.fileIcon(path); icon != nil {
						ui.Image(c, icon).Size(24, 24).Fit(ui.Contain)
					}
					ui.Text(c, app.Name).Grow(1).MaxLines(1).Ellipsis("…").FontSize(12.5).TextColor(p.text)
					if a.themeButton(c, "×").Label(locales.RemoveExcludedApp(app.Name)).Clicked() {
						remove = i
					}
				})
			}
			if remove >= 0 {
				a.setSetting(func(s *types.Settings) {
					s.ExcludedApps = slices.Delete(slices.Clone(s.ExcludedApps), remove, remove+1)
				})
			}
		})
		ui.Column(c).Gap(8).Children(func() {
			sectionTitle(c, p, locales.OtherRules())
			a.card(c, p).Children(func() {
				confidential := a.settings.ExcludeConfidential
				settingRow(c, p, locales.ExcludeConfidential(), "", func() {
					a.themeSwitch(c, &confidential).Label(locales.ExcludeConfidential())
				})
				if confidential != a.settings.ExcludeConfidential {
					a.setSetting(func(s *types.Settings) { s.ExcludeConfidential = confidential })
				}
				ui.Divider(c)
				whitespace := a.settings.ExcludeWhitespace
				settingRow(c, p, locales.ExcludeWhitespace(), "", func() {
					a.themeSwitch(c, &whitespace).Label(locales.ExcludeWhitespace())
				})
				if whitespace != a.settings.ExcludeWhitespace {
					a.setSetting(func(s *types.Settings) { s.ExcludeWhitespace = whitespace })
				}
				ui.Divider(c)
				limit := float64(a.settings.ImageLimitMiB)
				settingRow(c, p, locales.ImageSizeLimit(), locales.ImageSizeLimitHint(), func() {
					a.themeNumberInput(c, &limit, 0, 65536, 1).Label(locales.ImageSizeLimit())
					ui.Text(c, "MiB").MarginStart(6).TextColor(p.muted)
				})
				if int(limit) != a.settings.ImageLimitMiB {
					a.setSetting(func(s *types.Settings) { s.ImageLimitMiB = int(limit) })
				}
			})
		})
	})
}

// loadInstalledApps 在后台读取本机应用列表并在主线程更新设置界面
func (a *App) loadInstalledApps() {
	win := a.setWin
	go func() {
		apps, err := mygo.Shell.InstalledApps()
		if err != nil {
			mygo.Dialog.Error(locales.LoadAppsFailed(), err.Error())
			return
		}
		win.Update(func() { a.installedApps = apps })
	}()
}

// excludedAppPath 返回已安装排除应用的位置以显示应用与系统图标
func excludedAppPath(app types.ExcludedApp, installed []mygo.InstalledApp) string {
	if app.Path != "" {
		if _, err := os.Stat(app.Path); err == nil {
			return app.Path
		}
		return ""
	}
	for _, candidate := range installed {
		if strings.EqualFold(candidate.ID, app.ID) || !isDarwin &&
			(strings.EqualFold(filepath.Base(candidate.ID), app.ID) || strings.EqualFold(candidate.Name, app.Name)) {
			return candidate.Path
		}
	}
	return ""
}

// chooseExcludedApps 异步打开系统选择器并在主线程保存所选应用
func (a *App) chooseExcludedApps() {
	win := a.setWin
	a.choosingExcludedApps = true
	go func() {
		paths, err := chooseApps(win)
		win.Update(func() {
			a.choosingExcludedApps = false
			if err == nil && len(paths) > 0 {
				err = a.addExcludedApps(paths)
			}
			if err != nil {
				go mygo.Dialog.Error(locales.AddExcludedAppFailed(), err.Error())
			}
		})
	}()
}

// addExcludedApps 读取所选应用标识与位置，去重后保存排除列表
func (a *App) addExcludedApps(paths []string) error {
	apps := slices.Clone(a.settings.ExcludedApps)
	for _, path := range paths {
		id, err := native.AppIdentifier(path)
		if err != nil {
			return err
		}
		name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		app := types.ExcludedApp{ID: id, Name: name, Path: path}
		if i := slices.IndexFunc(apps, func(existing types.ExcludedApp) bool { return strings.EqualFold(existing.ID, id) }); i >= 0 {
			apps[i] = app
		} else {
			apps = append(apps, app)
		}
	}
	if !slices.Equal(apps, a.settings.ExcludedApps) {
		a.setSetting(func(s *types.Settings) { s.ExcludedApps = apps })
	}
	return nil
}
