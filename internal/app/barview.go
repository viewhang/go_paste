package app

import (
	"math"

	"github.com/egoist/mygo/ui"

	"gopaste/internal/locales"
)

// barView 按当前主题显示贴底长条并在预览时向上扩展
func (a *App) barView(c *ui.Context, p palette) {
	col := ui.Column(c).Fill().Clip()
	if c.Vibrancy() {
		col.Background(p.surface.Alpha(0.93))
	} else {
		col.Background(p.bg)
	}
	a.themeBackground(c, col, p)
	col.Children(func() {
		if a.preview {
			a.previewPane(c, p)
			hairline(c, p)
		}
		a.barBody(c, p)
	})
}

// barBody 将搜索、卡片和状态栏铺满长条的可用高度
func (a *App) barBody(c *ui.Context, p palette) {
	ui.Column(c).Grow(1).Gap(6).Children(func() {
		a.barHeader(c, p)
		a.barStrip(c, p)
		a.statusRow(c, p)
	})
}

// barHeader 显示长条的搜索、用户列表、新增入口与统一设置菜单
func (a *App) barHeader(c *ui.Context, p palette) {
	ui.Row(c).Padding(9, 18).Gap(8).AlignItems(ui.Center).Children(func() {
		a.searchBox(c, p, true)
		ui.Box(c).Width(4)

		scroll := ui.ScrollHorizontal(c).Grow(1).Height(30)
		scroll.Children(func() {
			row := ui.Row(c).Gap(3).AlignItems(ui.Center)
			row.Children(func() {
				for i, tab := range a.tabs() {
					i, tab := i, tab
					active := i == a.tab
					chip := ui.Row(c).Key(tab.ID).Padding(3, 8).Radius(6).Focusable().Cursor(ui.CursorPointer)
					if active {
						chip.Background(p.accentSoft)
					} else if chip.Hovered() {
						chip.Background(p.hover)
					}
					chip.Children(func() {
						t := ui.Text(c, tab.Name).FontSize(11)
						if active {
							t.TextColor(p.accent).Bold()
							if a.settings.Theme == themeGlass {
								t.TextColor(p.text)
							}
						} else {
							t.TextColor(p.muted)
						}
					})
					if chip.Clicked() {
						a.tab = i
						a.sel = 0
						a.vis = a.filtered()
						a.strip.X = 0
					}
					a.listDropTarget(chip, p, tab)
					if tab.ID != "all" {
						chip.ContextMenu(func(m *ui.Menu) { a.tabMenu(m, tab) })
					}
				}
				a.addTabButton(c, p)
			})
		})

		a.settingsButton(c, p)
	})
}

// barCardSize 根据贴边窗口高度计算等比缩放的卡片宽高
func (a *App) barCardSize(height float32) (width, cardHeight float32) {
	if a.preview {
		height -= barPreview
	}
	cardHeight = max(cardH, height-barHeight+cardH)
	return cardHeight * cardW / cardH, cardHeight
}

// visibleCards 按当前卡片宽度计算横向可见范围并在两侧各预留两张
func visibleCards(offset, width, cardWidth float32, n int) (first, last int) {
	step := cardWidth + cardGap
	first = min(n, max(0, int(offset/step)-2))
	last = min(n, max(first, int(math.Ceil(float64((offset+width)/step)))+2))
	return
}

// barStrip 按缩放后的卡片宽度构建可见内容并用占位维持滚动范围
func (a *App) barStrip(c *ui.Context, p palette) {
	n := len(a.vis)
	if n == 0 {
		a.emptyState(c, p)
		return
	}
	scroll := ui.ScrollHorizontal(c).Grow(1).TrackScroll(&a.strip).Padding(0, 18)
	width, height := c.Size()
	cardWidth, _ := a.barCardSize(height)
	step := cardWidth + cardGap
	shown := min(n, 90)
	first, last := visibleCards(a.strip.X, width-36, cardWidth, shown)
	scroll.Children(func() {
		row := ui.Row(c).Gap(cardGap).AlignItems(ui.Center)
		row.Children(func() {
			if first > 0 {
				ui.Box(c).Width(float32(first)*step - cardGap).Shrink(0)
			}
			for i := first; i < last; i++ {
				c.Key(a.vis[i].ID)
				a.barCard(c, a.recordAt(i), i)
			}
			if last < shown {
				ui.Box(c).Width(float32(shown-last)*step - cardGap).Shrink(0)
			}
			if n > shown {
				ui.Column(c).Width(130).Gap(4).Padding(0, 8).Children(func() {
					ui.Text(c, locales.MoreRecords(n-shown)).FontSize(11).TextColor(p.faint)
					ui.Text(c, locales.OlderRecordsHint()).FontSize(10.5).TextColor(p.faint)
				})
			}
		})
	})
}
