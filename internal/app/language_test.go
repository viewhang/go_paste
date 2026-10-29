package app

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"gopaste/internal/locales"
	"gopaste/internal/types"
)

// TestLanguageFollowsSystem 验证系统语言为简体中文时仍显示跟随系统且重绘后保留选项
func TestLanguageFollowsSystem(t *testing.T) {
	t.Setenv("MYGO_LOCALE", "zh-Hans")
	for _, theme := range []int{themeClassic, themeGlass} {
		t.Run(fmt.Sprintf("theme-%d", theme), func(t *testing.T) {
			a := NewApp()
			a.settings.Theme = theme
			tt := ui.NewTester(func(c *ui.Context) { a.settingsLanguage(c, pal(c)) }, 400, 70)
			for range 2 {
				if !tt.HasText("跟随系统") || tt.HasText("简体中文") {
					t.Fatal("跟随系统被显示为最终解析的简体中文")
				}
				tt.Frame()
			}
		})
	}
}

// TestLocalizedViews 验证两种面板切换语言后更新文案并保留用户列表名称
func TestLocalizedViews(t *testing.T) {
	for _, form := range []int{formBar, formPanel} {
		t.Run(fmt.Sprint(form), func(t *testing.T) {
			t.Setenv("MYGO_LOCALE", "zh-Hans")
			a := NewApp()
			a.settings = types.DefaultSettings()
			a.form = form
			a.settings.CustomTabs = []types.Tab{{ID: "user", Name: "我的工作"}}
			a.records = []*types.Record{{ID: "one", Kind: types.KindText, Text: "用户复制的内容"}}
			initTestStore(t, a)
			w, h := panelWidth, panelHeight
			if form == formBar {
				w, h = 1180, barHeight
			}
			tt := ui.NewTester(a.view, w, h)
			for _, tag := range []string{"en", "zh-Hans"} {
				t.Setenv("MYGO_LOCALE", tag)
				tt.SetLocale(tag)
				tt.Frame()
				messages := locales.In(tag)
				if !tt.HasText(messages.All()) || !tt.HasText(messages.RecordsCount(1)) || !tt.HasText("我的工作") {
					t.Fatalf("形态 %d 未切换为 %s 或改变了用户列表名称", form, tag)
				}
				if _, ok := tt.Find(messages.SearchClipboard()); !ok {
					t.Fatalf("搜索框未切换为 %s", tag)
				}
				if os.Getenv("SHOTS") != "" && tag == "en" {
					dir := filepath.Join("..", "..", ".mygo", "shots")
					if err := os.MkdirAll(dir, 0o755); err != nil {
						t.Fatal(err)
					}
					writePNG(t, filepath.Join(dir, fmt.Sprintf("locale-en-%d.png", form)), tt.Image())
				}
			}
		})
	}
}

// TestLocalizedEditorAndShortcuts 验证编辑表单与快捷键标题在切换语言后更新且不丢失输入
func TestLocalizedEditorAndShortcuts(t *testing.T) {
	t.Setenv("MYGO_LOCALE", "zh-Hans")
	a := NewApp()
	a.settings = types.DefaultSettings()
	a.form = formPanel
	initTestStore(t, a)
	a.newTab(true)
	a.prompt.value = "我的智能列表"
	tt := ui.NewTester(a.view, panelWidth, panelHeight)
	shortcuts := ui.NewTester(func(c *ui.Context) { a.settingsShortcuts(c, pal(c)) }, 560, 650)
	for _, tag := range []string{"en", "zh-Hans"} {
		t.Setenv("MYGO_LOCALE", tag)
		tt.SetLocale(tag)
		tt.Frame()
		shortcuts.Frame()
		messages := locales.In(tag)
		if !tt.HasText(messages.NewSmartList()) || !tt.HasText(messages.MatchAny()) || a.prompt.value != "我的智能列表" {
			t.Fatalf("编辑表单未切换为 %s 或丢失了输入", tag)
		}
		if _, ok := shortcuts.Find(messages.ShortcutLabel(messages.Preview())); !ok {
			t.Fatalf("快捷键标题未切换为 %s", tag)
		}
		if !shortcuts.HasText(messages.ShortcutPreviousList()) || !shortcuts.HasText(messages.ShortcutNextList()) {
			t.Fatalf("列表切换快捷键标题未切换为 %s", tag)
		}
	}
}

// TestLocalizedPluralMessages 验证官方语言匹配与英语单复数文案
func TestLocalizedPluralMessages(t *testing.T) {
	if locales.In("zh-CN").All() != "全部" || locales.In("en-US").All() != "All" {
		t.Fatal("地区语言未匹配到应用支持的语言")
	}
	if locales.In("en").RecordsCount(1) != "1 record" || locales.In("en").RecordsCount(2) != "2 records" {
		t.Fatal("英语记录数量未使用正确的单复数")
	}
	if locales.In("en").RetentionDays(1) != "1 day" || locales.In("en").RetentionDays(7) != "7 days" {
		t.Fatal("英语保留时间未使用正确的单复数")
	}
}
