package app

import (
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

const (
	// themeClassic 使用经典面板与卡片外观
	themeClassic = iota
	// themeGlass 使用 Liquid Glass 材质
	themeGlass
)

// glassHeaderHeight 保留玻璃主题搜索栏与列表标签的初始展示高度
const glassHeaderHeight = 90

// palette 按外观主题提供界面配色
func (a *App) palette(c *ui.Context) palette {
	p := pal(c)
	if a.settings.Theme == themeGlass {
		if p.dark {
			p.bg = ui.Hex("#202226")
			p.surface = ui.Hex("#2b2e33")
			p.hover = ui.Hex("#2b2f36")
		} else {
			p.bg = ui.Hex("#f1f3f5")
			p.hover = ui.Hex("#e8ebef")
			p.accentSoft = p.accent.Alpha(0.12)
		}
		p.raised = p.hover
		p.faint = p.muted
	}
	return p
}

// themeBackground 为玻璃主题绘制中性底色并透出系统毛玻璃
func (a *App) themeBackground(c *ui.Context, e ui.Element, p palette) {
	if a.settings.Theme == themeGlass {
		background := p.bg
		if c.Vibrancy() {
			background = background.Alpha(0.72)
		}
		e.Background(background)
	}
}

// themeSurface 按主题绘制控件材质并为可点击的玻璃控件启用按压反馈
func (a *App) themeSurface(e ui.Element, background ui.Color, interactive bool) ui.Element {
	if a.settings.Theme == themeGlass {
		return e.Material(glass.Glass{Interactive: interactive})
	}
	return e.Background(background)
}
