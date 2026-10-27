package app

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"gopaste/internal/locales"
	"gopaste/internal/store"
)

// mirrorRoundTripper 记录镜像测试中的出站请求
type mirrorRoundTripper func(*http.Request) (*http.Response, error)

// RoundTrip 执行测试指定的请求处理
func (f mirrorRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

// TestNormalizeUpdateMirror 验证空地址、路径前缀与 HTTPS 地址的校验边界
func TestNormalizeUpdateMirror(t *testing.T) {
	for _, tc := range []struct {
		raw, want string
		invalid   bool
	}{
		{raw: " "},
		{raw: " https://example.com/// ", want: "https://example.com"},
		{raw: "https://mirror.example/proxy/", want: "https://mirror.example/proxy"},
		{raw: "http://mirror.example", invalid: true},
		{raw: "mirror.example", invalid: true},
		{raw: "https:///proxy", invalid: true},
		{raw: "https://user:password@mirror.example", invalid: true},
		{raw: "https://mirror.example?token=key", invalid: true},
		{raw: "https://mirror.example#fragment", invalid: true},
	} {
		got, err := normalizeUpdateMirror(tc.raw)
		if (err != nil) != tc.invalid || err == nil && got != tc.want {
			t.Errorf("镜像地址 %q：得到 %q、%v", tc.raw, got, err)
		}
	}
}

// TestUpdateMirrorRequests 验证清单、完整包和增量包通过镜像下载且相对重定向与请求头保持有效
func TestUpdateMirrorRequests(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("User-Agent") != "mygo-updater" || req.Header.Get("Range") != "bytes=0-1023" {
			t.Error("镜像请求丢失了更新请求头")
		}
		switch strings.TrimPrefix(req.URL.Path, "/proxy") {
		case "/https://github.com/viewhang/go_paste/releases/latest/download/update-darwin-arm64.json":
			w.Header().Set("Location", "/manifest.json")
			w.WriteHeader(http.StatusFound)
		case "/manifest.json":
			io.WriteString(w, `{"version":"0.1.4"}`)
		case "/https://github.com/viewhang/go_paste/releases/download/v0.1.4/gopaste.tar.gz":
			io.WriteString(w, "archive")
		case "/https://github.com/viewhang/go_paste/releases/download/v0.1.4/gopaste.delta":
			io.WriteString(w, "delta")
		default:
			http.NotFound(w, req)
		}
	}))
	defer server.Close()
	transport := &updateMirrorTransport{base: server.Client().Transport}
	client := &http.Client{Transport: transport}
	for _, tc := range []struct{ path, body, prefix string }{
		{"latest/download/update-darwin-arm64.json", `{"version":"0.1.4"}`, ""},
		{"download/v0.1.4/gopaste.tar.gz", "archive", ""},
		{"download/v0.1.4/gopaste.delta", "delta", ""},
		{"download/v0.1.4/gopaste.tar.gz", "archive", "/proxy"},
	} {
		prefix := server.URL + tc.prefix
		transport.prefix.Store(&prefix)
		original := "https://github.com/viewhang/go_paste/releases/" + tc.path
		req, err := http.NewRequest(http.MethodGet, original, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("User-Agent", "mygo-updater")
		req.Header.Set("Range", "bytes=0-1023")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil || resp.StatusCode != http.StatusOK || string(body) != tc.body {
			t.Fatalf("镜像下载 %s 失败：%s、%q、%v", tc.path, resp.Status, body, err)
		}
		if !strings.HasPrefix(resp.Request.URL.String(), prefix+"/") {
			t.Fatal("后续下载未使用新镜像前缀")
		}
		if req.URL.String() != original || req.Host != "github.com" {
			t.Fatal("镜像转发修改了原始请求")
		}
	}
}

// TestUpdateMirrorScope 验证链接预览和其他仓库保持直连且镜像失败时不改走原站
func TestUpdateMirrorScope(t *testing.T) {
	for _, tc := range []struct{ address, agent, prefix string }{
		{"https://github.com/viewhang/go_paste/releases/latest/download/update.json", "GoPaste/0.1", "https://mirror.example"},
		{"https://github.com/other/repo/releases/latest/download/update.json", "mygo-updater", "https://mirror.example"},
		{"https://example.com/file", "mygo-updater", "https://mirror.example"},
		{"https://github.com/viewhang/go_paste/releases/latest/download/update.json", "mygo-updater", ""},
	} {
		transport := &updateMirrorTransport{base: mirrorRoundTripper(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != tc.address {
				t.Errorf("非镜像请求被改写为 %s", req.URL)
			}
			return &http.Response{Header: make(http.Header), Body: http.NoBody}, nil
		})}
		transport.prefix.Store(&tc.prefix)
		req, _ := http.NewRequest(http.MethodGet, tc.address, nil)
		req.Header.Set("User-Agent", tc.agent)
		if _, err := transport.RoundTrip(req); err != nil {
			t.Fatal(err)
		}
	}
	want := errors.New("mirror unavailable")
	calls := 0
	transport := &updateMirrorTransport{base: mirrorRoundTripper(func(req *http.Request) (*http.Response, error) {
		calls++
		if !strings.HasPrefix(req.URL.String(), "https://mirror.example/https://github.com/") {
			t.Errorf("镜像失败后访问了原站：%s", req.URL)
		}
		return nil, want
	})}
	prefix := "https://mirror.example"
	transport.prefix.Store(&prefix)
	req, _ := http.NewRequest(http.MethodGet, "https://github.com/viewhang/go_paste/releases/latest/download/update.json", nil)
	req.Header.Set("User-Agent", "mygo-updater")
	if _, err := transport.RoundTrip(req); err != want || calls != 1 {
		t.Fatalf("镜像错误未直接返回：请求 %d 次，错误 %v", calls, err)
	}
}

// TestUpdateMirrorSettings 验证中英文镜像输入、校验、持久化与清空后恢复直连
func TestUpdateMirrorSettings(t *testing.T) {
	for _, tag := range []string{"zh-Hans", "en"} {
		t.Run(tag, func(t *testing.T) {
			t.Setenv("MYGO_LOCALE", tag)
			st, err := store.Open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { st.Close() })
			a := NewApp()
			a.st, a.settings = st, st.Settings()
			tt := ui.NewTester(func(c *ui.Context) { a.settingsUpdates(c, pal(c)) }, 560, 350)
			tt.SetLocale(tag)
			messages := locales.In(tag)
			if err := tt.Click(messages.UpdateMirrorAddress()); err != nil {
				t.Fatal(err)
			}
			tt.Type("http://mirror.example")
			tt.Key(0, ui.KeyEnter)
			if !tt.HasText(messages.InvalidUpdateMirror()) || st.Settings().UpdateMirror != "" {
				t.Fatal("无效镜像地址被保存或未显示错误")
			}
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("https://mirror.example/proxy/")
			if err := tt.Click(messages.Save()); err != nil {
				t.Fatal(err)
			}
			if st.Settings().UpdateMirror != "https://mirror.example/proxy" || *a.updateTransport.prefix.Load() != st.Settings().UpdateMirror {
				t.Fatal("镜像地址未保存并立即生效")
			}
			reopened, err := store.Open(st.Dir())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { reopened.Close() })
			if reopened.Settings().UpdateMirror != st.Settings().UpdateMirror {
				t.Fatal("重启后镜像地址丢失")
			}
			if err := tt.Click(messages.UpdateMirrorAddress()); err != nil {
				t.Fatal(err)
			}
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Key(0, ui.KeyBackspace)
			tt.Key(0, ui.KeyEnter)
			if st.Settings().UpdateMirror != "" || *a.updateTransport.prefix.Load() != "" {
				t.Fatal("清空镜像地址后未恢复直连")
			}
		})
	}
}
