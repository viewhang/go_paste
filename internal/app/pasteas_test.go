package app

import (
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"

	"gopaste/internal/locales"
	"gopaste/internal/store"
	"gopaste/internal/types"
)

// TestPasteAsText 验证纯文本、大小写与命名转换保留原记录且只提供文本格式
func TestPasteAsText(t *testing.T) {
	a := NewApp()
	a.settings.PastePlain = true
	for _, tc := range []struct {
		format     pasteFormat
		text, want string
	}{
		{pastePlainText, " \tHello\r\n世界 ", " \tHello\r\n世界 "},
		{pasteUpperCase, " \tHello é世界\r\n", " \tHELLO É世界\r\n"},
		{pasteLowerCase, " \tHELLO É世界\r\n", " \thello é世界\r\n"},
		{pastePascalCase, "hello world", "HelloWorld"},
		{pasteCamelCase, "hello world", "helloWorld"},
		{pasteSnakeCase, "hello world", "hello_world"},
		{pasteKebabCase, "hello world", "hello-world"},
		{pastePascalCase, "XMLHttpRequest", "XmlHttpRequest"},
		{pasteCamelCase, "XMLHttpRequest", "xmlHttpRequest"},
		{pasteSnakeCase, "userID HTTP2Server", "user_id_http2_server"},
		{pasteKebabCase, "userID HTTP2Server", "user-id-http2-server"},
		{pasteCamelCase, "  foo__BAR-baz.\nqux  ", "fooBarBazQux"},
		{pastePascalCase, "école déjàVu", "ÉcoleDéjàVu"},
		{pasteCamelCase, "中文 hello World", "中文HelloWorld"},
		{pasteSnakeCase, "中文 hello World", "中文_hello_world"},
		{pasteCamelCase, "", ""},
		{pastePascalCase, " -- _\n", ""},
	} {
		rec := &types.Record{ID: "original", Kind: types.KindText, Text: tc.text, HTML: "<b>原文</b>"}
		original := *rec
		data, err := a.pasteAsData(rec, tc.format)
		if err != nil {
			t.Fatal(err)
		}
		text, err := data.Read(transfer.Text)
		if err != nil || string(text) != tc.want || !slices.Equal(data.Formats(), []transfer.Format{transfer.Text}) {
			t.Fatalf("格式 %d 转换 %q 得到 %q，期望 %q，格式 %v，错误 %v", tc.format, tc.text, text, tc.want, data.Formats(), err)
		}
		if !reflect.DeepEqual(*rec, original) {
			t.Fatal("转换改写了原记录")
		}
	}
}

// TestPasteAsColor 验证颜色格式以纯文本粘贴且不改写原记录
func TestPasteAsColor(t *testing.T) {
	a := NewApp()
	rec := &types.Record{Kind: types.KindColor, Text: "#f00", Color: "#ff0000"}
	original := *rec
	for _, tc := range []struct {
		format pasteFormat
		want   string
	}{
		{pasteColorHex, "#FF0000"},
		{pasteColorRGB, "rgb(255, 0, 0)"},
		{pasteColorHSL, "hsl(360, 100%, 50%)"},
	} {
		data, err := a.pasteAsData(rec, tc.format)
		if err != nil {
			t.Fatal(err)
		}
		text, err := data.Read(transfer.Text)
		if err != nil || string(text) != tc.want || !slices.Equal(data.Formats(), []transfer.Format{transfer.Text}) || !reflect.DeepEqual(*rec, original) {
			t.Fatalf("颜色格式 %d 的粘贴内容或原记录不正确：%q，%v", tc.format, text, err)
		}
	}
}

// TestPasteAsMarkdown 验证完整链接、特殊字符及本地图片路径的 Markdown 转换
func TestPasteAsMarkdown(t *testing.T) {
	t.Setenv("MYGO_LOCALE", "zh-Hans")
	st, err := store.Open(filepath.Join(t.TempDir(), "图片 (test) & raw"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	a := NewApp()
	a.st = st
	link := &types.Record{
		ID: "link", Kind: types.KindLink, Text: "https://example.test/a(b)?x=1&copy;",
		URL: "https://example.test/a(b)?x=1&copy;", Title: "标题 [A] *B* & <C>", HTML: "<a>原链接</a>",
	}
	if _, _, err := st.Capture(link, nil, nil); err != nil {
		t.Fatal(err)
	}
	summary := &types.Record{ID: link.ID, Kind: types.KindLink, Summary: true, Title: "摘要"}
	data, err := a.pasteAsData(summary, pasteMarkdown)
	if err != nil {
		t.Fatal(err)
	}
	text, err := data.Read(transfer.Text)
	if want := `[标题 \[A\] \*B\* \& \<C\>](<https://example.test/a(b)?x=1\&copy;>)`; err != nil || string(text) != want {
		t.Fatalf("未读取完整链接或正确转义：%q，%v", text, err)
	}
	if full, _ := st.Get(link.ID); full.Title != link.Title || full.Text != link.Text || full.HTML != link.HTML {
		t.Fatal("转换修改了数据库内的原记录")
	}
	link.Title = ""
	data, err = a.pasteAsData(link, pasteMarkdown)
	if err != nil {
		t.Fatal(err)
	}
	if text, _ := data.Read(transfer.Text); string(text) != `[https://example.test/a(b)?x=1\&copy;](<https://example.test/a(b)?x=1\&copy;>)` {
		t.Fatalf("无标题链接未使用原网址：%q", text)
	}
	name, err := st.SaveBlob(sampleIcon(), "png")
	if err != nil {
		t.Fatal(err)
	}
	image := &types.Record{Kind: types.KindImage, Image: name}
	data, err = a.pasteAsData(image, pasteMarkdown)
	if err != nil {
		t.Fatal(err)
	}
	text, _ = data.Read(transfer.Text)
	if !strings.HasPrefix(string(text), "![图片](<file:///") || !strings.HasSuffix(string(text), ">)") || !slices.Equal(data.Formats(), []transfer.Format{transfer.Text}) {
		t.Fatalf("图片未生成纯文本 Markdown 文件引用：%q，格式 %v", text, data.Formats())
	}
	target := strings.TrimSuffix(strings.TrimPrefix(string(text), "![图片](<"), ">)")
	uri, err := url.Parse(strings.ReplaceAll(target, `\&`, "&"))
	if err != nil || strings.TrimPrefix(uri.Path, "/") != strings.TrimPrefix(filepath.ToSlash(st.BlobPath(name)), "/") {
		t.Fatalf("图片引用未保留原路径：%q，%v", target, err)
	}
	if err := os.Remove(st.BlobPath(name)); err != nil {
		t.Fatal(err)
	}
	if data, err := a.pasteAsData(image, pasteMarkdown); err == nil || len(data.Items()) != 0 {
		t.Fatal("图片丢失时仍生成了粘贴内容")
	}
}

// TestPasteAsImageFile 验证 Finder 图片文件的菜单、摘要加载与原文件路径引用
func TestPasteAsImageFile(t *testing.T) {
	t.Setenv("MYGO_LOCALE", "zh-Hans")
	path := filepath.Join(t.TempDir(), "图片 [A] & (1).PNG")
	if err := os.WriteFile(path, sampleIcon(), 0o600); err != nil {
		t.Fatal(err)
	}
	a := NewApp()
	a.records = []*types.Record{{ID: "finder-image", Kind: types.KindFile, Files: []types.FileRef{{Path: path, Name: filepath.Base(path)}}}}
	initTestStore(t, a)
	rec := a.recordAt(0)
	if !rec.Summary {
		t.Fatal("未读取到图片文件摘要")
	}
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Button(c, "record").ContextMenu(func(m *ui.Menu) { a.cardMenu(c, m, rec) })
	}, 300, 200)
	if err := tt.RightClick("record"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem(locales.PasteAs(), "Markdown"); err != nil || len(a.pending) != 1 {
		t.Fatalf("Finder 图片文件未提供 Markdown 粘贴入口：%v", err)
	}
	data, err := a.pasteAsData(rec, pasteMarkdown)
	if err != nil {
		t.Fatal(err)
	}
	text, _ := data.Read(transfer.Text)
	prefix := `![图片 \[A\] \& (1).PNG](<`
	if !strings.HasPrefix(string(text), prefix) || !strings.HasSuffix(string(text), ">)") || !slices.Equal(data.Formats(), []transfer.Format{transfer.Text}) {
		t.Fatalf("图片文件未生成带文件名的纯文本 Markdown：%q", text)
	}
	target := strings.TrimSuffix(strings.TrimPrefix(string(text), prefix), ">)")
	uri, err := url.Parse(strings.ReplaceAll(target, `\&`, "&"))
	if err != nil || uri.Scheme != "file" || strings.TrimPrefix(uri.Path, "/") != strings.TrimPrefix(filepath.ToSlash(path), "/") {
		t.Fatalf("Markdown 未引用原图片文件：%q，%v", target, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if data, err := a.pasteAsData(rec, pasteMarkdown); err == nil || len(data.Items()) != 0 {
		t.Fatal("原图片文件丢失后仍生成粘贴内容")
	}
}

// TestPasteAsFileInfo 验证完整文件列表与剪贴板图片的名称、扩展名及路径转换
func TestPasteAsFileInfo(t *testing.T) {
	a := NewApp()
	dir := t.TempDir()
	files := []types.FileRef{
		{Path: filepath.Join(dir, "图片 [A].PNG")},
		{Path: filepath.Join(dir, "视频.mov")},
		{Path: filepath.Join(dir, "archive.tar.gz")},
		{Path: filepath.Join(dir, ".gitignore")},
		{Path: filepath.Join(dir, "folder.with.dot"), Dir: true},
	}
	a.records = []*types.Record{{ID: "files", Kind: types.KindFile, Files: files}}
	initTestStore(t, a)
	rec := a.recordAt(0)
	if !rec.Summary || len(rec.Files) >= len(files) {
		t.Fatal("未使用截断的文件摘要验证完整列表加载")
	}
	var paths []string
	for _, file := range files {
		paths = append(paths, file.Path)
	}
	check := func(rec *types.Record, format pasteFormat, want string) {
		t.Helper()
		data, err := a.pasteAsData(rec, format)
		if err != nil {
			t.Fatal(err)
		}
		text, err := data.Read(transfer.Text)
		if err != nil || string(text) != want || !slices.Equal(data.Formats(), []transfer.Format{transfer.Text}) {
			t.Fatalf("文件信息格式 %d 得到 %q，期望 %q，错误 %v", format, text, want, err)
		}
	}
	check(rec, pasteFileName, "图片 [A].PNG\n视频.mov\narchive.tar.gz\n.gitignore\nfolder.with.dot")
	check(rec, pasteFileStem, "图片 [A]\n视频\narchive.tar\n.gitignore\nfolder.with.dot")
	check(rec, pasteFilePath, strings.Join(paths, "\n"))
	name, err := a.st.SaveBlob(sampleIcon(), "png")
	if err != nil {
		t.Fatal(err)
	}
	image := &types.Record{Kind: types.KindImage, Image: name}
	check(image, pasteFileName, name)
	check(image, pasteFileStem, strings.TrimSuffix(name, ".png"))
	check(image, pasteFilePath, a.st.BlobPath(name))
}

// TestPasteAsFileMenu 验证图片、视频及普通文件在卡片和预览中共用文件信息粘贴菜单
func TestPasteAsFileMenu(t *testing.T) {
	for _, tag := range []string{"zh-Hans", "en"} {
		t.Setenv("MYGO_LOCALE", tag)
		for _, name := range []string{"图片.PNG", "视频.mp4", "文档.txt"} {
			for _, preview := range []bool{false, true} {
				a := NewApp()
				rec := &types.Record{ID: "file", Kind: types.KindFile, Files: []types.FileRef{{Path: filepath.Join(t.TempDir(), name), Name: name}}}
				tt := ui.NewTester(func(c *ui.Context) {
					if preview {
						a.previewBody(c, pal(c), rec)
					} else {
						ui.Button(c, name).ContextMenu(func(m *ui.Menu) { a.cardMenu(c, m, rec) })
					}
				}, 520, 250)
				tt.SetLocale(tag)
				for _, label := range []string{locales.FileNameWithExtension(), locales.FileNameWithoutExtension(), locales.FilePath()} {
					if err := tt.RightClick(name); err != nil {
						t.Fatal(err)
					}
					for _, item := range tt.Menu() {
						if strings.HasPrefix(item, "复制") || strings.HasPrefix(item, "Copy") {
							t.Fatalf("文件菜单仍显示复制入口：%s", item)
						}
					}
					if err := tt.ChooseMenuItem(locales.PasteAs(), label); err != nil || len(a.pending) != 1 {
						t.Fatalf("%s 文件 %s 的 %s 菜单不正确，预览 %t：%v", tag, name, label, preview, err)
					}
					a.pending = nil
				}
			}
		}
	}
}

// TestPasteAsMenu 验证中英文记录菜单无复制入口且按类型提供可执行的粘贴格式
func TestPasteAsMenu(t *testing.T) {
	for _, tag := range []string{"zh-Hans", "en"} {
		t.Setenv("MYGO_LOCALE", tag)
		for _, kind := range []types.Kind{types.KindText, types.KindLink, types.KindImage, types.KindColor, types.KindFile} {
			a := NewApp()
			rec := &types.Record{Kind: kind}
			tt := ui.NewTester(func(c *ui.Context) {
				ui.Button(c, "record").ContextMenu(func(m *ui.Menu) { a.cardMenu(c, m, rec) })
			}, 300, 200)
			tt.SetLocale(tag)
			if kind == types.KindLink {
				preview := ui.NewTester(func(c *ui.Context) { a.previewBody(c, pal(c), rec) }, 520, 250)
				preview.SetLocale(tag)
				if preview.HasText("复制链接") || preview.HasText("Copy URL") {
					t.Fatal("链接预览仍显示复制链接按钮")
				}
			}
			fileOptions := []string{locales.FileNameWithExtension(), locales.FileNameWithoutExtension(), locales.FilePath()}
			colorOptions := []string{"HEX", "RGB", "HSL"}
			options := append([]string{locales.PlainText(), "Markdown", locales.UpperCase(), locales.LowerCase(), locales.PascalCase(), locales.CamelCase(), locales.SnakeCase(), locales.KebabCase()}, fileOptions...)
			for _, label := range append(options, colorOptions...) {
				if err := tt.RightClick("record"); err != nil {
					t.Fatal(err)
				}
				if slices.Contains(tt.Menu(), locales.PastePlain()) {
					t.Fatal("纯文本仍作为独立的顶层菜单项显示")
				}
				for _, item := range tt.Menu() {
					if strings.HasPrefix(item, "复制") || strings.HasPrefix(item, "Copy") {
						t.Fatalf("%s 类型 %s 的菜单仍显示复制入口：%s", tag, kind, item)
					}
				}
				fileOption := slices.Contains(fileOptions, label)
				colorOption := slices.Contains(colorOptions, label)
				want := kind == types.KindText && label != "Markdown" && !fileOption && !colorOption ||
					(kind == types.KindLink && (label == locales.PlainText() || label == "Markdown")) ||
					kind == types.KindImage && (label == "Markdown" || fileOption) || kind == types.KindColor && (label == locales.PlainText() || colorOption)
				err := tt.ChooseMenuItem(locales.PasteAs(), label)
				queued := 0
				if want {
					queued = 1
				}
				if (err == nil) != want || len(a.pending) != queued {
					t.Fatalf("%s 类型 %s 的 %s 选项不正确：%v，待执行 %d", tag, kind, label, err, len(a.pending))
				}
				a.pending = nil
				tt.CloseMenu()
			}
		}
	}
}
