package app

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/transfer"

	"gopaste/internal/types"
)

// TestClipboardTextFormats 验证纯文本粘贴移除格式且普通复制保留原始文本与格式
func TestClipboardTextFormats(t *testing.T) {
	a := NewApp()
	rec := &types.Record{Kind: types.KindText, Text: "内容", HTML: "<b>内容</b>"}
	for _, plain := range []bool{false, true} {
		data, err := a.clipboardData(rec, plain, "hex")
		if err != nil {
			t.Fatal(err)
		}
		text, err := data.Read(transfer.Text)
		if err != nil || string(text) != rec.Text {
			t.Fatalf("纯文本内容丢失：%q，%v", text, err)
		}
		if slices.Contains(data.Formats(), transfer.HTML) == plain {
			t.Fatalf("纯文本模式 %v 的格式不正确：%v", plain, data.Formats())
		}
	}
}
