package app

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"gopaste/internal/store"
	"gopaste/internal/types"
)

// TestShortcutRecorder 验证默认呼出键、录制、冲突拦截、取消与快捷键持久化
func TestShortcutRecorder(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	a := NewApp()
	a.st, a.settings = st, st.Settings()
	if mods, key := a.shortcutKeys("toggle"); mods != ui.Cmd|ui.Shift || key != ui.KeyV {
		t.Fatal("默认呼出键应为 Command/Ctrl+Shift+V")
	}
	tt := ui.NewTester(func(c *ui.Context) { a.settingsShortcuts(c, pal(c)) }, 560, 650)
	if err := tt.Click("预览快捷键"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Cmd|ui.Shift, ui.KeyP)
	mods, key := a.shortcutKeys("preview")
	if mods != ui.Cmd|ui.Shift || key != ui.KeyP || a.recording != "" {
		t.Fatal("自定义快捷键录制未生效")
	}
	if err := tt.Click("聚焦搜索快捷键"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Cmd|ui.Shift, ui.KeyP)
	if a.shortcutError == "" || a.settings.Shortcuts["search"] != "" {
		t.Fatal("重复按键未被拦截")
	}
	tt.Key(0, ui.KeyEscape)
	if a.recording != "" {
		t.Fatal("Esc 未取消录制")
	}
	reopened, err := store.Open(st.Dir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	if reopened.Settings().Shortcuts["preview"] != a.settings.Shortcuts["preview"] {
		t.Fatal("快捷键设置未保存")
	}
	if err := tt.Click("恢复默认快捷键"); err != nil {
		t.Fatal(err)
	}
	if mods, key := a.shortcutKeys("preview"); mods != 0 || key != ui.KeySpace {
		t.Fatal("恢复默认快捷键未生效")
	}
	if mods, key := a.shortcutKeys("toggle"); mods != ui.Cmd|ui.Shift || key != ui.KeyV {
		t.Fatal("恢复默认后呼出键应为 Command/Ctrl+Shift+V")
	}
}

// TestCustomPreviewShortcut 验证两种形态使用新快捷键且搜索输入保留空格
func TestCustomPreviewShortcut(t *testing.T) {
	for _, form := range []int{formBar, formPanel} {
		a := NewApp()
		a.settings = types.DefaultSettings()
		a.form = form
		a.records = []*types.Record{{ID: "preview", Kind: types.KindText, Text: "hello world"}}
		initTestStore(t, a)
		tt := ui.NewTester(a.view, 1180, panelHeight)
		tt.Type("hello")
		tt.Key(0, ui.KeySpace)
		tt.Type(" ")
		if a.preview || a.query != "hello " {
			t.Fatalf("形态 %d 的搜索空格被预览快捷键占用：%q", form, a.query)
		}
		if err := a.setShortcut("preview", "CmdOrCtrl+Shift+P"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		tt.Key(ui.Cmd|ui.Shift, ui.KeyP)
		if !a.preview || a.detail == nil || a.detail.ID != a.records[0].ID {
			t.Fatalf("形态 %d 的自定义预览快捷键未生效：预览 %v，记录 %v，结果 %d，按键 %q，动作 %v", form, a.preview, a.detail, len(a.vis), a.shortcutAccel("preview"), a.acted)
		}
	}
}

// TestSpacePreviewFromSearch 验证空搜索框预览当前记录且保留搜索空格与自定义快捷键
func TestSpacePreviewFromSearch(t *testing.T) {
	for _, form := range []int{formBar, formPanel} {
		a := NewApp()
		a.settings = types.DefaultSettings()
		a.form = form
		a.records = []*types.Record{
			{ID: "first", Kind: types.KindText, Text: "hello world"},
			{ID: "second", Kind: types.KindText, Text: "second record"},
		}
		initTestStore(t, a)
		tt := ui.NewTester(a.view, 1180, panelHeight)
		next := ui.KeyDown
		if form == formBar {
			next = ui.KeyRight
		}
		tt.Key(0, next)
		for range 2 {
			tt.TypeKey(0, ui.KeySpace, " ")
			if !a.preview || a.detail == nil || a.detail.ID != a.records[1].ID || a.query != "" {
				t.Fatalf("形态 %d 的空格未预览当前记录：预览 %v，关键词 %q，选择 %d", form, a.preview, a.query, a.sel)
			}
			tt.Key(0, ui.KeyEscape)
		}
		tt.TypeKey(ui.Shift, ui.KeySpace, " ")
		if a.preview || a.query != " " {
			t.Fatal("Shift+Space 不应触发无修饰键的预览快捷键")
		}
		tt.Key(ui.Cmd, ui.KeyA)
		tt.Type("hello")
		tt.TypeKey(0, ui.KeySpace, " ")
		if a.preview || a.query != "hello " {
			t.Fatalf("形态 %d 的关键词空格触发了预览：%q", form, a.query)
		}
		if err := a.setShortcut("preview", "CmdOrCtrl+Shift+P"); err != nil {
			t.Fatal(err)
		}
		tt.Key(ui.Cmd, ui.KeyA)
		tt.Key(0, ui.KeyBackspace)
		tt.TypeKey(0, ui.KeySpace, " ")
		if a.preview || a.query != " " {
			t.Fatal("自定义预览键后空格仍触发预览")
		}
	}
}

// TestListShortcuts 验证两种面板切换列表、首尾停止、保留搜索及自定义按键
func TestListShortcuts(t *testing.T) {
	for _, form := range []int{formBar, formPanel} {
		a := NewApp()
		a.form, a.settings = form, types.DefaultSettings()
		a.settings.CustomTabs = []types.Tab{
			{ID: "work", Name: "工作"},
			{ID: "smart", Name: "智能", Smart: true, Rules: []types.ListRule{{Field: types.RuleKind, Value: "text"}}},
		}
		a.records = []*types.Record{
			{ID: "one", Kind: types.KindText, Text: "needle 1", ListID: "work"},
			{ID: "two", Kind: types.KindText, Text: "needle 2", ListID: "work"},
			{ID: "three", Kind: types.KindText, Text: "needle 3"},
		}
		initTestStore(t, a)
		for id, key := range map[string]ui.Key{"previousList": ui.KeyBracketLeft, "nextList": ui.KeyBracketRight} {
			if mods, got := a.shortcutKeys(id); mods != ui.Cmd || got != key {
				t.Fatalf("%s 默认快捷键不正确：%v %v", id, mods, got)
			}
		}
		tt := ui.NewTester(a.view, 1180, panelHeight)
		tt.Type("needle")
		for _, step := range []struct {
			key ui.Key
			tab int
		}{
			{ui.KeyBracketLeft, 0}, {ui.KeyBracketRight, 1}, {ui.KeyBracketRight, 2},
			{ui.KeyBracketRight, 2}, {ui.KeyBracketLeft, 1}, {ui.KeyBracketLeft, 0},
		} {
			a.sel = 1
			previous := a.tab
			tt.Key(ui.Cmd, step.key)
			if a.tab != step.tab || a.query != "needle" || previous != a.tab && a.sel != 0 {
				t.Fatalf("形态 %d 切换列表失败：列表 %d，搜索 %q，选择 %d", form, a.tab, a.query, a.sel)
			}
			want := 3
			if a.tab == 1 {
				want = 2
			}
			if len(a.vis) != want || !tt.Focused("搜索记录") {
				t.Fatal("切换列表未更新筛选或丢失搜索焦点")
			}
		}
		if err := a.setShortcut("nextList", "CmdOrCtrl+Shift+N"); err != nil {
			t.Fatal(err)
		}
		tt.Key(ui.Cmd, ui.KeyBracketRight)
		if a.tab != 0 {
			t.Fatal("自定义后仍响应默认切换键")
		}
		tt.Key(ui.Cmd|ui.Shift, ui.KeyN)
		if a.tab != 1 {
			t.Fatal("自定义列表切换键未生效")
		}
		a.setSetting(func(s *types.Settings) { s.CustomTabs = nil })
		a.tab = 0
		tt.Frame()
		for _, key := range []ui.Key{ui.KeyBracketLeft, ui.KeyN} {
			mods := ui.Cmd
			if key == ui.KeyN {
				mods |= ui.Shift
			}
			tt.Key(mods, key)
			if a.tab != 0 {
				t.Fatal("只有全部列表时切换到了无效列表")
			}
		}
	}
}

// TestDeleteShortcut 验证两种面板的删除快捷键录制、搜索编辑、预览删除与自定义按键
func TestDeleteShortcut(t *testing.T) {
	for _, form := range []int{formBar, formPanel} {
		a := NewApp()
		a.form, a.settings = form, types.DefaultSettings()
		a.records = []*types.Record{
			{ID: "first", Kind: types.KindText, Text: "first record"},
			{ID: "second", Kind: types.KindText, Text: "needle record"},
			{ID: "third", Kind: types.KindText, Text: "third record"},
			{ID: "fourth", Kind: types.KindText, Text: "last record"},
		}
		initTestStore(t, a)
		if mods, key := a.shortcutKeys("delete"); mods != 0 || key != ui.KeyBackspace {
			t.Fatal("默认删除快捷键应为 Backspace")
		}
		recorder := ui.NewTester(func(c *ui.Context) { a.settingsShortcuts(c, pal(c)) }, 560, 700)
		if err := recorder.Click("删除这条记录快捷键"); err != nil {
			t.Fatal(err)
		}
		recorder.Key(0, ui.KeyBackspace)
		if a.recording != "" || a.st.Settings().Shortcuts["delete"] != "Backspace" {
			t.Fatal("Backspace 删除快捷键未录制并保存")
		}
		tt := ui.NewTester(a.view, 1180, panelHeight)
		tt.Key(0, ui.KeyBackspace)
		if _, ok := a.st.Get("first"); ok || len(a.records) != 3 {
			t.Fatalf("形态 %d 的空搜索框未删除选中记录", form)
		}
		tt.Type("needle")
		tt.Key(0, ui.KeyBackspace)
		if a.query != "needl" || len(a.records) != 3 {
			t.Fatal("搜索框中的 Backspace 应只删除文字")
		}
		tt.Key(0, ui.KeyTab)
		tt.Key(0, ui.KeyBackspace)
		if _, ok := a.st.Get("second"); ok || len(a.records) != 2 || a.query != "needl" {
			t.Fatal("离开搜索框后未删除筛选记录或改变了搜索内容")
		}
		tt.Key(ui.Cmd, ui.KeyF)
		tt.Key(ui.Cmd, ui.KeyA)
		tt.Key(0, ui.KeyBackspace)
		if a.query != "" || len(a.records) != 2 {
			t.Fatal("清空搜索文字时不应删除记录")
		}
		if err := a.setShortcut("delete", "F8"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		tt.Key(0, ui.KeyBackspace)
		if len(a.records) != 2 {
			t.Fatal("自定义删除键后仍响应默认 Backspace")
		}
		tt.Key(0, ui.KeyF8)
		if _, ok := a.st.Get("third"); ok || len(a.records) != 1 {
			t.Fatal("自定义删除快捷键未生效")
		}
		if err := a.setShortcut("delete", "Backspace"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		tt.TypeKey(0, ui.KeySpace, " ")
		if !a.preview {
			t.Fatal("未打开记录预览")
		}
		tt.Key(0, ui.KeyBackspace)
		if a.preview || len(a.records) != 0 {
			t.Fatal("预览中的 Backspace 未删除记录并关闭预览")
		}
	}
}

// TestShortcutReservedKeys 验证文本编辑、快速粘贴与已有操作的按键不被覆盖
func TestShortcutReservedKeys(t *testing.T) {
	a := NewApp()
	a.settings = types.DefaultSettings()
	for _, choice := range []struct{ id, accel string }{
		{"toggle", "Shift+P"},
		{"preview", "P"},
		{"preview", "Backspace"},
		{"paste", "CmdOrCtrl+A"},
		{"preview", "CmdOrCtrl+C"},
		{"paste", "CmdOrCtrl+V"},
		{"paste", "CmdOrCtrl+1"},
		{"preview", "CmdOrCtrl+0"},
		{"preview", "CmdOrCtrl+["},
		{"preview", "CmdOrCtrl+]"},
	} {
		if err := a.setShortcut(choice.id, choice.accel); err == nil {
			t.Errorf("不应允许 %s 使用 %s", choice.id, choice.accel)
		}
	}
}

// TestPasteAsShortcut 验证默认与自定义按键在两种面板及预览中只打开粘贴格式菜单
func TestPasteAsShortcut(t *testing.T) {
	for _, form := range []int{formBar, formPanel} {
		for _, theme := range []int{themeClassic, themeGlass} {
			a := NewApp()
			a.form, a.settings = form, types.DefaultSettings()
			a.settings.Theme = theme
			a.records = []*types.Record{
				{ID: "first", Kind: types.KindText, Text: "needle first"},
				{ID: "second", Kind: types.KindText, Text: "needle second"},
			}
			initTestStore(t, a)
			if mods, key := a.shortcutKeys("pasteAs"); mods != ui.Shift || key != ui.KeyEnter {
				t.Fatal("粘贴为默认快捷键应为 Shift+Enter")
			}
			if _, key := a.shortcutKeys("pastePlain"); key != ui.KeyUnknown {
				t.Fatal("仍保留独立的纯文本快捷键")
			}
			tt := ui.NewTester(func(c *ui.Context) {
				a.applyTheme(c)
				a.acted = false
				a.vis = a.filtered()
				a.clampSel()
				a.keyboard(c)
				if form == formBar {
					a.barView(c, pal(c))
				} else {
					a.panelView(c, pal(c))
				}
			}, 1180, panelHeight)
			tt.Type("needle")
			a.sel = 1
			tt.Frame()
			for _, preview := range []bool{false, true} {
				if preview {
					a.openPreview()
					tt.Frame()
				}
				tt.Key(ui.Shift, ui.KeyEnter)
				if len(a.pending) != 1 || a.sel != 1 || a.query != "needle" || a.preview != preview {
					t.Fatalf("形态 %d 主题 %d 预览 %v 的快捷键未打开唯一菜单或改变记录状态：%d %d %q", form, theme, preview, len(a.pending), a.sel, a.query)
				}
				a.pending = nil
			}
			a.closePreview()
			tt.Frame()
			if err := a.setShortcut("pasteAs", "CmdOrCtrl+Shift+P"); err != nil {
				t.Fatal(err)
			}
			tt.Key(ui.Shift, ui.KeyEnter)
			if len(a.pending) != 0 {
				t.Fatal("修改快捷键后仍响应旧按键")
			}
			tt.Key(ui.Cmd|ui.Shift, ui.KeyP)
			if len(a.pending) != 1 || a.st.Settings().Shortcuts["pasteAs"] != "CmdOrCtrl+Shift+P" {
				t.Fatal("自定义粘贴为快捷键未生效或未保存")
			}
			a.pending = nil
			tt.Type(" missing")
			tt.Key(ui.Cmd|ui.Shift, ui.KeyP)
			if len(a.pending) != 0 {
				t.Fatal("无匹配记录时不应打开菜单")
			}
			recorder := ui.NewTester(func(c *ui.Context) { a.settingsShortcuts(c, pal(c)) }, 560, 700)
			if !recorder.HasText("粘贴为") || recorder.HasText("粘贴为纯文本") {
				t.Fatal("设置页仍显示旧的纯文本快捷键")
			}
		}
	}
}
