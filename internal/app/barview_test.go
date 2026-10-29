package app

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"gopaste/internal/types"
)

// TestPasteFailureKeepsSearch 验证两种面板在粘贴失败及隐藏后仍保留搜索内容
func TestPasteFailureKeepsSearch(t *testing.T) {
	for _, form := range []int{formBar, formPanel} {
		for _, reset := range []bool{false, true} {
			a := NewApp()
			a.form = form
			a.settings = types.DefaultSettings()
			a.settings.ResetPanelAfterPaste = reset
			a.records = []*types.Record{{
				ID: "missing", Kind: types.KindFile, Text: "needle",
				Files: []types.FileRef{{Path: filepath.Join(t.TempDir(), "missing.txt")}},
			}}
			initTestStore(t, a)
			tt := ui.NewTester(a.view, 1180, panelHeight)
			tt.Type("needle")
			tt.Key(0, ui.KeyEnter)
			a.hide()
			tt.Frame()
			if a.query != "needle" || len(a.vis) != 1 {
				t.Fatalf("形态 %d 的搜索在粘贴失败或隐藏后丢失，重置开关 %v", form, reset)
			}
		}
	}
}

// TestPanelClickDirectionKeys 验证鼠标选中记录后方向键与空格预览使用当前选择
func TestPanelClickDirectionKeys(t *testing.T) {
	for _, theme := range []int{themeClassic, themeGlass} {
		t.Run(fmt.Sprintf("theme-%d", theme), func(t *testing.T) {
			a := NewApp()
			a.form, a.settings = formPanel, types.DefaultSettings()
			a.settings.Theme = theme
			for i := range 20 {
				a.records = append(a.records, &types.Record{ID: fmt.Sprint(i), Kind: types.KindText, Text: fmt.Sprintf("记录 %d", i)})
			}
			initTestStore(t, a)
			tt := ui.NewTester(a.view, panelWidth, panelHeight)
			if err := tt.Click("记录 1"); err != nil || a.sel != 1 {
				t.Fatalf("鼠标未选中记录：%d，%v", a.sel, err)
			}
			for i, key := range []ui.Key{ui.KeyDown, ui.KeyDown, ui.KeyUp} {
				tt.Key(0, key)
				if want := []int{2, 3, 2}[i]; a.sel != want {
					t.Fatalf("鼠标选中后方向键 %v 未切换选择：得到 %d，期望 %d", key, a.sel, want)
				}
			}
			tt.Key(0, ui.KeySpace)
			if !a.preview || a.detail == nil || a.detail.ID != a.records[2].ID || a.sel != 2 {
				t.Fatalf("鼠标选中后空格未预览当前选择：预览 %v，选择 %d", a.preview, a.sel)
			}
		})
	}
}

// TestOperationHintLifetime 验证普通反馈自动消失且不会误清除后续错误提示
func TestOperationHintLifetime(t *testing.T) {
	for _, keepError := range []bool{false, true} {
		a := NewApp()
		a.settings = types.DefaultSettings()
		a.notify("已复制")
		tt := ui.NewTester(func(c *ui.Context) { a.statusRow(c, pal(c)) }, 560, 30)
		tt.Advance(2 * time.Second)
		if !tt.HasText("已复制") {
			t.Fatal("普通反馈提前消失")
		}
		if keepError {
			a.toast("自动粘贴失败，内容已在剪贴板")
			tt.Frame()
		}
		tt.Advance(2 * time.Second)
		if tt.HasText("已复制") {
			t.Fatal("普通反馈未在到期后消失")
		}
		if keepError && !tt.HasText("自动粘贴失败，内容已在剪贴板") {
			t.Fatal("错误提示被普通反馈的到期重绘清除")
		}
	}
}

// TestBarSearchInput 验证反复切换形态后可直接输入搜索并更新筛选结果
func TestBarSearchInput(t *testing.T) {
	a := NewApp()
	a.form = formPanel
	a.settings = types.DefaultSettings()
	a.records = []*types.Record{
		{ID: "target", Kind: types.KindText, Text: "目标记录 needle"},
		{ID: "other", Kind: types.KindText, Text: "其他记录"},
	}
	initTestStore(t, a)
	a.search.Focus()
	tt := ui.NewTester(a.view, panelWidth, panelHeight)
	for _, form := range []int{formBar, formPanel, formBar} {
		a.setForm(form)
		if form == formBar {
			tt.SetSize(1180, barHeight)
		} else {
			tt.SetSize(panelWidth, panelHeight)
		}
		if !tt.Focused("搜索记录") {
			t.Fatalf("切换到形态 %d 后搜索框未自动获得输入焦点", form)
		}
		tt.Type("needle")
		if a.query != "needle" || len(a.vis) != 1 || a.vis[0].ID != a.records[0].ID {
			t.Fatalf("搜索未生效：关键词 %q，结果 %d 条", a.query, len(a.vis))
		}
		tt.Key(ui.Cmd, ui.KeyA)
		tt.Key(0, ui.KeyBackspace)
	}
}

// TestPanelDirectionKeys 验证两种形态的选择方向及长条搜索框的光标编辑
func TestPanelDirectionKeys(t *testing.T) {
	a := NewApp()
	a.settings = types.DefaultSettings()
	a.records = []*types.Record{
		{ID: "first", Kind: types.KindText, Text: "needle 1"},
		{ID: "second", Kind: types.KindText, Text: "needle 2"},
		{ID: "third", Kind: types.KindText, Text: "needle 3"},
	}
	initTestStore(t, a)
	tt := ui.NewTester(a.view, 1180, barHeight)
	for _, form := range []int{formBar, formPanel, formBar} {
		a.setForm(form)
		a.sel = 0
		if form == formBar {
			tt.SetSize(1180, barHeight)
		} else {
			tt.SetSize(panelWidth, panelHeight)
		}
		next, previous, unusedNext, unusedPrevious := ui.KeyDown, ui.KeyUp, ui.KeyRight, ui.KeyLeft
		if form == formBar {
			next, previous, unusedNext, unusedPrevious = ui.KeyRight, ui.KeyLeft, ui.KeyDown, ui.KeyUp
		}
		for _, key := range []ui.Key{unusedNext, unusedPrevious} {
			tt.Key(0, key)
			if a.sel != 0 {
				t.Fatalf("形态 %d 被错误方向键 %v 改变了选择", form, key)
			}
		}
		for i, key := range []ui.Key{next, next, next, previous} {
			tt.Key(0, key)
			if want := []int{1, 2, 2, 1}[i]; a.sel != want {
				t.Fatalf("形态 %d 的方向键 %v 未正确选择：得到 %d", form, key, a.sel)
			}
		}
	}
	tt.Type("needle")
	tt.Key(0, ui.KeyLeft)
	tt.Type("x")
	if a.query != "needlxe" {
		t.Fatalf("长条搜索框的左右光标编辑失效：%q", a.query)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("needle")
	tt.Key(0, ui.KeyTab)
	tt.Key(0, ui.KeyRight)
	if a.sel != 1 {
		t.Fatal("退出搜索框后，左右键未能选择筛选结果")
	}
}
