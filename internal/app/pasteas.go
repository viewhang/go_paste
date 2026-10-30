package app

import (
	"errors"
	"io/fs"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"

	"gopaste/internal/locales"
	"gopaste/internal/types"
)

// pasteFormat 标识显式选择的粘贴格式
type pasteFormat uint8

const (
	// pastePlainText 保留原文并移除富文本格式
	pastePlainText pasteFormat = iota
	// pasteMarkdown 将图片或链接转换为 Markdown
	pasteMarkdown
	// pasteUpperCase 将文本转换为大写
	pasteUpperCase
	// pasteLowerCase 将文本转换为小写
	pasteLowerCase
	// pastePascalCase 将文本转换为大驼峰命名
	pastePascalCase
	// pasteCamelCase 将文本转换为小驼峰命名
	pasteCamelCase
	// pasteSnakeCase 将文本转换为下划线命名
	pasteSnakeCase
	// pasteKebabCase 将文本转换为短横线命名
	pasteKebabCase
	// pasteFileName 粘贴包含扩展名的文件名
	pasteFileName
	// pasteFileStem 粘贴不包含扩展名的文件名
	pasteFileStem
	// pasteFilePath 粘贴文件的完整路径
	pasteFilePath
	// pasteColorHex 将颜色粘贴为十六进制表示
	pasteColorHex
	// pasteColorRGB 将颜色粘贴为 RGB 表示
	pasteColorRGB
	// pasteColorHSL 将颜色粘贴为 HSL 表示
	pasteColorHSL
)

// colorPasteFormats 定义颜色粘贴的格式与界面名称
var colorPasteFormats = []struct {
	format pasteFormat
	label  string
}{
	{pasteColorHex, "HEX"},
	{pasteColorRGB, "RGB"},
	{pasteColorHSL, "HSL"},
}

// pasteAsMenu 按记录类型显示可用的粘贴格式并保留纯文本快捷键
func (a *App) pasteAsMenu(m *ui.Menu, rec *types.Record) {
	image := markdownImageRecord(rec)
	files := rec.Kind == types.KindImage || rec.Kind == types.KindFile && recordFileCount(rec) > 0
	if !files && rec.Kind != types.KindText && rec.Kind != types.KindLink && rec.Kind != types.KindColor {
		return
	}
	m.Submenu(locales.PasteAs(), func(s *ui.Menu) {
		if !files {
			if s.Item(locales.PlainText()).Shortcut(a.shortcutKeys("pastePlain")).Chosen() {
				a.later(func() { a.pasteAs(rec, pastePlainText) })
			}
		}
		if image || rec.Kind == types.KindLink {
			if s.Item("Markdown").Chosen() {
				a.later(func() { a.pasteAs(rec, pasteMarkdown) })
			}
		}
		if files {
			if image {
				s.Separator()
			}
			for _, option := range []struct {
				format pasteFormat
				label  string
			}{
				{pasteFileName, locales.FileNameWithExtension()},
				{pasteFileStem, locales.FileNameWithoutExtension()},
				{pasteFilePath, locales.FilePath()},
			} {
				if s.Item(option.label).Chosen() {
					a.later(func() { a.pasteAs(rec, option.format) })
				}
			}
		}
		if rec.Kind == types.KindText {
			s.Separator()
			for _, option := range []struct {
				format pasteFormat
				label  string
			}{
				{pasteUpperCase, locales.UpperCase()},
				{pasteLowerCase, locales.LowerCase()},
				{pastePascalCase, locales.PascalCase()},
				{pasteCamelCase, locales.CamelCase()},
				{pasteSnakeCase, locales.SnakeCase()},
				{pasteKebabCase, locales.KebabCase()},
			} {
				if s.Item(option.label).Chosen() {
					a.later(func() { a.pasteAs(rec, option.format) })
				}
			}
		}
		if rec.Kind == types.KindColor {
			s.Separator()
			for _, option := range colorPasteFormats {
				if s.Item(option.label).Chosen() {
					a.later(func() { a.pasteAs(rec, option.format) })
				}
			}
		}
	})
}

// markdownImageRecord 判断记录是否为剪贴板图片或单个图片文件
func markdownImageRecord(rec *types.Record) bool {
	if rec.Kind == types.KindImage {
		return true
	}
	return rec.Kind == types.KindFile && recordFileCount(rec) == 1 && len(rec.Files) == 1 && !rec.Files[0].Dir &&
		strings.HasPrefix(mime.TypeByExtension(filepath.Ext(rec.Files[0].Path)), "image/")
}

// pasteAs 按指定格式生成剪贴板内容并复用正常粘贴流程
func (a *App) pasteAs(rec *types.Record, format pasteFormat) {
	data, err := a.pasteAsData(rec, format)
	if err != nil {
		a.toast(err.Error())
		return
	}
	a.pasteData(rec.ID, data)
}

// pasteAsData 按需加载完整记录并生成纯文本、Markdown、命名格式或文件信息
func (a *App) pasteAsData(rec *types.Record, format pasteFormat) (transfer.Data, error) {
	if format == pastePlainText {
		return a.clipboardData(rec, true)
	}
	rec = a.fullRecord(rec)
	if rec == nil {
		return transfer.Data{}, fs.ErrNotExist
	}
	var text string
	switch format {
	case pasteMarkdown:
		switch {
		case markdownImageRecord(rec):
			var path string
			label := locales.ImageKind()
			if rec.Kind == types.KindImage {
				path = a.st.BlobPath(rec.Image)
			} else {
				path = rec.Files[0].Path
				label = filepath.Base(path)
			}
			if _, err := os.Stat(path); err != nil {
				return transfer.Data{}, errors.New(locales.ImageFileMissing())
			}
			files, err := transfer.FileData(path)
			if err != nil {
				return transfer.Data{}, err
			}
			uri, err := files.Read(transfer.URIList)
			if err != nil {
				return transfer.Data{}, err
			}
			text = "!" + markdownLink(label, string(uri))
		case rec.Kind == types.KindLink:
			label := rec.Title
			if label == "" {
				label = rec.URL
			}
			text = markdownLink(label, rec.URL)
		default:
			return transfer.Data{}, errors.New(locales.NoPasteContent())
		}
	case pasteUpperCase:
		text = strings.ToUpper(rec.Text)
	case pasteLowerCase:
		text = strings.ToLower(rec.Text)
	case pastePascalCase, pasteCamelCase, pasteSnakeCase, pasteKebabCase:
		words := identifierWords(rec.Text)
		for i, word := range words {
			word = strings.ToLower(word)
			if format == pastePascalCase || format == pasteCamelCase && i > 0 {
				runes := []rune(word)
				runes[0] = unicode.ToUpper(runes[0])
				word = string(runes)
			}
			words[i] = word
		}
		separator := ""
		if format == pasteSnakeCase {
			separator = "_"
		} else if format == pasteKebabCase {
			separator = "-"
		}
		text = strings.Join(words, separator)
	case pasteFileName, pasteFileStem, pasteFilePath:
		files := rec.Files
		if rec.Kind == types.KindImage {
			files = []types.FileRef{{Path: a.st.BlobPath(rec.Image)}}
		}
		values := make([]string, len(files))
		for i, file := range files {
			value := file.Path
			if format != pasteFilePath {
				value = filepath.Base(value)
				if format == pasteFileStem && !file.Dir {
					if ext := filepath.Ext(value); ext != value {
						value = strings.TrimSuffix(value, ext)
					}
				}
			}
			values[i] = value
		}
		text = strings.Join(values, "\n")
	case pasteColorHex:
		text = ColorString(rec.Color, "hex")
	case pasteColorRGB:
		text = ColorString(rec.Color, "rgb")
	case pasteColorHSL:
		text = ColorString(rec.Color, "hsl")
	default:
		return transfer.Data{}, errors.New(locales.NoPasteContent())
	}
	return transfer.TextData(text), nil
}

// identifierWords 按分隔符、大小写切换及连续大写缩写拆分命名单词
func identifierWords(text string) []string {
	runes := []rune(text)
	var words []string
	start := -1
	for i, r := range runes {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsMark(r) {
			if start >= 0 {
				words = append(words, string(runes[start:i]))
				start = -1
			}
			continue
		}
		if start < 0 {
			start = i
		} else if unicode.IsUpper(r) && (unicode.IsLower(runes[i-1]) || unicode.IsDigit(runes[i-1]) ||
			unicode.IsUpper(runes[i-1]) && i+1 < len(runes) && unicode.IsLower(runes[i+1])) {
			words = append(words, string(runes[start:i]))
			start = i
		}
	}
	if start >= 0 {
		words = append(words, string(runes[start:]))
	}
	return words
}

// markdownLink 转义显示文本与目标地址以生成完整的 Markdown 链接
func markdownLink(label, target string) string {
	label = strings.NewReplacer("\\", "\\\\", "[", "\\[", "]", "\\]", "*", "\\*", "_", "\\_", "`", "\\`",
		"<", "\\<", ">", "\\>", "&", "\\&", "\r", " ", "\n", " ").Replace(label)
	target = strings.NewReplacer("\\", "%5C", "<", "%3C", ">", "%3E", "&", "\\&",
		" ", "%20", "\t", "%09", "\r", "%0D", "\n", "%0A").Replace(target)
	return "[" + label + "](<" + target + ">)"
}
