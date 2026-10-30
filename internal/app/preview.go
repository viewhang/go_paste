package app

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"gopaste/internal/locales"
	"gopaste/internal/types"
)

// previewPane 显示当前记录的完整内容与预览操作
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
		a.releasePreview()
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
	kc := p.muted
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

// previewBody 显示记录对应的文本、静态图片、动图、链接、颜色或文件预览
func (a *App) previewBody(c *ui.Context, p palette, r *types.Record) {
	body := ui.Column(c).Grow(1).Gap(8)
	w, h := c.Size()
	if a.form == formBar {
		h = barPreview
	}
	imagePath := ""
	if r.Kind == types.KindImage && r.Animated && (r.ImageFormat == "gif" || r.ImageFormat == "webp") {
		imagePath = a.st.BlobPath(r.Image)
	} else if r.Kind == types.KindFile && len(r.Files) == 1 {
		ext := strings.ToLower(filepath.Ext(r.Files[0].Path))
		if ext == ".gif" || ext == ".webp" {
			imagePath = r.Files[0].Path
		}
	}
	if imagePath != "" {
		body.Padding(12).Children(func() {
			player, bitmap := a.previewImageFile(imagePath, max(w-24, h-100))
			if player != nil {
				ui.Video(c, player).Fill().Fit(ui.Contain).Radius(8).Label(recordTitle(r)).
					ContextMenu(func(m *ui.Menu) { a.cardMenu(c, m, r) })
			} else if bitmap != nil {
				ui.Image(c, bitmap).Fill().Fit(ui.Contain).Radius(8).Label(recordTitle(r)).
					ContextMenu(func(m *ui.Menu) { a.cardMenu(c, m, r) })
			} else {
				ui.Text(c, locales.ImageMissing()).FontSize(12).TextColor(p.warning).Center()
			}
		})
		return
	}
	if a.previewMediaPath != "" {
		a.releasePreview()
	}
	switch r.Kind {
	case types.KindImage:
		body.Padding(12).Children(func() {
			box := ui.Box(c).Fill().Radius(8).Clip()
			box.Background(tint(p.muted, p.bg, 0.05))
			box.Children(func() {
				if b := a.previewBitmap(r.Image, max(w-24, h-100)); b != nil {
					ui.Image(c, b).Fill().Fit(ui.Contain)
				} else {
					ui.Text(c, locales.ImageMissing()).FontSize(12).TextColor(p.warning).Center()
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
				if a.themePrimaryButton(c, locales.OpenInBrowser()).Clicked() {
					a.later(func() { _ = mygo.Shell.OpenExternal(r.URL) })
				}
			})
		})
	case types.KindColor:
		body.Padding(12).Gap(8).Children(func() {
			ui.Box(c).Grow(1).Radius(10).Background(ui.Hex(r.Color)).
				InsetShadow(0, 0, 0, 1, ui.RGBA(0, 0, 0, 0.18))
			for _, option := range colorPasteFormats {
				val := ColorString(r.Color, strings.ToLower(option.label))
				rowEl := ui.Row(c).Padding(7, 10).Radius(7).Gap(10).AlignItems(ui.Center).
					Focusable().Cursor(ui.CursorPointer).Tooltip(locales.PasteAs() + " " + option.label)
				rowEl.Border(1, p.border)
				if rowEl.Hovered() {
					rowEl.Background(p.hover)
				}
				rowEl.Children(func() {
					ui.Text(c, option.label).FontSize(9.5).Bold().
						TextColor(p.muted).Width(30)
					ui.Text(c, val).FontSize(12).TextColor(p.text).Grow(1).SingleLine()
					ui.Icon(c, icoClipboard).Size(11, 11).TextColor(p.faint)
				})
				if rowEl.Clicked() {
					a.later(func() { a.pasteAs(r, option.format) })
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
					a.pasteAsMenu(m, &types.Record{ID: r.ID, Kind: types.KindFile, Files: []types.FileRef{f}})
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
						ui.Text(c, locales.FileMoved()).FontSize(10).TextColor(p.warning)
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

// previewFoot 显示预览记录的粘贴与删除入口
func (a *App) previewFoot(c *ui.Context, p palette, r *types.Record) {
	ui.Row(c).Padding(8, 12).Gap(7).AlignItems(ui.Center).BorderWidth(1, 0, 0, 0).
		BorderColor(p.hair).Children(func() {
		if a.themePrimaryButton(c, locales.Paste()).Clicked() {
			a.later(func() { a.paste(r, false) })
		}
		ui.Spacer(c)
		a.iconButton(c, p, icoTrash, locales.DeleteRecord(), func() { a.later(func() { a.remove(r) }) }).TextColor(p.danger)
	})
}
