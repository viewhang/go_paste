package app

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"gopaste/internal/store"
	"gopaste/internal/types"
)

// TestThemeSwitchAndPersistence 验证设置中切换主题会改变面板绘制并保存选择
func TestThemeSwitchAndPersistence(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	a := NewApp()
	a.st, a.settings = st, st.Settings()
	a.records = []*types.Record{{ID: "preview", Kind: types.KindText, Text: "主题预览"}}
	panel := ui.NewTester(a.view, 1180, barHeight)
	panel.Frame()
	classic := bytes.Clone(panel.Image().Pix)
	settings := ui.NewTester(func(c *ui.Context) { a.settingsAppearance(c, pal(c)) }, 560, 240)
	for _, choice := range []struct {
		label string
		theme int
	}{
		{label: "Liquid Glass", theme: themeGlass},
		{label: "经典", theme: themeClassic},
		{label: "Liquid Glass", theme: themeGlass},
	} {
		if err := settings.Click(choice.label); err != nil {
			t.Fatal(err)
		}
		if a.settings.Theme != choice.theme {
			t.Fatalf("主题未切换为 %s", choice.label)
		}
		panel.Frame()
		if same := bytes.Equal(classic, panel.Image().Pix); same != (choice.theme == themeClassic) {
			t.Fatalf("主题 %s 的面板绘制未正确切换", choice.label)
		}
	}
	reopened, err := store.Open(st.Dir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	if reopened.Settings().Theme != themeGlass {
		t.Fatal("重新读取设置后主题选择丢失")
	}
}

// TestGlassFloatingHeader 验证内容穿过悬浮控制区且搜索、滚动与键盘选择正常
func TestGlassFloatingHeader(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, size := range []struct{ w, h int }{{panelWidth, panelHeight}, {panelMinWidth, panelMinHeight}} {
			t.Run(fmt.Sprintf("dark-%v-size-%dx%d", dark, size.w, size.h), func(t *testing.T) {
				st, err := store.Open(t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { st.Close() })
				photo, err := st.SaveBlob(samplePNG(400, 200, color.NRGBA{230, 50, 90, 255}, color.NRGBA{20, 180, 220, 255}), "png")
				if err != nil {
					t.Fatal(err)
				}
				a := NewApp()
				a.st, a.settings, a.form = st, st.Settings(), formPanel
				a.settings.Theme = themeGlass
				for i := range 30 {
					a.records = append(a.records, &types.Record{ID: fmt.Sprint(i), Kind: types.KindLink, PreviewImage: photo, URL: fmt.Sprintf("https://example.test/%d", i), Title: fmt.Sprintf("悬浮测试记录 %02d", i)})
				}
				for i := len(a.records) - 1; i >= 0; i-- {
					st.Add(a.records[i])
				}
				tt := ui.NewTester(a.view, size.w, size.h)
				tt.SetDark(dark)
				tt.Frame()
				search, ok := tt.Find("搜索记录")
				if !ok || !tt.Focused("搜索记录") {
					t.Fatal("悬浮搜索框未显示或自动聚焦")
				}
				first, ok := tt.Find(recordTitle(a.records[0]))
				if !ok || first.Y < glassHeaderHeight {
					t.Fatal("初始记录被悬浮控制区遮挡")
				}
				region := image.Rect(int(search.X), int(search.Y), int(search.X+search.W), int(search.Y+search.H))
				crop := image.NewRGBA(region)
				draw.Draw(crop, region, tt.Image(), region.Min, draw.Src)
				before := bytes.Clone(crop.Pix)
				tt.Scroll(float32(size.w-3), 75, 0, 120)
				if current, ok := tt.Find("搜索记录"); !ok || current != search {
					t.Fatal("滚动改变了悬浮搜索框的位置")
				}
				under, ok := tt.Find(recordTitle(a.records[1]))
				if !ok || under.Y >= glassHeaderHeight {
					t.Fatal("覆盖层拦截滚动或列表未经过玻璃下方")
				}
				draw.Draw(crop, region, tt.Image(), region.Min, draw.Src)
				if bytes.Equal(before, crop.Pix) {
					t.Fatal("搜索框玻璃未采样滚动内容")
				}
				if os.Getenv("SHOTS") != "" {
					out := filepath.Join("..", "..", ".mygo", "shots")
					if err := os.MkdirAll(out, 0o755); err != nil {
						t.Fatal(err)
					}
					writePNG(t, filepath.Join(out, fmt.Sprintf("glass-floating-%v-%d.png", dark, size.w)), tt.Image())
				}
				a.sel = 14
				a.list.ScrollTo(14, ui.Center)
				tt.Frame()
				for range 10 {
					tt.Key(0, ui.KeyUp)
					r, ok := tt.Find(recordTitle(a.records[a.sel]))
					if !ok || r.Y < glassHeaderHeight || r.Y+r.H > float32(size.h-30) {
						t.Fatalf("键盘选择的记录被悬浮控制区遮挡：索引 %d，位置 %+v", a.sel, r)
					}
				}
				tt.Type("悬浮测试记录 00")
				if len(a.vis) != 1 || a.vis[0].ID != a.records[0].ID {
					t.Fatal("悬浮搜索框未更新筛选结果")
				}
				tt.Key(ui.Cmd, ui.KeyA)
				tt.Key(0, ui.KeyBackspace)
				tt.TypeKey(0, ui.KeySpace, " ")
				if !a.preview {
					t.Fatal("悬浮搜索框中无法通过空格预览")
				}
				tt.Key(0, ui.KeyEscape)
				if a.preview || !tt.Focused("搜索记录") {
					t.Fatal("关闭预览后未恢复悬浮搜索框")
				}
				if err := tt.Click("添加列表"); err != nil {
					t.Fatal(err)
				}
				if err := tt.ChooseMenuItem("智能列表"); err != nil || a.prompt == nil || !a.prompt.tab.Smart {
					t.Fatalf("悬浮新增入口无法创建智能列表：%v", err)
				}
			})
		}
	}
}

// TestGlassSettingsHeader 验证设置标题栏与菜单固定、窗口按钮避让及主题切换
func TestGlassSettingsHeader(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, size := range []struct {
			w, h int
			bar  ui.TitleBar
		}{
			{780, 600, ui.TitleBar{Height: 28, Left: 72}},
			{720, 520, ui.TitleBar{Height: 48, Right: 138}},
		} {
			t.Run(fmt.Sprintf("dark-%v-size-%dx%d", dark, size.w, size.h), func(t *testing.T) {
				st, err := store.Open(t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { st.Close() })
				a := NewApp()
				a.st, a.settings = st, st.Settings()
				a.settings.Theme = themeGlass
				tt := ui.NewTester(a.settingsView, size.w, size.h)
				tt.SetDark(dark)
				tt.SetTitleBar(size.bar)
				height := max(size.bar.Height, 46)
				title, ok := tt.Find("设置")
				if !ok || title.X < size.bar.Left+16 {
					t.Fatal("设置标题未避让系统窗口按钮")
				}
				if _, ok := tt.Find("完成"); ok {
					t.Fatal("设置窗口仍显示完成按钮")
				}
				menu, ok := tt.Find("通用")
				if !ok || menu.X >= 160 || menu.Y < height {
					t.Fatal("分类菜单未显示在标题栏下方的左侧区域")
				}
				appearance, ok := tt.Find("外观与形态")
				if !ok || appearance.Y < height+20 {
					t.Fatal("初始设置分组被标题栏遮挡")
				}
				tt.Scroll(float32(size.w)/2, height+20, 0, 160)
				if current, ok := tt.Find("设置"); !ok || current != title {
					t.Fatal("滚动改变了设置标题的位置")
				}
				if current, ok := tt.Find("通用"); !ok || current != menu {
					t.Fatal("滚动改变了分类菜单的位置")
				}
				if current, ok := tt.Find("外观与形态"); !ok || current.Y >= appearance.Y {
					t.Fatal("分类明细未独立滚动")
				}
				tt.Scroll(float32(size.w)/2, height+20, 0, -160)
				if err := tt.Click("经典"); err != nil || a.settings.Theme != themeClassic {
					t.Fatalf("设置页无法切回经典主题：%v", err)
				}
				if err := tt.Click("Liquid Glass"); err != nil || a.settings.Theme != themeGlass {
					t.Fatalf("设置页无法切回玻璃主题：%v", err)
				}
			})
		}
	}
}
