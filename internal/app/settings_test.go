package app

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"gopaste/internal/locales"
	"gopaste/internal/store"
)

// TestSettingsNavigation 验证中英文设置分类、快捷键录制生命周期与设置即时保存
func TestSettingsNavigation(t *testing.T) {
	for _, tag := range []string{"zh-Hans", "en"} {
		for _, theme := range []int{themeClassic, themeGlass} {
			t.Run(fmt.Sprintf("%s-theme-%d", tag, theme), func(t *testing.T) {
				t.Setenv("MYGO_LOCALE", tag)
				st, err := store.Open(t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { st.Close() })
				a := NewApp()
				a.st, a.settings = st, st.Settings()
				a.settings.Theme = theme
				tt := ui.NewTester(a.settingsView, 720, 520)
				tt.SetTitleBar(ui.TitleBar{Height: 28, Left: 72})
				messages := locales.In(tag)
				for _, label := range []string{messages.General(), messages.Shortcuts(), messages.Privacy(), messages.Advanced(), messages.About()} {
					if r, ok := tt.Find(label); !ok || r.X >= 160 || r.Y < 46 || r.Y+r.H > 520 {
						t.Fatalf("分类菜单 %s 未完整显示在左侧", label)
					}
				}
				if tt.HasText(messages.Done()) || !tt.HasText(messages.Language()) || !tt.HasText(messages.HistoryStorageLimit()) || tt.HasText(messages.ExcludedApps()) {
					t.Fatal("通用页的设置分组不正确或仍显示完成按钮")
				}
				if err := tt.Click(messages.Advanced()); err != nil {
					t.Fatal(err)
				}
				plain, ok := tt.Find(messages.PastePlainDefault())
				if !ok || !tt.HasText(messages.RecordLocation()) || !tt.HasText(messages.ClearHistory()) || tt.HasText(messages.Theme()) {
					t.Fatal("高级页未正确显示粘贴与数据设置")
				}
				// 点击设置行右侧开关以验证修改后直接保存
				tt.ClickAt(664, plain.Y+plain.H)
				if !a.settings.PastePlain || !st.Settings().PastePlain {
					t.Fatal("默认纯文本设置未即时生效并保存")
				}
				moveToFront, ok := tt.Find(messages.MoveToFrontOnPaste())
				if !ok || a.settings.MoveToFrontOnPaste {
					t.Fatal("粘贴后移至最前设置未显示或默认启用")
				}
				tt.ClickAt(664, moveToFront.Y+moveToFront.H/2)
				if !a.settings.MoveToFrontOnPaste || !st.Settings().MoveToFrontOnPaste {
					t.Fatal("粘贴后移至最前设置未即时保存")
				}
				reset, ok := tt.Find(messages.ResetPanelAfterPaste())
				if !ok || a.settings.ResetPanelAfterPaste {
					t.Fatal("粘贴后重置面板设置未显示或默认启用")
				}
				tt.ClickAt(664, reset.Y+reset.H/2)
				if !a.settings.ResetPanelAfterPaste || !st.Settings().ResetPanelAfterPaste {
					t.Fatal("粘贴后重置面板设置未即时保存")
				}
				if err := tt.Click(messages.Shortcuts()); err != nil {
					t.Fatal(err)
				}
				if err := tt.Click(messages.ShortcutLabel(messages.Preview())); err != nil || a.recording != "preview" {
					t.Fatalf("快捷键页无法开始录制：%v", err)
				}
				if err := tt.Click(messages.Privacy()); err != nil || a.recording != "" || !tt.HasText(messages.ExcludedApps()) || tt.HasText(messages.RestoreShortcuts()) {
					t.Fatalf("切换隐私页未结束录制或分组不正确：%v", err)
				}
				if err := tt.Click(messages.General()); err != nil {
					t.Fatal(err)
				}
				tt.Scroll(500, 300, 0, 500)
				if err := tt.Click(messages.About()); err != nil {
					t.Fatal(err)
				}
				if version, ok := tt.Find(messages.CurrentVersion()); !ok || version.Y < 46 || version.Y+version.H > 520 || !tt.HasText(messages.CheckUpdates()) || tt.HasText(messages.PauseRecording()) {
					t.Fatal("关于页未独立显示版本与更新设置")
				}
				if err := tt.Click(messages.Advanced()); err != nil || !a.settings.PastePlain || !a.settings.MoveToFrontOnPaste {
					t.Fatalf("切换分类丢失设置：%v", err)
				}
				reopened, err := store.Open(st.Dir())
				if err != nil {
					t.Fatal(err)
				}
				defer reopened.Close()
				if !reopened.Settings().PastePlain || !reopened.Settings().MoveToFrontOnPaste {
					t.Fatal("重新读取后设置未持久化")
				}
				if !reopened.Settings().ResetPanelAfterPaste {
					t.Fatal("重新读取后粘贴后重置面板设置丢失")
				}
				reset, _ = tt.Find(messages.ResetPanelAfterPaste())
				tt.ClickAt(664, reset.Y+reset.H/2)
				if a.settings.ResetPanelAfterPaste || st.Settings().ResetPanelAfterPaste {
					t.Fatal("粘贴后重置面板设置无法关闭")
				}
				tt.Frame()
				if os.Getenv("SHOTS") != "" {
					out := filepath.Join("..", "..", ".mygo", "shots")
					if err := os.MkdirAll(out, 0o755); err != nil {
						t.Fatal(err)
					}
					writePNG(t, filepath.Join(out, fmt.Sprintf("settings-%s-theme-%d-minimum.png", tag, theme)), tt.Image())
				}
			})
		}
	}
}
