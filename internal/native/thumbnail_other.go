//go:build !darwin

package native

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"

	_ "golang.org/x/image/bmp"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// Thumbnail 从磁盘读取图片并保留方向按最长边缩小为 RGBA
func Thumbnail(path string, maxSide int) (*image.RGBA, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if format == "jpeg" {
		if o := jpegOrientation(data); o > 1 {
			img = orientedImage{Image: img, orientation: o}
		}
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	if side := max(w, h); side > maxSide {
		w, h = max(1, w*maxSide/side), max(1, h*maxSide/side)
	}
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.ApproxBiLinear.Scale(out, out.Bounds(), img, img.Bounds(), draw.Src, nil)
	return out, nil
}

// jpegOrientation 读取 JPEG 的 EXIF 方向并校验元数据范围
func jpegOrientation(data []byte) int {
	for p := 2; p+4 <= len(data) && data[p] == 0xff; {
		size := int(binary.BigEndian.Uint16(data[p+2:]))
		if data[p+1] == 0xda || size < 2 || p+2+size > len(data) {
			break
		}
		seg := data[p+4 : p+2+size]
		if data[p+1] == 0xe1 && len(seg) >= 14 && string(seg[:6]) == "Exif\x00\x00" {
			tiff := seg[6:]
			var order binary.ByteOrder
			switch string(tiff[:2]) {
			case "II":
				order = binary.LittleEndian
			case "MM":
				order = binary.BigEndian
			default:
				return 1
			}
			at := int(order.Uint32(tiff[4:]))
			if at < 8 || at+2 > len(tiff) {
				return 1
			}
			n := int(order.Uint16(tiff[at:]))
			for at += 2; n > 0 && at+12 <= len(tiff); at, n = at+12, n-1 {
				if order.Uint16(tiff[at:]) == 0x112 && order.Uint16(tiff[at+2:]) == 3 && order.Uint32(tiff[at+4:]) == 1 {
					o := int(order.Uint16(tiff[at+8:]))
					if o >= 1 && o <= 8 {
						return o
					}
				}
			}
			return 1
		}
		p += 2 + size
	}
	return 1
}

// orientedImage 按 EXIF 方向映射原图像素以直接缩放到目标尺寸
type orientedImage struct {
	image.Image
	orientation int
}

// Bounds 返回按 EXIF 方向调整后的像素范围
func (img orientedImage) Bounds() image.Rectangle {
	b := img.Image.Bounds()
	if img.orientation >= 5 {
		return image.Rect(0, 0, b.Dy(), b.Dx())
	}
	return b
}

// At 返回方向变换后位置对应的原图像素
func (img orientedImage) At(x, y int) color.Color {
	b := img.Image.Bounds()
	w, h := b.Dx(), b.Dy()
	switch img.orientation {
	case 2:
		x = w - 1 - x
	case 3:
		x, y = w-1-x, h-1-y
	case 4:
		y = h - 1 - y
	case 5:
		x, y = y, x
	case 6:
		x, y = y, h-1-x
	case 7:
		x, y = w-1-y, h-1-x
	case 8:
		x, y = w-1-y, x
	}
	return img.Image.At(x+b.Min.X, y+b.Min.Y)
}
