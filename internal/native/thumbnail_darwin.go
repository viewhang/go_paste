//go:build darwin

package native

import (
	"errors"
	"image"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// imageRect 描述 Core Graphics 绘制图片的目标区域
type imageRect struct{ X, Y, W, H float64 }

// thumbnailRGBA 指定预乘透明度且以 RGBA 顺序存储像素
const thumbnailRGBA = 1 | 4<<12

// imageIOOnce 初始化图片降采样所需的系统接口
var imageIOOnce sync.Once

var (
	imageSourceCreate    func(uintptr, uintptr) uintptr
	imageSourceThumbnail func(uintptr, uintptr, uintptr) uintptr
	imageWidth           func(uintptr) int
	imageHeight          func(uintptr) int
	imageRelease         func(uintptr)
	colorSpaceCreate     func() uintptr
	colorSpaceRelease    func(uintptr)
	bitmapContextCreate  func(unsafe.Pointer, int, int, int, int, uintptr, uint32) uintptr
	contextDrawImage     func(uintptr, imageRect, uintptr)
	contextRelease       func(uintptr)
	cfRelease            func(uintptr)
	imageIOKeys          [4]uintptr
	cfFalse              uintptr
)

// loadImageIO 注册 ImageIO 降采样与 RGBA 转换接口
func loadImageIO() {
	imageIOOnce.Do(func() {
		load()
		lib := mustDlopen("/System/Library/Frameworks/ImageIO.framework/ImageIO")
		purego.RegisterLibFunc(&imageSourceCreate, lib, "CGImageSourceCreateWithURL")
		purego.RegisterLibFunc(&imageSourceThumbnail, lib, "CGImageSourceCreateThumbnailAtIndex")
		purego.RegisterLibFunc(&imageWidth, libCG, "CGImageGetWidth")
		purego.RegisterLibFunc(&imageHeight, libCG, "CGImageGetHeight")
		purego.RegisterLibFunc(&imageRelease, libCG, "CGImageRelease")
		purego.RegisterLibFunc(&colorSpaceCreate, libCG, "CGColorSpaceCreateDeviceRGB")
		purego.RegisterLibFunc(&colorSpaceRelease, libCG, "CGColorSpaceRelease")
		purego.RegisterLibFunc(&bitmapContextCreate, libCG, "CGBitmapContextCreate")
		purego.RegisterLibFunc(&contextDrawImage, libCG, "CGContextDrawImage")
		purego.RegisterLibFunc(&contextRelease, libCG, "CGContextRelease")
		purego.RegisterLibFunc(&cfRelease, libCF, "CFRelease")
		for i, name := range []string{"kCGImageSourceShouldCache", "kCGImageSourceCreateThumbnailFromImageAlways", "kCGImageSourceCreateThumbnailWithTransform", "kCGImageSourceThumbnailMaxPixelSize"} {
			imageIOKeys[i] = *(*uintptr)(unsafe.Pointer(mustDlsym(lib, name)))
		}
		cfFalse = *(*uintptr)(unsafe.Pointer(mustDlsym(libCF, "kCFBooleanFalse")))
	})
}

// Thumbnail 从磁盘按最长边降采样图片并保留透明度与方向
func Thumbnail(path string, maxSide int) (out *image.RGBA, err error) {
	loadImageIO()
	withPool(func() {
		str := nsString(path)
		defer send(str, "release")
		url := send(class("NSURL"), "fileURLWithPath:", uintptr(str))
		opts := send(class("NSMutableDictionary"), "dictionary")
		send(opts, "setObject:forKey:", cfFalse, imageIOKeys[0])
		source := imageSourceCreate(uintptr(url), uintptr(opts))
		if source == 0 {
			err = errors.New("native: cannot open image")
			return
		}
		defer cfRelease(source)
		yes := send(class("NSNumber"), "numberWithBool:", 1)
		send(opts, "setObject:forKey:", uintptr(yes), imageIOKeys[1])
		send(opts, "setObject:forKey:", uintptr(yes), imageIOKeys[2])
		size := send(class("NSNumber"), "numberWithInteger:", uintptr(maxSide))
		send(opts, "setObject:forKey:", uintptr(size), imageIOKeys[3])
		img := imageSourceThumbnail(source, 0, uintptr(opts))
		if img == 0 {
			err = errors.New("native: cannot decode thumbnail")
			return
		}
		defer imageRelease(img)
		w, h := imageWidth(img), imageHeight(img)
		out = image.NewRGBA(image.Rect(0, 0, w, h))
		space := colorSpaceCreate()
		defer colorSpaceRelease(space)
		ctx := bitmapContextCreate(unsafe.Pointer(&out.Pix[0]), w, h, 8, out.Stride, space, thumbnailRGBA)
		if ctx == 0 {
			out, err = nil, errors.New("native: cannot create image context")
			return
		}
		defer contextRelease(ctx)
		contextDrawImage(ctx, imageRect{W: float64(w), H: float64(h)}, img)
		runtime.KeepAlive(out)
	})
	return
}
