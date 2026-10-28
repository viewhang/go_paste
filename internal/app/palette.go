package app

import (
	"math"

	"github.com/egoist/mygo/ui"

	"gopaste/internal/locales"
	"gopaste/internal/types"
)

// palette is the app's own set of colors, derived from the system theme so
// that it follows the desktop's light and dark appearance.
type palette struct {
	dark bool

	bg      ui.Color
	surface ui.Color
	raised  ui.Color
	hover   ui.Color
	border  ui.Color
	hair    ui.Color
	shadow  ui.Color

	text  ui.Color
	muted ui.Color
	faint ui.Color

	accent     ui.Color
	accentText ui.Color
	accentSoft ui.Color
	accentLine ui.Color
}

func pal(c *ui.Context) palette {
	t := c.Theme()
	p := palette{dark: t.Dark}
	if t.Dark {
		p.bg = ui.Hex("#15161a")
		p.surface = ui.Hex("#1f2127")
		p.raised = ui.Hex("#262931")
		p.hover = ui.Hex("#2b2f38")
		p.border = ui.RGBA(255, 255, 255, 0.10)
		p.hair = ui.RGBA(255, 255, 255, 0.07)
		p.shadow = ui.RGBA(0, 0, 0, 0.5)
		p.text = ui.Hex("#e9ecf2")
		p.muted = ui.Hex("#98a0ae")
		p.faint = ui.Hex("#6d7482")
		p.accent = t.Accent
		p.accentText = t.AccentText
		p.accentSoft = t.Accent.Alpha(0.18)
		p.accentLine = t.Accent.Alpha(0.45)
	} else {
		p.bg = ui.Hex("#f3f4f7")
		p.surface = ui.Hex("#ffffff")
		p.raised = ui.Hex("#ffffff")
		p.hover = ui.Hex("#eef0f5")
		p.border = ui.RGBA(18, 22, 40, 0.10)
		p.hair = ui.RGBA(18, 22, 40, 0.07)
		p.shadow = ui.RGBA(18, 22, 40, 0.16)
		p.text = ui.Hex("#1a1d26")
		p.muted = ui.Hex("#6b7280")
		p.faint = ui.Hex("#9aa1ad")
		p.accent = t.Accent
		p.accentText = t.AccentText
		p.accentSoft = t.Accent.Alpha(0.10)
		p.accentLine = t.Accent.Alpha(0.40)
	}
	return p
}

// applyTheme 统一当前主题的控件配色、间距、圆角与焦点状态
func (a *App) applyTheme(c *ui.Context) {
	current := c.Theme()
	t := *ui.LightTheme()
	if current.Dark {
		t = *ui.DarkTheme()
	}
	t.Font, t.FontSize = current.Font, current.FontSize
	t.Radius = 9
	t.Spacing = 4
	if t.FontSize <= 0 || t.FontSize > 14 {
		t.FontSize = 12.5
	}
	if t.Dark {
		t.Accent = ui.Hex("#7d8cff")
		t.AccentHover = ui.Hex("#8e9bff")
		t.AccentPressed = ui.Hex("#6f7ef2")
		t.AccentText = ui.Hex("#0e1020")
		t.Focus = ui.Hex("#7d8cff")
	} else {
		t.Accent = ui.Hex("#5468ff")
		t.AccentHover = ui.Hex("#4356f5")
		t.AccentPressed = ui.Hex("#3948e0")
		t.AccentText = ui.Hex("#ffffff")
		t.Focus = ui.Hex("#5468ff")
	}
	if a.settings.Theme == themeGlass {
		c.SetTheme(&t)
		p := a.palette(c)
		t.Background, t.Surface = p.bg, p.surface
		t.SurfaceHover, t.SurfacePressed = p.hover, p.accentSoft
		t.Text, t.TextMuted, t.Border = p.text, p.muted, p.border
		t.Selection, t.Focus = p.accentSoft, p.text.Alpha(0.45)
		t.Scrollbar = p.muted.Alpha(0.6)
	}
	c.SetTheme(&t)
}

// kindColor is the accent color of a content type, used for the card's
// color band, its icon and its preview.
func kindColor(k types.Kind, dark bool) ui.Color {
	if dark {
		switch k {
		case types.KindImage:
			return ui.Hex("#3ecfba")
		case types.KindLink:
			return ui.Hex("#b98cff")
		case types.KindColor:
			return ui.Hex("#f0b95c")
		case types.KindFile:
			return ui.Hex("#93a4bf")
		default:
			return ui.Hex("#8b97ff")
		}
	}
	switch k {
	case types.KindImage:
		return ui.Hex("#0f9d8b")
	case types.KindLink:
		return ui.Hex("#8b5cf6")
	case types.KindColor:
		return ui.Hex("#d98518")
	case types.KindFile:
		return ui.Hex("#64748b")
	default:
		return ui.Hex("#5468ff")
	}
}

// kindLabel 按当前语言返回剪贴板内容类型名称
func kindLabel(k types.Kind) string {
	switch k {
	case types.KindImage:
		return locales.ImageKind()
	case types.KindLink:
		return locales.LinkKind()
	case types.KindColor:
		return locales.ColorKind()
	case types.KindFile:
		return locales.FileKind()
	default:
		return locales.TextKind()
	}
}

// tint mixes a color into the surface, for chips and icon tiles.
func tint(fg, bg ui.Color, amount float32) ui.Color {
	return bg.Mix(fg, amount)
}

// onColor returns black or white, whichever reads better on c.
func onColor(c ui.Color) ui.Color {
	lum := 0.2126*linear(c.R) + 0.7152*linear(c.G) + 0.0722*linear(c.B)
	if lum > 0.42 {
		return ui.Hex("#1a1d26")
	}
	return ui.Hex("#ffffff")
}

func linear(v uint8) float64 {
	f := float64(v) / 255
	if f <= 0.03928 {
		return f / 12.92
	}
	return math.Pow((f+0.055)/1.055, 2.4)
}
