// Package clip watches the system clipboard and turns what the user copies
// into records.
package clip

import (
	"errors"
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

// pollInterval 控制剪贴板变化的检测间隔
const pollInterval = 300 * time.Millisecond

// Watcher 轮询剪贴板变化并保存通过排除规则的新内容
type Watcher struct {
	st       *store.Store
	onChange func(*types.Record)
	onError  func(error)
	isPaused func() bool

	mu      sync.Mutex
	last    int64
	icons   map[string][]byte
	started bool
}

// New 创建剪贴板采集器并在轮询线程回调记录变化或保存错误
func New(st *store.Store, onChange func(*types.Record), isPaused func() bool, onError func(error)) *Watcher {
	return &Watcher{st: st, onChange: onChange, isPaused: isPaused, onError: onError, icons: map[string][]byte{}}
}

// Start 初始化来源跟踪并仅采集启动后的剪贴板变化
func (w *Watcher) Start() {
	w.mu.Lock()
	if w.started {
		w.mu.Unlock()
		return
	}
	w.started = true
	w.last, _ = native.ClipboardSource()
	w.mu.Unlock()

	go func() {
		t := time.NewTicker(pollInterval)
		defer t.Stop()
		for range t.C {
			w.poll()
		}
	}()
}

// Resync 将当前剪贴板变化计数设为采集基线
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
	if !same {
		w.last = change
	}
	w.mu.Unlock()
	if same {
		return
	}
	rec, err := w.captureClipboard(change)
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

// captureClipboard 在主线程读取同一次剪贴板变化的来源与内容快照
func (w *Watcher) captureClipboard(change int64) (*types.Record, error) {
	var data transfer.Data
	var app native.App
	var err error
	mygo.RunOnMain(func() {
		current, source := native.ClipboardSource()
		if current != change {
			return
		}
		app = source
		formats := mygo.Clipboard.Formats()
		if excludedClipboard(formats, app, w.st.Settings()) {
			return
		}
		if containsFormat(formats, transfer.FileList) {
			data, err = mygo.Clipboard.Read(transfer.FileList, transfer.URIList)
		} else {
			data, err = mygo.Clipboard.Read(transfer.URIList, transfer.Text, transfer.HTML, transfer.Markdown, transfer.PNG,
				ownClipboardFormat, "org.nspasteboard.ConcealedType", "ExcludeClipboardContentFromMonitorProcessing")
		}
		if native.ChangeCount() != change {
			data, err = transfer.Data{}, nil
		}
	})
	if errors.Is(err, mygo.ErrClipboardChanged) || errors.Is(err, transfer.ErrFormat) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return w.capture(data, app)
}

// capture 应用排除规则并一次保存内容快照与来源图标
func (w *Watcher) capture(data transfer.Data, app native.App) (*types.Record, error) {
	settings := w.st.Settings()
	if excludedClipboard(data.Formats(), app, settings) {
		return nil, nil
	}
	rec, image, err := w.build(data)
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

// excludedClipboard 判断本次内容是否来自自身写入、排除应用或已启用的机密规则
func excludedClipboard(formats []transfer.Format, app native.App, settings types.Settings) bool {
	return containsFormat(formats, ownClipboardFormat) || excludedApp(app.ID, settings.ExcludedApps) ||
		settings.ExcludeConfidential && confidentialContent(formats)
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

// build 从内容快照生成记录并保留原始文本、HTML 与图片字节
func (w *Watcher) build(data transfer.Data) (*types.Record, []byte, error) {
	// Files first: copying files in Finder or Explorer also offers their
	// names as text, and an image preview on some sources.
	if paths, err := data.Files(); err == nil && len(paths) > 0 {
		return w.buildFiles(paths), nil, nil
	}

	formats := data.Formats()
	hasPNG := containsFormat(formats, transfer.PNG)
	hasHTML := containsFormat(formats, transfer.HTML)
	hasText := containsFormat(formats, transfer.Text) || containsFormat(formats, transfer.Markdown)

	if !hasPNG && !hasHTML && !hasText {
		return nil, nil, nil
	}

	var htmlStr, text string
	if hasHTML {
		if b, err := data.Read(transfer.HTML); err == nil {
			htmlStr = string(b)
		}
	}
	if hasText {
		if b, err := data.Read(transfer.Text); err == nil {
			text = string(b)
		}
	}
	if text == "" && htmlStr != "" {
		text = detect.HTMLText(htmlStr)
	}

	// An image wins only when the payload is not real formatted text: a
	// spreadsheet range or a rich document also offers a bitmap.
	if hasPNG {
		png, err := data.Read(transfer.PNG)
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
			return rec, nil, nil
		}
		if c, ok := detect.Color(trimmed); ok {
			rec.Kind = types.KindColor
			rec.Color = c
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
