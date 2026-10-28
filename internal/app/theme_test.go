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

	"gopaste/internal/locales"
	"gopaste/internal/store"
	"gopaste/internal/types"
)

// TestThemedControls 验证两种主题明暗外观中的按钮禁用、文本编辑、数字范围与选择控件键盘操作
func TestThemedControls(t *testing.T) {
	for _, theme := range []int{themeClassic, themeGlass} {
		for _, dark := range []bool{false, true} {
			t.Run(fmt.Sprintf("theme-%d-dark-%v", theme, dark), func(t *testing.T) {
				a := NewApp()
				a.settings.Theme = theme
				text, choice := "", "甲"
				number, segment, clicks, disabled := float64(2), 0, 0, false
				tt := ui.NewTester(func(c *ui.Context) {
					a.applyTheme(c)
					c.Root().Background(c.Theme().Background)
					ui.Column(c).Padding(12).Gap(8).Children(func() {
						ui.Row(c).Gap(8).Children(func() {
							if a.themeButton(c, "执行操作").Clicked() {
								clicks++
							}
							if a.themePrimaryButton(c, "不可用操作").Disabled(true).Clicked() {
								disabled = true
							}
						})
						a.themeTextInput(c, &text).Label("输入框")
						a.themeNumberInput(c, &number, 1, 4, 1).AlignSelf(ui.Start).Label("数量")
						a.themeSegmented(c, &segment, "第一项", "第二项")
						a.themeSelect(c, &choice, []string{"甲", "乙"}).Label("选择框")
					})
				}, 360, 320)
				tt.SetDark(dark)
				// click 点击指定控件并检查其可访问入口
				click := func(label string) {
					t.Helper()
					if err := tt.Click(label); err != nil {
						t.Fatal(err)
					}
				}
				click("执行操作")
				tt.Key(0, ui.KeySpace)
				click("不可用操作")
				if clicks != 2 || disabled {
					t.Fatal("按钮点击、键盘或禁用行为不正确")
				}
				click("输入框")
				tt.Type("测试输入")
				if text != "测试输入" {
					t.Fatal("输入框未保留编辑内容")
				}
				click("数量")
				tt.Key(ui.Cmd, ui.KeyA)
				tt.Type("3")
				tt.Key(0, ui.KeyUp)
				tt.Key(0, ui.KeyUp)
				if number != 4 {
					t.Fatalf("数字编辑或步进范围不正确：%v", number)
				}
				click("第二项")
				if segment != 1 {
					t.Fatal("分段点击未切换选项")
				}
				tt.Key(0, ui.KeyLeft)
				if segment != 0 {
					t.Fatal("分段方向键未切换选项")
				}
				click("选择框")
				tt.Key(0, ui.KeyDown)
				tt.Key(0, ui.KeyEnter)
				if choice != "乙" {
					t.Fatal("下拉框方向键与确认未选择选项")
				}
				click("选择框")
				if os.Getenv("SHOTS") != "" {
					out := filepath.Join("..", "..", ".mygo", "shots")
					if err := os.MkdirAll(out, 0o755); err != nil {
						t.Fatal(err)
					}
					writePNG(t, filepath.Join(out, fmt.Sprintf("controls-theme-%d-dark-%v.png", theme, dark)), tt.Image())
				}
				tt.Key(0, ui.KeyEscape)
				if choice != "乙" || tt.HasText("甲") {
					t.Fatal("关闭下拉浮层后选择丢失或选项未隐藏")
				}
			})
		}
	}
}

// TestThemeSwitchInteraction 验证两种主题的开关均支持点击聚焦与空格切换
func TestThemeSwitchInteraction(t *testing.T) {
	for _, theme := range []int{themeClassic, themeGlass} {
		t.Run(fmt.Sprintf("theme-%d", theme), func(t *testing.T) {
			a := NewApp()
			a.settings.Theme = theme
			on := false
			tt := ui.NewTester(func(c *ui.Context) {
				a.applyTheme(c)
				a.themeSwitch(c, &on).Label("测试开关")
			}, 80, 40)
			if err := tt.Click("测试开关"); err != nil || !on || !tt.Focused("测试开关") {
				t.Fatalf("点击开关未启用或未聚焦：%v", err)
			}
			tt.Key(0, ui.KeySpace)
			if on {
				t.Fatal("空格未关闭开关")
			}
		})
	}
}

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

// TestPanelSettingsMenu 验证两种主题与布局共用设置菜单且底栏不显示快捷键提示
func TestPanelSettingsMenu(t *testing.T) {
	for _, theme := range []int{themeClassic, themeGlass} {
		for _, form := range []int{formPanel, formBar} {
			t.Run(fmt.Sprintf("theme-%d-form-%d", theme, form), func(t *testing.T) {
				a := NewApp()
				a.form, a.settings = form, types.DefaultSettings()
				a.settings.Theme = theme
				initTestStore(t, a)
				width, height := panelWidth, panelHeight
				if form == formBar {
					width, height = 1180, barHeight
				}
				tt := ui.NewTester(a.view, width, height)
				hint := locales.PanelHint(prettyHotkey(a.shortcutAccel("paste")), prettyHotkey(a.shortcutAccel("preview")))
				if tt.HasText(hint) {
					t.Fatal("底栏仍显示快捷键提示")
				}
				for _, paused := range []bool{true, false} {
					if err := tt.Click(locales.Settings()); err != nil {
						t.Fatal(err)
					}
					if err := tt.ChooseMenuItem(locales.PauseRecording()); err != nil {
						t.Fatal(err)
					}
					if a.settings.Paused != paused || a.st.Settings().Paused != paused {
						t.Fatal("设置菜单未保存暂停记录状态")
					}
				}
				if err := tt.Click(locales.Settings()); err != nil {
					t.Fatal(err)
				}
				target, label := formBar, locales.SwitchToBar()
				if form == formBar {
					target, label = formPanel, locales.SwitchToPointer()
				}
				if err := tt.ChooseMenuItem(label); err != nil || a.form != target {
					t.Fatalf("设置菜单无法切换面板形态：%v", err)
				}
			})
		}
	}
}

// TestTabsScrollbarDrag 验证两种主题的横向滚动条保持悬停显示并支持拖动
func TestTabsScrollbarDrag(t *testing.T) {
	for _, theme := range []int{themeClassic, themeGlass} {
		t.Run(fmt.Sprintf("theme-%d", theme), func(t *testing.T) {
			a := NewApp()
			a.form, a.settings = formPanel, types.DefaultSettings()
			a.settings.Theme = theme
			for i := range 12 {
				a.settings.CustomTabs = append(a.settings.CustomTabs, types.Tab{ID: fmt.Sprint(i), Name: fmt.Sprintf("工作列表%02d", i)})
			}
			tt := ui.NewTester(func(c *ui.Context) {
				a.applyTheme(c)
				p := a.palette(c)
				c.Root().Background(p.bg)
				a.tabsRow(c, p)
			}, panelMinWidth, 40)
			before, ok := tt.Find(locales.All())
			if !ok {
				t.Fatal("列表分组未显示")
			}
			tt.Move(before.X+before.W/2, before.Y+before.H/2)
			y := float32(26)
			if theme == themeGlass {
				y = 34
			}
			thumb := tt.Image().RGBAAt(32, int(y))
			tt.Move(32, y)
			if tt.Image().RGBAAt(32, int(y)) != thumb {
				t.Fatal("鼠标移入滚动条后滚动条消失")
			}
			tt.Press(32, y)
			tt.Move(192, y)
			tt.Release(192, y)
			if after, ok := tt.Find(locales.All()); ok && after == before {
				t.Fatal("拖动滚动条未改变分组位置")
			}
		})
	}
}

// TestGlassFloatingHeader 验证整块玻璃顶栏保持固定且搜索、滚动与键盘选择正常
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
				region := image.Rect(0, 0, size.w, glassHeaderHeight)
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
					t.Fatal("顶栏玻璃未采样滚动内容")
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

// TestNewRecordScrollPosition 验证新增记录在列表顶部完整显示并保留用户主动滚动的偏移
func TestNewRecordScrollPosition(t *testing.T) {
	for _, theme := range []int{themeClassic, themeGlass} {
		for _, scroll := range []float32{0, 120} {
			t.Run(fmt.Sprintf("theme-%d-scroll-%.0f", theme, scroll), func(t *testing.T) {
				a := NewApp()
				a.form, a.settings = formPanel, types.DefaultSettings()
				a.settings.Theme = theme
				for i := range 30 {
					a.records = append(a.records, &types.Record{ID: fmt.Sprint(i), Kind: types.KindText, Text: fmt.Sprintf("记录 %02d", i)})
				}
				initTestStore(t, a)
				tt := ui.NewTester(a.view, panelWidth, panelHeight)
				tt.Frame()
				first, ok := tt.Find("记录 00")
				if !ok {
					t.Fatal("初始记录未显示")
				}
				if scroll > 0 {
					tt.Scroll(panelWidth-3, 200, 0, scroll)
					if a.listScroll.Y <= 0 {
						t.Fatal("列表未滚动")
					}
				}
				offset := a.listScroll.Y
				if _, _, err := a.st.Capture(&types.Record{ID: "latest", Kind: types.KindText, Text: "最新记录"}, nil, nil); err != nil {
					t.Fatal(err)
				}
				a.reload()
				tt.Frame()
				if a.listScroll.Y != offset {
					t.Fatalf("新增记录改变了滚动偏移：%v → %v", offset, a.listScroll.Y)
				}
				if scroll == 0 {
					if latest, ok := tt.Find("最新记录"); !ok || latest.Y != first.Y {
						t.Fatalf("最新记录被顶部控制区遮挡：%+v，原位置 %+v", latest, first)
					}
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
