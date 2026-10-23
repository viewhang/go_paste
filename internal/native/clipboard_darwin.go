//go:build darwin

package native

import (
	"sync"

	"github.com/ebitengine/purego/objc"
)

// clipboardSourceState 按剪贴板变化计数保留应用切换前的复制来源
type clipboardSourceState struct {
	change int64
	active App
	source App
}

// observe 将新的剪贴板变化关联到当时的活动应用
func (s *clipboardSourceState) observe(change int64) {
	if change != s.change {
		s.change, s.source = change, s.active
	}
}

// activate 先保存切换前的复制来源再更新活动应用
func (s *clipboardSourceState) activate(change int64, app App) {
	s.observe(change)
	s.active = app
}

var (
	sourceOnce  sync.Once
	sourceMu    sync.Mutex
	sourceState clipboardSourceState
)

// ClipboardSource 在主线程返回剪贴板计数及复制时的活动应用
func ClipboardSource() (int64, App) {
	sourceOnce.Do(trackClipboardSource)
	sourceMu.Lock()
	defer sourceMu.Unlock()
	sourceState.observe(ChangeCount())
	return sourceState.change, sourceState.source
}

// trackClipboardSource 在应用激活与失活通知中保存切换前的剪贴板来源
func trackClipboardSource() {
	withPool(func() {
		workspace := send(class("NSWorkspace"), "sharedWorkspace")
		app := runningApp(send(workspace, "frontmostApplication"))
		sourceState = clipboardSourceState{change: ChangeCount(), active: app, source: app}
		center := send(workspace, "notificationCenter")
		for _, name := range []string{"NSWorkspaceDidDeactivateApplicationNotification", "NSWorkspaceDidActivateApplicationNotification"} {
			block := objc.NewBlock(func(_ objc.Block, notification objc.ID) {
				withPool(func() {
					sourceMu.Lock()
					defer sourceMu.Unlock()
					change := ChangeCount()
					if name == "NSWorkspaceDidActivateApplicationNotification" {
						key := nsString("NSWorkspaceApplicationKey")
						defer send(key, "release")
						cur := send(send(notification, "userInfo"), "objectForKey:", uintptr(key))
						sourceState.activate(change, runningApp(cur))
					} else {
						sourceState.observe(change)
					}
				})
			})
			key := nsString(name)
			send(center, "addObserverForName:object:queue:usingBlock:", uintptr(key), 0, 0, uintptr(block))
			send(key, "release")
			block.Release()
		}
	})
}
