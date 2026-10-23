package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// TestIconFile 验证应用图标保留 ICNS 尺寸、PNG 内容与透明边缘
func TestIconFile(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	col := color.NRGBA{R: 24, G: 140, B: 240, A: 255}
	for y := 2; y < 6; y++ {
		for x := 2; x < 6; x++ {
			src.SetNRGBA(x, y, col)
		}
	}
	var input bytes.Buffer
	if err := png.Encode(&input, src); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "icon.png")
	if err := os.WriteFile(path, input.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := iconFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 8 || string(data[:4]) != "icns" || int(binary.BigEndian.Uint32(data[4:8])) != len(data) {
		t.Fatal("ICNS 文件头或长度不正确")
	}
	at := 8
	for i, kind := range []string{"ic07", "ic08", "ic09", "ic10"} {
		if at+8 > len(data) || string(data[at:at+4]) != kind {
			t.Fatalf("缺少图标尺寸 %s", kind)
		}
		n := int(binary.BigEndian.Uint32(data[at+4 : at+8]))
		if n < 8 || at+n > len(data) {
			t.Fatal("图标数据长度不正确")
		}
		img, err := png.Decode(bytes.NewReader(data[at+8 : at+n]))
		size := 128 << i
		if err != nil || img.Bounds() != image.Rect(0, 0, size, size) {
			t.Fatalf("图标尺寸 %s 无法正确读取：%v", kind, err)
		}
		if center := color.NRGBAModel.Convert(img.At(size/2, size/2)); center != col {
			t.Fatalf("图标主体颜色改变：%v", center)
		}
		if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
			t.Fatal("图标透明边缘丢失")
		}
		at += n
	}
	if at != len(data) {
		t.Fatal("ICNS 包含多余数据")
	}
}
