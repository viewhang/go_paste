package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"gopaste/internal/store"
	"gopaste/internal/types"
)

// TestHistoryStorageSetting 验证容量输入可见、可编辑并持久化设置
func TestHistoryStorageSetting(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	a := NewApp()
	a.st, a.settings = st, st.Settings()
	tt := ui.NewTester(func(c *ui.Context) { a.settingsHistory(c, pal(c)) }, 560, 400)
	unit, ok := tt.Find("MiB")
	if !ok || unit.Y < 0 || unit.Y+unit.H > 400 {
		t.Fatal("历史容量控件未显示")
	}
	if os.Getenv("SHOTS") != "" {
		out := filepath.Join("..", "..", ".mygo", "shots")
		if err := os.MkdirAll(out, 0o755); err != nil {
			t.Fatal(err)
		}
		writePNG(t, filepath.Join(out, "history-storage.png"), tt.Image())
	}
	// 输入框位于容量单位左侧，避开右侧的步进按钮
	tt.ClickAt(unit.X-110, unit.Y+unit.H/2)
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("512")
	if a.settings.HistoryLimitMiB != 512 || st.Settings().HistoryLimitMiB != 512 {
		t.Fatalf("容量修改未保存：%+v", a.settings)
	}
	a.setSetting(func(s *types.Settings) { s.Theme = themeGlass })
	if st.Settings().HistoryLimitMiB != 512 {
		t.Fatal("切换主题丢失容量设置")
	}
}
