package app

import (
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"gopaste/internal/locales"
	"gopaste/internal/types"
)

// panelView 按主题显示记录面板并让整块玻璃顶栏悬浮于滚动列表上方
func (a *App) panelView(c *ui.Context, p palette) {
	box := ui.Column(c).Fill().Radius(14).Clip()
	if a.settings.Theme == themeGlass {
		box.Radius(20).Background(p.bg)
		if c.Vibrancy() {
			box.Background(p.bg.Alpha(0.92))
		}
	} else if c.Vibrancy() {
		box.Background(p.bg.Alpha(0.9))
	} else {
		box.Background(p.bg)
	}
	box.Children(func() {
		if a.settings.Theme == themeGlass && !a.preview {
			ui.Box(c).Grow(1).Children(func() {
				a.listPane(c, p)
				ui.Column(c).Absolute().Top(0).Left(0).Right(0).PassThrough().Children(func() {
					a.panelHeader(c, p)
				})
			})
		} else {
			a.panelHeader(c, p)
			if a.preview {
				a.previewPane(c, p)
			} else {
				a.listPane(c, p)
			}
		}
		a.statusRow(c, p)
	})
}

// panelHeader 按主题将搜索与列表分组绘制为统一顶栏
func (a *App) panelHeader(c *ui.Context, p palette) {
	header := ui.Column(c)
	if a.settings.Theme == themeGlass {
		header.Height(glassHeaderHeight).PassThrough().Material(glass.Glass{Tint: p.chrome.Alpha(0.6)})
		header.BorderWidth(0, 0, 1, 0).BorderColor(p.hair)
	} else {
		header.Background(p.chrome)
	}
	header.Children(func() {
		a.headerRow(c, p, false)
		a.tabsRow(c, p)
		if a.settings.Theme != themeGlass {
			hairline(c, p)
		}
	})
}

// headerRow 显示搜索框与统一设置菜单入口
func (a *App) headerRow(c *ui.Context, p palette, compact bool) {
	row := ui.Row(c).Padding(11, 12).Gap(7).AlignItems(ui.Center)
	if a.settings.Theme == themeGlass {
		row.PassThrough()
		if !compact {
			row.Padding(14, 14, 10)
		}
	}
	row.Children(func() {
		a.searchBox(c, p, compact)
		a.settingsButton(c, p)
	})
}

// settingsButton 为两种主题和布局提供设置分类、形态切换与暂停记录菜单
func (a *App) settingsButton(c *ui.Context, p palette) {
	button := ui.ButtonBase(c).Size(30, 30).Radius(8).Cursor(ui.CursorPointer).
		Label(locales.Settings()).Tooltip(locales.Settings())
	if button.Hovered() {
		button.Background(p.hover)
	}
	button.Children(func() {
		ui.Icon(c, icoGear).Size(18, 18).TextColor(p.muted)
	})
	button.Menu(func(m *ui.Menu) {
		for _, page := range []struct{ id, title string }{
			{"general", locales.Appearance()},
			{"shortcuts", locales.Shortcuts()},
			{"about", locales.About()},
		} {
			if m.Item(page.title).Chosen() {
				a.settingsPage = page.id
				a.later(a.openSettings)
			}
		}
		m.Separator()
		form, label := formBar, locales.SwitchToBar()
		if a.form == formBar {
			form, label = formPanel, locales.SwitchToPointer()
		}
		if m.Item(label).Chosen() {
			a.later(func() { a.setForm(form) })
		}
		if m.Item(locales.PauseRecording()).Checked(a.settings.Paused).Chosen() {
			a.setSetting(func(s *types.Settings) { s.Paused = !s.Paused })
			if !a.settings.Paused && a.watch != nil {
				a.watch.Resync()
			}
		}
	})
}

// searchBox 绘制自动聚焦的搜索框并处理搜索、删除、预览、粘贴与长条导航
func (a *App) searchBox(c *ui.Context, p palette, compact bool) {
	box := ui.Row(c).Gap(6).AlignItems(ui.Center).Radius(8).Padding(0, 8).Background(p.surface)
	box.Border(1, p.border)
	if compact {
		box.Width(240).Height(30)
	} else {
		box.Grow(1).Height(32)
	}
	if a.settings.Theme == themeGlass {
		box.Radius(10).BorderColor(p.hair)
		if !compact {
			box.Height(38)
		}
	}
	if a.search.Focused(c) {
		box.Border(1, p.accentLine)
	}
	box.Children(func() {
		ui.Icon(c, icoSearch).Size(13, 13).TextColor(p.faint)
		field := ui.TextInputBase(c, &a.query).Bind(&a.search).Grow(1).AutoFocus()
		field.Placeholder(locales.SearchClipboard()).Label(locales.SearchClipboard()).FontSize(12.5)
		width, height := c.Size()
		field.OnTextKey(func(k ui.TextKey) bool {
			if a.recording != "" || a.prompt != nil || k.Text() != "" {
				return false
			}
			if mods, key := a.shortcutKeys("delete"); k.Key == key && k.Mods == mods {
				if rec := a.selected(); rec != nil {
					a.later(func() { a.remove(rec) })
				}
				return true
			}
			if !compact || k.Mods != 0 {
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

// tabsRow 显示列表分组与新增入口，切换分组时保留输入焦点与滚动区域外的对称留白
func (a *App) tabsRow(c *ui.Context, p palette) {
	tabs := a.tabs()
	scroll := ui.ScrollHorizontal(c).Height(32).Margin(0, 8).Gap(4)
	if a.settings.Theme == themeGlass {
		scroll.Height(40).Margin(0, 14).Padding(0, 0, 10)
	}
	scroll.Children(func() {
		row := ui.Row(c).Gap(4).AlignItems(ui.Center)
		row.Children(func() {
			for i, tab := range tabs {
				i, tab := i, tab
				active := i == a.tab
				chip := ui.Row(c).Key(tab.ID).Padding(4, 9).Radius(7).Gap(5).Focusable().KeepFocus()
				if a.settings.Theme == themeGlass {
					chip.Height(28).Padding(4, 11).Radius(8)
				}
				chip.Cursor(ui.CursorPointer)
				if active {
					chip.Background(p.accentSoft)
				} else if chip.Hovered() {
					chip.Background(p.hover)
				}
				chip.Children(func() {
					label := ui.Text(c, tab.Name).FontSize(11.5)
					if active {
						label.TextColor(p.accent).Bold()
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
	if plus.Hovered() {
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

// listPane 保持滚动偏移展示虚拟记录列表并为悬浮玻璃控制区预留空间
func (a *App) listPane(c *ui.Context, p palette) {
	n := len(a.vis)
	a.list.KeepOffset = true
	a.list.Key = func(i int) any { return a.vis[i].ID }
	a.list.Label = func(i int) string { return recordTitle(a.recordAt(i)) }
	list := ui.List(c, &a.list, n, func(i int) {
		a.panelRow(c, a.recordAt(i), i)
	}).Grow(1).TrackScroll(&a.listScroll).Padding(panelListPadding, 7).Gap(panelRowGap)
	if a.settings.Theme == themeGlass {
		list.Padding(glassHeaderHeight+panelListPadding, 10, panelListPadding).ScrollbarInsets(glassHeaderHeight, 0, 0)
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
		} else if a.tab == 0 {
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

// statusRow 显示列表数量、操作反馈与权限状态
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
				ui.Icon(c, icoShield).Size(11, 11).TextColor(p.warning)
				ui.Text(c, locales.EnableAutoPaste()).FontSize(10.5).TextColor(p.warning)
			})
			warn.Tooltip(locales.OpenSystemSettingsHint())
			if warn.Clicked() {
				a.later(a.requestAccess)
			}
		}
	})
}

// ---- small shared widgets ----

func hairline(c *ui.Context, p palette) {
	ui.Box(c).Height(1).Background(p.hair)
}

// iconButton 返回支持语义着色的轻量图标按钮并显示悬停反馈
func (a *App) iconButton(c *ui.Context, p palette, svg *ui.SVG, tip string, onClick func()) ui.Element {
	b := ui.ButtonBase(c).Size(28, 28).Padding(0).Radius(8).Label(tip).TextColor(p.muted)
	b.Cursor(ui.CursorPointer)
	if b.Hovered() {
		b.Background(p.hover)
	}
	b.Tooltip(tip)
	b.Children(func() {
		ui.Icon(c, svg).Size(14, 14)
	})
	if b.Clicked() {
		onClick()
	}
	return b
}

func (a *App) needsAccess() bool {
	if !isDarwin {
		return false
	}
	return !accessTrusted()
}
