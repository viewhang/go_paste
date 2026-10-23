//go:build !darwin && !windows

package native

import "errors"

// ChangeCount is not implemented on this platform.
func ChangeCount() int64 { return 0 }

// Frontmost is not implemented on this platform.
func Frontmost() App { return App{Name: "Desktop"} }

// AppIdentifier 报告当前平台不支持读取应用标识
func AppIdentifier(path string) (string, error) {
	return "", errors.New("application identifiers are not supported on this platform")
}

// AccessibilityTrusted is always true where synthetic input is unrestricted.
func AccessibilityTrusted() bool { return true }

// PromptAccessibility does nothing here.
func PromptAccessibility() {}

// OpenAccessibilitySettings does nothing here.
func OpenAccessibilitySettings() {}

// PasteKeystroke is not implemented on this platform.
func PasteKeystroke() error { return errors.New("automatic paste is not supported on this platform") }
