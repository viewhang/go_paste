//go:build windows

package native

import (
	"errors"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	processQueryLimitedInformation = 0x1000
	vkControl                      = 0x11
	vkV                            = 0x56
	keyEventKeyUp                  = 0x0002
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procGetClipboardSequenceNumber = user32.NewProc("GetClipboardSequenceNumber")
	procGetClipboardOwner          = user32.NewProc("GetClipboardOwner")
	procGetWindowThreadProcessID   = user32.NewProc("GetWindowThreadProcessId")
	procKeybdEvent                 = user32.NewProc("keybd_event")
	procOpenProcess                = kernel32.NewProc("OpenProcess")
	procQueryFullProcessImageNameW = kernel32.NewProc("QueryFullProcessImageNameW")
	procCloseHandle                = kernel32.NewProc("CloseHandle")
)

// ChangeCount returns the clipboard sequence number, which changes every
// time the clipboard is updated.
func ChangeCount() int64 {
	n, _, _ := procGetClipboardSequenceNumber.Call()
	return int64(n)
}

// ClipboardSource 返回剪贴板变化计数及其所属窗口的应用信息
func ClipboardSource() (int64, App) {
	change := ChangeCount()
	hwnd, _, _ := procGetClipboardOwner.Call()
	if hwnd == 0 {
		return change, App{Name: "Windows"}
	}
	var pid uint32
	procGetWindowThreadProcessID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	app := App{Pid: int(pid)}

	h, _, _ := procOpenProcess.Call(processQueryLimitedInformation, 0, uintptr(pid))
	if h == 0 {
		return change, app
	}
	defer procCloseHandle.Call(h)

	buf := make([]uint16, syscall.MAX_LONG_PATH)
	size := uint32(len(buf))
	ok, _, _ := procQueryFullProcessImageNameW.Call(h, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if ok == 0 {
		return change, app
	}
	exe := syscall.UTF16ToString(buf[:size])
	app.Path = exe
	base := filepath.Base(exe)
	app.Name = strings.TrimSuffix(base, filepath.Ext(base))
	app.ID = strings.ToLower(base)
	return change, app
}

// AppIdentifier 以所选 Windows 可执行文件名作为应用标识
func AppIdentifier(path string) (string, error) {
	return strings.ToLower(filepath.Base(path)), nil
}

// AccessibilityTrusted always reports true on Windows: synthetic input
// needs no permission.
func AccessibilityTrusted() bool { return true }

// PromptAccessibility does nothing on Windows.
func PromptAccessibility() {}

// PasteKeystroke posts Ctrl-V to the foreground window.
func PasteKeystroke() error {
	procKeybdEvent.Call(vkControl, 0, 0, 0)
	procKeybdEvent.Call(vkV, 0, 0, 0)
	time.Sleep(8 * time.Millisecond)
	procKeybdEvent.Call(vkV, 0, keyEventKeyUp, 0)
	procKeybdEvent.Call(vkControl, 0, keyEventKeyUp, 0)
	if procKeybdEvent.Find() != nil {
		return errors.New("user32.dll is unavailable")
	}
	return nil
}
