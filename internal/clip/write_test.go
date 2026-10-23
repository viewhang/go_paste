package clip

import (
	"bytes"
	"path/filepath"
	"slices"
	"testing"

	"github.com/egoist/mygo/transfer"

	"gopaste/internal/native"
	"gopaste/internal/store"
	"gopaste/internal/types"
)

// TestOwnClipboardMarker 验证来源标记保留文本、图片与多文件内容
func TestOwnClipboardMarker(t *testing.T) {
	files, err := transfer.FileData(filepath.Join(t.TempDir(), "a.txt"), filepath.Join(t.TempDir(), "b.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, data := range []transfer.Data{
		transfer.TextData("文本"),
		transfer.New(transfer.NewItem(transfer.Bytes(transfer.Text, []byte("内容")), transfer.Bytes(transfer.HTML, []byte("<b>内容</b>")))),
		transfer.New(transfer.NewItem(transfer.Bytes(transfer.PNG, []byte{1, 2, 3}))),
		files,
	} {
		marked := markOwnClipboard(data)
		if !slices.Contains(marked.Formats(), ownClipboardFormat) || len(marked.Items()) != len(data.Items()) {
			t.Fatal("自身标记缺失或改变了剪贴项目数量")
		}
		for i, item := range data.Items() {
			for _, format := range item.Formats() {
				want, _ := item.Read(format)
				got, err := marked.Items()[i].Read(format)
				if err != nil || !bytes.Equal(got, want) {
					t.Fatalf("第 %d 项的 %s 内容改变：%v", i, format, err)
				}
			}
		}
	}
}

// TestCaptureAfterOwnWrite 验证自身写入后立即复制仍被记录并沿用当前排除规则
func TestCaptureAfterOwnWrite(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	settings := st.Settings()
	settings.ExcludedApps = []types.ExcludedApp{{ID: "com.example.private", Name: "排除应用"}}
	if err := st.SetSettings(settings); err != nil {
		t.Fatal(err)
	}
	w := New(st, nil, nil, nil)
	app := native.App{ID: "com.example.editor", Name: "编辑器"}
	data := transfer.TextData("连续复制内容")
	if rec, err := w.capture(markOwnClipboard(data), app); err != nil || rec != nil {
		t.Fatalf("记录了自身写入：%+v，%v", rec, err)
	}
	if rec, err := w.capture(data, app); err != nil || rec == nil || rec.Source.AppID != app.ID {
		t.Fatalf("自身写入后立即复制未记录或来源错误：%+v，%v", rec, err)
	}
	app = native.App{ID: "com.example.private", Name: "排除应用"}
	private := transfer.TextData("切换应用前复制的内容")
	if rec, err := w.capture(private, app); err != nil || rec != nil {
		t.Fatalf("未按原始复制来源排除记录：%+v，%v", rec, err)
	}
	settings.ExcludedApps = nil
	if err := st.SetSettings(settings); err != nil {
		t.Fatal(err)
	}
	if rec, err := w.capture(private, app); err != nil || rec == nil || rec.Source.AppID != app.ID {
		t.Fatalf("移除规则后仍被排除：%+v，%v", rec, err)
	}
}
