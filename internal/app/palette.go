package app

import (
	"math"

	"github.com/egoist/mygo/ui"

	"gopaste/internal/locales"
	"gopaste/internal/types"
)

// palette 定义两种主题共用并随系统明暗外观切换的语义色板
type palette struct {
	bg      ui.Color
	chrome  ui.Color
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
	danger     ui.Color
	warning    ui.Color
	success    ui.Color
}

// pal 按系统明暗外观提供容器、文字、交互与状态颜色
func pal(c *ui.Context) palette {
	p := palette{}
	if c.Theme().Dark {
		p.bg = ui.Hex("#1c1c1e")
		p.chrome = ui.Hex("#222225")
		p.surface = ui.Hex("#242426")
		p.hover = ui.Hex("#303034")
		p.border = ui.Hex("#3c3c43")
		p.shadow = ui.RGBA(0, 0, 0, 0.5)
		p.text = ui.Hex("#f2f2f5")
		p.muted = ui.Hex("#b0b0b9")
		p.accent = ui.Hex("#6cb6ff")
		p.accentText = ui.Hex("#101828")
		p.accentSoft = ui.Hex("#24364d")
		p.danger = ui.Hex("#ff8d87")
		p.warning = ui.Hex("#e7ba71")
		p.success = ui.Hex("#72d794")
	} else {
		p.bg = ui.Hex("#f5f5f7")
		p.chrome = ui.Hex("#ededf0")
		p.surface = ui.Hex("#ffffff")
		p.hover = ui.Hex("#eaecef")
		p.border = ui.Hex("#dadae0")
		p.shadow = ui.RGBA(0, 0, 0, 0.16)
		p.text = ui.Hex("#202124")
		p.muted = ui.Hex("#63666f")
		p.accent = ui.Hex("#006adc")
		p.accentText = ui.Hex("#ffffff")
		p.accentSoft = ui.Hex("#e6effb")
		p.danger = ui.Hex("#c73532")
		p.warning = ui.Hex("#8a5c13")
		p.success = ui.Hex("#247647")
	}
	p.raised, p.hair, p.faint = p.hover, p.border, p.muted
	p.accentLine = p.accent
	return p
}

// applyTheme 统一当前主题的控件配色、间距、圆角与焦点状态
func (a *App) applyTheme(c *ui.Context) {
	current := c.Theme()
	p := pal(c)
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
	t.Background, t.Surface = p.bg, p.surface
	t.SurfaceHover, t.SurfacePressed = p.hover, p.hover.Mix(p.text, 0.06)
	t.Text, t.TextMuted, t.Border = p.text, p.muted, p.border
	t.Accent, t.AccentText = p.accent, p.accentText
	t.AccentHover, t.AccentPressed = p.accent.Mix(p.text, 0.08), p.accent.Mix(p.text, 0.16)
	t.Selection, t.Focus = p.accentSoft, p.accent
	t.Danger, t.Warning, t.Success = p.danger, p.warning, p.success
	t.Scrollbar = p.muted.Alpha(0.6)
	c.SetTheme(&t)
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
