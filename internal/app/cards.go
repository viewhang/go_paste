package app

import (
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"gopaste/internal/locales"
	"gopaste/internal/types"
)

// appIcon 读取记录保存的来源应用图标
func (a *App) appIcon(r *types.Record) *ui.Bitmap {
	if r.Source.Icon == "" {
		return nil
	}
	return a.thumb(r.Source.Icon, 40)
}

// recordThumb 读取记录内容的图片、链接预览或文件图标
func (a *App) recordThumb(r *types.Record) *ui.Bitmap {
	switch r.Kind {
	case types.KindImage:
		return a.thumb(r.Image, 56)
	case types.KindLink:
		return a.thumb(r.PreviewImage, 56)
	case types.KindFile:
		if recordFileCount(r) > 0 {
			return a.fileIcon(r.Files[0].Path)
		}
	}
	return nil
}

// ---- shared pieces ----

func cardTime(c *ui.Context, r *types.Record, size float32) ui.Element {
	p := pal(c)
	return ui.Text(c, relTime(r.CreatedAt)).FontSize(size).TextColor(p.faint).
		Tooltip(fullTime(r.CreatedAt))
}

// thumbTile is a rounded square holding a picture, or the type icon on a
// tinted background when there is none.
func thumbTile(c *ui.Context, b *ui.Bitmap, r *types.Record, size float32) ui.Element {
	p := pal(c)
	box := ui.Box(c).Size(size, size).Radius(size * 0.26).Clip()
	if b != nil {
		box.Children(func() {
			ui.Image(c, b).Fill().Fit(ui.Cover)
		})
		return box
	}
	kc := kindColor(r.Kind, p.dark)
	box.Background(tint(kc, p.surface, 0.14))
	box.Children(func() {
		a := ui.Row(c).Fill().Center()
		a.Children(func() {
			ui.Icon(c, kindIcon(r.Kind)).Size(size*0.42, size*0.42).TextColor(kc)
		})
	})
	return box
}

func swatchTile(c *ui.Context, color string, size float32) ui.Element {
	col := ui.Hex(color)
	box := ui.Box(c).Size(size, size).Radius(size * 0.26).Background(col)
	box.InsetShadow(0, 0, 0, 1, ui.RGBA(0, 0, 0, 0.16))
	return box
}

// ---- panel rows ----

// panelRow 按主题展示记录、支持拖拽并将点击后的键盘焦点交给列表
func (a *App) panelRow(c *ui.Context, r *types.Record, i int) {
	p := a.palette(c)
	sel := i == a.sel
	row := ui.Row(c).Height(a.panelRowHeight()).Padding(9, 10).Gap(10).Radius(10).AlignItems(ui.Center)
	row.Focusable().Cursor(ui.CursorPointer)
	background, border := p.surface, p.hair
	iconSize, titleSize, timeSize := float32(40), float32(12.5), float32(10)
	if a.settings.Theme == themeGlass {
		background, border = ui.Transparent, ui.Transparent
		iconSize, titleSize, timeSize = 30, 13, 11
		row.Radius(11).Gap(11)
	}
	if sel {
		background = p.accentSoft
		if a.settings.Theme != themeGlass {
			border = p.accentLine
		}
	} else if row.Hovered() {
		background = p.raised
		if a.settings.Theme != themeGlass {
			border = p.border
		}
	}
	row.Background(background).Border(1, border)
	row.ContextMenu(func(m *ui.Menu) { a.cardMenu(c, m, r) })
	a.recordDrag(c, row, r)
	row.Children(func() {
		thumbTile(c, a.appIcon(r), r, iconSize).Tooltip(r.Source.AppName).Label(locales.SourceApp(r.Source.AppName))
		if r.Kind == types.KindColor {
			swatchTile(c, r.Color, 40)
		} else if b := a.recordThumb(r); b != nil {
			ui.Image(c, b).Size(56, 40).Fit(ui.Contain)
		}
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			ui.Text(c, recordTitle(r)).FontSize(titleSize).SingleLine().TextColor(p.text)
		})
		ui.Column(c).Gap(3).AlignItems(ui.End).Children(func() {
			cardTime(c, r, timeSize).TextColor(p.faint)
			if hint := a.quickPasteHint(c, i); hint != "" {
				ui.Text(c, hint).FontSize(11).TextColor(p.text)
			}
		})
	})
	if row.Clicked() {
		a.sel = i
		a.list.Focus(c)
	}
	if row.DoubleClicked() {
		a.later(func() { a.paste(r, false) })
	}
}

// ---- bar cards ----

// barCard 按主题显示等比缩放的卡片和快速粘贴编号并支持拖拽
func (a *App) barCard(c *ui.Context, r *types.Record, i int) {
	p := a.palette(c)
	sel := i == a.sel
	kc := kindColor(r.Kind, p.dark)
	_, height := c.Size()
	width, cardHeight := a.barCardSize(height)
	card := ui.Column(c).Size(width, cardHeight).Radius(12).Clip().Focusable()
	card.Border(2, p.border)
	if a.settings.Theme == themeGlass {
		card.BorderWidth(1)
	}
	if sel {
		card.BorderColor(p.accent)
	}
	card.Cursor(ui.CursorPointer)
	background := p.surface
	if card.Hovered() && !sel {
		background = p.raised
	}
	if sel && a.settings.Theme == themeGlass {
		background = p.accentSoft
		card.BorderColor(p.accentLine)
	}
	card.Background(background)
	card.ContextMenu(func(m *ui.Menu) { a.cardMenu(c, m, r) })
	a.recordDrag(c, card, r)
	card.Children(func() {
		ui.Column(c).Padding(8, 9).Gap(5).Grow(1).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Grow(1).Gap(2).Children(func() {
					ui.Text(c, kindLabel(r.Kind)).FontSize(11).Bold().TextColor(kc)
					cardTime(c, r, 9.5)
				})
				thumbTile(c, a.appIcon(r), r, 32).Tooltip(r.Source.AppName).Label(locales.SourceApp(r.Source.AppName))
			})
			barBody(c, a, r, p)
		})
		if hint := a.quickPasteHint(c, i); hint != "" {
			ui.Text(c, hint).Absolute().Right(7).Bottom(6).Padding(2, 4).Radius(4).
				Background(background).FontSize(11).TextColor(p.text).PassThrough()
		}
	})
	if card.Clicked() {
		if a.once() {
			a.paste(r, false)
		}
	}
	if card.Pressed() && !sel {
		a.sel = i
	}
}

// barBody 显示卡片对应的文本、图片、链接、颜色或文件内容
func barBody(c *ui.Context, a *App, r *types.Record, p palette) {
	body := ui.Box(c).Grow(1)
	switch r.Kind {
	case types.KindImage:
		body.Radius(7).Clip()
		body.Children(func() {
			_, height := c.Size()
			width, bodyHeight := a.barCardSize(height)
			width -= 22
			bodyHeight -= 56
			size := max(width, bodyHeight)
			if r.ImageW > 0 && r.ImageH > 0 {
				scale := max(width/float32(r.ImageW), bodyHeight/float32(r.ImageH))
				size = float32(max(r.ImageW, r.ImageH)) * scale
			}
			if b := a.thumb(r.Image, size); b != nil {
				ui.Image(c, b).Fill().Fit(ui.Cover)
			}
			if r.Animated {
				ui.Badge(c, "GIF").Absolute().Bottom(4).Left(4).FontSize(9)
			}
		})
	case types.KindLink:
		body.Gap(4)
		body.Children(func() {
			ui.Text(c, recordTitle(r)).FontSize(11.5).MaxLines(2).TextColor(p.text)
			ui.Text(c, hostOf(r.URL)).FontSize(10).TextColor(p.faint).SingleLine()
		})
	case types.KindColor:
		body.Radius(7).Clip().Background(ui.Hex(r.Color))
		body.InsetShadow(0, 0, 0, 1, ui.RGBA(0, 0, 0, 0.16))
		body.Children(func() {
			row := ui.Row(c).Fill().Padding(6).AlignItems(ui.End).Justify(ui.End)
			row.Children(func() {
				ui.Text(c, strings.ToUpper(r.Color)).FontSize(10).Bold().
					TextColor(onColor(ui.Hex(r.Color)))
			})
		})
	case types.KindFile:
		body.Gap(3).Children(func() {
			names := make([]string, 0, 3)
			for i, f := range r.Files {
				if i == 3 {
					break
				}
				names = append(names, f.Name)
			}
			for _, n := range names {
				ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icoFile).Size(10, 10).TextColor(p.faint)
					ui.Text(c, n).FontSize(10.5).TextColor(p.text).SingleLine()
				})
			}
			if recordFileCount(r) > 3 {
				ui.Text(c, locales.MoreFiles(recordFileCount(r)-3)).FontSize(10).TextColor(p.faint)
			}
		})
	default:
		body.Children(func() {
			txt := cardBodyText(r.Text, 260)
			if txt == "" {
				txt = locales.EmptyText()
			}
			ui.Text(c, txt).FontSize(11.5).MaxLines(6).TextColor(p.text)
		})
	}
}

// ---- menus ----

// cardMenu 显示记录操作、粘贴格式、普通列表移动入口与自定义快捷键
func (a *App) cardMenu(c *ui.Context, m *ui.Menu, r *types.Record) {
	if m.Item(locales.Paste()).Shortcut(a.shortcutKeys("paste")).Chosen() {
		a.later(func() { a.paste(r, false) })
	}
	a.pasteAsMenu(m, r)
	m.Separator()
	m.Submenu(locales.MoveTo(), func(s *ui.Menu) {
		if s.Item(locales.All()).Checked(r.ListID == "").Disabled(r.ListID == "").Chosen() {
			a.later(func() { a.moveToList(r, types.Tab{ID: "all", Name: locales.All()}) })
		}
		for _, tab := range a.settings.CustomTabs {
			if tab.Smart {
				continue
			}
			if s.Item(tab.Name).Checked(r.ListID == tab.ID).Disabled(r.ListID == tab.ID).Chosen() {
				a.later(func() { a.moveToList(r, tab) })
			}
		}
	})
	switch r.Kind {
	case types.KindLink:
		if m.Item(locales.OpenInBrowser()).Chosen() {
			a.later(func() {
				if full := a.fullRecord(r); full != nil {
					_ = mygo.Shell.OpenExternal(full.URL)
				}
			})
		}
	case types.KindImage:
		if m.Item(locales.ExportImage()).Chosen() {
			a.later(func() { a.exportImage(r) })
		}
	case types.KindFile:
		if recordFileCount(r) > 0 {
			if m.Item(locales.Open()).Chosen() {
				a.later(func() { _ = mygo.Shell.OpenPath(r.Files[0].Path) })
			}
			if m.Item(locales.ShowInFolder()).Chosen() {
				a.later(func() { mygo.Shell.ShowItemInFolder(r.Files[0].Path) })
			}
		}
	}
	m.Separator()
	if m.Item(locales.Delete()).Shortcut(a.shortcutKeys("delete")).Chosen() {
		a.later(func() { a.remove(r) })
	}
}

// later runs an action once the frame is built, safe for changes that take
// rows out of the collection being built.
func (a *App) later(fn func()) {
	a.pending = append(a.pending, fn)
}

func (a *App) runPending() {
	pending := a.pending
	a.pending = nil
	for _, fn := range pending {
		fn()
	}
}
