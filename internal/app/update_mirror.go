package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"sync/atomic"

	"github.com/egoist/mygo"

	"gopaste/internal/locales"
	"gopaste/internal/types"
)

// updateMirrorTransport 通过系统代理读取更新源并按配置转发到 GitHub 镜像
type updateMirrorTransport struct {
	base   http.RoundTripper
	prefix atomic.Pointer[string]
}

// Manifest 读取当前平台的稳定版更新清单
func (t *updateMirrorTransport) Manifest(ctx context.Context, _ bool) (io.ReadCloser, error) {
	return t.Open(ctx, "https://github.com/viewhang/go_paste/releases/latest/download/update-"+runtime.GOOS+"-"+runtime.GOARCH+".json")
}

// Open 下载更新文件并保留请求取消、镜像重定向与 HTTP 错误
func (t *updateMirrorTransport) Open(ctx context.Context, address string) (io.ReadCloser, error) {
	u, err := url.Parse(address)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "https" {
		return nil, fmt.Errorf("refusing to download %s: updates need HTTPS", address)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "mygo-updater")
	resp, err := (&http.Client{Transport: t}).Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("GET %s: %s", address, resp.Status)
	}
	return resp.Body, nil
}

// RoundTrip 为更新清单、完整包和增量包拼接镜像前缀并保留重定向语义
func (t *updateMirrorTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	prefix := t.prefix.Load()
	if prefix == nil || *prefix == "" || req.Method != http.MethodGet ||
		req.URL.Scheme != "https" || req.URL.Host != "github.com" ||
		!strings.HasPrefix(req.URL.Path, "/viewhang/go_paste/releases/") ||
		req.Header.Get("User-Agent") != "mygo-updater" {
		return t.base.RoundTrip(req)
	}
	target, err := url.Parse(*prefix + "/" + req.URL.String())
	if err != nil {
		return nil, err
	}
	if target.Scheme != "https" {
		return nil, errors.New(locales.InvalidUpdateMirror())
	}
	proxied := req.Clone(req.Context())
	proxied.URL, proxied.Host = target, target.Host
	resp, err := t.base.RoundTrip(proxied)
	if err != nil {
		return nil, err
	}
	if location := resp.Header.Get("Location"); location != "" {
		destination, err := target.Parse(location)
		if err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Header = resp.Header.Clone()
		resp.Header.Set("Location", destination.String())
	}
	return resp, nil
}

// normalizeUpdateMirror 校验 HTTPS 镜像前缀并移除空白与末尾斜杠
func normalizeUpdateMirror(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" ||
		u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return "", errors.New(locales.InvalidUpdateMirror())
	}
	return strings.TrimRight(u.String(), "/"), nil
}

// saveUpdateMirror 保存已校验的镜像地址并立即应用到后续更新请求
func (a *App) saveUpdateMirror() {
	prefix, err := normalizeUpdateMirror(a.updateMirrorInput)
	if err != nil {
		a.updateMirrorError = err.Error()
		return
	}
	a.setSetting(func(s *types.Settings) { s.UpdateMirror = prefix })
	if a.settings.UpdateMirror == prefix {
		a.setUpdateMirror(prefix)
		a.updateMirrorInput, a.updateMirrorError = prefix, ""
	}
}

// setUpdateMirror 应用镜像前缀并在清空时恢复框架默认更新源
func (a *App) setUpdateMirror(prefix string) {
	a.updateTransport.prefix.Store(&prefix)
	if prefix == "" {
		mygo.Updater.SetSource(nil)
	} else {
		mygo.Updater.SetSource(a.updateTransport)
	}
}
