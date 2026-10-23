package app

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"time"

	"gopaste/internal/linkprev"
	"gopaste/internal/store"
	"gopaste/internal/types"
)

// scheduleLinkPreviews 调度尚未获取标题与图片的链接预览
func (a *App) scheduleLinkPreviews() {
	if a.st == nil || !a.st.Settings().LinkPreview {
		return
	}
	records, err := a.st.PendingLinks()
	if err != nil {
		log.Print("reading pending previews: ", err)
		return
	}
	for _, r := range records {
		if r.Kind == types.KindLink && r.PreviewState == "" && r.URL != "" {
			go a.fetchPreview(r.ID, r.URL)
		}
	}
}

// fetchPreview 在共享的并发限制内获取链接标题与图片并刷新面板
func (a *App) fetchPreview(id, url string) {
	if a.st == nil || !a.st.Settings().LinkPreview {
		return
	}
	a.previewSlots <- struct{}{}
	defer func() { <-a.previewSlots }()
	if !a.st.Settings().LinkPreview {
		return
	}
	started := false
	_, ok := a.st.Update(id, func(r *types.Record) {
		if r.PreviewState == "" {
			r.PreviewState = "loading"
			started = true
		}
	})
	if !ok || !started {
		return
	}
	a.refresh()

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	res, err := linkprev.Fetch(ctx, url)
	if err != nil {
		a.st.Update(id, func(r *types.Record) { r.PreviewState = "failed" })
		a.refresh()
		return
	}
	var image []byte
	var ext string
	if res.ImageURL != "" {
		image, ext, _ = linkprev.DownloadImage(ctx, res.ImageURL)
	}
	apply := func(r *types.Record) {
		if res.Title != "" {
			r.Title = res.Title
		}
		if res.SiteName != "" && r.Source.AppName == "" {
			r.Source.AppName = res.SiteName
		}
		r.PreviewState = "done"
	}
	_, _, err = a.st.UpdatePreview(id, image, ext, apply)
	if errors.Is(err, store.ErrHistoryLimit) {
		_, _, err = a.st.UpdatePreview(id, nil, "", apply)
	}
	if err != nil && !errors.Is(err, fs.ErrClosed) {
		log.Print("saving link preview: ", err)
	}
	a.refresh()
}
