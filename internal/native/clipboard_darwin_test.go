//go:build darwin

package native

import "testing"

// TestClipboardSourceAcrossAppSwitch 验证复制后立即切换应用仍保留原来源且后续复制使用新来源
func TestClipboardSourceAcrossAppSwitch(t *testing.T) {
	private := App{ID: "com.example.private"}
	editor := App{ID: "com.example.editor"}
	browser := App{ID: "com.example.browser"}
	s := clipboardSourceState{change: 1, active: private}
	s.observe(2)
	s.activate(2, editor)
	s.activate(2, browser)
	s.observe(2)
	if s.source != private {
		t.Fatal("应用切换覆盖了复制来源")
	}
	s.observe(3)
	if s.source != browser {
		t.Fatal("新复制内容未关联到新的活动应用")
	}
	s.activate(4, editor)
	s.observe(4)
	if s.source != browser {
		t.Fatal("轮询前切换应用导致来源错误")
	}
	s.observe(5)
	if s.source != editor {
		t.Fatal("切换后的新复制仍沿用旧来源")
	}
}

// TestClipboardSourceNotifications 验证原生应用切换通知正确调用来源跟踪且不改写系统剪贴板
func TestClipboardSourceNotifications(t *testing.T) {
	ClipboardSource()
	private := App{ID: "com.example.private"}
	sourceMu.Lock()
	previous := sourceState
	sourceState = clipboardSourceState{change: -1, active: private}
	sourceMu.Unlock()
	t.Cleanup(func() {
		sourceMu.Lock()
		sourceState = previous
		sourceMu.Unlock()
	})
	withPool(func() {
		workspace := send(class("NSWorkspace"), "sharedWorkspace")
		center := send(workspace, "notificationCenter")
		cur := send(class("NSRunningApplication"), "currentApplication")
		if cur == 0 {
			t.Fatal("无法创建测试进程的应用信息")
		}
		active := runningApp(cur)
		key := nsString("NSWorkspaceApplicationKey")
		defer send(key, "release")
		info := send(class("NSDictionary"), "dictionaryWithObject:forKey:", uintptr(cur), uintptr(key))
		for _, name := range []string{"NSWorkspaceDidDeactivateApplicationNotification", "NSWorkspaceDidActivateApplicationNotification"} {
			notification := nsString(name)
			send(center, "postNotificationName:object:userInfo:", uintptr(notification), 0, uintptr(info))
			send(notification, "release")
		}
		sourceMu.Lock()
		defer sourceMu.Unlock()
		if sourceState.source != private || sourceState.active != active {
			t.Fatal("原生切换通知未保留旧来源或未更新活动应用")
		}
	})
}
