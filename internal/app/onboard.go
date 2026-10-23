package app

import (
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"gopaste/internal/locales"
	"gopaste/internal/native"
)

// showOnboarding 打开首次使用引导与辅助功能授权窗口
func (a *App) showOnboarding() {
	if a.obWin == nil {
		a.obWin = mygo.NewWindow(mygo.WindowOptions{
			Title:         locales.WelcomeTitle(),
			Width:         540,
			Height:        570,
			MinWidth:      480,
			MinHeight:     480,
			TitleBarStyle: mygo.TitleBarHidden,
			Content:       ui.View(a.onboardingView),
		})
		a.obWin.OnClosed(func() { a.obWin = nil })
	}
	a.obWin.Show()
	a.obWin.Focus()
}

// onboardingView 显示首次使用说明、面板形态与权限引导
func (a *App) onboardingView(c *ui.Context) {
	applyTheme(c)
	p := pal(c)
	c.Root().Background(p.bg)
	bar := c.TitleBar()
	ui.Column(c).Fill().Children(func() {
		top := ui.Row(c).Height(max(bar.Height, 30)).DragWindow()
		top.Children(func() {})
		ui.Scroll(c).Grow(1).Padding(4, 30).Children(func() {
			ui.Column(c).Gap(18).Children(func() {
				// Brand and pitch.
				ui.Row(c).Gap(13).AlignItems(ui.Center).Children(func() {
					if bmp := a.appMark(); bmp != nil {
						ui.Image(c, bmp).Size(48, 48).Fit(ui.Contain)
					} else {
						mark := ui.Box(c).Size(46, 46).Radius(12)
						mark.Gradient(ui.Hex("#6d7dff"), ui.Hex("#9a6bff"), 35)
						mark.Children(func() {
							ui.Row(c).Fill().Center().Children(func() {
								ui.Icon(c, icoClipboard).Size(22, 22).TextColor(ui.Hex("#ffffff"))
							})
						})
					}
					ui.Column(c).Gap(2).Children(func() {
						ui.Text(c, "GoPaste").FontSize(19).Bold().TextColor(p.text)
						ui.Text(c, locales.WelcomeTagline()).FontSize(12).TextColor(p.muted)
					})
				})

				step(c, p, "1", locales.OnboardCopy(), locales.OnboardCopyHint())
				step(c, p, "2", locales.PressShortcut(a.hotkeyLabel()), locales.OnboardPasteHint(pasteKeyName()))
				step(c, p, "3", locales.OnboardOrganize(), locales.OnboardOrganizeHint())

				// Which form.
				ui.Column(c).Gap(7).Children(func() {
					ui.Text(c, locales.PanelForm()).FontSize(11).Bold().TextColor(p.faint)
					form := a.form
					ui.Segmented(c, &form, locales.BottomBar(), locales.FollowPointer()).Label(locales.PanelForm())
					if form != a.form {
						a.setForm(form)
					}
					ui.Text(c, locales.OnboardFormHint()).FontSize(11).TextColor(p.muted)
				})

				if isDarwin {
					a.permissionCard(c, p)
				}
			})
		})
		// The buttons stay at the foot of the window, whatever the content
		// above them grows to.
		hairline(c, p)
		ui.Row(c).Height(54).Padding(0, 30).Gap(9).AlignItems(ui.Center).Children(func() {
			if ui.PrimaryButton(c, locales.GetStarted()).Clicked() {
				a.finishOnboarding()
			}
			if ui.Button(c, locales.OpenSettings()).Clicked() {
				a.finishOnboarding()
				a.openSettings()
			}
			ui.Spacer(c)
			ui.Text(c, "v0.1.0").FontSize(10.5).TextColor(p.faint)
		})
	})
}

func step(c *ui.Context, p palette, num, title, desc string) {
	ui.Row(c).Gap(12).AlignItems(ui.Start).Children(func() {
		badge := ui.Box(c).Size(22, 22).Radius(11).Background(p.accentSoft)
		badge.Children(func() {
			ui.Row(c).Fill().Center().Children(func() {
				ui.Text(c, num).FontSize(11).Bold().TextColor(p.accent)
			})
		})
		ui.Column(c).Grow(1).Gap(3).Children(func() {
			ui.Text(c, title).FontSize(12.5).Bold().TextColor(p.text)
			ui.Text(c, desc).FontSize(11.5).TextColor(p.muted)
		})
	})
}

// permissionCard 显示自动粘贴权限状态与授权入口
func (a *App) permissionCard(c *ui.Context, p palette) {
	trusted := native.AccessibilityTrusted()
	card := ui.Column(c).Padding(13, 15).Gap(9).Radius(11)
	if trusted {
		card.Background(tint(ui.Hex("#1aa87a"), p.surface, 0.1)).Border(1, tint(ui.Hex("#1aa87a"), p.surface, 0.35))
	} else {
		card.Background(tint(ui.Hex("#d9962b"), p.surface, 0.1)).Border(1, tint(ui.Hex("#d9962b"), p.surface, 0.35))
	}
	card.Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			title := locales.AccessStep()
			if trusted {
				title = locales.AccessGranted()
			}
			ui.Icon(c, icoShield).Size(15, 15).TextColor(func() ui.Color {
				if trusted {
					return ui.Hex("#12916a")
				}
				return ui.Hex("#c98a2b")
			}())
			ui.Text(c, title).FontSize(12.5).Bold().TextColor(p.text)
		})
		ui.Text(c, locales.AccessExplanation()).
			FontSize(11).TextColor(p.muted)
		ui.Row(c).Gap(8).Children(func() {
			if trusted {
				ui.Text(c, locales.OnboardReady(a.hotkeyLabel())).
					FontSize(11).TextColor(p.muted)
			} else {
				if ui.PrimaryButton(c, locales.EnableAccess()).Clicked() {
					native.PromptAccessibility()
				}
				if ui.Button(c, locales.OpenSystemSettings()).Clicked() {
					native.OpenAccessibilitySettings()
				}
				ui.Text(c, locales.AccessNoRestart()).FontSize(10.5).TextColor(p.faint)
			}
		})
	})
	if !trusted {
		c.After(1200 * time.Millisecond)
	}
}

func (a *App) finishOnboarding() {
	if a.st != nil {
		st := a.st.Settings()
		st.Onboarded = true
		_ = a.st.SetSettings(st)
		a.settings.Onboarded = true
	}
	if a.obWin != nil {
		a.obWin.Close()
		a.obWin = nil
	}
	a.show()
}

func pasteKeyName() string {
	return prettyHotkey(pasteAccel())
}
