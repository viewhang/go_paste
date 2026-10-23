package app

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gopaste/internal/store"
	"gopaste/internal/types"
)

// TestLinkPreviewConcurrency 验证链接标题与图片请求共用两个并发名额
func TestLinkPreviewConcurrency(t *testing.T) {
	var active, peak atomic.Int32
	entered := make(chan struct{}, 16)
	gate := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); n > old && !peak.CompareAndSwap(old, n); old = peak.Load() {
		}
		entered <- struct{}{}
		<-gate
		if r.URL.Path == "/image" {
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte("image"))
			return
		}
		fmt.Fprint(w, `<title>Example</title><meta property="og:image" content="/image">`)
	}))
	defer server.Close()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	a := NewApp()
	a.st = st
	var wg sync.WaitGroup
	for i := range 6 {
		rec, _ := st.Add(&types.Record{Kind: types.KindLink, URL: fmt.Sprintf("%s/%d", server.URL, i), Hash: fmt.Sprint(i)})
		wg.Go(func() { a.fetchPreview(rec.ID, rec.URL) })
	}
	for range 2 {
		select {
		case <-entered:
		case <-time.After(3 * time.Second):
			close(gate)
			t.Fatal("链接预览任务未开始")
		}
	}
	select {
	case <-entered:
		close(gate)
		t.Fatal("同时开始了超过两个请求")
	case <-time.After(100 * time.Millisecond):
	}
	close(gate)
	wg.Wait()
	if peak.Load() != 2 {
		t.Fatalf("链接请求峰值异常：%d", peak.Load())
	}
	for _, rec := range st.List() {
		if rec.PreviewState != "done" || rec.PreviewImage == "" {
			t.Fatal("链接预览未完成")
		}
	}
}

// TestLinkPreviewDiscardedResults 验证重复任务不发请求且删除或关闭后下载结果不会产生孤立文件
func TestLinkPreviewDiscardedResults(t *testing.T) {
	for _, action := range []string{"delete", "close"} {
		t.Run(action, func(t *testing.T) {
			var pages, images atomic.Int32
			entered := make(chan struct{})
			gate := make(chan struct{})
			var release sync.Once
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/image" {
					if images.Add(1) == 1 {
						close(entered)
					}
					<-gate
					w.Header().Set("Content-Type", "image/png")
					w.Write([]byte("discarded image"))
					return
				}
				pages.Add(1)
				fmt.Fprint(w, `<title>Example</title><meta property="og:image" content="/image">`)
			}))
			defer server.Close()
			defer release.Do(func() { close(gate) })
			st, err := store.Open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer st.Close()
			a := NewApp()
			a.st = st
			r, _ := st.Add(&types.Record{Kind: types.KindLink, URL: server.URL})
			done := make(chan struct{})
			go func() {
				a.fetchPreview(r.ID, r.URL)
				close(done)
			}()
			select {
			case <-entered:
			case <-time.After(3 * time.Second):
				t.Fatal("预览图片请求未开始")
			}
			a.fetchPreview(r.ID, r.URL)
			if action == "delete" {
				if !st.Delete(r.ID) {
					t.Fatal("删除记录失败")
				}
			} else if err := st.Close(); err != nil {
				t.Fatal(err)
			}
			release.Do(func() { close(gate) })
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("预览任务未结束")
			}
			entries, err := os.ReadDir(st.BlobDir())
			if err != nil || len(entries) != 0 || pages.Load() != 1 || images.Load() != 1 {
				t.Fatalf("产生重复请求或孤立图片：pages=%d images=%d files=%d err=%v", pages.Load(), images.Load(), len(entries), err)
			}
		})
	}
}
