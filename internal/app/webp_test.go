package app

import (
	"bytes"
	"image/color"
	"io"
	"os"
	"testing"
	"time"
)

// TestWebPDecoder 验证有损与无损帧的偏移、混合、清除、时长及循环重置
func TestWebPDecoder(t *testing.T) {
	data, err := os.ReadFile("testdata/preview.webp")
	if err != nil {
		t.Fatal(err)
	}
	d, err := newWebPDecoder(data)
	if err != nil {
		t.Fatal(err)
	}
	red, blue := color.RGBA{R: 255, A: 255}, color.RGBA{B: 255, A: 255}
	clear, white := color.RGBA{}, color.RGBA{R: 255, G: 255, B: 255, A: 255}
	translucent := color.RGBA{R: 118, G: 118, B: 118, A: 128}
	for i, tc := range []struct {
		delay  time.Duration
		pixels [4]color.RGBA
	}{
		{100, [4]color.RGBA{red, red, red, clear}},
		{120, [4]color.RGBA{blue, blue, blue, blue}},
		{80, [4]color.RGBA{blue, {R: 128, B: 127, A: 255}, blue, blue}},
		{100, [4]color.RGBA{blue, clear, translucent, blue}},
		{60, [4]color.RGBA{white, clear, translucent, blue}},
	} {
		frame, err := d.Next()
		if err != nil || frame.Duration != tc.delay*time.Millisecond {
			t.Fatalf("第 %d 帧时长错误：%v，%v", i, frame.Duration, err)
		}
		for j, xy := range [][2]int{{0, 0}, {2, 2}, {4, 4}, {7, 7}} {
			if got := d.canvas.RGBAAt(xy[0], xy[1]); got != tc.pixels[j] {
				t.Fatalf("第 %d 帧 %v 的像素错误：%v，期望 %v", i, xy, got, tc.pixels[j])
			}
		}
	}
	if _, err := d.Next(); err != io.EOF {
		t.Fatalf("动画结尾错误：%v", err)
	}
	if err := d.Rewind(); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Next(); err != nil || d.canvas.RGBAAt(0, 0) != red || d.canvas.RGBAAt(7, 7) != clear {
		t.Fatalf("循环播放未恢复首帧或清空画布：%v", err)
	}
	data[bytes.Index(data, []byte("ANMF"))+8+6] = 8
	d, err = newWebPDecoder(data)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Next(); err == nil {
		t.Fatal("超出画布的动画帧未返回错误")
	}
}
