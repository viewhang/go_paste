package app

import (
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"gopaste/internal/locales"
	"gopaste/internal/native"
	"gopaste/internal/types"
)

// settingsView 显示固定的设置分类菜单与独立滚动的分类明细
func (a *App) settingsView(c *ui.Context) {
	applyTheme(c)
	p := a.palette(c)
	root := c.Root().Background(p.bg)
	a.themeBackground(c, root, p)
	ui.Column(c).Fill().Children(func() {
		a.settingsHeader(c, p)
		hairline(c, p)
		ui.Row(c).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
			pages := []struct{ id, title string }{
				{"general", locales.General()},
				{"shortcuts", locales.Shortcuts()},
				{"privacy", locales.Privacy()},
				{"advanced", locales.Advanced()},
				{"about", locales.About()},
			}
			menu := ui.Sidebar(c, &a.settingsPage, func() {
				for _, page := range pages {
					ui.SidebarItem(c, page.id, nil, page.title)
				}
			}).Width(160).Background(p.bg)
			if menu.Changed() {
				a.recording = ""
			}
			ui.Box(c).Width(1).Background(p.hair)
			title := ""
			for _, page := range pages {
				if a.settingsPage == page.id {
					title = page.title
				}
			}
			a.settingsContent(c, p, title)
		})
	})
}

// settingsHeader 绘制可拖动的设置标题栏并为系统窗口按钮预留空间
func (a *App) settingsHeader(c *ui.Context, p palette) ui.Element {
	bar := c.TitleBar()
	head := ui.Row(c).Height(max(bar.Height, 46)).
		Padding(0, bar.Right+14, 0, bar.Left+16).Gap(10).AlignItems(ui.Center).DragWindow()
	head.Children(func() {
		ui.Text(c, locales.Settings()).FontSize(13.5).Bold().TextColor(p.text)
	})
	return head
}

// settingsContent 显示当前分类的标题与独立滚动的设置项
func (a *App) settingsContent(c *ui.Context, p palette, title string) {
	ui.Scroll(c).Key(a.settingsPage).Grow(1).MinWidth(0).Padding(20, 22).Children(func() {
		ui.Column(c).Gap(20).Children(func() {
			ui.Text(c, title).FontSize(20).Bold().TextColor(p.text)
			switch a.settingsPage {
			case "general":
				a.settingsAppearance(c, p)
				a.settingsHistory(c, p)
			case "shortcuts":
				a.settingsShortcuts(c, p)
			case "privacy":
				a.settingsPrivacy(c, p)
			case "advanced":
				a.settingsPaste(c, p)
				a.settingsData(c, p)
			case "about":
				a.settingsUpdates(c, p)
			}
		})
	})
}

func sectionTitle(c *ui.Context, p palette, text string) {
	ui.Text(c, text).FontSize(10.5).Bold().TextColor(p.faint).LetterSpacing(0.6)
}

func settingRow(c *ui.Context, p palette, title, desc string, control func()) {
	ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			ui.Text(c, title).FontSize(12.5).TextColor(p.text)
			if desc != "" {
				ui.Text(c, desc).FontSize(11).TextColor(p.muted)
			}
		})
		ui.Row(c).Children(control)
	})
}

// card 按主题绘制设置分组卡片
func (a *App) card(c *ui.Context, p palette) ui.Element {
	e := ui.Column(c).Padding(14, 16).Gap(13).Radius(11)
	e.Background(p.surface).Border(1, p.border)
	if a.settings.Theme == themeGlass {
		e.Radius(16).Background(p.surface.Alpha(0.82)).BorderColor(p.border.Alpha(0.65))
	}
	return e
}

// settingsAppearance 显示语言、主题与面板形态设置
func (a *App) settingsAppearance(c *ui.Context, p palette) {
	ui.Column(c).Gap(8).Children(func() {
		sectionTitle(c, p, locales.Appearance())
		box := a.card(c, p)
		box.Children(func() {
			a.settingsLanguage(c, p)
			ui.Divider(c)
			theme := a.settings.Theme
			settingRow(c, p, locales.Theme(), locales.ThemeSystemHint(), func() {
				ui.Segmented(c, &theme, locales.ClassicTheme(), "Liquid Glass").Label(locales.Theme())
			})
			if theme != a.settings.Theme {
				a.setSetting(func(s *types.Settings) { s.Theme = theme })
			}
			ui.Divider(c)
			form := a.form
			settingRow(c, p, locales.PanelForm(), "", func() {
				ui.Segmented(c, &form, locales.BottomBar(), locales.FollowPointer()).Label(locales.PanelForm())
			})
			if form != a.form {
				a.setForm(form)
			}
		})
	})
}

// settingsPaste 显示粘贴方式、粘贴后排序与搜索重置及链接预览设置
func (a *App) settingsPaste(c *ui.Context, p palette) {
	ui.Column(c).Gap(8).Children(func() {
		sectionTitle(c, p, locales.Paste())
		box := a.card(c, p)
		box.Children(func() {
			plain := a.settings.PastePlain
			settingRow(c, p, locales.PastePlainDefault(), locales.PastePlainHint(), func() {
				ui.Switch(c, &plain).Label(locales.PastePlainDefault())
			})
			if plain != a.settings.PastePlain {
				a.setSetting(func(s *types.Settings) { s.PastePlain = plain })
			}
			ui.Divider(c)
			moveToFront := a.settings.MoveToFrontOnPaste
			settingRow(c, p, locales.MoveToFrontOnPaste(), "", func() {
				ui.Switch(c, &moveToFront).Label(locales.MoveToFrontOnPaste())
			})
			if moveToFront != a.settings.MoveToFrontOnPaste {
				a.setSetting(func(s *types.Settings) { s.MoveToFrontOnPaste = moveToFront })
			}
			ui.Divider(c)
			reset := a.settings.ResetPanelAfterPaste
			settingRow(c, p, locales.ResetPanelAfterPaste(), "", func() {
				ui.Switch(c, &reset).Label(locales.ResetPanelAfterPaste())
			})
			if reset != a.settings.ResetPanelAfterPaste {
				a.setSetting(func(s *types.Settings) { s.ResetPanelAfterPaste = reset })
			}
			ui.Divider(c)
			link := a.settings.LinkPreview
			settingRow(c, p, locales.LinkPreview(), locales.LinkPreviewHint(), func() {
				ui.Switch(c, &link).Label(locales.LinkPreview())
			})
			if link != a.settings.LinkPreview {
				a.setSetting(func(s *types.Settings) { s.LinkPreview = link })
			}
		})
	})
}

// settingsHistory 显示历史条数、容量、保留时间与启动记录设置
func (a *App) settingsHistory(c *ui.Context, p palette) {
	ui.Column(c).Gap(8).Children(func() {
		sectionTitle(c, p, locales.HistoryStartup())
		box := a.card(c, p)
		box.Children(func() {
			maxRecords := float64(a.settings.MaxRecords)
			settingRow(c, p, locales.MaxRecords(), locales.MaxRecordsHint(), func() {
				ui.NumberInput(c, &maxRecords, 50, 5000, 50).Label(locales.MaxRecords())
			})
			if int(maxRecords) != a.settings.MaxRecords {
				a.setSetting(func(s *types.Settings) { s.MaxRecords = int(maxRecords) })
			}
			ui.Divider(c)
			limit := float64(a.settings.HistoryLimitMiB)
			settingRow(c, p, locales.HistoryStorageLimit(), locales.HistoryStorageHint(), func() {
				ui.NumberInput(c, &limit, 1, 65536, 64).Label(locales.HistoryStorageLimit())
				ui.Text(c, "MiB").MarginStart(6).TextColor(p.muted)
			})
			if int(limit) != a.settings.HistoryLimitMiB {
				a.setSetting(func(s *types.Settings) { s.HistoryLimitMiB = int(limit) })
			}
			ui.Divider(c)
			days := a.settings.RetentionDays
			settingRow(c, p, locales.Retention(), "", func() {
				opts := []string{locales.Forever(), locales.RetentionDays(30), locales.RetentionDays(14), locales.RetentionDays(7), locales.RetentionDays(1)}
				values := []int{0, 30, 14, 7, 1}
				cur := locales.Forever()
				for i, v := range values {
					if v == days {
						cur = opts[i]
					}
				}
				chosen := cur
				ui.Select(c, &chosen, opts).Label(locales.Retention())
				if chosen != cur {
					for i, o := range opts {
						if o == chosen {
							v := values[i]
							a.setSetting(func(s *types.Settings) { s.RetentionDays = v })
						}
					}
				}
			})
			ui.Divider(c)
			login := a.settings.LaunchAtLogin
			settingRow(c, p, locales.LaunchAtLogin(), "", func() {
				ui.Switch(c, &login).Label(locales.LaunchAtLogin())
			})
			if login != a.settings.LaunchAtLogin {
				a.setSetting(func(s *types.Settings) { s.LaunchAtLogin = login })
			}
			ui.Divider(c)
			paused := a.settings.Paused
			settingRow(c, p, locales.PauseRecording(), "", func() {
				ui.Switch(c, &paused).Label(locales.PauseRecording())
			})
			if paused != a.settings.Paused {
				a.setSetting(func(s *types.Settings) { s.Paused = paused })
				if !paused && a.watch != nil {
					a.watch.Resync()
				}
			}
		})
	})
}

// settingsData 显示数据位置、历史清理与辅助功能权限设置
func (a *App) settingsData(c *ui.Context, p palette) {
	ui.Column(c).Gap(8).Children(func() {
		sectionTitle(c, p, locales.Data())
		box := a.card(c, p)
		box.Children(func() {
			settingRow(c, p, locales.RecordLocation(), a.dataDir(), func() {
				ui.Button(c, locales.ShowInFolder()).OnClick(func() {
					mygo.Shell.ShowItemInFolder(a.dataDir())
				})
			})
			ui.Divider(c)
			count := len(a.records)
			settingRow(c, p, locales.ClearHistory(), locales.RecordsPresent(count), func() {
				if ui.Button(c, locales.ClearAction()).Clicked() {
					res, err := mygo.Dialog.Message(mygo.MessageOptions{
						Type:          mygo.MessageWarning,
						Title:         locales.ClearHistoryTitle(),
						Message:       locales.ClearHistoryMessage(),
						Buttons:       []string{locales.ClearAction(), locales.Cancel()},
						DefaultButton: 1,
						CancelButton:  1,
					})
					if err == nil && res.Button == 0 && a.st != nil {
						if err := a.st.Clear(); err != nil {
							a.storageError(err)
							return
						}
						a.reload()
						if a.win != nil {
							a.win.Invalidate()
						}
					}
				}
			})
			if isDarwin {
				ui.Divider(c)
				trusted := native.AccessibilityTrusted()
				desc := locales.AccessNeeded()
				if trusted {
					desc = ""
				}
				settingRow(c, p, locales.AccessPermission(), desc, func() {
					if trusted {
						ui.Text(c, locales.Enabled()).FontSize(11.5).TextColor(p.muted)
					} else if ui.PrimaryButton(c, locales.EnableAccess()).Clicked() {
						native.PromptAccessibility()
					}
				})
			}
		})
	})
}

func (a *App) dataDir() string {
	if a.st != nil {
		return a.st.Dir()
	}
	return filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "GoPaste")
}
