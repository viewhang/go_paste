package app

import (
	"math"
	"reflect"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"gopaste/internal/types"
)

// TestWindowBounds 验证两种形态的尺寸恢复、预览扩展和多屏工作区限制
func TestWindowBounds(t *testing.T) {
	area := mygo.Rectangle{X: -1440, Y: 24, Width: 1440, Height: 876}
	pt := mygo.Point{X: -40, Y: 850}
	for _, tc := range []struct {
		name     string
		form     int
		preview  bool
		settings types.Settings
		want     mygo.Rectangle
	}{
		{name: "长条默认尺寸", want: mygo.Rectangle{X: -1440, Y: 662, Width: 1440, Height: 238}},
		{name: "长条恢复高度", settings: types.Settings{BarHeight: 400}, want: mygo.Rectangle{X: -1440, Y: 500, Width: 1440, Height: 400}},
		{name: "预览追加高度", preview: true, settings: types.Settings{BarHeight: 400}, want: mygo.Rectangle{X: -1440, Y: 194, Width: 1440, Height: 706}},
		{name: "预览不超出屏幕", preview: true, settings: types.Settings{BarHeight: 800}, want: area},
		{name: "面板默认尺寸", form: formPanel, want: mygo.Rectangle{X: -442, Y: 236, Width: 432, Height: 596}},
		{name: "面板恢复缩小尺寸", form: formPanel, settings: types.Settings{PanelWidth: 380, PanelHeight: 400}, want: mygo.Rectangle{X: -390, Y: 432, Width: 380, Height: 400}},
		{name: "面板限制最小尺寸", form: formPanel, settings: types.Settings{PanelWidth: 100, PanelHeight: 100}, want: mygo.Rectangle{X: -370, Y: 512, Width: 360, Height: 320}},
		{name: "面板限制最大尺寸", form: formPanel, settings: types.Settings{PanelWidth: 2000, PanelHeight: 1200}, want: mygo.Rectangle{X: -1430, Y: 34, Width: 1420, Height: 856}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := &App{form: tc.form, preview: tc.preview, settings: tc.settings}
			if got := a.windowBounds(pt, area); got != tc.want {
				t.Fatalf("窗口位置与尺寸：得到 %+v，期望 %+v", got, tc.want)
			}
			if !reflect.DeepEqual(a.settings, tc.settings) {
				t.Fatal("屏幕限制不应覆盖已保存的尺寸")
			}
		})
	}
}

// TestBarResizeLayout 验证贴边卡片随高度等比变宽且状态栏保持在底部
func TestBarResizeLayout(t *testing.T) {
	a := NewApp()
	a.vis = []*types.Record{
		{ID: "first", Kind: types.KindText, Text: "resize-check"},
		{ID: "second", Kind: types.KindText, Text: "resize-next"},
	}
	a.records = a.vis
	initTestStore(t, a)
	tt := ui.NewTester(func(c *ui.Context) { a.barView(c, pal(c)) }, 1180, barHeight)
	for _, height := range []int{barHeight, 400, barHeight, barHeight + barPreview} {
		a.preview = height == barHeight+barPreview
		a.detail = &types.Record{Kind: types.KindText, Text: "preview-detail"}
		tt.SetSize(1180, height)
		tt.Frame()
		r, ok := tt.Find("2 条")
		if !ok || r.Y < float32(height-30) || r.Y+r.H > float32(height) {
			t.Fatalf("窗口高度 %d 的状态栏位置不正确：%+v", height, r)
		}
		first, ok := tt.Find("resize-check")
		next, nextOK := tt.Find("resize-next")
		cardHeight := height - barHeight + cardH
		if a.preview {
			cardHeight -= barPreview
		}
		wantWidth := float32(cardHeight) * cardW / cardH
		if !ok || !nextOK || math.Abs(float64(next.X-first.X-cardGap-wantWidth)) > 0.1 {
			t.Fatalf("窗口高度 %d 的卡片宽度未等比调整：得到 %v，预期 %v", height, next.X-first.X-cardGap, wantWidth)
		}
	}
}
