// Package clip watches the system clipboard and turns what the user copies
// into records.
package clip

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"

	"gopaste/internal/detect"
	"gopaste/internal/native"
	"gopaste/internal/store"
	"gopaste/internal/types"
)

const pollInterval = 300 * time.Millisecond

// Watcher polls the clipboard's change counter and captures new content.
type Watcher struct {
	st       *store.Store
	onChange func(*types.Record)
	onError  func(error)
	isPaused func() bool

	mu            sync.Mutex
	last          int64
	suppressUntil time.Time
	icons         map[string][]byte
	started       bool
}

// New 创建剪贴板采集器并在轮询线程回调记录变化或保存错误
func New(st *store.Store, onChange func(*types.Record), isPaused func() bool, onError func(error)) *Watcher {
	return &Watcher{st: st, onChange: onChange, isPaused: isPaused, onError: onError, icons: map[string][]byte{}}
}

// Start begins polling. It takes the current clipboard as the baseline, so
// content copied before launch is not recorded.
func (w *Watcher) Start() {
	w.mu.Lock()
	if w.started {
		w.mu.Unlock()
		return
	}
	w.started = true
	w.last = native.ChangeCount()
	w.mu.Unlock()

	go func() {
		t := time.NewTicker(pollInterval)
		defer t.Stop()
		for range t.C {
			w.poll()
		}
	}()
}

// Suppress ignores clipboard changes for d, for writes the app makes itself.
func (w *Watcher) Suppress(d time.Duration) {
	w.mu.Lock()
	w.suppressUntil = time.Now().Add(d)
	w.last = native.ChangeCount()
	w.mu.Unlock()
}

// Resync makes the current clipboard the baseline.
func (w *Watcher) Resync() {
	w.mu.Lock()
	w.last = native.ChangeCount()
	w.mu.Unlock()
}

// poll 检测剪贴板变化并回调采集结果或保存错误
func (w *Watcher) poll() {
	if w.isPaused != nil && w.isPaused() {
		w.Resync()
		return
	}
	change := native.ChangeCount()
	w.mu.Lock()
	same := change == w.last
	suppressed := time.Now().Before(w.suppressUntil)
	if !same {
		w.last = change
	}
	w.mu.Unlock()
	if same || suppressed {
		return
	}
	rec, err := w.Capture()
	if err != nil {
		if w.onError != nil {
			w.onError(err)
		}
		return
	}
	if rec == nil {
		return
	}
	if w.onChange != nil {
		w.onChange(rec)
	}
}

// Capture 应用排除规则并在一次存储提交中保存剪贴板内容与来源图标
func (w *Watcher) Capture() (*types.Record, error) {
	settings := w.st.Settings()
	app := native.Frontmost()
	if excludedApp(app.ID, settings.ExcludedApps) {
		return nil, nil
	}
	if settings.ExcludeConfidential && confidentialContent(mygo.Clipboard.Formats()) {
		return nil, nil
	}
	rec, image, err := w.build()
	if err != nil {
		return nil, err
	}
	if rec == nil || excludedContent(rec, settings) {
		return nil, nil
	}
	icon := w.fillSource(rec, app)
	saved, _, err := w.st.Capture(rec, image, icon)
	return saved, err
}

// excludedContent 按空白文本规则与已启用的图片大小上限判断是否跳过记录
func excludedContent(rec *types.Record, settings types.Settings) bool {
	switch rec.Kind {
	case types.KindImage:
		return settings.ImageLimitMiB > 0 && rec.Bytes > int64(settings.ImageLimitMiB)<<20
	case types.KindText:
		return settings.ExcludeWhitespace && strings.TrimSpace(rec.Text) == "" && detect.HTMLText(rec.HTML) == ""
	}
	return false
}

// confidentialContent 判断剪贴板是否包含 macOS 或 Windows 的机密内容标记
func confidentialContent(formats []transfer.Format) bool {
	return containsFormat(formats, "org.nspasteboard.ConcealedType") ||
		containsFormat(formats, "ExcludeClipboardContentFromMonitorProcessing")
}

// excludedApp 按应用标识判断来源是否位于用户的排除列表
func excludedApp(appID string, apps []types.ExcludedApp) bool {
	for _, app := range apps {
		if strings.EqualFold(appID, app.ID) {
			return true
		}
	}
	return false
}

// build 读取剪贴板内容并暂存原始图片字节而不提前落盘
func (w *Watcher) build() (*types.Record, []byte, error) {
	// Files first: copying files in Finder or Explorer also offers their
	// names as text, and an image preview on some sources.
	if paths, err := mygo.Clipboard.ReadFiles(); err == nil && len(paths) > 0 {
		return w.buildFiles(paths), nil, nil
	}

	formats := mygo.Clipboard.Formats()
	hasPNG := containsFormat(formats, transfer.PNG)
	hasHTML := containsFormat(formats, transfer.HTML)
	hasText := containsFormat(formats, transfer.Text) || containsFormat(formats, transfer.Markdown)

	if !hasPNG && !hasHTML && !hasText {
		return nil, nil, nil
	}

	var htmlStr, text string
	if hasHTML {
		if b, err := mygo.Clipboard.ReadFormat(transfer.HTML); err == nil {
			htmlStr = string(b)
		}
	}
	if hasText {
		if b, err := mygo.Clipboard.ReadFormat(transfer.Text); err == nil {
			text = string(b)
		}
	}
	if text == "" && htmlStr != "" {
		text = detect.HTMLText(htmlStr)
	}

	// An image wins only when the payload is not real formatted text: a
	// spreadsheet range or a rich document also offers a bitmap.
	if hasPNG {
		png, err := mygo.Clipboard.ReadFormat(transfer.PNG)
		if err == nil && len(png) > 0 && !detect.HTMLHasText(htmlStr) && len([]rune(strings.TrimSpace(text))) < 200 {
			rec := &types.Record{Kind: types.KindImage, Text: ""}
			format, wpx, hpx, animated := detect.ImageInfo(png)
			rec.ImageFormat = format
			rec.ImageW, rec.ImageH = wpx, hpx
			rec.Animated = animated
			rec.Bytes = int64(len(png))
			return rec, png, nil
		}
	}

	trimmed := strings.TrimSpace(text)
	if text == "" && htmlStr == "" {
		return nil, nil, nil
	}
	rec := &types.Record{Text: text, HTML: htmlStr, Bytes: int64(len(text) + len(htmlStr))}

	if len([]rune(trimmed)) <= 4096 && !strings.ContainsAny(trimmed, "\n\t") {
		if u, ok := detect.URL(trimmed); ok {
			rec.Kind = types.KindLink
			rec.URL = u
			rec.Text = trimmed
			rec.HTML = ""
			return rec, nil, nil
		}
		if c, ok := detect.Color(trimmed); ok {
			rec.Kind = types.KindColor
			rec.Color = c
			rec.Text = trimmed
			rec.HTML = ""
			return rec, nil, nil
		}
	}

	rec.Kind = types.KindText
	return rec, nil, nil
}

// buildFiles 读取文件元数据并保留原始路径引用
func (w *Watcher) buildFiles(paths []string) *types.Record {
	rec := &types.Record{Kind: types.KindFile}
	var total int64
	for _, p := range paths {
		ref := types.FileRef{Path: p, Name: filepath.Base(p)}
		if info, err := os.Stat(p); err == nil {
			ref.Dir = info.IsDir()
			ref.Size = info.Size()
			total += info.Size()
		} else {
			ref.Missing = true
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext == ".gif" {
			rec.Animated = true
		}
		rec.Files = append(rec.Files, ref)
	}
	rec.Bytes = total
	return rec
}

// fillSource 填写来源信息并缓存可随记录重新落盘的原始图标
func (w *Watcher) fillSource(rec *types.Record, app native.App) []byte {
	rec.Source = types.Source{AppName: app.Name, AppID: app.ID}
	if app.Self {
		rec.Source.AppName = "GoPaste"
	}
	if app.ID == "" {
		return nil
	}
	w.mu.Lock()
	icon := w.icons[app.ID]
	w.mu.Unlock()
	if len(icon) > 0 || app.Path == "" {
		return icon
	}
	png, err := mygo.Shell.FileIcon(app.Path, 64)
	if err != nil || len(png) == 0 {
		return nil
	}
	w.mu.Lock()
	w.icons[app.ID] = png
	w.mu.Unlock()
	return png
}

func containsFormat(list []transfer.Format, f transfer.Format) bool {
	for _, g := range list {
		if g == f {
			return true
		}
	}
	return false
}
