package app

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// view 按当前主题绘制面板并处理键盘与待执行操作
func (a *App) view(c *ui.Context) {
	if a.win != nil {
		if !a.win.IsVisible() {
			c.Root().Background(ui.Transparent)
			return
		}
		a.imageScale = mygo.Screen.DisplayMatching(a.win.Bounds()).ScaleFactor
	}
	if !a.preview {
		a.releasePreview()
	}
	a.applyTheme(c)
	p := a.palette(c)
	a.acted = false
	a.vis = a.filtered()
	a.clampSel()

	if c.Vibrancy() {
		c.Root().Background(ui.Transparent)
	} else {
		c.Root().Background(p.bg)
	}

	a.keyboard(c)

	if a.form == formBar {
		a.barView(c, p)
	} else {
		a.panelView(c, p)
	}
	a.promptOverlay(c, p)
	a.runPending()
}
