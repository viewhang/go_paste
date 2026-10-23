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

// TestShortcutReservedKeys 验证文本编辑、快速粘贴与全局呼出的按键不被覆盖
func TestShortcutReservedKeys(t *testing.T) {
	a := NewApp()
	a.settings = types.DefaultSettings()
	for _, choice := range []struct{ id, accel string }{
		{"toggle", "Shift+P"},
		{"preview", "P"},
		{"paste", "CmdOrCtrl+A"},
		{"preview", "CmdOrCtrl+C"},
		{"paste", "CmdOrCtrl+V"},
		{"paste", "CmdOrCtrl+1"},
		{"preview", "CmdOrCtrl+0"},
	} {
		if err := a.setShortcut(choice.id, choice.accel); err == nil {
			t.Errorf("不应允许 %s 使用 %s", choice.id, choice.accel)
		}
	}
}
