package app

import (
	"errors"
	"io/fs"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/egoist/mygo"
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

// pasteOption 定义粘贴格式与菜单名称，空名称表示分隔线
type pasteOption struct {
	format pasteFormat
	label  string
}

// colorPasteFormats 定义颜色粘贴的格式与界面名称
var colorPasteFormats = []pasteOption{
	{pasteColorHex, "HEX"},
	{pasteColorRGB, "RGB"},
	{pasteColorHSL, "HSL"},
}

// pasteOptions 按记录类型提供右键菜单与快捷键共用的粘贴格式
func pasteOptions(rec *types.Record) []pasteOption {
	image := markdownImageRecord(rec)
	files := rec.Kind == types.KindImage || rec.Kind == types.KindFile && recordFileCount(rec) > 0
	if !files && rec.Kind != types.KindText && rec.Kind != types.KindLink && rec.Kind != types.KindColor {
		return nil
	}
	var options []pasteOption
	if !files {
		options = append(options, pasteOption{pastePlainText, locales.PlainText()})
	}
	if image || rec.Kind == types.KindLink {
		options = append(options, pasteOption{pasteMarkdown, "Markdown"})
	}
	if files {
		if image {
			options = append(options, pasteOption{})
		}
		options = append(options,
			pasteOption{pasteFileName, locales.FileNameWithExtension()},
			pasteOption{pasteFileStem, locales.FileNameWithoutExtension()},
			pasteOption{pasteFilePath, locales.FilePath()},
		)
	}
	if rec.Kind == types.KindText {
		options = append(options, pasteOption{},
			pasteOption{pasteUpperCase, locales.UpperCase()},
			pasteOption{pasteLowerCase, locales.LowerCase()},
			pasteOption{pastePascalCase, locales.PascalCase()},
			pasteOption{pasteCamelCase, locales.CamelCase()},
			pasteOption{pasteSnakeCase, locales.SnakeCase()},
			pasteOption{pasteKebabCase, locales.KebabCase()},
		)
	}
	if rec.Kind == types.KindColor {
		options = append(options, pasteOption{})
		options = append(options, colorPasteFormats...)
	}
	return options
}

// pasteAsMenu 按记录类型显示粘贴格式子菜单并标注打开菜单的快捷键
func (a *App) pasteAsMenu(m *ui.Menu, rec *types.Record) {
	options := pasteOptions(rec)
	if len(options) == 0 {
		return
	}
	m.Submenu(locales.PasteAs(), func(s *ui.Menu) {
		for _, option := range options {
			if option.label == "" {
				s.Separator()
			} else if s.Item(option.label).Chosen() {
				a.later(func() { a.pasteAs(rec, option.format) })
			}
		}
	}).Shortcut(a.shortcutKeys("pasteAs"))
}

// openPasteAs 在当前记录或预览区域右侧打开原生粘贴格式菜单并在选择后执行粘贴
func (a *App) openPasteAs(c *ui.Context) {
	rec := a.selected()
	if a.preview {
		rec = a.detail
	}
	if rec == nil {
		return
	}
	options := pasteOptions(rec)
	if len(options) == 0 {
		return
	}
	bounds := a.pasteAnchor.Bounds(c)
	a.later(func() {
		go mygo.RunOnMain(func() {
			var chosen *pasteOption
			items := make([]*mygo.MenuItem, 0, len(options))
			for _, option := range options {
				if option.label == "" {
					items = append(items, mygo.Separator())
				} else {
					items = append(items, &mygo.MenuItem{
						Label: option.label,
						Click: func(*mygo.MenuItem, *mygo.Window) { chosen = &option },
					})
				}
			}
			mygo.NewMenu(items).PopupAt(a.win, int(bounds.X+bounds.W), int(bounds.Y))
			if chosen != nil {
				a.pasteAs(rec, chosen.format)
			}
		})
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
