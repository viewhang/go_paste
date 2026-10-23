package app

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"gopaste/internal/store"
	"gopaste/internal/types"
)

// TestShots 在设置 SHOTS=1 时将界面状态渲染到项目根目录的截图文件夹
func TestShots(t *testing.T) {
	if os.Getenv("SHOTS") == "" {
		t.Skip("set SHOTS=1 to write screenshots")
	}
	t.Chdir(filepath.Join("..", ".."))
	out := ".mygo/shots"
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Log("seeding")
	a := seedApp(t)
	t.Log("seeded")

	shots := []struct {
		name string
		form int
		w, h int
		dark bool
		hold bool
		prep func()
	}{
		{name: "bar", form: formBar, w: 1180, h: barHeight},
		{name: "bar-dark", form: formBar, w: 1180, h: barHeight, dark: true},
		{name: "panel", form: formPanel, w: panelWidth, h: panelHeight},
		{name: "panel-dark", form: formPanel, w: panelWidth, h: panelHeight, dark: true},
		{name: "bar-resized", form: formBar, w: 1180, h: 400},
		{name: "panel-resized", form: formPanel, w: 600, h: 720},
		{name: "panel-minimum", form: formPanel, w: panelMinWidth, h: panelMinHeight},
		{name: "bar-glass", form: formBar, w: 1180, h: barHeight, prep: func() { a.settings.Theme = themeGlass }},
		{name: "bar-glass-dark", form: formBar, w: 1180, h: barHeight, dark: true, prep: func() { a.settings.Theme = themeGlass }},
		{name: "panel-glass", form: formPanel, w: panelWidth, h: panelHeight, prep: func() { a.settings.Theme = themeGlass }},
		{name: "panel-glass-dark", form: formPanel, w: panelWidth, h: panelHeight, dark: true, prep: func() { a.settings.Theme = themeGlass }},
		{name: "bar-quick-paste", form: formBar, w: 1180, h: barHeight, hold: true},
		{name: "panel-quick-paste", form: formPanel, w: panelWidth, h: panelHeight, hold: true},
		{name: "bar-glass-quick-paste", form: formBar, w: 1180, h: barHeight, dark: true, hold: true, prep: func() { a.settings.Theme = themeGlass }},
		{name: "panel-glass-quick-paste", form: formPanel, w: panelWidth, h: panelHeight, dark: true, hold: true, prep: func() { a.settings.Theme = themeGlass }},
		{name: "bar-preview", form: formBar, w: 1180, h: barHeight + barPreview, prep: func() {
			a.preview = true
			a.detail = a.records[2]
		}},
		{name: "panel-search", form: formPanel, w: panelWidth, h: panelHeight, prep: func() {
			a.query = "报告"
			a.tab = 0
		}},
		{name: "panel-smart-list", form: formPanel, w: panelWidth, h: panelHeight, prep: func() {
			a.newTab(true)
			a.prompt.value = "合同文档"
			a.prompt.tab.Rules = []types.ListRule{
				{Field: types.RuleExtension, Value: "pdf, docx"},
				{Field: types.RuleKeyword, Value: "合同"},
			}
		}},
		{name: "bar-smart-list", form: formBar, w: 1180, h: barHeight, prep: func() {
			a.newTab(true)
			a.prompt.value = "文档"
		}},
	}
	for _, s := range shots {
		s := s
		a.form = s.form
		a.preview = false
		a.detail = nil
		a.query = ""
		a.tab = 0
		a.sel = 1
		a.prompt = nil
		a.settings.Theme = themeClassic
		if s.prep != nil {
			s.prep()
		}
		t.Log("render", s.name)
		tt := ui.NewTester(a.view, s.w, s.h)
		tt.SetDark(s.dark)
		for i := 0; i < 3; i++ {
			tt.Frame()
		}
		if s.hold {
			tt.HoldModifiers(ui.Cmd)
		}
		writePNG(t, filepath.Join(out, s.name+".png"), tt.Image())
		t.Log("wrote", s.name)
	}

	// 设置使用独立窗口，预览同时覆盖两种主题与明暗外观
	a.form = formBar
	a.prompt = nil
	for _, shot := range []struct {
		name  string
		theme int
		dark  bool
	}{
		{"settings", themeClassic, false},
		{"settings-dark", themeClassic, true},
		{"settings-glass", themeGlass, false},
		{"settings-glass-dark", themeGlass, true},
	} {
		a.settings.Theme = shot.theme
		a.settingsPage = "general"
		set := ui.NewTester(a.settingsView, 780, 600)
		set.SetDark(shot.dark)
		set.SetTitleBar(ui.TitleBar{Height: 28, Left: 72})
		writePNG(t, filepath.Join(out, shot.name+".png"), set.Image())
		if shot.theme == themeGlass {
			set.Scroll(500, 100, 0, 160)
			writePNG(t, filepath.Join(out, shot.name+"-scrolled.png"), set.Image())
		}
		for _, page := range []string{"shortcuts", "privacy", "advanced", "about"} {
			a.settingsPage = page
			set.Frame()
			writePNG(t, filepath.Join(out, shot.name+"-"+page+".png"), set.Image())
		}
	}
}

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	if img == nil {
		t.Fatalf("%s: no image", path)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

// seedApp builds an app over a temporary store holding records of every
// type, as the panel would show after a day of copying.
func seedApp(t *testing.T) *App {
	t.Helper()
	dir := filepath.Join(".mygo", "shotdata")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	st.Clear()
	a := NewApp()
	a.st = st

	photo, _ := st.SaveBlob(samplePNG(880, 560, color.NRGBA{70, 110, 220, 255}, color.NRGBA{240, 190, 90, 255}), "png")
	chart, _ := st.SaveBlob(samplePNG(640, 400, color.NRGBA{30, 160, 140, 255}, color.NRGBA{250, 250, 250, 255}), "png")
	thumb, _ := st.SaveBlob(samplePNG(160, 96, color.NRGBA{90, 90, 110, 255}, color.NRGBA{200, 200, 220, 255}), "png")
	icon, _ := st.SaveBlob(sampleIcon(), "png")

	mk := func(r *types.Record) *types.Record {
		if r.Source.AppName == "" {
			r.Source.AppName = "Safari"
		}
		r.Source.Icon = icon
		rec, _ := st.Add(r)
		return rec
	}

	base := time.Now()
	mk(&types.Record{Kind: types.KindText, Text: "把复制过的文本、图片、链接、颜色与文件保存在清晰的卡片里，随时搜索、预览、再次粘贴。", Source: types.Source{AppName: "Notes"}})
	mk(&types.Record{Kind: types.KindLink, URL: "https://github.com/egoist/mygo", Title: "egoist/mygo: Desktop apps in Go, with native UI or a web frontend", PreviewImage: thumb, PreviewState: "done", Source: types.Source{AppName: "Safari"}})
	mk(&types.Record{Kind: types.KindColor, Color: "#FF6B35", Source: types.Source{AppName: "Figma"}})
	mk(&types.Record{Kind: types.KindImage, Image: photo, ImageW: 880, ImageH: 560, ImageFormat: "png", Bytes: 240_000, Source: types.Source{AppName: "CleanShot X"}})
	mk(&types.Record{Kind: types.KindText, Text: "SELECT id, name FROM users WHERE created_at > now() - interval '7 days' ORDER BY created_at DESC LIMIT 50;", HTML: "<pre>SELECT</pre>", Source: types.Source{AppName: "TablePlus"}})
	mk(&types.Record{Kind: types.KindFile, Files: []types.FileRef{{Path: filepath.Join(dir, "records.json"), Name: "records.json", Size: 18_240}, {Path: filepath.Join(dir, "blobs"), Name: "blobs", Dir: true}}, Bytes: 18_240, Source: types.Source{AppName: "Finder"}})
	mk(&types.Record{Kind: types.KindImage, Image: chart, ImageW: 640, ImageH: 400, ImageFormat: "png", Animated: true, Bytes: 90_000, Source: types.Source{AppName: "Keynote"}})
	mk(&types.Record{Kind: types.KindLink, URL: "https://www.anthropic.com/news/claude-4", Title: "Claude 4 发布：更强的推理与工具使用", PreviewState: "loading", Source: types.Source{AppName: "Chrome"}})
	mk(&types.Record{Kind: types.KindText, Text: "gopaste://settings", Source: types.Source{AppName: "Terminal"}})

	list := st.List()
	for i, r := range list {
		st.Update(r.ID, func(rec *types.Record) {
			rec.CreatedAt = base.Add(-time.Duration(i) * 7 * time.Minute)
		})
	}
	a.reload()
	return a
}

func samplePNG(w, h int, from, to color.NRGBA) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			t := float32(x) / float32(w)
			img.Set(x, y, color.NRGBA{
				R: uint8(float32(from.R) + t*(float32(to.R)-float32(from.R))),
				G: uint8(float32(from.G) + t*(float32(to.G)-float32(from.G))),
				B: uint8(float32(from.B) + t*(float32(to.B)-float32(from.B))),
				A: 255,
			})
		}
	}
	// A few bands, so that the picture reads as a picture.
	for i := 0; i < 5; i++ {
		r := image.Rect(w/10, h/10+i*h/9, w-w/10, h/10+i*h/9+h/22)
		draw.Draw(img, r, image.NewUniform(color.NRGBA{255, 255, 255, 26}), image.Point{}, draw.Over)
	}
	return encodePNG(img)
}

func sampleIcon() []byte {
	img := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.NRGBA{84, 104, 255, 255})
		}
	}
	draw.Draw(img, image.Rect(18, 16, 46, 48), image.NewUniform(color.NRGBA{255, 255, 255, 255}), image.Point{}, draw.Src)
	return encodePNG(img)
}

func encodePNG(img image.Image) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}
