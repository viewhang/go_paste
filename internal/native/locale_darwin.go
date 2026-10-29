//go:build darwin

package native

import (
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

// localeOnce 初始化应用语言偏好的读取接口
var localeOnce sync.Once

var (
	preferencesCopyValue func(uintptr, uintptr, uintptr, uintptr) uintptr
	preferencesScopes    [3]uintptr
)

// loadLocalePreferences 注册当前应用及用户的语言偏好接口
func loadLocalePreferences() {
	localeOnce.Do(func() {
		load()
		purego.RegisterLibFunc(&preferencesCopyValue, libCF, "CFPreferencesCopyValue")
		for i, name := range []string{"kCFPreferencesCurrentApplication", "kCFPreferencesCurrentUser", "kCFPreferencesAnyHost"} {
			preferencesScopes[i] = *(*uintptr)(unsafe.Pointer(mustDlsym(libCF, name)))
		}
	})
}

// LocaleOverride 读取应用明确选择的语言，空值表示跟随系统
func LocaleOverride() (tag string) {
	loadLocalePreferences()
	withPool(func() {
		key := nsString("AppleLanguages")
		defer send(key, "release")
		languages := objc.ID(preferencesCopyValue(uintptr(key), preferencesScopes[0], preferencesScopes[1], preferencesScopes[2]))
		if languages == 0 {
			return
		}
		defer send(languages, "release")
		if sendInt(languages, "count") > 0 {
			tag = goString(send(languages, "firstObject"))
		}
	})
	return
}
