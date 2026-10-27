package app

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/egoist/mygo/transfer"

	"gopaste/internal/types"
)

// TestClipboardFileAccess 验证文件写入前检查读取权限并保留已有文件与目录的传输
func TestClipboardFileAccess(t *testing.T) {
	a := NewApp()
	dir := t.TempDir()
	path := filepath.Join(dir, "附件.txt")
	if err := os.WriteFile(path, []byte("附件内容"), 0600); err != nil {
		t.Fatal(err)
	}
	rec := &types.Record{Kind: types.KindFile, Files: []types.FileRef{{Path: path}, {Path: dir}, {Path: filepath.Join(dir, "missing.txt")}}}
	data, err := a.clipboardData(rec, false)
	if err != nil {
		t.Fatal(err)
	}
	if paths, err := data.Files(); err != nil || !slices.Equal(paths, []string{path, dir}) {
		t.Fatalf("可访问文件与目录传输失败：%v，%v", paths, err)
	}
	if err := os.Chmod(path, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(path, 0600) })
	file, err := os.Open(path)
	if file != nil {
		file.Close()
	}
	if !errors.Is(err, fs.ErrPermission) {
		t.Skip("当前平台未应用文件读取权限限制")
	}
	if data, err := a.clipboardData(rec, false); !errors.Is(err, fs.ErrPermission) || len(data.Items()) != 0 {
		t.Fatalf("无权读取原文件时仍提供了文件引用：%v", err)
	}
}

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
