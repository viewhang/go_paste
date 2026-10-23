// Command package 按目标架构使用固定证书打包 GoPaste 并生成签名更新归档
//
//	go run ./tools/package
//
// Windows 应用使用 go tool mygo build -platform windows/amd64 构建
package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"golang.org/x/image/draw"

	"gopaste/internal/locales"
)

// config 保存应用打包、固定签名与更新发布配置
type config struct {
	Name       string `json:"name"`
	Identifier string `json:"identifier"`
	Version    string `json:"version"`
	Copyright  string `json:"copyright"`
	Icon       string `json:"icon"`
	Main       string `json:"main"`
	Out        string `json:"out"`
	Updates    *struct {
		PublicKey  string `json:"publicKey"`
		PrivateKey string `json:"privateKey"`
		GitHub     string `json:"github"`
		URL        string `json:"url"`
	} `json:"updates"`
	MacOS struct {
		Icon                 string `json:"icon"`
		MinimumSystemVersion string `json:"minimumSystemVersion"`
		SigningIdentity      string `json:"signingIdentity"`
	} `json:"macos"`
}

// main 按目标架构编译签名应用包并按需生成更新归档与清单
func main() {
	var (
		out     = flag.String("o", "", "output directory (default: out from mygo.json)")
		arch    = flag.String("arch", runtime.GOARCH, "macOS 目标架构：arm64 或 amd64")
		keepW   = flag.Bool("work", false, "let go keep its work directory")
		release = flag.Bool("release", false, "生成已签名的自动更新归档与清单")
	)
	flag.Parse()
	if *arch != "arm64" && *arch != "amd64" {
		fail("不支持的 macOS 架构 %q，请使用 arm64 或 amd64", *arch)
	}

	root, err := os.Getwd()
	must(err)
	c, err := loadConfig(filepath.Join(root, "mygo.json"))
	must(err)
	generate := exec.Command("go", "tool", "mygo", "generate")
	generate.Dir = root
	generate.Stdout, generate.Stderr = os.Stdout, os.Stderr
	must(generate.Run())
	if *out != "" {
		c.Out = *out
	}
	if c.Out == "" {
		c.Out = "build"
	}
	if c.Main == "" {
		c.Main = "."
	}
	if *release && c.Updates == nil {
		fail("生成更新包需要配置 mygo.json 的 updates")
	}

	app := filepath.Join(root, c.Out, c.Name+".app")
	exe := filepath.Join(app, "Contents", "MacOS", c.Name)
	res := filepath.Join(app, "Contents", "Resources")
	for _, d := range []string{filepath.Dir(exe), res} {
		must(os.MkdirAll(d, 0o755))
	}

	ldflags := strings.Join([]string{
		"-s -w",
		"-X github.com/egoist/mygo.packageName=" + c.Name,
		"-X github.com/egoist/mygo.packageVersion=" + c.Version,
		"-X github.com/egoist/mygo.packageIdentifier=" + c.Identifier,
		"-X github.com/egoist/mygo.production=1",
	}, " ")
	ldflags += updateFlags(c, *arch)
	args := []string{"build", "-trimpath", "-ldflags", ldflags, "-tags", "mygo_noinspector"}
	if *keepW {
		args = append(args, "-work")
	}
	args = append(args, "-o", exe, c.Main)

	step("Compiling darwin/"+*arch, "go "+strings.Join(args, " "))
	must(goBuild(root, args, *arch))

	must(os.WriteFile(filepath.Join(app, "Contents", "PkgInfo"), []byte("APPL????"), 0o644))
	must(os.WriteFile(filepath.Join(app, "Contents", "Info.plist"), infoPlist(c), 0o644))
	if c.Icon != "" {
		icns, err := iconFile(filepath.Join(root, c.Icon))
		if err != nil {
			note("no icon: " + err.Error())
		} else {
			must(os.WriteFile(filepath.Join(res, "AppIcon.icns"), icns, 0o644))
		}
	}
	for _, tag := range locales.Catalog.Languages() {
		dir := filepath.Join(res, tag+".lproj")
		must(os.MkdirAll(dir, 0o755))
		name := locales.Catalog.Format(tag, "mygo.appName", nil)
		must(os.WriteFile(filepath.Join(dir, "InfoPlist.strings"), []byte(fmt.Sprintf("\"CFBundleDisplayName\" = %q;\n", name)), 0o644))
	}
	must(bundleSQLite(root, res, *arch, c.MacOS.SigningIdentity))
	must(signApp(app, c.Identifier, c.MacOS.SigningIdentity))
	if *release {
		must(writeUpdate(c, app, *arch))
	}

	fmt.Printf("\n  ✓ %s %s\n\n    %s\n\n", c.Name, c.Version, rel(root, app))
	fmt.Printf("  Run it with:  open %s\n", rel(root, app))
	fmt.Printf("  Logs:         log stream --predicate 'process == \"%s\"'\n\n", c.Name)
}

// updateFlags 将目标 macOS 架构的更新源与验证公钥嵌入应用
func updateFlags(c *config, arch string) string {
	if c.Updates == nil {
		return ""
	}
	feed := fmt.Sprintf("%s/update-darwin-%s.json", updateBase(c, true), arch)
	return " -X github.com/egoist/mygo.packageUpdateFeed=" + feed +
		" -X github.com/egoist/mygo.packageUpdateKey=" + strings.TrimSpace(c.Updates.PublicKey)
}

// signApp 使用配置的固定证书签名完整应用包并封装元数据与资源
func signApp(path, identifier, identity string) error {
	if identity == "" || identity == "-" {
		return fmt.Errorf("macos.signingIdentity 必须指定固定的代码签名证书")
	}
	output, err := exec.Command("codesign", "--force", "--sign", identity, "--identifier", identifier, path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("codesign %s: %w\n%s", filepath.Base(path), err, output)
	}
	return nil
}

// loadConfig 读取并校验应用打包配置
func loadConfig(path string) (*config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	c := &config{}
	if err := json.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if err := validateUpdates(c); err != nil {
		return nil, err
	}
	return c, nil
}

// goBuild 使用指定 macOS 架构编译应用并输出编译诊断
func goBuild(root string, args []string, arch string) error {
	cmd := exec.Command("go", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=darwin", "GOARCH="+arch)
	out, err := cmd.CombinedOutput()
	if s := strings.TrimSpace(string(out)); s != "" {
		for _, line := range strings.Split(s, "\n") {
			if strings.HasPrefix(line, "WORK=") {
				continue // only noise from -work
			}
			fmt.Println("  go │ " + line)
		}
	}
	return err
}

// infoPlist 生成应用标识、系统要求、常驻行为与支持语言的元数据
func infoPlist(c *config) []byte {
	min := c.MacOS.MinimumSystemVersion
	if min == "" {
		min = "12.0"
	}
	type kv struct{ key, kind, value string }
	keys := []kv{
		{"CFBundleDevelopmentRegion", "string", locales.Catalog.Source()},
		{"CFBundleDisplayName", "string", c.Name},
		{"CFBundleExecutable", "string", c.Name},
		{"CFBundleIdentifier", "string", c.Identifier},
		{"CFBundleInfoDictionaryVersion", "string", "6.0"},
		{"CFBundleName", "string", c.Name},
		{"CFBundlePackageType", "string", "APPL"},
		{"CFBundleShortVersionString", "string", c.Version},
		{"CFBundleVersion", "string", c.Version},
		{"LSMinimumSystemVersion", "string", min},
		{"LSUIElement", "true", ""},
		{"NSHighResolutionCapable", "true", ""},
		{"NSPrincipalClass", "string", "NSApplication"},
		{"NSSupportsAutomaticGraphicsSwitching", "true", ""},
	}
	if c.Icon != "" {
		keys = append(keys, kv{"CFBundleIconFile", "string", "AppIcon"})
	}
	if c.Copyright != "" {
		keys = append(keys, kv{"NSHumanReadableCopyright", "string", c.Copyright})
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].key < keys[j].key })
	var b bytes.Buffer
	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	b.WriteString("<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n")
	b.WriteString("<plist version=\"1.0\">\n<dict>\n")
	for _, k := range keys {
		fmt.Fprintf(&b, "\t<key>%s</key>\n", k.key)
		if k.kind == "true" {
			b.WriteString("\t<true/>\n")
		} else {
			fmt.Fprintf(&b, "\t<string>%s</string>\n", escape(k.value))
		}
	}
	b.WriteString("\t<key>CFBundleLocalizations</key>\n\t<array>\n")
	for _, tag := range locales.Catalog.Languages() {
		fmt.Fprintf(&b, "\t\t<string>%s</string>\n", escape(tag))
	}
	b.WriteString("\t</array>\n</dict>\n</plist>\n")
	return b.Bytes()
}

func escape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&apos;")
	return r.Replace(s)
}

// iconFile renders a PNG as .icns, holding the sizes Finder and the Dock
// ask for. They are stored as PNG, which every version GoPaste runs on
// reads.
func iconFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	src, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	variants := []struct {
		kind string
		size int
	}{
		{"ic07", 128}, {"ic08", 256}, {"ic09", 512}, {"ic10", 1024},
	}
	var body bytes.Buffer
	for _, v := range variants {
		img := scale(src, v.size)
		var pngData bytes.Buffer
		if err := png.Encode(&pngData, img); err != nil {
			return nil, err
		}
		body.WriteString(v.kind)
		var length [4]byte
		binary.BigEndian.PutUint32(length[:], uint32(8+pngData.Len()))
		body.Write(length[:])
		body.Write(pngData.Bytes())
	}
	var out bytes.Buffer
	out.WriteString("icns")
	var total [4]byte
	binary.BigEndian.PutUint32(total[:], uint32(8+body.Len()))
	out.Write(total[:])
	out.Write(body.Bytes())
	return out.Bytes(), nil
}

// scale 使用双线性采样生成指定尺寸的应用图标
func scale(src image.Image, size int) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	draw.BiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
	return dst
}

func step(label, detail string) {
	fmt.Printf("  ○ %s\n    %s\n", label, muted(detail))
}

func note(s string) { fmt.Printf("  · %s\n", s) }

func muted(s string) string {
	if os.Getenv("NO_COLOR") != "" {
		return s
	}
	return "\x1b[2m" + s + "\x1b[0m"
}

func rel(root, path string) string {
	if r, err := filepath.Rel(root, path); err == nil {
		return r
	}
	return path
}

func must(err error) {
	if err != nil {
		fail("%v", err)
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "\n  ✗ %s\n\n", fmt.Sprintf(format, args...))
	os.Exit(1)
}
