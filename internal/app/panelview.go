package app

import (
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"gopaste/internal/locales"
)

// panelView 按主题显示记录面板并让玻璃控制区悬浮于滚动列表上方
func (a *App) panelView(c *ui.Context, p palette) {
	box := ui.Column(c).Fill().Radius(14).Clip()
	if a.settings.Theme == themeGlass {
		box.Radius(18)
	}
	if c.Vibrancy() {
		box.Background(p.surface.Alpha(0.9))
	} else {
		box.Background(p.bg)
	}
	a.themeBackground(c, box, p)
	box.Children(func() {
		if a.settings.Theme == themeGlass && !a.preview {
			ui.Box(c).Grow(1).Children(func() {
				a.listPane(c, p)
				ui.Box(c).Absolute().Top(0).Left(0).Right(0).Height(glassHeaderHeight + 10).
					PassThrough().Material(glass.ScrollEdge{Background: p.bg})
				ui.Column(c).Absolute().Top(0).Left(0).Right(0).PassThrough().Children(func() {
					a.headerRow(c, p, false)
					a.tabsRow(c, p)
				})
			})
		} else {
			a.headerRow(c, p, false)
			a.tabsRow(c, p)
			hairline(c, p)
			if a.preview {
				a.previewPane(c, p)
			} else {
				a.listPane(c, p)
			}
		}
		a.statusRow(c, p)
	})
}

// headerRow 显示搜索框、设置与面板形态切换入口
func (a *App) headerRow(c *ui.Context, p palette, compact bool) {
	row := ui.Row(c).Padding(11, 12).Gap(7).AlignItems(ui.Center)
	if a.settings.Theme == themeGlass {
		row.PassThrough()
	}
	row.Children(func() {
		a.searchBox(c, p, compact)
		a.iconButton(c, p, icoGear, locales.Settings(), a.openSettings)
		if compact {
			a.iconButton(c, p, icoPanel, locales.SwitchToPointer(), func() { a.setForm(formPanel) })
		} else {
			a.iconButton(c, p, icoBar, locales.SwitchToBar(), func() { a.setForm(formBar) })
		}
	})
}

// searchBox 绘制自动聚焦的搜索框并处理搜索、预览、粘贴与长条导航
func (a *App) searchBox(c *ui.Context, p palette, compact bool) {
	box := ui.Row(c).Gap(6).AlignItems(ui.Center).Radius(8).Padding(0, 8)
	a.themeSurface(box, p.hover, false)
	box.Border(1, p.border)
	if compact {
		box.Width(240).Height(30)
	} else {
		box.Grow(1).Height(32)
	}
	if a.settings.Theme == themeGlass {
		box.Radius(10).BorderColor(ui.Transparent)
		if !compact {
			box.Height(36)
		}
	}
	if a.search.Focused(c) {
		box.Border(1, p.accentLine)
	}
	box.Children(func() {
		ui.Icon(c, icoSearch).Size(13, 13).TextColor(p.faint)
		field := ui.TextInputBase(c, &a.query).Bind(&a.search).Grow(1).AutoFocus()
		field.Placeholder(locales.SearchClipboard()).Label(locales.SearchClipboard()).FontSize(12.5)
		if compact {
			width, height := c.Size()
			field.OnTextKey(func(k ui.TextKey) bool {
				if k.Mods != 0 || k.Text() != "" {
					return false
				}
				switch k.Key {
				case ui.KeyLeft:
					a.moveSel(-1, width, height)
				case ui.KeyRight:
					a.moveSel(1, width, height)
				default:
					return false
				}
				return true
			})
		}
		if field.Changed() {
			mods, key := a.shortcutKeys("preview")
			if a.query == " " && mods == 0 && key == ui.KeySpace && c.Modifiers() == 0 {
				a.query = ""
				a.openPreview()
			} else {
				a.sel = 0
				a.list.ScrollIntoView(0)
			}
		}
		if field.Submitted() && a.once() {
			mods := c.Modifiers()
			if m, k := a.shortcutKeys("pastePlain"); k == ui.KeyEnter && m == mods {
				a.pasteSelected(true)
			} else if m, k := a.shortcutKeys("paste"); k == ui.KeyEnter && m == mods {
				a.pasteSelected(false)
			}
		}
		if a.query != "" {
			clear := ui.Row(c).Size(16, 16).Radius(8).Focusable().Cursor(ui.CursorPointer)
			clear.Center()
			if clear.Hovered() {
				clear.Background(p.border)
			}
			clear.Tooltip(locales.Clear())
			clear.Children(func() {
				ui.Icon(c, icoClose).Size(9, 9).TextColor(p.muted)
			})
			if clear.Clicked() {
				a.query = ""
				a.sel = 0
				a.search.Focus()
			}
		}
	})
}

// tabsRow 显示全部记录、用户列表及新增列表入口
func (a *App) tabsRow(c *ui.Context, p palette) {
	tabs := a.tabs()
	scroll := ui.ScrollHorizontal(c).Height(32).Padding(0, 8).Gap(4)
	if a.settings.Theme == themeGlass {
		scroll.PassThrough()
	}
	scroll.Children(func() {
		row := ui.Row(c).Gap(4).AlignItems(ui.Center)
		row.Children(func() {
			for i, tab := range tabs {
				i, tab := i, tab
				active := i == a.tab
				chip := ui.Row(c).Key(tab.ID).Padding(4, 9).Radius(7).Gap(5).Focusable()
				chip.Cursor(ui.CursorPointer)
				if active {
					a.themeSurface(chip, p.accent, true)
				} else if chip.Hovered() {
					chip.Background(p.hover)
				}
				chip.Children(func() {
					label := ui.Text(c, tab.Name).FontSize(11.5)
					if active {
						label.TextColor(p.accentText).Bold()
						if a.settings.Theme == themeGlass {
							label.TextColor(p.text)
						}
					} else {
						label.TextColor(p.muted)
					}
				})
				if chip.Clicked() {
					a.tab = i
					a.sel = 0
					a.vis = a.filtered()
					a.list.ScrollIntoView(0)
				}
				a.listDropTarget(chip, p, tab)
				if tab.ID != "all" {
					chip.ContextMenu(func(m *ui.Menu) { a.tabMenu(m, tab) })
				}
			}
			a.addTabButton(c, p)
		})
	})
}

// addTabButton 显示两种面板共用的普通列表与智能列表创建入口
func (a *App) addTabButton(c *ui.Context, p palette) {
	plus := ui.Row(c).Padding(4, 7).Radius(7).Focusable().Cursor(ui.CursorPointer)
	plus.Label(locales.AddList()).Tooltip(locales.AddList())
	if a.settings.Theme == themeGlass {
		a.themeSurface(plus, p.hover, true)
	} else if plus.Hovered() {
		plus.Background(p.hover)
	}
	plus.Children(func() {
		ui.Icon(c, icoPlus).Size(12, 12).TextColor(p.faint)
	})
	plus.Menu(func(m *ui.Menu) {
		if m.Item(locales.ManualList()).Chosen() {
			a.later(func() { a.newTab(false) })
		}
		if m.Item(locales.SmartList()).Chosen() {
			a.later(func() { a.newTab(true) })
		}
	})
}

// listPane 虚拟化显示记录并为悬浮玻璃控制区预留内容与滚动条空间
func (a *App) listPane(c *ui.Context, p palette) {
	n := len(a.vis)
	a.list.Key = func(i int) any { return a.vis[i].ID }
	a.list.Label = func(i int) string { return recordTitle(a.vis[i]) }
	list := ui.List(c, &a.list, n, func(i int) {
		a.panelRow(c, a.vis[i], i)
	}).Grow(1).TrackScroll(&a.listScroll).Padding(panelListPadding, 7).Gap(panelRowGap)
	if a.settings.Theme == themeGlass {
		list.Padding(glassHeaderHeight+panelListPadding, 7, panelListPadding).ScrollbarInsets(glassHeaderHeight, 0, 0)
	}
	list.Children(func() {
		if n == 0 {
			a.emptyState(c, p)
		}
	})
}

// emptyState 显示当前列表或搜索结果为空时的提示
func (a *App) emptyState(c *ui.Context, p palette) {
	ui.Column(c).Fill().Center().Gap(8).Padding(24).Children(func() {
		ui.Icon(c, icoClipboard).Size(26, 26).TextColor(p.faint)
		ui.Text(c, a.emptyTitle()).FontSize(12.5).TextColor(p.muted)
		if a.query != "" {
			ui.Text(c, locales.EmptySearchHint()).FontSize(11).TextColor(p.faint)
		} else if a.tab != 0 {
			ui.Text(c, locales.EmptyFilterHint()).FontSize(11).TextColor(p.faint)
		} else {
			ui.Text(c, locales.EmptyHistoryHint()).FontSize(11).TextColor(p.faint)
		}
	})
}

// emptyTitle 按当前搜索条件返回空状态标题
func (a *App) emptyTitle() string {
	if a.query != "" {
		return locales.SearchNotFound(a.query)
	}
	return locales.EmptyHistory()
}

// statusRow 显示列表数量、操作反馈、权限状态与快捷键提示
func (a *App) statusRow(c *ui.Context, p palette) {
	if !a.hintUntil.IsZero() {
		if remaining := a.hintUntil.Sub(c.Now()); remaining > 0 {
			c.After(remaining)
		} else {
			a.hintFlash = ""
			a.hintUntil = time.Time{}
		}
	}
	row := ui.Row(c).Height(statusHeight).Padding(0, 12).Gap(10).AlignItems(ui.Center)
	row.BorderWidth(1, 0, 0, 0).BorderColor(p.hair)
	row.Children(func() {
		ui.Text(c, locales.RecordsCount(len(a.vis))).FontSize(10.5).TextColor(p.faint)
		if a.hintFlash != "" {
			ui.Text(c, "·").FontSize(10.5).TextColor(p.faint)
			ui.Text(c, a.hintFlash).FontSize(10.5).TextColor(p.muted).SingleLine()
		}
		ui.Spacer(c)
		if a.settings.Paused {
			ui.Text(c, locales.RecordingPaused()).FontSize(10.5).TextColor(p.muted)
		}
		if a.needsAccess() {
			warn := ui.Row(c).Gap(4).AlignItems(ui.Center).Focusable().Cursor(ui.CursorPointer)
			warn.Children(func() {
				ui.Icon(c, icoShield).Size(11, 11).TextColor(ui.Hex("#e0a33a"))
				ui.Text(c, locales.EnableAutoPaste()).FontSize(10.5).TextColor(ui.Hex("#c98a2b"))
			})
			warn.Tooltip(locales.OpenSystemSettingsHint())
			if warn.Clicked() {
				a.later(a.requestAccess)
			}
			return
		}
		ui.Text(c, a.hintLine()).FontSize(10.5).TextColor(p.faint)
	})
}

// hintLine 显示自定义粘贴与预览快捷键提示
func (a *App) hintLine() string {
	paste := prettyHotkey(a.shortcutAccel("paste"))
	preview := prettyHotkey(a.shortcutAccel("preview"))
	return locales.PanelHint(paste, preview)
}

// ---- small shared widgets ----

func hairline(c *ui.Context, p palette) {
	ui.Box(c).Height(1).Background(p.hair)
}

// iconButton 显示带提示的图标按钮并按主题配置玻璃按压反馈
func (a *App) iconButton(c *ui.Context, p palette, svg *ui.SVG, tip string, onClick func()) {
	b := ui.Button(c, "").Size(28, 28).Padding(0).Radius(8)
	b.Cursor(ui.CursorPointer)
	if a.settings.Theme == themeGlass {
		a.themeSurface(b.Radius(14), p.hover, true)
	} else if b.Hovered() {
		b.Background(p.hover)
	}
	b.Tooltip(tip)
	b.Children(func() {
		ui.Icon(c, svg).Size(14, 14).TextColor(p.muted)
	})
	if b.Clicked() {
		onClick()
	}
}

func (a *App) needsAccess() bool {
	if !isDarwin {
		return false
	}
	return !accessTrusted()
}
