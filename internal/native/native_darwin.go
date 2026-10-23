//go:build darwin

package native

import (
	"errors"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

const (
	nsUTF8StringEncoding = 4

	// CGEventFlags(kCGEventFlagMaskCommand)
	cgFlagCommand = 1 << 20
	// kCGHIDEventTap
	cgHIDEventTap = 0
	// kVK_ANSI_V
	keyCodeV = 9
)

var (
	libOnce     sync.Once
	libObjC     uintptr
	libAppKit   uintptr
	libCarbon   uintptr
	libServices uintptr
	libCG       uintptr
	libCF       uintptr

	msgSend                    uintptr
	poolPushAddr               uintptr
	poolPopAddr                uintptr
	cgEventCreateKeyboardEvent func(source uintptr, key uint16, down bool) uintptr
	cgEventSetFlags            func(event uintptr, flags uint64)
	cgEventPost                func(tap uint32, event uintptr)
	axIsProcessTrusted         func() bool
	axIsProcessTrustedOpts     func(options uintptr) bool

	selMu    sync.RWMutex
	selCache = map[string]objc.SEL{}
	clsCache = map[string]objc.ID{}
)

func mustDlopen(path string) uintptr {
	h, err := purego.Dlopen(path, purego.RTLD_GLOBAL|purego.RTLD_NOW)
	if err != nil {
		panic("native: " + err.Error())
	}
	return h
}

func mustDlsym(lib uintptr, name string) uintptr {
	p, err := purego.Dlsym(lib, name)
	if err != nil {
		panic("native: " + err.Error())
	}
	return p
}

func load() {
	libOnce.Do(func() {
		libObjC = mustDlopen("/usr/lib/libobjc.A.dylib")
		libAppKit = mustDlopen("/System/Library/Frameworks/AppKit.framework/AppKit")
		libCarbon = mustDlopen("/System/Library/Frameworks/Carbon.framework/Carbon")
		libServices = mustDlopen("/System/Library/Frameworks/ApplicationServices.framework/ApplicationServices")
		libCG = mustDlopen("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics")
		libCF = mustDlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation")
		mustDlopen("/System/Library/Frameworks/UniformTypeIdentifiers.framework/UniformTypeIdentifiers")

		msgSend = mustDlsym(libObjC, "objc_msgSend")
		poolPushAddr = mustDlsym(libObjC, "objc_autoreleasePoolPush")
		poolPopAddr = mustDlsym(libObjC, "objc_autoreleasePoolPop")

		purego.RegisterLibFunc(&cgEventCreateKeyboardEvent, libCG, "CGEventCreateKeyboardEvent")
		purego.RegisterLibFunc(&cgEventSetFlags, libCG, "CGEventSetFlags")
		purego.RegisterLibFunc(&cgEventPost, libCG, "CGEventPost")
		purego.RegisterLibFunc(&axIsProcessTrusted, libServices, "AXIsProcessTrusted")
		purego.RegisterLibFunc(&axIsProcessTrustedOpts, libServices, "AXIsProcessTrustedWithOptions")
	})
}

func sel(name string) objc.SEL {
	selMu.RLock()
	s, ok := selCache[name]
	selMu.RUnlock()
	if ok {
		return s
	}
	s = objc.RegisterName(name)
	selMu.Lock()
	selCache[name] = s
	selMu.Unlock()
	return s
}

func class(name string) objc.ID {
	selMu.RLock()
	c, ok := clsCache[name]
	selMu.RUnlock()
	if ok {
		return c
	}
	c = objc.ID(objc.GetClass(name))
	if c != 0 {
		selMu.Lock()
		clsCache[name] = c
		selMu.Unlock()
	}
	return c
}

//go:uintptrescapes
func send(obj objc.ID, selector string, args ...uintptr) objc.ID {
	var a [10]uintptr
	a[0], a[1] = uintptr(obj), uintptr(sel(selector))
	n := copy(a[2:], args)
	r, _, _ := purego.SyscallN(msgSend, a[:n+2]...)
	return objc.ID(r)
}

func sendInt(obj objc.ID, selector string, args ...uintptr) int {
	return int(send(obj, selector, args...))
}

func withPool(fn func()) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	load()
	pool, _, _ := purego.SyscallN(poolPushAddr)
	defer purego.SyscallN(poolPopAddr, pool)
	fn()
}

func goString(str objc.ID) string {
	if str == 0 {
		return ""
	}
	p := send(str, "UTF8String")
	if p == 0 {
		return ""
	}
	n := sendInt(str, "lengthOfBytesUsingEncoding:", nsUTF8StringEncoding)
	if n <= 0 {
		return ""
	}
	return string(unsafe.Slice(*(**byte)(unsafe.Pointer(&p)), n))
}

func nsString(s string) objc.ID {
	if s == "" {
		return send(send(class("NSString"), "alloc"), "init")
	}
	b := unsafe.StringData(s)
	str := send(send(class("NSString"), "alloc"), "initWithBytes:length:encoding:",
		uintptr(unsafe.Pointer(b)), uintptr(len(s)), nsUTF8StringEncoding)
	runtime.KeepAlive(s)
	return str
}

// ChangeCount returns the pasteboard's change counter, which increases
// every time its contents are replaced.
func ChangeCount() int64 {
	load()
	var n int64
	withPool(func() {
		pb := send(class("NSPasteboard"), "generalPasteboard")
		if pb == 0 {
			return
		}
		n = int64(sendInt(pb, "changeCount"))
	})
	return n
}

// Frontmost returns the application that is frontmost right now, which is
// the one the user copied from.
func Frontmost() App {
	load()
	var app App
	withPool(func() {
		ws := send(class("NSWorkspace"), "sharedWorkspace")
		if ws == 0 {
			return
		}
		cur := send(ws, "frontmostApplication")
		if cur == 0 {
			return
		}
		app.Name = goString(send(cur, "localizedName"))
		app.ID = goString(send(cur, "bundleIdentifier"))
		app.Pid = sendInt(cur, "processIdentifier")
		if u := send(cur, "bundleURL"); u != 0 {
			app.Path = goString(send(u, "path"))
		}
		me := send(class("NSBundle"), "mainBundle")
		if me != 0 {
			app.Self = app.ID != "" && app.ID == goString(send(me, "bundleIdentifier"))
		}
	})
	if app.Name == "" {
		app.Name = "macOS"
	}
	return app
}

// ChooseApps 在主线程打开仅允许选择 macOS 应用包的多选窗口
func ChooseApps(title string) []string {
	var paths []string
	withPool(func() {
		panel := send(class("NSOpenPanel"), "openPanel")
		label := nsString(title)
		defer send(label, "release")
		send(panel, "setTitle:", uintptr(label))
		identifier := nsString("com.apple.application-bundle")
		defer send(identifier, "release")
		appType := send(class("UTType"), "typeWithIdentifier:", uintptr(identifier))
		types := send(class("NSArray"), "arrayWithObject:", uintptr(appType))
		send(panel, "setAllowedContentTypes:", uintptr(types))
		send(panel, "setTreatsFilePackagesAsDirectories:", 0)
		send(panel, "setCanChooseDirectories:", 0)
		send(panel, "setCanChooseFiles:", 1)
		send(panel, "setAllowsMultipleSelection:", 1)
		dir := nsString("/Applications")
		defer send(dir, "release")
		url := send(class("NSURL"), "fileURLWithPath:", uintptr(dir))
		send(panel, "setDirectoryURL:", uintptr(url))
		if sendInt(panel, "runModal") != 1 {
			return
		}
		urls := send(panel, "URLs")
		for i, n := 0, sendInt(urls, "count"); i < n; i++ {
			url := send(urls, "objectAtIndex:", uintptr(i))
			paths = append(paths, goString(send(url, "path")))
		}
	})
	return paths
}

// AppIdentifier 读取所选 macOS 应用包的标识
func AppIdentifier(path string) (string, error) {
	var identifier string
	withPool(func() {
		str := nsString(path)
		defer send(str, "release")
		bundle := send(class("NSBundle"), "bundleWithPath:", uintptr(str))
		identifier = goString(send(bundle, "bundleIdentifier"))
	})
	if identifier == "" {
		return "", errors.New("native: application bundle has no identifier")
	}
	return identifier, nil
}

// AccessibilityTrusted reports whether the app may post synthetic key
// events, which needs the Accessibility permission.
func AccessibilityTrusted() bool {
	load()
	if axIsProcessTrusted == nil {
		return false
	}
	return axIsProcessTrusted()
}

// PromptAccessibility asks the system to show the Accessibility prompt and
// add the app to the list in System Settings.
func PromptAccessibility() {
	load()
	if axIsProcessTrustedOpts == nil {
		return
	}
	withPool(func() {
		keyAddr := mustDlsym(libServices, "kAXTrustedCheckOptionPrompt")
		key := *(*uintptr)(unsafe.Pointer(keyAddr))
		trueAddr := mustDlsym(libCF, "kCFBooleanTrue")
		val := *(*uintptr)(unsafe.Pointer(trueAddr))
		if key == 0 || val == 0 {
			return
		}
		dict := send(class("NSDictionary"), "dictionaryWithObject:forKey:", val, key)
		if dict == 0 {
			return
		}
		axIsProcessTrustedOpts(uintptr(dict))
	})
}

// OpenAccessibilitySettings opens the Accessibility pane of System Settings.
func OpenAccessibilitySettings() {
	load()
	withPool(func() {
		url := nsString("x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility")
		send(class("NSWorkspace"), "openURL:", uintptr(url))
	})
}

// PasteKeystroke posts Command-V to the frontmost application.
func PasteKeystroke() error {
	if !AccessibilityTrusted() {
		return errors.New("accessibility permission is required to paste automatically")
	}
	load()
	down := cgEventCreateKeyboardEvent(0, keyCodeV, true)
	if down == 0 {
		return errors.New("could not create the paste event")
	}
	cgEventSetFlags(down, cgFlagCommand)
	cgEventPost(cgHIDEventTap, down)
	up := cgEventCreateKeyboardEvent(0, keyCodeV, false)
	if up != 0 {
		cgEventSetFlags(up, cgFlagCommand)
		cgEventPost(cgHIDEventTap, up)
	}
	return nil
}
