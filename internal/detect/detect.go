// Package detect classifies clipboard payloads and reads image metadata.
package detect

import (
	"bytes"
	"fmt"
	"html"
	"image"
	"image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var (
	hexColor  = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3,4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)
	funcColor = regexp.MustCompile(`^(rgb|rgba|hsl|hsla)\s*\(([^)]*)\)$`)
	tagRe     = regexp.MustCompile(`(?s)<[^>]*>`)
	spaceRe   = regexp.MustCompile(`\s+`)
	// styleRe drops the CSS and scripts a copied fragment carries along:
	// what is inside them is not the text a reader wants.
	styleRe = regexp.MustCompile(`(?is)<(style|script|head)\b[^>]*>.*?</(style|script|head)>`)
	// blockRe keeps the words of adjacent blocks apart, so that a line
	// break does not become a missing space.
	blockRe = regexp.MustCompile(`(?i)</?(p|div|br|li|tr|h[1-6]|section|article|blockquote)\b[^>]*>`)
)

// URL 识别已有协议的 HTTP 链接并返回原始地址
func URL(s string) (string, bool) {
	if s == "" || len(s) > 4096 {
		return "", false
	}
	if strings.ContainsAny(s, " \t\r\n") {
		return "", false
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", false
	}
	// A host without a dot is a mistake, bar localhost, which developers
	// copy all the time — with or without a port.
	host := u.Hostname()
	if host == "" || (!strings.Contains(host, ".") && host != "localhost") {
		return "", false
	}
	return s, true
}

// Color reports the color a text stands for, in #rrggbb or #rrggbbaa form.
func Color(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 64 {
		return "", false
	}
	if hexColor.MatchString(s) {
		h := strings.ToLower(s[1:])
		switch len(h) {
		case 3, 4:
			var b strings.Builder
			for _, r := range h {
				b.WriteRune(r)
				b.WriteRune(r)
			}
			h = b.String()
		}
		return "#" + h, true
	}
	m := funcColor.FindStringSubmatch(strings.ToLower(s))
	if m == nil {
		return "", false
	}
	parts := strings.FieldsFunc(m[2], func(r rune) bool {
		return r == ',' || r == '/' || r == ' '
	})
	nums := make([]float64, 0, 4)
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		pct := strings.HasSuffix(p, "%")
		p = strings.TrimSuffix(p, "%")
		v, err := strconv.ParseFloat(p, 64)
		if err != nil {
			return "", false
		}
		if pct {
			v = v / 100
		}
		nums = append(nums, v)
	}
	if len(nums) < 3 {
		return "", false
	}
	var r, g, b, a float64
	switch m[1] {
	case "rgb", "rgba":
		scale := 1.0
		if len(nums) == 3 && (strings.Contains(m[2], "%")) {
			scale = 255
		}
		r, g, b = nums[0]*scale, nums[1]*scale, nums[2]*scale
		a = 1
		if len(nums) > 3 {
			a = nums[3]
			if a > 1 {
				a = a / 255
			}
		}
	case "hsl", "hsla":
		r, g, b = hslToRGB(nums[0], nums[1], nums[2])
		a = 1
		if len(nums) > 3 {
			a = nums[3]
			if a > 1 {
				a = a / 255
			}
		}
	}
	return formatColor(r, g, b, a), true
}

func formatColor(r, g, b, a float64) string {
	clamp := func(v float64) int {
		v = math.Round(v)
		if v < 0 {
			return 0
		}
		if v > 255 {
			return 255
		}
		return int(v)
	}
	if a >= 0.999 {
		return fmt.Sprintf("#%02x%02x%02x", clamp(r), clamp(g), clamp(b))
	}
	return fmt.Sprintf("#%02x%02x%02x%02x", clamp(r), clamp(g), clamp(b), clamp(a*255))
}

func hslToRGB(h, s, l float64) (float64, float64, float64) {
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}
	if s > 1 {
		s /= 100
	}
	if l > 1 {
		l /= 100
	}
	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := l - c/2
	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return (r + m) * 255, (g + m) * 255, (b + m) * 255
}

// HTMLText returns the visible text of an HTML fragment.
func HTMLText(markup string) string {
	s := styleRe.ReplaceAllString(markup, " ")
	s = blockRe.ReplaceAllString(s, " ")
	s = tagRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return strings.TrimSpace(spaceRe.ReplaceAllString(s, " "))
}

// HTMLHasText reports whether an HTML fragment carries text worth keeping as
// rich text, rather than only marking up an image or a single word.
func HTMLHasText(markup string) bool {
	text := HTMLText(markup)
	n := 0
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			n++
		}
	}
	return n >= 12
}

// ImageInfo reads the format, pixel size and animation of encoded image data.
func ImageInfo(data []byte) (format string, width, height int, animated bool) {
	cfg, f, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", 0, 0, false
	}
	switch f {
	case "gif":
		if g, err := gif.DecodeAll(bytes.NewReader(data)); err == nil {
			animated = len(g.Image) > 1
		}
	case "webp":
		// An animated WebP carries an ANIM chunk; the still ones do not.
		animated = bytes.Contains(data, []byte("ANIM"))
	}
	return f, cfg.Width, cfg.Height, animated
}
