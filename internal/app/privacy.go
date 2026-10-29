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
		a.settingsExcludedApps(c, p)
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

// settingsExcludedApps 显示交替底色的应用单选列表与底部添加、移除按钮
func (a *App) settingsExcludedApps(c *ui.Context, p palette) {
	var apps []types.ExcludedApp
	for _, app := range a.settings.ExcludedApps {
		if app.Path = excludedAppPath(app, a.installedApps); app.Path != "" {
			apps = append(apps, app)
		}
	}
	ui.Column(c).Gap(8).Children(func() {
		sectionTitle(c, p, locales.ExcludedApps())
		box := ui.Column(c).Radius(8).Border(1, p.border).Background(p.bg).Clip()
		selected := ui.Local(box, "selected", func() int { return -1 })
		state := ui.Local(box, "list", func() ui.ListState { return ui.ListState{} })
		state.Selected = selected
		state.Key = func(i int) any { return apps[i].ID }
		state.Label = func(i int) string { return apps[i].Name }
		box.Children(func() {
			ui.List(c, state, len(apps)).Height(float32(clampInt(len(apps), 3, 7)) * 28).Label(locales.ExcludedApps()).Rows(func(r ui.ListRow) {
				app := apps[r.Index]
				row := ui.Row(c).Height(28).Padding(0, 10).Gap(8).AlignItems(ui.Center).TextColor(p.text)
				if r.Selected() {
					row.Background(p.accent).TextColor(p.accentText)
				} else if r.Index%2 == 1 {
					row.Background(p.text.Alpha(0.035))
				}
				row.Children(func() {
					if icon := a.fileIcon(app.Path); icon != nil {
						ui.Image(c, icon).Size(18, 18).Fit(ui.Contain)
					}
					ui.Text(c, app.Name).Grow(1).MaxLines(1).Ellipsis("…").FontSize(12.5)
				})
			})
			ui.Box(c).Height(1).Background(p.border)
			ui.Row(c).Height(28).Padding(2, 4).AlignItems(ui.Center).Background(p.text.Alpha(0.035)).Role(ui.RoleToolbar).Children(func() {
				button := func(symbol, label string, disabled bool) ui.Element {
					b := ui.ButtonBase(c).Size(26, 24).Padding(0).Radius(3).Label(label).Tooltip(label).Disabled(disabled).TextColor(p.muted)
					if b.Hovered() && !disabled {
						b.Background(p.hover)
					}
					b.Children(func() { ui.Text(c, symbol).FontSize(18) })
					return b
				}
				button("+", locales.AddExcludedApp(), a.choosingExcludedApps).OnClick(a.chooseExcludedApps)
				ui.Box(c).Width(1).Height(16).Margin(0, 3).Background(p.border)
				label := locales.RemoveExcludedApp(locales.AppFilter())
				if *selected >= 0 {
					label = locales.RemoveExcludedApp(apps[*selected].Name)
				}
				if button("−", label, *selected < 0).Clicked() {
					id := apps[*selected].ID
					a.setSetting(func(s *types.Settings) {
						s.ExcludedApps = slices.DeleteFunc(slices.Clone(s.ExcludedApps), func(app types.ExcludedApp) bool { return app.ID == id })
					})
					*selected = -1
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
