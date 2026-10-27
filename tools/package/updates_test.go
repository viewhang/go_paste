package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestUpdateFlags 验证两种 macOS 架构分别嵌入对应更新源与公钥
func TestUpdateFlags(t *testing.T) {
	for _, source := range []struct{ config, base string }{
		{`"github":"owner/app"`, "https://github.com/owner/app/releases/latest/download"},
		{`"url":"https://downloads.example.com/gopaste/"`, "https://downloads.example.com/gopaste"},
	} {
		path := filepath.Join(t.TempDir(), "mygo.json")
		data := `{"updates":{"publicKey":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",` + source.config + `}}`
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
		c, err := loadConfig(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, arch := range []string{"arm64", "amd64"} {
			flags := updateFlags(c, arch)
			feed := source.base + "/update-darwin-" + arch + ".json"
			if !strings.Contains(flags, "packageUpdateFeed="+feed) || !strings.Contains(flags, "packageUpdateKey="+c.Updates.PublicKey) {
				t.Fatalf("架构 %s 的更新源或公钥未正确嵌入：%s", arch, flags)
			}
		}
	}
	if flags := updateFlags(&config{}, runtime.GOARCH); flags != "" {
		t.Fatalf("未配置更新时不应嵌入更新参数：%s", flags)
	}
}

// TestSignedUpdate 验证归档权限、签名校验与 MyGo 真实检查及退出后的安装
func TestSignedUpdate(t *testing.T) {
	public, private, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	configPath := filepath.Join(dir, "mygo.json")
	data := `{"name":"GoPaste","version":"0.2.0","updates":{"github":"owner/app","publicKey":"` + base64.StdEncoding.EncodeToString(public) + `"}}`
	if err := os.WriteFile(configPath, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := loadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("MYGO_UPDATER_PRIVATE_KEY", base64.StdEncoding.EncodeToString(private))
	app := filepath.Join(dir, "GoPaste.app")
	bin := filepath.Join(app, "Contents", "MacOS", "GoPaste")
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, []byte("test app"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeUpdate(c, app, runtime.GOARCH); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(dir, "update-darwin-"+runtime.GOARCH+".json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest updateManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	name := "gopaste-0.2.0-darwin-" + runtime.GOARCH + ".tar.gz"
	if manifest.Version != "0.2.0" || manifest.URL != "https://github.com/owner/app/releases/download/v0.2.0/"+name {
		t.Fatalf("更新清单的版本或地址不正确：%+v", manifest)
	}
	archive, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(archive)
	sig, err := base64.StdEncoding.DecodeString(manifest.Signature)
	if err != nil || !ed25519.Verify(public, digest[:], sig) || manifest.Size != int64(len(archive)) {
		t.Fatal("更新归档签名或大小不正确")
	}
	archive[0] ^= 1
	changed := sha256.Sum256(archive)
	if ed25519.Verify(public, changed[:], sig) {
		t.Fatal("被篡改的更新归档仍通过签名校验")
	}
	f, err := os.Open(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err != nil {
			t.Fatalf("更新归档缺少应用可执行文件：%v", err)
		}
		if header.Name == "GoPaste.app/Contents/MacOS/GoPaste" {
			body, err := io.ReadAll(tr)
			if err != nil || string(body) != "test app" || header.Mode&0o111 == 0 {
				t.Fatal("应用归档未保留内容或执行权限")
			}
			break
		}
	}
	if runtime.GOOS == "darwin" {
		server := httptest.NewServer(http.FileServer(http.Dir(dir)))
		defer server.Close()
		manifest.URL = server.URL + "/" + name
		data, err := json.Marshal(manifest)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
			t.Fatal(err)
		}
		old := filepath.Join(t.TempDir(), "GoPaste.app", "Contents", "MacOS", "GoPaste")
		if err := os.MkdirAll(filepath.Dir(old), 0o755); err != nil {
			t.Fatal(err)
		}
		source := filepath.Join(t.TempDir(), "main.go")
		code := `package main
import (
    "context"
    "github.com/egoist/mygo"
)
// main 检查并安装隔离测试应用的更新
func main() {
    update, err := mygo.Updater.Check(context.Background())
    if err != nil { panic(err) }
    if update == nil || update.Version != "0.2.0" { panic("未发现新版本") }
    if err := update.Install(context.Background(), nil); err != nil { panic(err) }
}`
		if err := os.WriteFile(source, []byte(code), 0o644); err != nil {
			t.Fatal(err)
		}
		flags := "-X github.com/egoist/mygo.production=1 -X github.com/egoist/mygo.packageVersion=0.1.0" +
			" -X github.com/egoist/mygo.packageUpdateFeed=" + server.URL + "/" + filepath.Base(manifestPath) +
			" -X github.com/egoist/mygo.packageUpdateKey=" + c.Updates.PublicKey
		if output, err := exec.Command("go", "build", "-ldflags", flags, "-o", old, source).CombinedOutput(); err != nil {
			t.Fatalf("编译更新集成测试失败：%v\n%s", err, output)
		}
		if output, err := exec.Command(old).CombinedOutput(); err != nil {
			t.Fatalf("MyGo 更新器未接受发布包：%v\n%s", err, output)
		}
		deadline := time.Now().Add(5 * time.Second)
		for {
			if installed, err := os.ReadFile(old); err == nil && string(installed) == "test app" {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("MyGo 更新器未在退出后替换隔离应用")
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	c.Updates.PublicKey = base64.StdEncoding.EncodeToString(make([]byte, ed25519.PublicKeySize))
	if _, err := updateKey(c); err == nil {
		t.Fatal("不匹配的更新公钥未被拦截")
	}
}
