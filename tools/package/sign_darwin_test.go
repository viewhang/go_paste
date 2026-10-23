//go:build darwin

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestSignAppKeepsIdentityAcrossBuilds 验证固定证书保护元数据并保持重建前后的签名身份一致
func TestSignAppKeepsIdentityAcrossBuilds(t *testing.T) {
	app := filepath.Join(t.TempDir(), "GoPaste.app")
	bin := filepath.Join(app, "Contents", "MacOS", "GoPaste")
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, data, 0o755); err != nil {
		t.Fatal(err)
	}
	c, err := loadConfig(filepath.Join("..", "..", "mygo.json"))
	if err != nil {
		t.Fatal(err)
	}
	c.Identifier = "dev.gopaste.sign-test"
	c.Version = "0.1.0"
	plist := filepath.Join(app, "Contents", "Info.plist")
	if err := os.WriteFile(plist, infoPlist(c), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := signApp(app, c.Identifier, c.MacOS.SigningIdentity); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("codesign", "--verify", "--strict", app).CombinedOutput(); err != nil {
		t.Fatalf("签名校验失败：%v\n%s", err, output)
	}
	output, err := exec.Command("codesign", "--display", "--verbose=4", app).CombinedOutput()
	if err != nil {
		t.Fatalf("无法读取签名：%v\n%s", err, output)
	}
	if !strings.Contains(string(output), "Identifier="+c.Identifier+"\n") {
		t.Fatalf("签名标识与配置不符：\n%s", output)
	}
	before, err := exec.Command("codesign", "--display", "-r-", app).CombinedOutput()
	if err != nil {
		t.Fatalf("无法读取签名身份：%v\n%s", err, before)
	}
	c.Version = "0.2.0"
	if err := os.WriteFile(plist, infoPlist(c), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("codesign", "--verify", "--strict", app).Run(); err == nil {
		t.Fatal("修改 Info.plist 后签名校验仍然通过，元数据未绑定")
	}
	if err := signApp(app, c.Identifier, c.MacOS.SigningIdentity); err != nil {
		t.Fatal(err)
	}
	after, err := exec.Command("codesign", "--display", "-r-", app).CombinedOutput()
	if err != nil {
		t.Fatalf("无法读取重建后的签名身份：%v\n%s", err, after)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("重建前后签名身份发生变化：\n%s\n%s", before, after)
	}
	if output, err := exec.Command("codesign", "--verify", "--strict", app).CombinedOutput(); err != nil {
		t.Fatalf("重建后签名校验失败：%v\n%s", err, output)
	}
}
