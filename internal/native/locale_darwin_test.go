//go:build darwin

package native

import (
	"testing"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

// TestLocaleOverride 验证应用明确选择的语言与跟随系统可以独立读取
func TestLocaleOverride(t *testing.T) {
	loadLocalePreferences()
	var setValue func(uintptr, uintptr, uintptr, uintptr, uintptr)
	purego.RegisterLibFunc(&setValue, libCF, "CFPreferencesSetValue")
	withPool(func() {
		key := nsString("AppleLanguages")
		defer send(key, "release")
		app, user, host := preferencesScopes[0], preferencesScopes[1], preferencesScopes[2]
		previous := preferencesCopyValue(uintptr(key), app, user, host)
		defer func() {
			setValue(uintptr(key), previous, app, user, host)
			send(objc.ID(previous), "release")
		}()
		for _, tag := range []string{"", "zh-Hans", "en", ""} {
			var value uintptr
			if tag != "" {
				str := nsString(tag)
				value = uintptr(send(class("NSArray"), "arrayWithObject:", uintptr(str)))
				send(str, "release")
			}
			setValue(uintptr(key), value, app, user, host)
			if got := LocaleOverride(); got != tag {
				t.Fatalf("应用语言偏好错误：得到 %q，期望 %q", got, tag)
			}
		}
	})
}
