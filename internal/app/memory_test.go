package app

import (
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"gopaste/internal/store"
	"gopaste/internal/types"
)

// TestTextSummaryLimits 验证摘要的字符边界、换行与空白处理
func TestTextSummaryLimits(t *testing.T) {
	for _, tc := range []struct {
		text        string
		limit       int
		title, body string
	}{
		{"", 5, "", ""},
		{" \t\r\n　", 5, "", ""},
		{"abc \t", 3, "abc", "abc"},
		{"你好🙂世界", 3, "你好🙂…", "你好🙂…"},
		{"甲\n乙", 3, "甲 ·…", "甲\n乙"},
		{"ab \r\n c", 4, "ab ·…", "ab…"},
		{" \t甲\r\n\r\n乙　 丙\r丁\n戊 \n", 90, "甲 · 乙 丙 · 丁", "甲\n\n乙　 丙\n丁\n戊"},
		{strings.Repeat("A", 1<<20), 260, strings.Repeat("A", 260) + "…", strings.Repeat("A", 260) + "…"},
	} {
		if got := snippet(tc.text, tc.limit); got != tc.title {
			t.Fatalf("标题结果不一致：%q，期望 %q", got, tc.title)
		}
		if got := cardBodyText(tc.text, tc.limit); got != tc.body {
			t.Fatalf("正文结果不一致：%q，期望 %q", got, tc.body)
		}
	}
	if text, truncated := textPrefix("甲\r\n🙂乙", 4); text != "甲\r\n🙂" || !truncated {
		t.Fatalf("预览截断改变换行或截断字符：%q，%v", text, truncated)
	}
	if text, truncated := textPrefix("甲🙂", 2); text != "甲🙂" || truncated {
		t.Fatal("达到字符上限的完整文本被错误截断")
	}
}

// TestFilterCacheChanges 验证重复绘制复用筛选结果且记录与规则变化即时生效
func TestFilterCacheChanges(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	st.Add(&types.Record{ID: "one", Kind: types.KindText, Text: "Alpha"})
	st.Add(&types.Record{ID: "two", Kind: types.KindText, Text: "Beta"})
	a := NewApp()
	a.st = st
	a.reload()
	a.query = "alpha"
	if rows := a.filtered(); len(rows) != 1 || rows[0].ID != "one" {
		t.Fatal("搜索结果错误")
	}
	if allocs := testing.AllocsPerRun(100, func() { a.filtered() }); allocs != 0 {
		t.Fatalf("重复筛选仍分配内存：%v", allocs)
	}
	st.Add(&types.Record{ID: "three", Kind: types.KindText, Text: "Alpha new"})
	a.reload()
	if len(a.filtered()) != 2 {
		t.Fatal("新增记录未进入搜索结果")
	}
	a.query = ""
	a.setSetting(func(s *types.Settings) {
		s.CustomTabs = []types.Tab{{ID: "smart", Name: "筛选", Smart: true, Rules: []types.ListRule{{Field: types.RuleKeyword, Value: "Alpha"}}}}
	})
	a.tab = 1
	if len(a.filtered()) != 2 {
		t.Fatal("切换列表未重新筛选")
	}
	a.setSetting(func(s *types.Settings) {
		s.CustomTabs = []types.Tab{{ID: "smart", Name: "筛选", Smart: true, Rules: []types.ListRule{{Field: types.RuleKeyword, Value: "Beta"}}}}
	})
	if rows := a.filtered(); len(rows) != 1 || rows[0].ID != "two" {
		t.Fatal("修改智能规则未重新筛选")
	}
	st.Delete("two")
	a.reload()
	if len(a.filtered()) != 0 {
		t.Fatal("删除记录仍留在筛选结果中")
	}
	a.tab = 0
	if len(a.filtered()) != 2 {
		t.Fatal("返回全部列表未恢复记录")
	}
}

// TestFilePreviewVirtualization 验证多文件预览仅构建可见行且能滚动到末尾
func TestFilePreviewVirtualization(t *testing.T) {
	r := &types.Record{ID: "files", Kind: types.KindFile}
	for i := range 600 {
		name := fmt.Sprintf("文件-%03d.txt", i)
		r.Files = append(r.Files, types.FileRef{Name: name, Path: "/preview/" + name})
	}
	a := NewApp()
	a.form = formPanel
	tt := ui.NewTester(func(c *ui.Context) { a.previewBody(c, pal(c), r) }, 432, 400)
	if !tt.HasText("文件-000.txt") || tt.HasText("文件-599.txt") || len(tt.Texts()) > 100 {
		t.Fatal("文件预览未按可见范围构建")
	}
	tt.Scroll(200, 180, 0, 100000)
	if !tt.HasText("文件-599.txt") || len(tt.Texts()) > 100 {
		t.Fatal("文件预览无法滚动到末尾或构建了全部文件")
	}
}
