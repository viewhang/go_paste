package detect

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"testing"
)

func TestURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"https://example.com/a?b=1", "https://example.com/a?b=1", true},
		{"  https://example.com  ", "https://example.com", true},
		{"example.com", "https://example.com", true},
		{"www.example.com/path", "https://www.example.com/path", true},
		{"10.0.0.5:15173", "https://10.0.0.5:15173", true},
		{"localhost:5173", "https://localhost:5173", true},
		{"https://localhost:8080/app", "https://localhost:8080/app", true},
		{"hello world", "", false},
		{"hello", "", false},
		{"ftp://example.com/file", "", false},
		{"http://foo", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		got, ok := URL(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("URL(%q) = %q, %v; want %q, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestColor(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"#ff6600", "#ff6600", true},
		{"#FF6600", "#ff6600", true},
		{"#abc", "#aabbcc", true},
		{"#abcd", "#aabbccdd", true},
		{"#ff660080", "#ff660080", true},
		{"rgb(255, 0, 0)", "#ff0000", true},
		{"rgba(255, 0, 0, 0.5)", "#ff000080", true},
		{"rgb(100% 0% 0%)", "#ff0000", true},
		{"hsl(0, 100%, 50%)", "#ff0000", true},
		{"hsl(120 100% 50%)", "#00ff00", true},
		{"rebeccapurple", "", false},
		{"#ff66", "#ffff6666", true},
		{"#12", "", false},
		{"rgb(1,2)", "", false},
	}
	for _, c := range cases {
		got, ok := Color(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("Color(%q) = %q, %v; want %q, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestHTMLText(t *testing.T) {
	cases := []struct{ in, want string }{
		{"<p>Hello <b>world</b></p>", "Hello world"},
		{"<div>a</div><div>b</div>", "a b"},
		{"<style>p{color:red}</style><p>x</p>", "x"},
		{"plain &amp; simple", "plain & simple"},
		{"", ""},
	}
	for _, c := range cases {
		if got := HTMLText(c.in); got != c.want {
			t.Errorf("HTMLText(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}

func TestHTMLHasText(t *testing.T) {
	if HTMLHasText("<img src=x>") {
		t.Error("an image-only fragment reported text")
	}
	if HTMLHasText("<p>hi</p>") {
		t.Error("a two-letter fragment counts as rich text")
	}
	if !HTMLHasText("<p>hello world, this is a paragraph</p>") {
		t.Error("a paragraph reported no text")
	}
}

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.NRGBA{R: 200, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return buf.Bytes()
}

func TestImageInfo(t *testing.T) {
	format, w, h, animated := ImageInfo(pngBytes(t, 40, 30))
	if format != "png" || w != 40 || h != 30 || animated {
		t.Fatalf("png: got %q %dx%d animated=%v", format, w, h, animated)
	}

	var buf bytes.Buffer
	pal := color.Palette{color.Black, color.White}
	first := image.NewPaletted(image.Rect(0, 0, 8, 8), pal)
	second := image.NewPaletted(image.Rect(0, 0, 8, 8), pal)
	second.Set(3, 3, color.White) // a second frame that really differs
	g := &gif.GIF{Image: []*image.Paletted{first, second}, Delay: []int{10, 10}}
	if err := gif.EncodeAll(&buf, g); err != nil {
		t.Fatalf("gif: %v", err)
	}
	format, w, h, animated = ImageInfo(buf.Bytes())
	if format != "gif" || w != 8 || h != 8 || !animated {
		t.Fatalf("gif: got %q %dx%d animated=%v", format, w, h, animated)
	}

	if format, _, _, _ := ImageInfo([]byte("not an image")); format != "" {
		t.Fatalf("junk reported as %q", format)
	}
}
