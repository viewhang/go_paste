package app

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"

	"gopaste/internal/locales"
	"gopaste/internal/types"
)

// updateMirrorTransport 将本项目的更新请求转发到用户指定的 GitHub 镜像
type updateMirrorTransport struct {
	base   http.RoundTripper
	prefix atomic.Pointer[string]
}

// RoundTrip 为更新清单、完整包和增量包拼接镜像前缀并保留重定向语义
func (t *updateMirrorTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	prefix := t.prefix.Load()
	if prefix == nil || *prefix == "" || req.Method != http.MethodGet ||
		req.URL.Scheme != "https" || req.URL.Host != "github.com" ||
		!strings.HasPrefix(req.URL.Path, "/viewhang/go_paste/releases/") ||
		!strings.HasPrefix(req.Header.Get("User-Agent"), "MyGo/") {
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
		a.updateTransport.prefix.Store(&prefix)
		a.updateMirrorInput, a.updateMirrorError = prefix, ""
	}
}
