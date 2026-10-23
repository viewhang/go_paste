// Package linkprev fetches a web page's title and preview image for link
// records, when the user enables link previews.
package linkprev

import (
	"bytes"
	"context"
	"errors"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	metaTag  = regexp.MustCompile(`(?is)<meta\s+[^>]*>`)
	titleTag = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	baseTag  = regexp.MustCompile(`(?is)<base\s+[^>]*href=["']([^"']+)["']`)
	attrRe   = regexp.MustCompile(`(?is)([a-zA-Z:-]+)\s*=\s*["']([^"']*)["']`)
	spaceRe  = regexp.MustCompile(`\s+`)
)

// Result is what a page offers as a preview.
type Result struct {
	Title    string
	SiteName string
	ImageURL string
}

// Fetch reads the page at raw and returns its title and preview image URL.
func Fetch(ctx context.Context, raw string) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15 GoPaste/0.1")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return Result{}, errors.New("link preview: " + resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Result{}, err
	}
	page := decode(body)

	var res Result
	base := raw
	if m := baseTag.FindStringSubmatch(page); m != nil {
		if u, err := url.Parse(m[1]); err == nil {
			if abs := resolve(raw, u.String()); abs != "" {
				base = abs
			}
		}
	}

	metas := map[string]string{}
	for _, tag := range metaTag.FindAllString(page, -1) {
		attrs := map[string]string{}
		for _, a := range attrRe.FindAllStringSubmatch(tag, -1) {
			attrs[strings.ToLower(a[1])] = a[2]
		}
		key := strings.ToLower(attrs["property"])
		if key == "" {
			key = strings.ToLower(attrs["name"])
		}
		if key == "" || attrs["content"] == "" {
			continue
		}
		if _, ok := metas[key]; !ok {
			metas[key] = attrs["content"]
		}
	}

	res.Title = firstNonEmpty(metas["og:title"], metas["twitter:title"])
	if m := titleTag.FindStringSubmatch(page); res.Title == "" && m != nil {
		res.Title = strings.TrimSpace(m[1])
	}
	res.Title = clean(res.Title)
	res.SiteName = clean(firstNonEmpty(metas["og:site_name"], metas["application-name"]))
	img := firstNonEmpty(metas["og:image"], metas["og:image:url"], metas["twitter:image"], metas["twitter:image:src"])
	if img != "" {
		res.ImageURL = resolve(base, img)
	}
	return res, nil
}

// DownloadImage fetches a preview image, returning its bytes and extension.
func DownloadImage(ctx context.Context, raw string) ([]byte, string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 GoPaste/0.1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, "", errors.New("link preview image: " + resp.Status)
	}
	ctype := strings.ToLower(resp.Header.Get("Content-Type"))
	if !strings.HasPrefix(ctype, "image/") {
		return nil, "", errors.New("link preview: not an image")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, "", err
	}
	ext := "jpg"
	switch {
	case strings.Contains(ctype, "png"):
		ext = "png"
	case strings.Contains(ctype, "gif"):
		ext = "gif"
	case strings.Contains(ctype, "webp"):
		ext = "webp"
	case strings.Contains(ctype, "svg"):
		ext = "svg"
	}
	return data, ext, nil
}

func resolve(base, ref string) string {
	b, err := url.Parse(base)
	if err != nil {
		return ""
	}
	r, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	u := b.ResolveReference(r)
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	return u.String()
}

func clean(s string) string {
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return strings.TrimSpace(spaceRe.ReplaceAllString(s, " "))
}

// decode strips a UTF-8 BOM and keeps pages that are valid UTF-8. Pages in
// other encodings are kept as is, which leaves their ASCII structure usable.
func decode(b []byte) string {
	b = bytes.TrimPrefix(b, []byte("\xef\xbb\xbf"))
	if utf8.Valid(b) {
		return string(b)
	}
	return strings.ToValidUTF8(string(b), "")
}

func firstNonEmpty(list ...string) string {
	for _, s := range list {
		if strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}
