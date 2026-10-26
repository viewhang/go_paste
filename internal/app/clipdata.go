package app

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io/fs"
	"os"
	"strconv"
	"strings"

	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"

	"gopaste/internal/clip"
	"gopaste/internal/locales"
	"gopaste/internal/native"
	"gopaste/internal/types"
)

// writeClipboard 按记录类型将内容写回系统剪贴板
func (a *App) writeClipboard(rec *types.Record, plain bool) error {
	data, err := a.clipboardData(rec, plain)
	if err != nil {
		return err
	}
	return clip.Write(data)
}

// clipboardData 保留记录原文与格式，仅在指定纯文本时移除 HTML
func (a *App) clipboardData(rec *types.Record, plain bool) (transfer.Data, error) {
	rec = a.fullRecord(rec)
	if rec == nil {
		return transfer.Data{}, fs.ErrNotExist
	}
	switch rec.Kind {
	case types.KindText, types.KindLink, types.KindColor:
		if !plain && rec.HTML != "" {
			item := transfer.NewItem(
				transfer.Bytes(transfer.HTML, []byte(rec.HTML)),
				transfer.Bytes(transfer.Text, []byte(rec.Text)),
			)
			return transfer.New(item), nil
		}
		return transfer.TextData(rec.Text), nil
	case types.KindImage:
		data, err := a.st.ReadBlob(rec.Image)
		if err != nil {
			return transfer.Data{}, errors.New(locales.ImageFileMissing())
		}
		if _, err := png.Decode(bytes.NewReader(data)); err != nil {
			return transfer.Data{}, err
		}
		return transfer.New(transfer.NewItem(transfer.Bytes(transfer.PNG, data))), nil
	case types.KindFile:
		var paths []string
		for _, f := range rec.Files {
			if _, err := os.Stat(f.Path); err == nil {
				paths = append(paths, f.Path)
			}
		}
		if len(paths) == 0 {
			return transfer.Data{}, errors.New(locales.OriginalFileMissing())
		}
		return transfer.FileData(paths...)
	}
	if rec.Text != "" {
		return transfer.TextData(rec.Text), nil
	}
	return transfer.Data{}, errors.New(locales.NoPasteContent())
}

// recordDrag 绑定记录移动与拖出，并在拖拽开始时生成避开鼠标的紧凑预览
func (a *App) recordDrag(c *ui.Context, source ui.Element, rec *types.Record) {
	if a.dragImage == nil {
		a.dragImage = image.NewRGBA(image.Rect(0, 0, 96, 32))
	}
	preview := a.dragImage
	p, theme := a.palette(c), *c.Theme()
	source.Drag(rec).DragDataFrom(func() transfer.Data {
		img := ui.Render(func(c *ui.Context) {
			c.SetTheme(&theme)
			ui.Row(c).Fill().Padding(5).Gap(5).Radius(7).Background(p.surface).AlignItems(ui.Center).Children(func() {
				thumbTile(c, a.recordThumb(rec), rec, 22)
				ui.Text(c, recordTitle(rec)).FontSize(10).TextColor(p.text).SingleLine().Grow(1)
			})
		}, 96, 32, 1)
		draw.Draw(preview, preview.Bounds(), img, image.Point{}, draw.Src)
		return a.dragData(rec)
	}, transfer.DragOptions{Preview: preview, Hotspot: image.Pt(-12, -12)})
}

// dragData 保留拖出内容的原文与格式并按需读取原始图片
func (a *App) dragData(rec *types.Record) transfer.Data {
	rec = a.fullRecord(rec)
	if rec == nil {
		return transfer.Data{}
	}
	switch rec.Kind {
	case types.KindImage:
		name := rec.Image
		return transfer.New(transfer.NewItem(transfer.Lazy(transfer.PNG, func() ([]byte, error) {
			return a.st.ReadBlob(name)
		})))
	case types.KindFile:
		var paths []string
		for _, f := range rec.Files {
			paths = append(paths, f.Path)
		}
		data, _ := transfer.FileData(paths...)
		return data
	}
	if rec.HTML != "" {
		return transfer.New(transfer.NewItem(
			transfer.Bytes(transfer.HTML, []byte(rec.HTML)),
			transfer.Bytes(transfer.Text, []byte(rec.Text)),
		))
	}
	return transfer.TextData(rec.Text)
}

// ColorString renders a canonical color in the requested notation.
func ColorString(hex, format string) string {
	r, g, b, alpha := parseHex(hex)
	switch strings.ToLower(format) {
	case "rgb":
		if alpha < 255 {
			return fmt.Sprintf("rgba(%d, %d, %d, %.2f)", r, g, b, float64(alpha)/255)
		}
		return fmt.Sprintf("rgb(%d, %d, %d)", r, g, b)
	case "hsl":
		h, s, l := rgbToHSL(r, g, b)
		if alpha < 255 {
			return fmt.Sprintf("hsla(%d, %d%%, %d%%, %.2f)", h, s, l, float64(alpha)/255)
		}
		return fmt.Sprintf("hsl(%d, %d%%, %d%%)", h, s, l)
	default:
		return strings.ToUpper(hex)
	}
}

func parseHex(hex string) (uint8, uint8, uint8, uint8) {
	h := strings.TrimPrefix(hex, "#")
	var r, g, b, a uint8 = 0, 0, 0, 255
	if len(h) >= 6 {
		rv, _ := strconv.ParseUint(h[0:2], 16, 8)
		gv, _ := strconv.ParseUint(h[2:4], 16, 8)
		bv, _ := strconv.ParseUint(h[4:6], 16, 8)
		r, g, b = uint8(rv), uint8(gv), uint8(bv)
	}
	if len(h) == 8 {
		av, _ := strconv.ParseUint(h[6:8], 16, 8)
		a = uint8(av)
	}
	return r, g, b, a
}

// rgbToHSL 将 RGB 色值转换为 HSL 显示值
func rgbToHSL(r, g, b uint8) (int, int, int) {
	rf, gf, bf := float64(r)/255, float64(g)/255, float64(b)/255
	mx := max(rf, gf, bf)
	mn := min(rf, gf, bf)
	l := (mx + mn) / 2
	var h, s float64
	if mx != mn {
		d := mx - mn
		if l > 0.5 {
			s = d / (2 - mx - mn)
		} else {
			s = d / (mx + mn)
		}
		switch mx {
		case rf:
			h = (gf-bf)/d + map[bool]float64{true: 0, false: 6}[gf < bf]
		case gf:
			h = (bf-rf)/d + 2
		default:
			h = (rf-gf)/d + 4
		}
		h *= 60
	}
	return int(h + 0.5), int(s*100 + 0.5), int(l*100 + 0.5)
}

// accessTrusted reports whether automatic pasting is allowed.
func accessTrusted() bool { return native.AccessibilityTrusted() }
