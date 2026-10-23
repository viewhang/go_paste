package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
	"gopaste/internal/store"
	"gopaste/internal/types"
)

// TestPagedHistoryActions 验证跨页展示、完整内容复制与预览及摘要缓存上限
func TestPagedHistoryActions(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	text := strings.Repeat("中文🙂", 1024)
	for i := range 160 {
		r := &types.Record{ID: fmt.Sprint(i), Kind: types.KindText, Text: fmt.Sprint(i) + text, HTML: "<p>" + text + "</p>"}
		if _, _, err := st.Capture(r, nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	a := NewApp()
	a.st = st
	a.form = formPanel
	a.reload()
	tt := ui.NewTester(a.view, 432, 596)
	for _, i := range []int{0, 49, 50, 99, 100, 159, 0} {
		a.list.ScrollTo(i, ui.Center)
		tt.Frame()
		r := a.recordAt(i)
		if r.ID != fmt.Sprint(159-i) || !r.Summary {
			t.Fatal("跨页记录错误")
		}
		data, err := a.clipboardData(r, false, "hex")
		if err != nil {
			t.Fatal(err)
		}
		raw, err := data.Read(transfer.Text)
		if err != nil || string(raw) != fmt.Sprint(159-i)+text {
			t.Fatal("复制内容被摘要截断")
		}
		if len(a.pages) > 2 {
			t.Fatal("滚动累积过多摘要页")
		}
	}
	a.sel = 159
	a.openPreview()
	if a.detail == nil || a.detail.Text != "0"+text || a.detail.Summary {
		t.Fatal("预览未加载完整内容")
	}
	a.closePreview()
	a.query = "中文🙂"
	a.filtered()
	if len(a.vis) != 160 || len(a.pages) != 0 {
		t.Fatal("搜索结果或分页失效错误")
	}
}

// TestPagedFileTransfer 验证文件摘要仍能复制和拖出全部文件
func TestPagedFileTransfer(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	r := &types.Record{ID: "files", Kind: types.KindFile}
	dir := t.TempDir()
	for i := range 6 {
		name := fmt.Sprintf("file-%d.txt", i)
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
		r.Files = append(r.Files, types.FileRef{Path: path, Name: name})
	}
	st.Add(r)
	a := NewApp()
	a.st = st
	a.reload()
	summary := a.recordAt(0)
	if len(summary.Files) != 3 || recordFileCount(summary) != 6 {
		t.Fatal("文件摘要错误")
	}
	data, err := a.clipboardData(summary, false, "hex")
	if err != nil {
		t.Fatal(err)
	}
	for _, data := range []transfer.Data{data, a.dragData(summary)} {
		paths, err := data.Files()
		if err != nil || len(paths) != 6 {
			t.Fatalf("只传输了摘要中的文件：%v", paths)
		}
	}
}
