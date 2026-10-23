package app

import (
	"image"
	"image/color"
	"image/draw"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"gopaste/internal/store"
	"gopaste/internal/types"
)

// TestSourceIconWithImagePreview 验证两种形态同时显示来源应用图标与图片内容
func TestSourceIconWithImagePreview(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	iconColor := color.RGBA{R: 233, G: 66, B: 54, A: 255}
	previewColor := color.RGBA{R: 22, G: 177, B: 244, A: 255}
	save := func(col color.RGBA) string {
		img := image.NewRGBA(image.Rect(0, 0, 16, 16))
		draw.Draw(img, img.Bounds(), image.NewUniform(col), image.Point{}, draw.Src)
		name, err := st.SaveBlob(encodePNG(img), "png")
		if err != nil {
			t.Fatal(err)
		}
		return name
	}
	rec := &types.Record{
		Kind:      types.KindImage,
		CreatedAt: time.Now(),
		Source:    types.Source{AppName: "测试应用", Icon: save(iconColor)},
		Image:     save(previewColor),
		ImageW:    16,
		ImageH:    16,
	}
	for _, form := range []int{formBar, formPanel} {
		a := NewApp()
		a.st, a.form = st, form
		width, height := cardW, barHeight
		if form == formPanel {
			width, height = panelWidth, 58
		}
		tt := ui.NewTester(func(c *ui.Context) {
			if form == formBar {
				a.barCard(c, rec, 0)
			} else {
				a.panelRow(c, rec, 0)
			}
		}, width, height)
		if _, ok := tt.Find("来源应用：测试应用"); !ok {
			t.Fatalf("形态 %d 未显示来源应用图标", form)
		}
		iconPixels, previewPixels := 0, 0
		img := tt.Image()
		for y := range img.Rect.Dy() {
			for x := range img.Rect.Dx() {
				switch img.RGBAAt(x, y) {
				case iconColor:
					iconPixels++
				case previewColor:
					previewPixels++
				}
			}
		}
		if iconPixels == 0 || previewPixels == 0 {
			t.Fatalf("形态 %d 的图标或预览未绘制：图标 %d 像素，预览 %d 像素", form, iconPixels, previewPixels)
		}
	}
}

// TestBarSelectionKeepsTextLayout 验证切换选中状态时卡片正文的位置和换行区域保持一致
func TestBarSelectionKeepsTextLayout(t *testing.T) {
	a := NewApp()
	rec := &types.Record{Kind: types.KindText, Text: "选中前后正文的宽度应保持一致，避免文字重新换行"}
	tt := ui.NewTester(func(c *ui.Context) { a.barCard(c, rec, 0) }, cardW, barHeight)
	selected, ok := tt.Find(rec.Text)
	if !ok {
		t.Fatal("未找到卡片正文")
	}
	a.sel = 1
	tt.Frame()
	unselected, ok := tt.Find(rec.Text)
	if !ok || selected != unselected {
		t.Fatalf("选中状态改变了正文布局：选中 %+v，未选中 %+v", selected, unselected)
	}
}
