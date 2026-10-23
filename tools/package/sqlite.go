package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// sqliteLibrary 描述插件清单中指定架构的原生库
type sqliteLibrary struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

// bundleSQLite 将当前 MyGo 版本声明的目标架构库校验后放入应用资源目录
func bundleSQLite(root, resources, arch, identity string) error {
	cmd := exec.Command("go", "list", "-f", "{{.Dir}}", "github.com/egoist/mygo/plugins/sqlite")
	cmd.Dir = root
	dir, err := cmd.Output()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(strings.TrimSpace(string(dir)), "mygo-plugin.json"))
	if err != nil {
		return err
	}
	var manifest struct {
		Libraries []struct {
			Files map[string]sqliteLibrary `json:"files"`
		} `json:"libraries"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return err
	}
	for _, library := range manifest.Libraries {
		file, ok := library.Files["darwin-"+arch]
		if !ok {
			return fmt.Errorf("SQLite 未提供 darwin/%s 原生库", arch)
		}
		cache, err := os.UserCacheDir()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(filepath.Join(cache, "mygo", "natives", file.SHA256, file.Name))
		if err != nil || !sqliteDigestMatches(data, file.SHA256) {
			client := http.Client{Timeout: 2 * time.Minute}
			response, err := client.Get(file.URL)
			if err != nil {
				return err
			}
			data, err = io.ReadAll(response.Body)
			response.Body.Close()
			if err != nil {
				return err
			}
			if response.StatusCode != http.StatusOK {
				return fmt.Errorf("下载 SQLite：HTTP %d", response.StatusCode)
			}
		}
		if !sqliteDigestMatches(data, file.SHA256) {
			return fmt.Errorf("SQLite 原生库 SHA-256 不匹配")
		}
		dest := filepath.Join(resources, file.Name)
		if err := os.WriteFile(dest, data, 0755); err != nil {
			return err
		}
		if err := signApp(dest, "dev.gopaste.sqlite", identity); err != nil {
			return err
		}
	}
	return nil
}

// sqliteDigestMatches 验证原生库内容与插件声明的摘要一致
func sqliteDigestMatches(data []byte, digest string) bool {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]) == digest
}
