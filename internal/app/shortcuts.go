package app

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"gopaste/internal/locales"
	"gopaste/internal/types"
)

// shortcutActions 定义可自定义的操作与默认快捷键
var shortcutActions = []struct {
	id    string
	title func() string
	accel string
}{
	{"toggle", locales.ShortcutToggle, types.DefaultHotkey},
	{"paste", locales.Paste, "Enter"},
	{"pasteAs", locales.PasteAs, "Shift+Enter"},
	{"preview", locales.Preview, "Space"},
	{"delete", locales.DeleteRecord, "Backspace"},
	{"search", locales.ShortcutSearch, "CmdOrCtrl+F"},
	{"previousList", locales.ShortcutPreviousList, "CmdOrCtrl+["},
	{"nextList", locales.ShortcutNextList, "CmdOrCtrl+]"},
	{"settings", locales.OpenSettings, "CmdOrCtrl+,"},
}

// shortcutAccel 返回操作的自定义按键或默认按键
func (a *App) shortcutAccel(id string) string {
	if id == "toggle" {
		return a.hotkeyAccel()
	}
	if acc := a.settings.Shortcuts[id]; acc != "" {
		return acc
	}
	for _, action := range shortcutActions {
		if action.id == id {
			return action.accel
		}
	}
	return ""
}

// shortcutKeys 解析操作快捷键供面板与右键菜单使用
func (a *App) shortcutKeys(id string) (ui.Modifiers, ui.Key) {
	return parseShortcut(a.shortcutAccel(id))
}

// shortcutPressed 检查当前帧是否按下指定操作的快捷键
func (a *App) shortcutPressed(c *ui.Context, id string) bool {
	return c.Shortcut(a.shortcutKeys(id))
}

// parseShortcut 将录制的快捷键与默认按键转换为界面按键
func parseShortcut(acc string) (ui.Modifiers, ui.Key) {
	parts := strings.Split(acc, "+")
	var mods ui.Modifiers
	for _, part := range parts[:len(parts)-1] {
		switch part {
		case "Ctrl":
			mods |= ui.Ctrl
		case "Alt":
			mods |= ui.Alt
		case "Shift":
			mods |= ui.Shift
		case "Super":
			mods |= ui.Super
		case "CmdOrCtrl":
			mods |= ui.Cmd
		}
	}
	for key, name := range keyNames {
		if strings.EqualFold(name, parts[len(parts)-1]) {
			return mods, key
		}
	}
	return mods, ui.KeyUnknown
}

// setShortcut 校验按键冲突并保存操作的自定义快捷键
func (a *App) setShortcut(id, acc string) error {
	mods, key := parseShortcut(acc)
	if id == "toggle" && mods&(ui.Ctrl|ui.Alt|ui.Super) == 0 {
		return errors.New(locales.ShortcutNeedsModifier())
	}
	if mods == 0 && key != ui.KeyEnter && key != ui.KeySpace && !(id == "delete" && key == ui.KeyBackspace) && !(key >= ui.KeyF1 && key <= ui.KeyF12) {
		return errors.New(locales.ShortcutNeedsCombination())
	}
	if mods == ui.Cmd && (key == ui.KeyA || key == ui.KeyX || key == ui.KeyV || key == ui.KeyZ || key == ui.KeyC) {
		return errors.New(locales.ShortcutTextEditing())
	}
	if mods == ui.Cmd && slices.Contains(quickPasteKeys[:], key) {
		return errors.New(locales.ShortcutConflict(locales.QuickPaste()))
	}
	for _, action := range shortcutActions {
		otherMods, otherKey := a.shortcutKeys(action.id)
		if action.id != id && mods == otherMods && key == otherKey {
			return fmt.Errorf("%s", locales.ShortcutConflict(action.title()))
		}
	}
	a.shortcutError = ""
	a.setSetting(func(s *types.Settings) {
		if id == "toggle" {
			s.Hotkey = acc
			return
		}
		s.Shortcuts = maps.Clone(s.Shortcuts)
		if s.Shortcuts == nil {
			s.Shortcuts = make(map[string]string)
		}
		s.Shortcuts[id] = acc
	})
	if a.shortcutError != "" {
		return fmt.Errorf("%s", a.shortcutError)
	}
	return nil
}

// settingsShortcuts 显示快捷键录制、冲突提示与恢复默认入口
func (a *App) settingsShortcuts(c *ui.Context, p palette) {
	ui.Column(c).Gap(8).Children(func() {
		a.card(c, p).Children(func() {
			for i, action := range shortcutActions {
				if i > 0 {
					ui.Divider(c)
				}
				desc := ""
				if action.id == "toggle" {
					desc = locales.ShortcutScopeHint()
				}
				settingRow(c, p, action.title(), desc, func() {
					a.shortcutRecorder(c, p, action.id, action.title())
				})
			}
			if a.shortcutError != "" {
				ui.Text(c, a.shortcutError).FontSize(11).TextColor(p.danger)
			}
			ui.Text(c, locales.ShortcutRecorderHint()).FontSize(11).TextColor(p.muted)
			if a.themeButton(c, locales.RestoreShortcuts()).Clicked() {
				a.recording, a.shortcutError = "", ""
				a.setSetting(func(s *types.Settings) {
					s.Hotkey, s.Shortcuts = types.DefaultHotkey, nil
				})
			}
		})
	})
}

// shortcutRecorder 录制指定操作的按键并显示校验结果
func (a *App) shortcutRecorder(c *ui.Context, p palette, id, title string) {
	recording := a.recording == id
	rec := ui.Box(c).Key(id).Height(28).MinWidth(110).Padding(0, 11).Radius(7).Center()
	rec.Label(locales.ShortcutLabel(title)).Focusable().Cursor(ui.CursorPointer).Border(1, p.border)
	background := p.surface
	if recording {
		rec.Border(1, c.Theme().Focus).AutoFocus()
		background = p.accentSoft
	} else if rec.Hovered() {
		background = p.hover
	}
	a.themeSurface(rec, background, true)
	rec.Children(func() {
		txt := prettyHotkey(a.shortcutAccel(id))
		if recording {
			txt = locales.RecordShortcut()
		}
		ui.Text(c, txt).FontSize(11.5).TextColor(p.text)
	})
	if rec.Clicked() {
		a.shortcutError = ""
		if recording {
			a.recording = ""
		} else {
			a.recording = id
			rec.Focus()
		}
	}
	if a.recording == id {
		rec.HandleInput(func(ev ui.InputEvent) bool {
			if ev.Kind != ui.InputKeyDown {
				return false
			}
			if ev.Key == ui.KeyEscape {
				a.recording = ""
				return true
			}
			if acc, ok := accelFrom(ev.Mods, ev.Key, ev.PhysicalKey); ok {
				if err := a.setShortcut(id, acc); err != nil {
					a.shortcutError = err.Error()
				} else {
					a.recording = ""
				}
			}
			return true
		})
	}
}

// accelFrom 将键盘事件转换为快捷键字符串
func accelFrom(mods ui.Modifiers, key, physical ui.Key) (string, bool) {
	name := keyName(key)
	if name == "" {
		name = keyName(physical)
	}
	if name == "" {
		return "", false
	}
	var b strings.Builder
	if mods&ui.Ctrl != 0 {
		b.WriteString("Ctrl+")
	}
	if mods&ui.Alt != 0 {
		b.WriteString("Alt+")
	}
	if mods&ui.Shift != 0 {
		b.WriteString("Shift+")
	}
	if mods&ui.Super != 0 {
		b.WriteString("Super+")
	}
	b.WriteString(name)
	return b.String(), true
}

// keyNames 保存快捷键录制与解析共用的按键名称
var keyNames = map[ui.Key]string{
	ui.KeyA: "A", ui.KeyB: "B", ui.KeyC: "C", ui.KeyD: "D", ui.KeyE: "E",
	ui.KeyF: "F", ui.KeyG: "G", ui.KeyH: "H", ui.KeyI: "I", ui.KeyJ: "J",
	ui.KeyK: "K", ui.KeyL: "L", ui.KeyM: "M", ui.KeyN: "N", ui.KeyO: "O",
	ui.KeyP: "P", ui.KeyQ: "Q", ui.KeyR: "R", ui.KeyS: "S", ui.KeyT: "T",
	ui.KeyU: "U", ui.KeyV: "V", ui.KeyW: "W", ui.KeyX: "X", ui.KeyY: "Y",
	ui.KeyZ: "Z",
	ui.Key0: "0", ui.Key1: "1", ui.Key2: "2", ui.Key3: "3", ui.Key4: "4",
	ui.Key5: "5", ui.Key6: "6", ui.Key7: "7", ui.Key8: "8", ui.Key9: "9",
	ui.KeyF1: "F1", ui.KeyF2: "F2", ui.KeyF3: "F3", ui.KeyF4: "F4",
	ui.KeyF5: "F5", ui.KeyF6: "F6", ui.KeyF7: "F7", ui.KeyF8: "F8",
	ui.KeyF9: "F9", ui.KeyF10: "F10", ui.KeyF11: "F11", ui.KeyF12: "F12",
	ui.KeySpace: "Space", ui.KeyEnter: "Enter", ui.KeyTab: "Tab", ui.KeyBackspace: "Backspace",
	ui.KeyLeft: "Left", ui.KeyRight: "Right", ui.KeyUp: "Up", ui.KeyDown: "Down",
	ui.KeyHome: "Home", ui.KeyEnd: "End", ui.KeyPageUp: "PageUp", ui.KeyPageDown: "PageDown",
	ui.KeyMinus: "-", ui.KeyEqual: "=", ui.KeyComma: ",", ui.KeyPeriod: ".",
	ui.KeySlash: "/", ui.KeySemicolon: ";", ui.KeyQuote: "'",
	ui.KeyBracketLeft: "[", ui.KeyBracketRight: "]", ui.KeyBackslash: "\\",
	ui.KeyBackquote: "`",
}

// keyName 返回录制按键的快捷键名称
func keyName(k ui.Key) string {
	return keyNames[k]
}
