package app

import (
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode"

	"gopaste/internal/locales"
	"gopaste/internal/types"
)

// relTime 按当前语言显示记录复制时间
func relTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	now := time.Now()
	d := now.Sub(t)
	switch {
	case d < 45*time.Second:
		return locales.JustNow()
	case d < time.Hour:
		return locales.MinutesAgo(int(d.Minutes()))
	}
	y, m, day := now.Date()
	ty, tm, td := t.Date()
	switch {
	case y == ty && m == tm && day == td:
		return locales.TimeOfDay(t)
	case y == ty && m == tm && day-td == 1:
		return locales.Yesterday(t)
	case y == ty:
		return locales.MonthDay(t)
	}
	return locales.YearMonthDay(t)
}

// fullTime 按当前语言显示完整日期与时间
func fullTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return locales.FullTime(t)
}

func sizeText(n int64) string {
	switch {
	case n <= 0:
		return ""
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.0f KB", float64(n)/1024)
	case n < 1024*1024*1024:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
	return fmt.Sprintf("%.2f GB", float64(n)/(1024*1024*1024))
}

func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	h := strings.TrimPrefix(u.Host, "www.")
	if h == "" {
		return raw
	}
	return h
}

// snippet 按字符上限合并前三个非空行并折叠行内空白
func snippet(s string, n int) string {
	var out strings.Builder
	out.Grow(min(len(s), n*4))
	count, lines := 0, 1
	separator := ""
	for _, r := range s {
		if unicode.IsSpace(r) {
			if out.Len() > 0 {
				if r == '\r' || r == '\n' {
					separator = " · "
				} else if separator == "" {
					separator = " "
				}
			}
			continue
		}
		if separator == " · " {
			if lines == 3 {
				return out.String()
			}
			lines++
		}
		for _, space := range separator {
			if count == n {
				return out.String() + "…"
			}
			out.WriteRune(space)
			count++
		}
		separator = ""
		if count == n {
			return out.String() + "…"
		}
		out.WriteRune(r)
		count++
	}
	return out.String()
}

// cardBodyText 在字符上限内保留换行并统一回车格式
func cardBodyText(s string, n int) string {
	s = strings.TrimSpace(s)
	var out strings.Builder
	out.Grow(min(len(s), n*4))
	count, carriageReturn := 0, false
	for _, r := range s {
		if r == '\n' && carriageReturn {
			carriageReturn = false
			continue
		}
		carriageReturn = r == '\r'
		if carriageReturn {
			r = '\n'
		}
		if count == n {
			return strings.TrimRight(out.String(), " \n\t") + "…"
		}
		out.WriteRune(r)
		count++
	}
	return out.String()
}

// textPrefix 截取指定数量的字符并报告是否存在未显示内容
func textPrefix(s string, n int) (string, bool) {
	for i := range s {
		if n == 0 {
			return s[:i], true
		}
		n--
	}
	return s, false
}

// recordTitle 按记录类型返回卡片标题
func recordTitle(r *types.Record) string {
	switch r.Kind {
	case types.KindLink:
		if r.Title != "" {
			return r.Title
		}
		return r.URL
	case types.KindColor:
		return r.Color
	case types.KindFile:
		if recordFileCount(r) == 0 {
			return locales.FileKind()
		}
		if recordFileCount(r) == 1 {
			return r.Files[0].Name
		}
		return locales.FileGroupTitle(r.Files[0].Name, recordFileCount(r))
	case types.KindImage:
		if r.ImageW > 0 {
			return fmt.Sprintf("%d × %d", r.ImageW, r.ImageH)
		}
		return locales.ImageKind()
	}
	if s := snippet(r.Text, 90); s != "" {
		return s
	}
	return locales.EmptyText()
}

// prettyHotkey 按当前平台显示快捷键与常用按键符号
func prettyHotkey(acc string) string {
	if acc == "" {
		return locales.NotSet()
	}
	if !isDarwin {
		return strings.ReplaceAll(acc, "CmdOrCtrl+", "Ctrl+")
	}
	return strings.NewReplacer(
		"CmdOrCtrl+", "⌘", "Cmd+", "⌘", "Super+", "⌘",
		"Ctrl+", "⌃", "Alt+", "⌥", "Shift+", "⇧",
		"Enter", "⏎", "Space", "␣",
	).Replace(acc)
}

// recordFileCount 返回摘要或完整记录的文件数量
func recordFileCount(r *types.Record) int {
	if r.Summary {
		return r.FileCount
	}
	return len(r.Files)
}
