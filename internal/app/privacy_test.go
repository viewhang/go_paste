package app

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"gopaste/internal/locales"
	"gopaste/internal/store"
	"gopaste/internal/types"
)

// TestExcludedAppsSettings 验证应用去重、重启保留、图标显示及排除规则即时保存
func TestExcludedAppsSettings(t *testing.T) {
	if !isDarwin && runtime.GOOS != "windows" {
		t.Skip("当前平台不支持应用标识")
	}
	path := filepath.Join(t.TempDir(), "Test App.exe")
	wantID := "test app.exe"
	if isDarwin {
		path = filepath.Join(t.TempDir(), "Test App.app")
		contents := filepath.Join(path, "Contents")
		if err := os.MkdirAll(contents, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(contents, "Info.plist"), []byte(`<?xml version="1.0"?><plist version="1.0"><dict><key>CFBundleIdentifier</key><string>com.example.TestApp</string><key>CFBundlePackageType</key><string>APPL</string></dict></plist>`), 0o644); err != nil {
			t.Fatal(err)
		}
		wantID = "com.example.TestApp"
	} else if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	a := NewApp()
	a.st, a.settings = st, st.Settings()
	a.setSetting(func(s *types.Settings) { s.ExcludedApps = nil })
	if err := a.addExcludedApps([]string{path, path}); err != nil {
		t.Fatal(err)
	}
	if len(a.settings.ExcludedApps) != 1 || a.settings.ExcludedApps[0].ID != wantID || a.settings.ExcludedApps[0].Name != "Test App" {
		t.Fatalf("应用标识、名称或去重错误：%+v", a.settings.ExcludedApps)
	}
	if isDarwin {
		if err := a.addExcludedApps([]string{filepath.Join(t.TempDir(), "Missing.app")}); err == nil {
			t.Fatal("无有效标识的应用被加入排除列表")
		}
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	st = reopened
	a.st, a.settings = st, st.Settings()
	if len(a.settings.ExcludedApps) != 1 || a.settings.ExcludedApps[0].ID != wantID {
		t.Fatal("重启后排除规则丢失")
	}
	snapshot := st.Settings()
	snapshot.ExcludedApps[0].ID = "changed"
	if st.Settings().ExcludedApps[0].ID != wantID {
		t.Fatal("修改设置副本影响了采集线程的排除规则")
	}
	icon, err := ui.DecodeBitmap(sampleIcon())
	if err != nil {
		t.Fatal(err)
	}
	a.images.put(bitmapKey{name: "file:" + path, side: a.imagePixels(40)}, icon)
	ready := shellReady
	shellReady = true
	t.Cleanup(func() { shellReady = ready })
	var tt *ui.Tester
	for _, lang := range []string{"zh-Hans", "en"} {
		t.Setenv("MYGO_LOCALE", lang)
		for _, theme := range []int{themeClassic, themeGlass} {
			a.settings.Theme = theme
			tt = ui.NewTester(func(c *ui.Context) { a.settingsPrivacy(c, a.palette(c)) }, 520, 400)
			tt.SetLocale(lang)
			if !tt.HasText(locales.AddExcludedApp()) || !tt.HasText("Test App") {
				t.Fatalf("%s 主题 %d 未显示应用排除设置", lang, theme)
			}
			if name, ok := tt.Find("Test App"); !ok || name.X < 50 {
				t.Fatal("应用名称前未显示应用图标")
			}
			if !tt.HasText(locales.OtherRules()) || !tt.HasText(locales.ExcludeConfidential()) {
				t.Fatal("其他规则未显示机密内容开关")
			}
			if !tt.HasText(locales.ExcludeWhitespace()) || !tt.HasText(locales.ImageSizeLimit()) {
				t.Fatal("其他规则未显示空白内容与图片大小设置")
			}
			if os.Getenv("SHOTS") != "" {
				out := filepath.Join("..", "..", ".mygo", "shots")
				if err := os.MkdirAll(out, 0o755); err != nil {
					t.Fatal(err)
				}
				writePNG(t, filepath.Join(out, fmt.Sprintf("privacy-%s-%d.png", lang, theme)), tt.Image())
			}
		}
	}
	if a.settings.ExcludeConfidential {
		t.Fatal("机密内容排除规则默认启用")
	}
	if !a.settings.ExcludeWhitespace || a.settings.ImageLimitMiB != 5 {
		t.Fatal("空白规则与图片大小默认值不正确")
	}
	whitespace, _ := tt.Find(locales.ExcludeWhitespace())
	tt.ClickAt(486, whitespace.Y+whitespace.H/2)
	unit, _ := tt.Find("MiB")
	tt.ClickAt(unit.X-110, unit.Y+unit.H/2)
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("2")
	if st.Settings().ExcludeWhitespace || st.Settings().ImageLimitMiB != 2 {
		t.Fatal("空白规则与图片大小修改未即时保存")
	}
	unit, _ = tt.Find("MiB")
	tt.ClickAt(unit.X-110, unit.Y+unit.H/2)
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("0")
	if a.settings.ImageLimitMiB != 0 || st.Settings().ImageLimitMiB != 0 {
		t.Fatal("输入 0 未关闭图片大小限制并保存")
	}
	rule, _ := tt.Find(locales.ExcludeConfidential())
	tt.ClickAt(486, rule.Y+rule.H/2)
	if !a.settings.ExcludeConfidential || !st.Settings().ExcludeConfidential {
		t.Fatal("机密内容排除开关未即时生效并保存")
	}
	if err := tt.Click(locales.RemoveExcludedApp("Test App")); err != nil {
		t.Fatal(err)
	}
	if len(a.settings.ExcludedApps) != 0 || len(st.Settings().ExcludedApps) != 0 {
		t.Fatal("移除应用后排除规则未更新")
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	st, err = store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	a.st, a.settings = st, st.Settings()
	if !a.settings.ExcludeConfidential {
		t.Fatal("重启后机密内容排除规则丢失")
	}
	if a.settings.ExcludeWhitespace || a.settings.ImageLimitMiB != 0 {
		t.Fatal("重启后空白规则与图片大小设置丢失")
	}
	tt.Frame()
	rule, _ = tt.Find(locales.ExcludeConfidential())
	tt.ClickAt(486, rule.Y+rule.H/2)
	if a.settings.ExcludeConfidential || st.Settings().ExcludeConfidential {
		t.Fatal("机密内容排除规则无法关闭")
	}
}

// TestDefaultExcludedAppsSettings 验证仅显示已安装的默认项，保留自定义项及移除结果
func TestDefaultExcludedAppsSettings(t *testing.T) {
	defaults := types.DefaultExcludedApps()
	if len(defaults) == 0 {
		t.Skip("当前平台没有默认排除应用")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{"excludedApps":[{"id":"com.example.Custom","name":"Custom"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	a := NewApp()
	a.st, a.settings = st, st.Settings()
	if len(a.settings.ExcludedApps) != len(defaults)+1 || a.settings.ExcludedApps[0].Name != "Custom" {
		t.Fatal("初始化默认项时丢失或覆盖了自定义应用")
	}
	for _, app := range defaults {
		if app.Name == "1Password" {
			a.installedApps = []mygo.InstalledApp{{ID: app.ID, Name: app.Name, Path: dir}}
		}
	}
	var tt *ui.Tester
	for _, lang := range []string{"zh-Hans", "en"} {
		t.Setenv("MYGO_LOCALE", lang)
		for _, theme := range []int{themeClassic, themeGlass} {
			a.settings.Theme = theme
			tt = ui.NewTester(func(c *ui.Context) { a.settingsPrivacy(c, a.palette(c)) }, 520, 680)
			tt.SetLocale(lang)
			for _, app := range defaults {
				if tt.HasText(app.Name) != (app.Name == "1Password") {
					t.Fatalf("%s 主题 %d 的应用 %s 显示状态与安装状态不符", lang, theme, app.Name)
				}
			}
			if os.Getenv("SHOTS") != "" {
				out := filepath.Join("..", "..", ".mygo", "shots")
				if err := os.MkdirAll(out, 0o755); err != nil {
					t.Fatal(err)
				}
				writePNG(t, filepath.Join(out, fmt.Sprintf("privacy-defaults-%s-%d.png", lang, theme)), tt.Image())
			}
		}
	}
	if err := tt.Click(locales.RemoveExcludedApp("1Password")); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	st = reopened
	if apps := st.Settings().ExcludedApps; len(apps) != len(defaults) || slices.ContainsFunc(apps, func(app types.ExcludedApp) bool { return app.Name == "1Password" }) {
		t.Fatal("重启后恢复了已移除的默认应用")
	}
	settings := st.Settings()
	settings.ExcludedApps = nil
	if err := st.SetSettings(settings); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err = store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	st = reopened
	if len(st.Settings().ExcludedApps) != 0 {
		t.Fatal("清空排除列表后重启恢复了默认项")
	}
}

// TestExcludedAppPath 验证应用路径解析及自定义应用移除后不再显示
func TestExcludedAppPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Custom.app")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	app := types.ExcludedApp{ID: "com.example.Custom", Name: "Custom", Path: path}
	if excludedAppPath(app, nil) != path {
		t.Fatal("应用选择器选中的非标准安装位置未显示")
	}
	installed := []mygo.InstalledApp{{ID: app.ID, Name: app.Name, Path: path}}
	if excludedAppPath(types.ExcludedApp{ID: app.ID, Name: app.Name}, installed) != path {
		t.Fatal("默认应用未解析到系统安装路径")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if excludedAppPath(app, nil) != "" {
		t.Fatal("已移除的应用仍显示在列表中")
	}
}
