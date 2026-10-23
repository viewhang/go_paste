package app

import (
	"bytes"
	"container/list"
	"image"
	"math"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"golang.org/x/image/draw"

	"gopaste/internal/assets"
	"gopaste/internal/native"
)

const (
	// thumbnailBudget 限制缩略图与图标及其缩小层级的解码缓存
	thumbnailBudget = 12 << 20
	// thumbnailMaxSide 限制列表单张缩略图的最长像素边
	thumbnailMaxSide = 512
	// previewMaxSide 限制当前预览及其缩小层级合计不超过 8 MiB
	previewMaxSide = 1200
)

// bitmapKey 区分图片来源与目标像素尺寸
type bitmapKey struct {
	name string
	side int
}

// cachedBitmap 保存位图与包含缩小层级的解码字节预算
type cachedBitmap struct {
	key   bitmapKey
	image *ui.Bitmap
	bytes int
}

// bitmapCache 按来源复用位图并以最近使用顺序限制解码字节数
type bitmapCache struct {
	items map[string]*list.Element
	order list.List
	bytes int
}

// get 复用满足目标尺寸的缓存并更新最近使用顺序
func (b *bitmapCache) get(key bitmapKey) (*ui.Bitmap, bool) {
	e, ok := b.items[key.name]
	if !ok || e.Value.(cachedBitmap).key.side < key.side {
		return nil, false
	}
	b.order.MoveToFront(e)
	return e.Value.(cachedBitmap).image, true
}

// put 替换同一来源的位图并淘汰超出预算的最久未使用项
func (b *bitmapCache) put(key bitmapKey, img *ui.Bitmap) *ui.Bitmap {
	if b.items == nil {
		b.items = make(map[string]*list.Element)
	}
	entry := cachedBitmap{key: key, image: img, bytes: bitmapBytes(img)}
	if e, ok := b.items[key.name]; ok {
		b.bytes -= e.Value.(cachedBitmap).bytes
		e.Value = entry
		b.order.MoveToFront(e)
	} else {
		b.items[key.name] = b.order.PushFront(entry)
	}
	b.bytes += entry.bytes
	b.trim(thumbnailBudget)
	return img
}

// trim 将缓存缩减至指定解码预算并在零预算时移除全部条目
func (b *bitmapCache) trim(budget int) {
	for b.order.Len() > 0 && (b.bytes > budget || budget == 0) {
		e := b.order.Back()
		entry := e.Value.(cachedBitmap)
		delete(b.items, entry.key.name)
		b.bytes -= entry.bytes
		b.order.Remove(e)
	}
}

// bitmapBytes 计算 RGBA 位图及全部缩小层级的像素字节数
func bitmapBytes(img *ui.Bitmap) int {
	if img == nil {
		return 0
	}
	w, h := img.Size()
	n := 4 * w * h
	for w > 1 || h > 1 {
		w, h = max(1, (w+1)/2), max(1, (h+1)/2)
		n += 4 * w * h
	}
	return n
}

// imagePixels 将展示尺寸换算为当前屏幕的目标像素尺寸
func (a *App) imagePixels(size float32) int {
	return max(1, int(math.Ceil(float64(size)*max(a.imageScale, 1))))
}

// bitmapSide 将目标像素尺寸向上对齐到六十四像素档位并限制最长边
func bitmapSide(side, limit int) int {
	return min(limit, (side+63)/64*64)
}

// thumb 按屏幕倍率与尺寸档位读取缩略图并在缩小时复用已有位图
func (a *App) thumb(name string, size float32) *ui.Bitmap {
	if name == "" {
		return nil
	}
	key := bitmapKey{name: name, side: bitmapSide(a.imagePixels(size), thumbnailMaxSide)}
	if b, ok := a.images.get(key); ok {
		return b
	}
	img, err := native.Thumbnail(a.st.BlobPath(name), key.side)
	if err != nil {
		return a.images.put(key, nil)
	}
	return a.images.put(key, ui.NewBitmap(img))
}

// previewBitmap 按尺寸档位降采样当前预览并在缩小时复用已有位图
func (a *App) previewBitmap(name string, size float32) *ui.Bitmap {
	if name == "" {
		return nil
	}
	key := bitmapKey{name: name, side: bitmapSide(a.imagePixels(size), previewMaxSide)}
	if a.previewImage.key.name != key.name || a.previewImage.key.side < key.side {
		a.previewImage = cachedBitmap{key: key}
		if img, err := native.Thumbnail(a.st.BlobPath(name), key.side); err == nil {
			a.previewImage.image = ui.NewBitmap(img)
			a.previewImage.bytes = bitmapBytes(a.previewImage.image)
		}
	}
	return a.previewImage.image
}

// appMark 缩小并缓存引导页展示的应用图标
func (a *App) appMark() *ui.Bitmap {
	if a.obWin != nil {
		a.imageScale = mygo.Screen.DisplayMatching(a.obWin.Bounds()).ScaleFactor
	}
	key := bitmapKey{name: "app:GoPaste", side: a.imagePixels(48)}
	if b, ok := a.images.get(key); ok {
		return b
	}
	img, _, err := image.Decode(bytes.NewReader(assets.AppIcon))
	if err != nil {
		return a.images.put(key, nil)
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	if side := max(w, h); side > key.side {
		w, h = max(1, w*key.side/side), max(1, h*key.side/side)
	}
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.ApproxBiLinear.Scale(out, out.Bounds(), img, img.Bounds(), draw.Src, nil)
	return a.images.put(key, ui.NewBitmap(out))
}

// fileIcon 读取并缓存受统一字节预算限制的文件图标
func (a *App) fileIcon(path string) *ui.Bitmap {
	if path == "" || !shellReady {
		return nil
	}
	key := bitmapKey{name: "file:" + path, side: a.imagePixels(40)}
	if b, ok := a.images.get(key); ok {
		return b
	}
	var b *ui.Bitmap
	if data, err := mygo.Shell.FileIcon(path, key.side); err == nil {
		b, _ = ui.DecodeBitmap(data)
	}
	return a.images.put(key, b)
}

// releaseImages 释放应用持有的列表缓存与当前预览
func (a *App) releaseImages() {
	a.previewImage = cachedBitmap{}
	a.images.trim(0)
}
