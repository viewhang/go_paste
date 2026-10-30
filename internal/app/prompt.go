package app

import (
	"github.com/egoist/mygo/ui"

	"gopaste/internal/locales"
	"gopaste/internal/types"
)

// promptState 保存列表编辑表单的内容与校验结果
type promptState struct {
	title  func() string
	label  func() string
	value  string
	okText func() string
	err    string
	tab    *types.Tab
}

// promptOverlay 在面板范围内显示可滚动的列表编辑表单
func (a *App) promptOverlay(c *ui.Context, p palette) {
	ps := a.prompt
	if ps == nil {
		return
	}
	ui.Overlay(c, func() {
		if c.Shortcut(0, ui.KeyEscape) {
			a.prompt = nil
			return
		}
		w, h := c.Size()
		back := ui.Box(c).Fill().Center().Focusable().Background(ui.RGBA(0, 0, 0, 0.12))
		back.Children(func() {
			card := ui.Column(c).Key(ps).Width(min(420, w-24)).MaxHeight(h-16).Padding(16).Gap(11).Radius(12).
				Background(p.surface).Border(1, p.border)
			card.Shadow(0, 12, 34, 0, p.shadow)
			a.themeSurface(card, p.surface, false)
			card.Children(func() {
				ui.Text(c, ps.title()).FontSize(13).Bold().TextColor(p.text)
				ui.Scroll(c).Shrink(1).MaxHeight(h - 126).Children(func() {
					ui.Column(c).Gap(8).Children(func() {
						ui.Text(c, ps.label()).FontSize(11).TextColor(p.muted)
						field := a.themeTextInput(c, &ps.value).Label(locales.InputLabel(ps.label())).AutoFocus()
						if field.Submitted() {
							a.acceptPrompt()
						}
						if ps.tab.Smart {
							a.listRules(c, ps)
						}
					})
				})
				if ps.err != "" {
					ui.Text(c, ps.err).FontSize(11).TextColor(p.danger)
				}
				ui.Row(c).Gap(7).Justify(ui.End).Children(func() {
					if a.themeButton(c, locales.Cancel()).Clicked() {
						a.prompt = nil
					}
					if a.themePrimaryButton(c, ps.okText()).Clicked() {
						a.acceptPrompt()
					}
				})
			})
		})
	})
}

// acceptPrompt 校验并保存表单内容，校验失败时保留表单
func (a *App) acceptPrompt() {
	ps := a.prompt
	if ps == nil {
		return
	}
	if err := a.saveTab(*ps.tab, ps.value); err != nil {
		ps.err = err.Error()
		return
	}
	a.prompt = nil
}
