package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// updateManifest 保存 MyGo 更新器使用的版本、归档地址与签名
type updateManifest struct {
	Version   string `json:"version"`
	Date      string `json:"date"`
	URL       string `json:"url"`
	Size      int64  `json:"size"`
	Signature string `json:"signature"`
}

// validateUpdates 校验更新公钥与唯一的 HTTPS 发布来源
func validateUpdates(c *config) error {
	u := c.Updates
	if u == nil {
		return nil
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(u.PublicKey))
	if err != nil || len(key) != ed25519.PublicKeySize {
		return fmt.Errorf("updates.publicKey 必须是 mygo keygen 生成的公钥")
	}
	if (u.GitHub == "") == (u.URL == "") {
		return fmt.Errorf("updates.github 与 updates.url 必须指定且只能指定一个")
	}
	if u.GitHub != "" {
		parts := strings.Split(u.GitHub, "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return fmt.Errorf("updates.github 必须使用 owner/repo 格式")
		}
	} else if parsed, err := url.Parse(u.URL); err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("updates.url 必须是 HTTPS 发布目录")
	}
	return nil
}

// updateBase 返回更新清单或指定版本归档的发布目录
func updateBase(c *config, latest bool) string {
	u := c.Updates
	if u.GitHub == "" {
		return strings.TrimRight(u.URL, "/")
	}
	base := "https://github.com/" + u.GitHub + "/releases/"
	if latest {
		return base + "latest/download"
	}
	return base + "download/v" + c.Version
}

// updateKey 读取更新私钥并校验其与内置公钥一致
func updateKey(c *config) (ed25519.PrivateKey, error) {
	text := os.Getenv("MYGO_UPDATER_PRIVATE_KEY")
	if text == "" && c.Updates.PrivateKey != "" {
		path := c.Updates.PrivateKey
		if rest, ok := strings.CutPrefix(path, "~/"); ok {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, err
			}
			path = filepath.Join(home, rest)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		text = string(data)
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(text))
	if err != nil || len(key) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("请通过 updates.privateKey 或 MYGO_UPDATER_PRIVATE_KEY 提供有效的更新签名私钥")
	}
	private := ed25519.PrivateKey(key)
	public := base64.StdEncoding.EncodeToString(private.Public().(ed25519.PublicKey))
	if public != strings.TrimSpace(c.Updates.PublicKey) {
		return nil, fmt.Errorf("更新私钥与 updates.publicKey 不匹配")
	}
	return private, nil
}

// writeUpdate 按目标 macOS 架构归档已签名应用并生成签名更新清单
func writeUpdate(c *config, app, arch string) error {
	key, err := updateKey(c)
	if err != nil {
		return err
	}
	target := "darwin-" + arch
	name := strings.ToLower(c.Name) + "-" + c.Version + "-" + target + ".tar.gz"
	archive := filepath.Join(filepath.Dir(app), name)
	cmd := exec.Command("tar", "-czf", archive, "-C", filepath.Dir(app), filepath.Base(app))
	cmd.Env = append(os.Environ(), "COPYFILE_DISABLE=1")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("生成更新归档失败：%w\n%s", err, output)
	}
	data, err := os.ReadFile(archive)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(data)
	manifest := updateManifest{
		Version:   c.Version,
		Date:      time.Now().UTC().Format(time.RFC3339),
		URL:       updateBase(c, false) + "/" + name,
		Size:      int64(len(data)),
		Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(key, digest[:])),
	}
	data, err = json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(filepath.Dir(app), "update-"+target+".json")
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
