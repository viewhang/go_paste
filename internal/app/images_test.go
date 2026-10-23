package app

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"os"
	"testing"

	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"

	"gopaste/internal/store"
	"gopaste/internal/types"
)

// TestImageBudget 验证缓存按字节淘汰且保留最近访问的图片
func TestImageBudget(t *testing.T) {
	var cache bitmapCache
	first := bitmapKey{name: "first"}
	cache.put(first, ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 512, 512))))
	for i := range 16 {
		cache.get(first)
		cache.put(bitmapKey{name: fmt.Sprint(i)}, ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 512, 512))))
		if cache.bytes > thumbnailBudget {
			t.Fatalf("缓存超出预算：%d", cache.bytes)
		}
	}
	if _, ok := cache.get(first); !ok {
		t.Fatal("最近访问的图片被淘汰")
	}
	if _, ok := cache.get(bitmapKey{name: "0"}); ok {
		t.Fatal("最早未使用的图片没有被淘汰")
	}
	cache.trim(0)
	if cache.bytes != 0 || len(cache.items) != 0 {
		t.Fatal("清理后仍保留图片缓存")
	}
}

// TestImageSizesAndLifetime 验证屏幕倍率、预览预算及关闭与隐藏后的缓存释放
func TestImageSizesAndLifetime(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	name, err := st.SaveBlob(encodePNG(image.NewRGBA(image.Rect(0, 0, 2400, 1800))), "png")
	if err != nil {
		t.Fatal(err)
	}
	a := NewApp()
	a.st = st
	for _, scale := range []float64{1, 2} {
		a.imageScale = scale
		b := a.thumb(name, 56)
		if b == nil {
			t.Fatal("缩略图解码失败")
		}
		w, _ := b.Size()
		if w < int(56*scale) || w >= int(56*scale)+64 {
			t.Fatalf("屏幕倍率 %v 的图片宽度错误：%d", scale, w)
		}
	}
	b := a.previewBitmap(name, 2000)
	if b == nil || bitmapBytes(b) > 8<<20 {
		t.Fatal("预览未遵守独立预算")
	}
	a.closePreview()
	if a.previewImage.image != nil || a.images.bytes == 0 {
		t.Fatal("关闭预览未单独释放图片")
	}
	a.previewBitmap(name, 2000)
	a.hide()
	if a.previewImage.image != nil || len(a.images.items) != 0 {
		t.Fatal("隐藏面板后仍持有应用图片缓存")
	}
}

// TestImageResizeReuse 验证连续缩放只保留一份缩略图且缩小时复用位图
func TestImageResizeReuse(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	name, err := st.SaveBlob(encodePNG(image.NewRGBA(image.Rect(0, 0, 512, 512))), "png")
	if err != nil {
		t.Fatal(err)
	}
	a := NewApp()
	a.st = st
	for side := 128; side < 158; side++ {
		if a.thumb(name, float32(side)) == nil {
			t.Fatal("连续缩放时图片解码失败")
		}
	}
	large := a.thumb(name, 157)
	if len(a.images.items) != 1 || a.images.order.Len() != 1 || a.images.bytes != bitmapBytes(large) {
		t.Fatal("同一图片保留了多份尺寸缓存或重复累计预算")
	}
	if a.thumb(name, 80) != large {
		t.Fatal("缩小图片时未复用已有位图")
	}
	preview := a.previewBitmap(name, 300)
	if preview == nil || a.previewBitmap(name, 280) != preview || a.previewBitmap(name, 180) != preview {
		t.Fatal("预览在同一档位或缩小时重复解码")
	}
}

// TestLazyImageDrag 验证图片只在拖拽目标请求时读取且使用原始文件
func TestLazyImageDrag(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	name, err := st.SaveBlob([]byte("before"), "png")
	if err != nil {
		t.Fatal(err)
	}
	a := NewApp()
	a.st = st
	data := a.dragData(&types.Record{Kind: types.KindImage, Image: name})
	want := []byte("original image at request time")
	if err := os.WriteFile(st.BlobPath(name), want, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := data.Read(transfer.PNG)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("图片在请求前被读取或发生变化：%q，%v", got, err)
	}
}

// TestCompactDragPreview 验证两种面板仅在开始拖拽时生成固定小尺寸预览
func TestCompactDragPreview(t *testing.T) {
	for _, form := range []int{formBar, formPanel} {
		for _, dark := range []bool{false, true} {
			t.Run(fmt.Sprintf("form-%d-dark-%v", form, dark), func(t *testing.T) {
				a := NewApp()
				a.form, a.settings = form, types.DefaultSettings()
				a.records = []*types.Record{{ID: "drag", Kind: types.KindText, Text: "拖拽预览内容"}}
				w, h := panelWidth, panelHeight
				if form == formBar {
					w, h = 1180, barHeight
				}
				tt := ui.NewTester(a.view, w, h)
				tt.SetDark(dark)
				before := bytes.Clone(a.dragImage.Pix)
				if !bytes.Equal(before, make([]byte, len(before))) {
					t.Fatal("尚未拖拽就生成了预览")
				}
				from, ok := tt.Find("拖拽预览内容")
				if !ok {
					t.Fatal("拖拽来源未显示")
				}
				x, y := from.X+from.W/2, from.Y+from.H/2
				tt.Press(x, y)
				tt.Move(x+8, y)
				if a.dragImage.Bounds().Dx() > 96 || a.dragImage.Bounds().Dy() > 32 || bytes.Equal(before, a.dragImage.Pix) {
					t.Fatal("拖拽预览未生成或尺寸过大")
				}
				if os.Getenv("SHOTS") != "" {
					writePNG(t, fmt.Sprintf("../../.mygo/shots/drag-preview-%d-%v.png", form, dark), a.dragImage)
				}
				tt.Key(0, ui.KeyEscape)
				tt.Release(x+8, y)
			})
		}
	}
}

// TestBarVisibleImages 验证屏幕外卡片不会解码且滚动范围保持完整
func TestBarVisibleImages(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	a := NewApp()
	a.st = st
	for i := range 90 {
		img := image.NewRGBA(image.Rect(0, 0, 8, 8))
		img.SetRGBA(0, 0, color.RGBA{R: uint8(i), A: 255})
		name, err := st.SaveBlob(encodePNG(img), "png")
		if err != nil {
			t.Fatal(err)
		}
		a.vis = append(a.vis, &types.Record{ID: fmt.Sprint(i), Kind: types.KindImage, Image: name})
	}
	tt := ui.NewTester(func(c *ui.Context) { a.barStrip(c, pal(c)) }, 900, 180)
	if len(a.images.items) > 8 {
		t.Fatalf("屏幕外图片被解码：%d 张", len(a.images.items))
	}
	maxX := a.strip.MaxX
	if maxX < 14000 {
		t.Fatalf("未保留完整滚动范围：%v", maxX)
	}
	a.releaseImages()
	a.strip.X = 50 * (cardW + cardGap)
	tt.Frame()
	if len(a.images.items) > 10 || a.strip.MaxX != maxX {
		t.Fatalf("滚动后的构建数量或范围异常：%d 张，%v / %v", len(a.images.items), a.strip.MaxX, maxX)
	}
	for key := range a.images.items {
		if key == a.vis[0].Image || key == a.vis[89].Image {
			t.Fatal("远离可见区域的图片被解码")
		}
	}
}
