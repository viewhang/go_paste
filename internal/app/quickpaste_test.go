package app

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"gopaste/internal/locales"
	"gopaste/internal/types"
)

// TestQuickPaste 验证两种形态与主题的可见项目编号、滚动、筛选和直接粘贴
func TestQuickPaste(t *testing.T) {
	for _, form := range []int{formBar, formPanel} {
		for _, theme := range []int{themeClassic, themeGlass} {
			t.Run(fmt.Sprintf("%d-%d", form, theme), func(t *testing.T) {
				a := NewApp()
				a.form = form
				a.settings = types.DefaultSettings()
				a.settings.Theme = theme
				for i := range 30 {
					a.records = append(a.records, &types.Record{
						ID: fmt.Sprint(i), Kind: types.KindFile, Text: fmt.Sprintf("needle %d", i), CreatedAt: time.Now(),
						Files: []types.FileRef{{Name: fmt.Sprintf("记录 %02d", i), Path: filepath.Join(t.TempDir(), "missing.txt")}},
					})
				}
				initTestStore(t, a)
				width, height := panelWidth, panelHeight
				if form == formBar {
					width, height = 1180, barHeight
				}
				first, last := 0, 0
				tt := ui.NewTester(func(c *ui.Context) {
					a.view(c)
					first, last = a.quickPasteRange(c)
				}, width, height)
				prefix := "Ctrl+"
				if isDarwin {
					prefix = "⌘"
				}
				if tt.HasText(prefix + "1") {
					t.Fatal("未按住修饰键时显示了编号")
				}
				tt.HoldModifiers(ui.Cmd)
				if first != 0 || last < 2 || last > 10 || !tt.HasText(prefix+"1") {
					t.Fatalf("初始可见项目未编号：%d..%d", first, last)
				}
				tt.Key(ui.Cmd, ui.Key2)
				if a.sel != 1 || a.hintFlash != locales.OriginalFileMissing() || a.query != "" {
					t.Fatalf("数字键未直接粘贴或修改了搜索：选中 %d，提示 %q，搜索 %q", a.sel, a.hintFlash, a.query)
				}
				tt.HoldModifiers(0)
				if tt.HasText(prefix + "1") {
					t.Fatal("松开修饰键后未隐藏编号")
				}
				if form == formBar {
					tt.SetSize(1180, 400)
					tt.HoldModifiers(ui.Cmd)
					if first != 0 || last != 3 || tt.HasText(prefix+"4") {
						t.Fatalf("卡片放大后编号未匹配可见项目：%d..%d", first, last)
					}
					tt.HoldModifiers(0)
					for range 6 {
						tt.Key(0, ui.KeyRight)
					}
					r, ok := tt.Find(recordTitle(a.selected()))
					if !ok || r.X < 18 || r.X+r.W > 1162 {
						t.Fatalf("放大后方向键未将选中项滚动到可见范围：%+v", r)
					}
					tt.HoldModifiers(ui.Cmd)
					want := first
					a.hintFlash = ""
					tt.Key(ui.Cmd, ui.Key1)
					if want == 0 || a.sel != want || a.hintFlash != locales.OriginalFileMissing() {
						t.Fatal("放大并滚动后快速粘贴未匹配首个可见项目")
					}
					tt.HoldModifiers(0)
					tt.SetSize(1180, barHeight)
				}
				if form == formBar {
					a.strip.X = 8*(cardW+cardGap) + 20
				} else {
					a.list.ScrollTo(8, ui.Start)
				}
				tt.Frame()
				tt.HoldModifiers(ui.Cmd)
				if first < 8 || !tt.HasText(prefix+"1") {
					t.Fatalf("滚动后编号未更新：%d..%d", first, last)
				}
				want := first
				a.hintFlash = ""
				tt.Key(ui.Cmd, ui.Key1)
				if a.sel != want || a.hintFlash != locales.OriginalFileMissing() {
					t.Fatalf("滚动后粘贴了错误项目：得到 %d，预期 %d", a.sel, want)
				}
				tt.HoldModifiers(ui.Cmd | ui.Shift)
				if tt.HasText(prefix + "1") {
					t.Fatal("额外修饰键仍显示快速粘贴编号")
				}
				tt.HoldModifiers(0)
				tt.Type("needle 29")
				tt.HoldModifiers(ui.Cmd)
				if len(a.vis) != 1 || !tt.HasText(prefix+"1") || tt.HasText(prefix+"2") {
					t.Fatalf("搜索结果编号错误：%d..%d，结果 %d 条", first, last, len(a.vis))
				}
				a.hintFlash = ""
				tt.Key(ui.Cmd, ui.Key2)
				if a.hintFlash != "" {
					t.Fatal("未显示的编号触发了粘贴")
				}
				tt.Key(ui.Cmd, ui.Key1)
				if a.selected().ID != a.records[29].ID || a.hintFlash != locales.OriginalFileMissing() || a.query != "needle 29" {
					t.Fatal("未粘贴搜索结果或未保留搜索")
				}
				a.preview = true
				a.detail = a.selected()
				tt.Frame()
				if tt.HasText(prefix + "1") {
					t.Fatal("预览中仍显示快速粘贴编号")
				}
				a.preview = false
				a.query = ""
				a.list.ScrollIntoView(0)
				a.strip.X = 0
				if form == formBar {
					tt.SetSize(2400, barHeight)
				} else {
					tt.SetSize(panelWidth, 1000)
				}
				if last-first != 10 || !tt.HasText(prefix+"0") {
					t.Fatalf("未限制为十项或缺少零编号：%d..%d", first, last)
				}
				tt.Key(ui.Cmd, ui.Key0)
				if a.sel != first+9 {
					t.Fatal("数字零未粘贴第十个可见项目")
				}
			})
		}
	}
}
