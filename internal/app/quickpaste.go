package app

import (
	"fmt"
	"math"

	"github.com/egoist/mygo/ui"
)

// quickPasteKeys 按可见项目顺序对应数字键一至九及零
var quickPasteKeys = [...]ui.Key{
	ui.Key1, ui.Key2, ui.Key3, ui.Key4, ui.Key5,
	ui.Key6, ui.Key7, ui.Key8, ui.Key9, ui.Key0,
}

// quickPasteRange 返回按住平台修饰键时完整可见的前十个项目范围
func (a *App) quickPasteRange(c *ui.Context) (first, last int) {
	if c.Modifiers() != ui.Cmd || a.preview || a.prompt != nil || a.recording != "" {
		return 0, 0
	}
	n := len(a.vis)
	if a.form == formBar {
		width, height := c.Size()
		cardWidth, _ := a.barCardSize(height)
		step := float64(cardWidth + cardGap)
		first = int(math.Ceil(float64(a.strip.X) / step))
		last = int(math.Floor(float64(a.strip.X+width-36-cardWidth)/step)) + 1
		n = min(n, 90)
	} else {
		_, visibleLast := a.list.Visible()
		_, height := c.Size()
		rowHeight := a.panelRowHeight()
		step := float64(rowHeight + panelRowGap)
		first = int(math.Ceil(float64(a.listScroll.Y-panelListPadding) / step))
		padding := float32(panelListPadding)
		if a.settings.Theme == themeGlass {
			padding += glassHeaderHeight
		}
		bottom := height - statusHeight - a.list.Bounds(c).Y
		last = min(visibleLast+1, int(math.Floor(float64(a.listScroll.Y+bottom-padding-rowHeight)/step))+1)
	}
	first = min(n, max(0, first))
	last = max(first, min(n, last, first+len(quickPasteKeys)))
	return
}

// quickPasteHint 返回当前可见项目的快速粘贴快捷键提示
func (a *App) quickPasteHint(c *ui.Context, i int) string {
	first, last := a.quickPasteRange(c)
	if i < first || i >= last {
		return ""
	}
	prefix := "Ctrl+"
	if isDarwin {
		prefix = "⌘"
	}
	return fmt.Sprintf("%s%d", prefix, (i-first+1)%10)
}

// quickPaste 将数字键对应的可见项目直接粘贴
func (a *App) quickPaste(c *ui.Context) bool {
	first, last := a.quickPasteRange(c)
	for i := first; i < last; i++ {
		if c.Shortcut(ui.Cmd, quickPasteKeys[i-first]) && a.once() {
			a.sel = i
			rec := a.recordAt(i)
			a.later(func() { a.paste(rec, false) })
			return true
		}
	}
	return false
}
