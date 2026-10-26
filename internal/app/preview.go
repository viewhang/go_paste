package app

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"gopaste/internal/locales"
	"gopaste/internal/types"
)

// previewPane shows the chosen record in full: the whole text, the big
// picture, the page behind a link, a color in three notations, the files
// of a file record.
func (a *App) previewPane(c *ui.Context, p palette) {
	rec := a.detail
	if rec == nil {
		rec = a.selected()
	}
	box := ui.Column(c)
	if a.form == formBar {
		box.Height(barPreview)
	} else {
		box.Grow(1)
	}
	if rec == nil {
		box.Children(func() { a.emptyState(c, p) })
		return
	}
	box.Children(func() {
		a.previewHead(c, p, rec)
		hairline(c, p)
		a.previewBody(c, p, rec)
		a.previewFoot(c, p, rec)
	})
}

// previewHead 显示预览记录的类型、摘要与关闭入口
func (a *App) previewHead(c *ui.Context, p palette, r *types.Record) {
	kc := kindColor(r.Kind, p.dark)
	ui.Row(c).Padding(9, 12).Gap(8).AlignItems(ui.Center).Children(func() {
		tile := ui.Row(c).Padding(3, 7).Radius(6).Gap(5).AlignItems(ui.Center)
		tile.Background(tint(kc, p.surface, 0.16))
		tile.Children(func() {
			ui.Icon(c, kindIcon(r.Kind)).Size(11, 11).TextColor(kc)
			ui.Text(c, kindLabel(r.Kind)).FontSize(10.5).TextColor(kc).Bold()
		})
		meta := previewMeta(r)
		if meta != "" {
			ui.Text(c, meta).FontSize(10.5).TextColor(p.faint).SingleLine()
		}
		ui.Spacer(c)
		ui.Text(c, fullTime(r.CreatedAt)).FontSize(10).TextColor(p.faint)
		a.iconButton(c, p, icoClose, locales.ClosePreview(), a.closePreview)
	})
}

// previewMeta 按当前语言返回预览记录的摘要
func previewMeta(r *types.Record) string {
	switch r.Kind {
	case types.KindImage:
		parts := []string{}
		if r.ImageW > 0 {
			parts = append(parts, fmt.Sprintf("%d × %d", r.ImageW, r.ImageH))
		}
		if r.ImageFormat != "" {
			parts = append(parts, strings.ToUpper(r.ImageFormat))
		}
		if r.Animated {
			parts = append(parts, locales.AnimatedImage())
		}
		if s := sizeText(r.Bytes); s != "" {
			parts = append(parts, s)
		}
		return strings.Join(parts, " · ")
	case types.KindLink:
		return hostOf(r.URL)
	case types.KindFile:
		return locales.FilePreviewMeta(len(r.Files), sizeText(r.Bytes))
	case types.KindText:
		return locales.CharacterCount(utf8.RuneCountInString(r.Text))
	}
	return ""
}

// previewBody 显示记录对应的文本、图片、链接、颜色或文件预览
func (a *App) previewBody(c *ui.Context, p palette, r *types.Record) {
	body := ui.Column(c).Grow(1).Gap(8)
	w, h := c.Size()
	if a.form == formBar {
		h = barPreview
	}
	switch r.Kind {
	case types.KindImage:
		body.Padding(12).Children(func() {
			box := ui.Box(c).Fill().Radius(8).Clip()
			box.Background(tint(kindColor(r.Kind, p.dark), p.bg, 0.05))
			box.Children(func() {
				if b := a.previewBitmap(r.Image, max(w-24, h-100)); b != nil {
					ui.Image(c, b).Fill().Fit(ui.Contain)
				} else {
					ui.Text(c, locales.ImageMissing()).FontSize(12).TextColor(p.faint).Center()
				}
			})
		})
	case types.KindLink:
		body.Padding(12).Gap(10).Children(func() {
			if b := a.previewBitmap(r.PreviewImage, max(w-24, 150)); b != nil {
				ui.Box(c).Height(150).Radius(8).Clip().Children(func() {
					ui.Image(c, b).Fill().Fit(ui.Cover)
				})
			}
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, recordTitle(r)).FontSize(15).Bold().MaxLines(2).TextColor(p.text)
				ui.Text(c, r.URL).FontSize(11).TextColor(p.accent).SingleLine()
			})
			ui.Row(c).Gap(7).Children(func() {
				if ui.PrimaryButton(c, locales.OpenInBrowser()).Clicked() {
					a.later(func() { _ = mygo.Shell.OpenExternal(r.URL) })
				}
				if ui.Button(c, locales.CopyURL()).Clicked() {
					a.later(func() { a.copyText(r.Text) })
				}
			})
		})
	case types.KindColor:
		body.Padding(12).Gap(8).Children(func() {
			ui.Box(c).Grow(1).Radius(10).Background(ui.Hex(r.Color)).
				InsetShadow(0, 0, 0, 1, ui.RGBA(0, 0, 0, 0.18))
			for _, f := range []string{"hex", "rgb", "hsl"} {
				f := f
				val := ColorString(r.Color, f)
				rowEl := ui.Row(c).Padding(7, 10).Radius(7).Gap(10).AlignItems(ui.Center).
					Focusable().Cursor(ui.CursorPointer)
				rowEl.Border(1, p.border)
				if rowEl.Hovered() {
					rowEl.Background(p.hover)
				}
				rowEl.Children(func() {
					ui.Text(c, strings.ToUpper(f)).FontSize(9.5).Bold().
						TextColor(kindColor(types.KindColor, p.dark)).Width(30)
					ui.Text(c, val).FontSize(12).TextColor(p.text).Grow(1).SingleLine()
					ui.Icon(c, icoCopy).Size(11, 11).TextColor(p.faint)
				})
				if rowEl.Clicked() {
					a.later(func() { a.copyText(val) })
				}
			}
		})
	case types.KindFile:
		body.Padding(12).Children(func() {
			ui.List(c.Key(r.ID), nil, len(r.Files), func(i int) {
				f := r.Files[i]
				rowEl := ui.Row(c).Padding(6, 8).Radius(7).Gap(8).AlignItems(ui.Center).
					Focusable().Cursor(ui.CursorPointer)
				if rowEl.Hovered() {
					rowEl.Background(p.hover)
				}
				rowEl.ContextMenu(func(m *ui.Menu) {
					if m.Item(locales.Open()).Chosen() {
						a.later(func() { _ = mygo.Shell.OpenPath(f.Path) })
					}
					if m.Item(locales.ShowInFolder()).Chosen() {
						a.later(func() { mygo.Shell.ShowItemInFolder(f.Path) })
					}
					if m.Item(locales.CopyFilePath()).Chosen() {
						a.later(func() { a.copyText(f.Path) })
					}
				})
				rowEl.Children(func() {
					if b := a.fileIcon(f.Path); b != nil {
						ui.Image(c, b).Size(20, 20)
					} else {
						ui.Icon(c, icoFile).Size(14, 14).TextColor(p.faint)
					}
					ui.Column(c).Grow(1).Gap(1).Children(func() {
						ui.Text(c, f.Name).FontSize(12).TextColor(p.text).SingleLine()
						ui.Text(c, f.Path).FontSize(10).TextColor(p.faint).SingleLine()
					})
					if f.Missing {
						ui.Text(c, locales.FileMoved()).FontSize(10).TextColor(ui.Hex("#c98a2b"))
					} else if s := sizeText(f.Size); s != "" {
						ui.Text(c, s).FontSize(10).TextColor(p.faint)
					}
					ui.Icon(c, icoFolder).Size(12, 12).TextColor(p.faint)
				})
				if rowEl.Clicked() {
					a.later(func() { _ = mygo.Shell.OpenPath(f.Path) })
				}
			}).Fill().Gap(2)
		})
	default:
		body.Padding(12).Children(func() {
			text, truncated := textPrefix(r.Text, 20000)
			if truncated {
				text += "\n\n" + locales.PreviewTruncated()
			}
			ui.Scroll(c).Fill().Children(func() {
				ui.Text(c, text).Selectable().FontSize(12.5).TextColor(p.text)
			})
		})
	}
}

// previewFoot 显示预览记录的粘贴、复制与删除入口
func (a *App) previewFoot(c *ui.Context, p palette, r *types.Record) {
	ui.Row(c).Padding(8, 12).Gap(7).AlignItems(ui.Center).BorderWidth(1, 0, 0, 0).
		BorderColor(p.hair).Children(func() {
		if ui.PrimaryButton(c, locales.Paste()).Clicked() {
			a.later(func() { a.paste(r, false) })
		}
		if ui.Button(c, locales.PlainText()).Clicked() {
			a.later(func() { a.paste(r, true) })
		}
		if ui.Button(c, locales.Copy()).Clicked() {
			a.later(func() { a.copy(r, false) })
		}
		ui.Spacer(c)
		del := ui.Button(c, "")
		del.Size(28, 28).Padding(0).Radius(7).Cursor(ui.CursorPointer)
		if del.Hovered() {
			del.Background(p.hover)
		}
		del.Tooltip(locales.DeleteRecord())
		del.Children(func() {
			ui.Icon(c, icoTrash).Size(13, 13).TextColor(p.muted)
		})
		if del.Clicked() {
			a.later(func() { a.remove(r) })
		}
	})
}
