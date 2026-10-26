package app

import (
	"os"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"

	"gopaste/internal/clip"
	"gopaste/internal/locales"
	"gopaste/internal/native"
	"gopaste/internal/types"
)

// keyboard 处理方向键导航、可见项目快速粘贴与自定义操作快捷键
func (a *App) keyboard(c *ui.Context) {
	if a.recording != "" || a.prompt != nil {
		return
	}
	if a.preview {
		if c.Shortcut(0, ui.KeyEscape) && a.once() {
			a.closePreview()
		}
		if a.shortcutPressed(c, "pastePlain") && a.once() {
			a.pasteSelected(true)
		} else if a.shortcutPressed(c, "paste") && a.once() {
			a.pasteSelected(false)
		}
		return
	}
	if a.quickPaste(c) {
		return
	}
	if c.Shortcut(0, ui.KeyEscape) && a.once() {
		a.escape()
	}
	previous, next := ui.KeyUp, ui.KeyDown
	if a.form == formBar {
		previous, next = ui.KeyLeft, ui.KeyRight
	}
	width, height := c.Size()
	if c.Shortcut(0, next) && a.once() {
		a.moveSel(1, width, height)
	}
	if c.Shortcut(0, previous) && a.once() {
		a.moveSel(-1, width, height)
	}
	if (a.shortcutPressed(c, "search") || a.settings.Shortcuts["search"] == "" && c.Shortcut(ui.Cmd, ui.KeyK)) && a.once() {
		a.search.Focus()
	}
	if a.shortcutPressed(c, "settings") && a.once() {
		a.openSettings()
	}
	if c.Shortcut(0, ui.KeyBackspace) && a.once() && a.query == "" && !a.search.Focused(c) {
		if rec := a.selected(); rec != nil {
			a.remove(rec)
		}
	}
	if a.shortcutPressed(c, "copy") && a.once() && !a.search.Focused(c) {
		a.copySelected(false)
	}
	if a.settings.Shortcuts["preview"] == "" && c.Shortcut(ui.Cmd, ui.KeyY) && a.once() {
		a.openPreview()
	}
	if a.shortcutPressed(c, "pastePlain") && a.once() {
		a.pasteSelected(true)
	} else if a.shortcutPressed(c, "paste") && a.once() {
		a.pasteSelected(false)
	}
	previewMods, _ := a.shortcutKeys("preview")
	if (!a.search.Focused(c) || previewMods != 0) && a.shortcutPressed(c, "preview") && a.once() {
		a.openPreview()
	}
}

func (a *App) escape() {
	if a.preview {
		a.closePreview()
		return
	}
	if a.query != "" {
		a.query = ""
		a.vis = a.filtered()
		a.sel = 0
		a.search.Focus()
		return
	}
	if a.tab != 0 {
		a.tab = 0
		a.vis = a.filtered()
		a.sel = 0
		return
	}
	a.hide()
}

// moveSel 按当前窗口与卡片尺寸移动选择并滚动至选中记录
func (a *App) moveSel(d int, width, height float32) {
	if len(a.vis) == 0 {
		return
	}
	i := clampInt(a.sel+d, 0, len(a.vis)-1)
	if i == a.sel {
		return
	}
	a.sel = i
	if a.form == formPanel && a.settings.Theme == themeGlass {
		a.list.ScrollTo(i, ui.Center)
	} else {
		a.list.ScrollIntoView(i)
	}
	if a.form == formBar {
		cardWidth, _ := a.barCardSize(height)
		step := cardWidth + cardGap
		viewW := width - 36
		target := float32(i)*step - (viewW-cardWidth)/2
		if target < 0 {
			target = 0
		}
		if a.strip.MaxX > 0 && target > a.strip.MaxX {
			target = a.strip.MaxX
		}
		a.strip.X = target
	}
}

func (a *App) pasteSelected(plain bool) {
	rec := a.selected()
	if rec == nil {
		return
	}
	a.paste(rec, plain)
}

// paste 按纯文本偏好写入所选记录并执行粘贴、排序与搜索重置
func (a *App) paste(rec *types.Record, plain bool) {
	if rec == nil {
		return
	}
	if err := a.writeClipboard(rec, plain || a.settings.PastePlain); err != nil {
		a.toast(err.Error())
		return
	}
	if !accessTrusted() {
		a.toast(locales.CopiedNeedAccess())
		native.PromptAccessibility()
		return
	}
	a.hide()
	time.Sleep(70 * time.Millisecond)
	if err := native.PasteKeystroke(); err != nil {
		a.toast(locales.AutoPasteFailed())
		return
	}
	if a.settings.ResetPanelAfterPaste {
		a.query = ""
	}
	if a.settings.MoveToFrontOnPaste {
		if err := a.st.MoveToFront(rec.ID); err != nil {
			a.storageError(err)
		}
	}
}

func (a *App) copySelected(plain bool) {
	if rec := a.selected(); rec != nil {
		a.copy(rec, plain)
	}
}

// copy 将记录写入剪贴板并显示操作结果
func (a *App) copy(rec *types.Record, plain bool) {
	if err := a.writeClipboard(rec, plain); err != nil {
		a.toast(err.Error())
		return
	}
	a.notify(locales.Copied())
}

// copyText 将文本写入剪贴板并显示操作结果
func (a *App) copyText(s string) {
	if err := clip.Write(transfer.TextData(s)); err != nil {
		a.toast(err.Error())
		return
	}
	a.notify(locales.Copied())
}

func (a *App) remove(rec *types.Record) {
	if a.st == nil {
		return
	}
	a.st.Delete(rec.ID)
	if a.detail != nil && a.detail.ID == rec.ID {
		a.closePreview()
	}
	a.reload()
	if a.win != nil {
		a.win.Invalidate()
	}
}

// exportImage 将图片记录保存到用户选择的文件
func (a *App) exportImage(rec *types.Record) {
	data, err := a.st.ReadBlob(rec.Image)
	if err != nil {
		a.toast(locales.ImageFileMissing())
		return
	}
	ext := rec.ImageFormat
	if ext == "" {
		ext = "png"
	}
	name := "GoPaste-" + rec.CreatedAt.Format("20060102-150405") + "." + ext
	dest, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		DefaultPath: name,
		Filters:     []mygo.FileFilter{{Name: locales.ImageFilter(strings.ToUpper(ext)), Extensions: []string{ext}}},
	})
	if err != nil || dest == "" {
		return
	}
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		a.toast(err.Error())
		return
	}
	a.notify(locales.Exported())
}

// openPreview 按需读取完整记录并打开预览
func (a *App) openPreview() {
	rec := a.fullRecord(a.selected())
	if rec == nil {
		return
	}
	a.detail = rec
	a.preview = true
	a.place()
	if a.win != nil {
		a.win.Invalidate()
	}
}

// closePreview 关闭预览并释放当前预览图片
func (a *App) closePreview() {
	a.preview = false
	a.detail = nil
	a.previewImage = cachedBitmap{}
	a.place()
	if a.win != nil {
		a.win.Invalidate()
	}
}

func (a *App) requestAccess() {
	native.PromptAccessibility()
}
