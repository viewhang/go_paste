package app

import (
	"errors"
	"fmt"
	"image"
	"log"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"gopaste/internal/clip"
	"gopaste/internal/locales"
	"gopaste/internal/store"
	"gopaste/internal/types"
)

// The panel has two forms: a strip along the bottom of the screen, as
// Paste's, and a panel beside the pointer.
const (
	formBar   = 0
	formPanel = 1
)

const (
	barHeight  = 238
	barPreview = 306
	cardW      = 164
	cardH      = 140
	cardGap    = 10

	panelWidth  = 432
	panelHeight = 596
	// panelRowHeight 设置浮动面板记录行的高度
	panelRowHeight = 58
	// panelRowGap 设置浮动面板记录行的间距
	panelRowGap = 3
	// panelListPadding 设置浮动面板记录列表的纵向内边距
	panelListPadding = 6
	// statusHeight 设置面板底部状态栏的高度
	statusHeight = 30
	// panelMinWidth 限制浮动面板的最小宽度
	panelMinWidth = 360
	// panelMinHeight 限制浮动面板的最小高度
	panelMinHeight = 320
)

// shellReady says the app's loop runs, so that the desktop APIs about
// files, icons and folders may be asked: before that, as in a test, they
// would wait for a main loop that is not there.
var shellReady bool

// App 管理应用状态、窗口和交互，界面状态通过主线程更新
type App struct {
	st     *store.Store
	watch  *clip.Watcher
	win    *mygo.Window
	setWin *mygo.Window
	tray   *mygo.Tray

	settings types.Settings
	records  []*types.Record

	query string
	tab   int
	sel   int

	preview bool
	detail  *types.Record

	form int
	// placing 标记程序正在设置窗口位置与尺寸
	placing bool

	search ui.Handle
	list   ui.ListState
	strip  ui.ScrollState
	// listScroll 保存浮动面板列表的滚动位置
	listScroll ui.ScrollState

	hotkey string

	images       bitmapCache
	previewImage cachedBitmap
	imageScale   float64
	previewSlots chan struct{}
	// dragImage 复用当前拖拽的紧凑预览像素缓冲
	dragImage *image.RGBA

	// pages 最多保留两个分页摘要，避免滚动后积累完整历史
	pages     map[int][]*types.Record
	pageOrder []int
	vis       []*types.Record
	acted     bool
	// filterValid 标记筛选结果是否仍对应当前记录与列表规则
	filterValid bool
	// filterQuery 保存最近一次筛选使用的搜索词
	filterQuery string
	// filterTab 保存最近一次筛选使用的列表位置
	filterTab int

	pending []func()

	prompt         *promptState
	recording      string
	shortcutError  string
	updatesEnabled bool
	// settingsPage 保存设置窗口当前选中的分类
	settingsPage string
	// installedApps 缓存设置窗口打开时的本机应用列表
	installedApps []mygo.InstalledApp
	// choosingExcludedApps 标记应用选择器正在打开以避免重复弹窗
	choosingExcludedApps bool
	hintFlash            string
	hintUntil            time.Time
	lastReload           time.Time
}

// NewApp 创建应用状态并限制链接预览的并发任务数
func NewApp() *App {
	return &App{
		imageScale:   1,
		previewSlots: make(chan struct{}, 2),
		settingsPage: "general",
	}
}

// ready 初始化应用资源、启动剪贴板记录并在手动启动时显示面板
func (a *App) ready() {
	a.updatesEnabled = mygo.Updater.Enabled()
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil || dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, "Library", "Application Support", "GoPaste")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		mygo.Dialog.Error(locales.CreateDataFailed(), err.Error())
		return
	}
	st, err := store.Open(dir)
	if err != nil {
		mygo.Dialog.Error(locales.ReadHistoryFailed(), err.Error())
		return
	}
	a.st = st
	a.reload()
	if a.settings.Form == formPanel {
		a.form = formPanel
	}
	shellReady = true

	a.win = mygo.NewWindow(mygo.WindowOptions{
		Title:             "GoPaste",
		Width:             panelWidth,
		Height:            panelHeight,
		Frameless:         !isWindows,
		TitleBarStyle:     mygo.TitleBarHidden,
		HideWindowButtons: true,
		Transparent:       true,
		Vibrancy:          mygo.VibrancyPopover,
		Panel:             true,
		AlwaysOnTop:       true,
		SkipTaskbar:       true,
		Hidden:            true,
		DisableMinimize:   true,
		DisableMaximize:   true,
		DisableFullScreen: true,
		DisableShadow:     false,
		Content:           ui.View(a.view),
	})
	offClose := a.win.OnClose(func(e *mygo.CloseEvent) {
		e.PreventDefault()
		a.hide()
	})
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { offClose() })
	a.win.OnBlur(func() {
		a.hide()
	})
	a.win.OnResize(a.resize)
	a.place()

	a.setUpTray()

	a.watch = clip.New(st, func(rec *types.Record) {
		if rec != nil && rec.Kind == types.KindLink && rec.URL != "" && rec.PreviewState == "" {
			go a.fetchPreview(rec.ID, rec.URL)
		}
		a.refresh()
	}, func() bool {
		return st.Settings().Paused
	}, func(err error) {
		a.win.Update(func() { a.storageError(err) })
	})
	a.watch.Start()

	a.scheduleLinkPreviews()
	a.applyHotkey()

	if !mygo.App.WasOpenedAtLogin() {
		a.show()
	}
}

func (a *App) shutdown() {
	if a.tray != nil {
		a.tray.Destroy()
	}
	if a.st != nil {
		_ = a.st.Close()
	}
}

// refresh rebuilds the panel from any goroutine.
func (a *App) refresh() {
	if a.win == nil {
		return
	}
	a.win.Update(func() { a.reload() })
}

// reload 在主线程读取存储的只读记录并更新筛选与预览状态
func (a *App) reload() {
	if a.st == nil {
		return
	}
	a.lastReload = time.Now()
	a.settings = a.st.Settings()
	var err error
	a.records, err = a.st.IDs(types.Tab{ID: "all"}, "")
	if err != nil {
		a.storageError(err)
	}
	a.filterValid = false
	a.vis = a.filtered()
	a.clampSel()
	if a.detail != nil {
		if r, ok := a.st.Get(a.detail.ID); ok {
			a.detail = r
		} else {
			a.detail = nil
			a.preview = false
			a.previewImage = cachedBitmap{}
		}
	}
}

func (a *App) clampSel() {
	if len(a.vis) == 0 {
		a.sel = 0
		return
	}
	if a.sel >= len(a.vis) {
		a.sel = len(a.vis) - 1
	}
	if a.sel < 0 {
		a.sel = 0
	}
}

// ---- tabs and filtering ----

// filtered 在记录、列表或搜索词变化时筛选记录并复用未变化的结果
func (a *App) filtered() []*types.Record {
	if a.filterValid && a.filterQuery == a.query && a.filterTab == a.tab {
		return a.vis
	}
	out, err := a.st.IDs(a.currentTab(), a.query)
	if err != nil {
		a.storageError(err)
	}
	a.pages = make(map[int][]*types.Record)
	a.pageOrder = nil
	a.vis = out
	a.filterValid, a.filterQuery, a.filterTab = true, a.query, a.tab
	return out
}

// selected 返回键盘选中的卡片摘要
func (a *App) selected() *types.Record {
	if a.sel >= 0 && a.sel < len(a.vis) {
		return a.recordAt(a.sel)
	}
	return nil
}

// recordAt 按五十条一页读取卡片摘要并淘汰较早访问的分页
func (a *App) recordAt(i int) *types.Record {
	// pageSize 设置每页摘要数量
	const pageSize = 50
	page := i / pageSize
	if records, ok := a.pages[page]; ok {
		a.pageOrder = append(slices.DeleteFunc(a.pageOrder, func(p int) bool { return p == page }), page)
		return records[i%pageSize]
	}
	start, end := page*pageSize, min((page+1)*pageSize, len(a.vis))
	ids := make([]string, end-start)
	for j := range ids {
		ids[j] = a.vis[start+j].ID
	}
	records, err := a.st.Summaries(ids)
	if err != nil {
		a.storageError(err)
		return a.vis[i]
	}
	byID := make(map[string]*types.Record, len(records))
	for _, r := range records {
		byID[r.ID] = r
	}
	ordered := make([]*types.Record, len(ids))
	for j, id := range ids {
		ordered[j] = byID[id]
		if ordered[j] == nil {
			ordered[j] = a.vis[start+j]
		}
	}
	if a.pages == nil {
		a.pages = make(map[int][]*types.Record)
	}
	a.pages[page] = ordered
	a.pageOrder = append(a.pageOrder, page)
	if len(a.pageOrder) > 2 {
		delete(a.pages, a.pageOrder[0])
		a.pageOrder = a.pageOrder[1:]
	}
	return ordered[i%pageSize]
}

// fullRecord 在预览、复制或拖出摘要时读取原始内容
func (a *App) fullRecord(r *types.Record) *types.Record {
	if r == nil || !r.Summary {
		return r
	}
	full, _ := a.st.Get(r.ID)
	return full
}

// ---- window geometry ----

// place 按当前形态和已保存尺寸将窗口放到鼠标所在屏幕
func (a *App) place() {
	if a.win == nil {
		return
	}
	pt := mygo.Screen.CursorScreenPoint()
	d := mygo.Screen.DisplayNearestPoint(pt)
	area := d.WorkArea
	if area.Width <= 0 {
		area = mygo.Screen.PrimaryDisplay().WorkArea
	}
	if area.Width <= 0 {
		return
	}
	a.placing = true
	defer func() { a.placing = false }()
	bounds := a.windowBounds(pt, area)
	if a.form == formBar {
		minHeight := barHeight
		if a.preview {
			minHeight += barPreview
		}
		a.win.SetMinimumSize(area.Width, min(minHeight, area.Height))
		a.win.SetMaximumSize(area.Width, area.Height)
	} else {
		a.win.SetMinimumSize(min(panelMinWidth, area.Width-20), min(panelMinHeight, area.Height-20))
		a.win.SetMaximumSize(area.Width-20, area.Height-20)
	}
	a.win.SetBounds(bounds)
}

// windowBounds 计算当前形态在屏幕工作区内的位置与尺寸
func (a *App) windowBounds(pt mygo.Point, area mygo.Rectangle) mygo.Rectangle {
	if a.form == formBar {
		h := max(a.settings.BarHeight, barHeight)
		if a.preview {
			h += barPreview
		}
		h = min(h, area.Height)
		return mygo.Rectangle{X: area.X, Y: area.Y + area.Height - h, Width: area.Width, Height: h}
	}
	w, h := a.settings.PanelWidth, a.settings.PanelHeight
	if w == 0 {
		w = panelWidth
	}
	if h == 0 {
		h = panelHeight
	}
	w = clampInt(w, min(panelMinWidth, area.Width-20), area.Width-20)
	h = clampInt(h, min(panelMinHeight, area.Height-20), area.Height-20)
	x := pt.X - w/2
	y := pt.Y + 18
	if y+h > area.Y+area.Height-12 {
		y = pt.Y - h - 18
	}
	x = clampInt(x, area.X+10, area.X+area.Width-w-10)
	y = clampInt(y, area.Y+10, area.Y+area.Height-h-10)
	return mygo.Rectangle{X: x, Y: y, Width: w, Height: h}
}

// resize 保存用户调整后的尺寸并保持长条底部贴合屏幕工作区
func (a *App) resize() {
	if a.placing {
		return
	}
	bounds := a.win.Bounds()
	if a.form == formBar {
		area := mygo.Screen.DisplayMatching(bounds).WorkArea
		y := area.Y + area.Height - bounds.Height
		if bounds.Y != y {
			a.win.SetPosition(bounds.X, y)
		}
		h := bounds.Height
		if a.preview {
			h -= barPreview
		}
		if h != a.settings.BarHeight {
			a.setSetting(func(s *types.Settings) { s.BarHeight = h })
		}
	} else if bounds.Width != a.settings.PanelWidth || bounds.Height != a.settings.PanelHeight {
		a.setSetting(func(s *types.Settings) {
			s.PanelWidth, s.PanelHeight = bounds.Width, bounds.Height
		})
	}
}

// clampInt 将整数限制在指定范围内
func clampInt(v, lo, hi int) int {
	return max(lo, min(v, hi))
}

// ---- showing and hiding ----

func (a *App) toggle() {
	if a.win == nil {
		return
	}
	if a.win.IsVisible() {
		a.hide()
		return
	}
	a.show()
}

// show 保留搜索内容并展示刷新后的剪贴板面板
func (a *App) show() {
	if a.win == nil {
		return
	}
	if a.win.IsVisible() && a.win.IsFocused() {
		a.hide()
		return
	}
	a.reload()
	a.preview = false
	a.detail = nil
	a.previewImage = cachedBitmap{}
	a.sel = 0
	a.place()
	a.win.Show()
	a.win.Focus()
	a.search.Focus()
	a.list.ScrollIntoView(0)
	a.showOnScreenHint()
}

func (a *App) showOnScreenHint() {
	// Give the panel a moment to find the pointers of the screen it was
	// placed on, then draw once more: a first frame can be a little off.
	if a.win != nil {
		a.win.Invalidate()
	}
}

// hide 隐藏剪贴板面板并释放应用持有的图片缓存
func (a *App) hide() {
	a.preview = false
	a.detail = nil
	a.releaseImages()
	if a.win != nil {
		a.win.Hide()
	}
}

// openPanel brings the panel up for the menu bar, without closing it when
// it is already there.
func (a *App) openPanel() {
	if a.win == nil {
		return
	}
	if a.win.IsVisible() {
		a.reload()
		a.place()
		a.win.Focus()
		a.search.Focus()
		return
	}
	a.show()
}

func (a *App) setForm(form int) {
	if form == a.form {
		return
	}
	a.form = form
	a.preview = false
	a.detail = nil
	a.previewImage = cachedBitmap{}
	a.setSetting(func(s *types.Settings) { s.Form = form })
	if a.win != nil {
		a.place()
		a.win.Invalidate()
	}
	a.refreshTray()
}

// ---- settings ----

// setSetting 保存偏好并应用快捷键、启动项与记录行为
func (a *App) setSetting(fn func(*types.Settings)) {
	prev := a.settings
	next := prev
	fn(&next)
	if next.MaxRecords <= 0 {
		next.MaxRecords = 600
	}
	if next.Hotkey == "" {
		next.Hotkey = types.DefaultHotkey
	}
	if next.Hotkey != prev.Hotkey {
		if err := a.registerHotkey(next.Hotkey); err != nil {
			a.shortcutError = err.Error()
			return
		}
	}
	a.settings = next
	a.filterValid = false
	if a.st != nil {
		err := a.st.SetSettings(next)
		a.settings = a.st.Settings()
		if a.settings.MaxRecords != prev.MaxRecords || a.settings.RetentionDays != prev.RetentionDays || a.settings.HistoryLimitMiB != prev.HistoryLimitMiB {
			a.reload()
		}
		if err != nil {
			a.storageError(err)
			return
		}
	}
	if next.LaunchAtLogin != prev.LaunchAtLogin {
		if err := mygo.App.SetOpenAtLogin(next.LaunchAtLogin); err != nil {
			log.Print("login item: ", err)
		}
	}
	if next.LinkPreview && !prev.LinkPreview {
		a.scheduleLinkPreviews()
	}
	if next.Paused != prev.Paused || next.Hotkey != prev.Hotkey {
		a.refreshTray()
	}
	if a.win != nil {
		a.win.Invalidate()
	}
	if a.setWin != nil {
		a.setWin.Invalidate()
	}
}

// applyHotkey 注册设置中的全局呼出快捷键
func (a *App) applyHotkey() error {
	return a.registerHotkey(a.hotkeyAccel())
}

// registerHotkey 注册新的呼出快捷键，成功后再释放原有按键
func (a *App) registerHotkey(acc string) error {
	if acc == a.hotkey && mygo.GlobalShortcut.IsRegistered(acc) {
		return nil
	}
	if err := mygo.GlobalShortcut.Register(acc, a.toggle); err != nil {
		return fmt.Errorf("%s", locales.ShortcutOccupied(prettyHotkey(acc)))
	}
	if a.hotkey != "" && a.hotkey != acc {
		mygo.GlobalShortcut.Unregister(a.hotkey)
	}
	a.hotkey = acc
	return nil
}

// ---- small helpers used by the views ----

// storageError 显示历史容量或文件保存失败的必要提示
func (a *App) storageError(err error) {
	if errors.Is(err, store.ErrHistoryLimit) {
		a.toast(locales.ImageStorageLimitReached())
		return
	}
	a.toast(locales.SaveHistoryFailed(err.Error()))
}

// toast 显示持续保留的错误与权限提示
func (a *App) toast(msg string) {
	a.hintFlash = msg
	a.hintUntil = time.Time{}
	if a.win != nil {
		a.win.Invalidate()
	}
}

// notify 显示三秒后自动消失的操作提示
func (a *App) notify(msg string) {
	a.toast(msg)
	a.hintUntil = time.Now().Add(3 * time.Second)
}

func (a *App) once() bool {
	if a.acted {
		return false
	}
	a.acted = true
	return true
}

// openSettings 显示设置窗口并在关闭后结束快捷键录制
func (a *App) openSettings() {
	if a.setWin == nil {
		a.setWin = mygo.NewWindow(mygo.WindowOptions{
			Title:           locales.SettingsTitle(),
			Width:           780,
			Height:          600,
			MinWidth:        720,
			MinHeight:       520,
			TitleBarStyle:   mygo.TitleBarHidden,
			Vibrancy:        mygo.VibrancyPopover,
			Content:         ui.View(a.settingsView),
			BackgroundColor: "",
		})
		a.setWin.OnClosed(func() {
			a.setWin = nil
			a.recording = ""
		})
	}
	a.setWin.Show()
	a.setWin.Focus()
	a.loadInstalledApps()
}

// hotkeyAccel 返回已注册、已保存或默认的全局呼出快捷键
func (a *App) hotkeyAccel() string {
	if a.hotkey != "" {
		return a.hotkey
	}
	if a.settings.Hotkey != "" {
		return a.settings.Hotkey
	}
	return types.DefaultHotkey
}
