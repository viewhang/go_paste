package app

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/transfer"

	"gopaste/internal/types"
)

// TestClipboardTextFormats 验证文本、链接与颜色的复制和拖出保留原文且仅纯文本粘贴移除格式
func TestClipboardTextFormats(t *testing.T) {
	a := NewApp()
	for _, rec := range []*types.Record{
		{Kind: types.KindText, Text: " \tuser@example.com\r\n", HTML: "<b>user@example.com</b>"},
		{Kind: types.KindText, Text: "example.com"},
		{Kind: types.KindLink, Text: " \tHTTPS://Example.COM/路径\r\n", URL: "HTTPS://Example.COM/路径", HTML: "<a>链接</a>"},
		{Kind: types.KindLink, Text: "user@example.com", URL: "https://user@example.com"},
		{Kind: types.KindColor, Text: " \t#aBc\r\n", Color: "#aabbcc", HTML: "<span>#aBc</span>"},
		{Kind: types.KindColor, Text: "rgb(255, 0, 0)", Color: "#ff0000"},
	} {
		for _, plain := range []bool{false, true} {
			data, err := a.clipboardData(rec, plain)
			if err != nil {
				t.Fatal(err)
			}
			text, err := data.Read(transfer.Text)
			if err != nil || string(text) != rec.Text {
				t.Fatalf("%s 原文被改写：%q，期望 %q，%v", rec.Kind, text, rec.Text, err)
			}
			if slices.Contains(data.Formats(), transfer.HTML) != (!plain && rec.HTML != "") || slices.Contains(data.Formats(), transfer.URIList) {
				t.Fatalf("%s 纯文本模式 %v 的格式不正确：%v", rec.Kind, plain, data.Formats())
			}
			if !plain && rec.HTML != "" {
				html, err := data.Read(transfer.HTML)
				if err != nil || string(html) != rec.HTML {
					t.Fatalf("%s 原始 HTML 被改写：%q，%v", rec.Kind, html, err)
				}
			}
		}
		data := a.dragData(rec)
		text, err := data.Read(transfer.Text)
		if err != nil || string(text) != rec.Text {
			t.Fatalf("拖出 %s 改写了原文：%q，%v", rec.Kind, text, err)
		}
		if rec.HTML != "" {
			html, err := data.Read(transfer.HTML)
			if err != nil || string(html) != rec.HTML {
				t.Fatalf("拖出 %s 改写了 HTML：%q，%v", rec.Kind, html, err)
			}
		}
	}
}
