package app

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/draw"
	"io"
	"time"

	"github.com/egoist/mygo/ui"
	"golang.org/x/image/riff"
	"golang.org/x/image/webp"
)

// webPDecoder 逐帧解码 WebP 并合成透明画布供播放器复用
type webPDecoder struct {
	data    []byte
	chunks  *riff.Reader
	canvas  *image.RGBA
	dispose image.Rectangle
}

// newWebPDecoder 根据 WebP 画布尺寸创建动画解码器
func newWebPDecoder(data []byte) (*webPDecoder, error) {
	cfg, err := webp.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	d := &webPDecoder{data: data, canvas: image.NewRGBA(image.Rect(0, 0, cfg.Width, cfg.Height))}
	if err := d.Rewind(); err != nil {
		return nil, err
	}
	return d, nil
}

// Next 按帧偏移、透明混合与清除标志合成下一帧并返回显示时长
func (d *webPDecoder) Next() (ui.VideoFrame, error) {
	for {
		id, _, r, err := d.chunks.Next()
		if err != nil {
			return ui.VideoFrame{}, err
		}
		if string(id[:]) != "ANMF" {
			continue
		}
		data, err := io.ReadAll(r)
		if err != nil {
			return ui.VideoFrame{}, err
		}
		if len(data) < 16 {
			return ui.VideoFrame{}, errors.New("webp: incomplete animation frame")
		}
		x, y := webPUint24(data)*2, webPUint24(data[3:])*2
		w, h := webPUint24(data[6:])+1, webPUint24(data[9:])+1
		rect := image.Rect(x, y, x+w, y+h)
		if !rect.In(d.canvas.Bounds()) {
			return ui.VideoFrame{}, errors.New("webp: frame exceeds canvas")
		}
		img, err := decodeWebPFrame(data)
		if err != nil {
			return ui.VideoFrame{}, err
		}
		draw.Draw(d.canvas, d.dispose, image.Transparent, image.Point{}, draw.Src)
		op := draw.Over
		if data[15]&2 != 0 {
			op = draw.Src
		}
		draw.Draw(d.canvas, rect, img, img.Bounds().Min, op)
		d.dispose = image.Rectangle{}
		if data[15]&1 != 0 {
			d.dispose = rect
		}
		delay := time.Duration(webPUint24(data[12:])) * time.Millisecond
		if delay <= 10*time.Millisecond {
			delay = 100 * time.Millisecond
		}
		return ui.VideoFrame{Image: d.canvas, Duration: delay}, nil
	}
}

// Rewind 重置帧读取位置并清空画布以重新播放动画
func (d *webPDecoder) Rewind() error {
	_, chunks, err := riff.NewReader(bytes.NewReader(d.data))
	if err != nil {
		return err
	}
	d.chunks, d.dispose = chunks, image.Rectangle{}
	clear(d.canvas.Pix)
	return nil
}

// decodeWebPFrame 为动画帧补齐静态 WebP 头并交给现有解码库处理
func decodeWebPFrame(data []byte) (image.Image, error) {
	var header [30]byte
	copy(header[:], "RIFF")
	binary.LittleEndian.PutUint32(header[4:], uint32(len(data)-16+22))
	copy(header[8:], "WEBPVP8X")
	binary.LittleEndian.PutUint32(header[16:], 10)
	copy(header[24:30], data[6:12])
	_, chunks, err := riff.NewListReader(uint32(len(data)-16+4), io.MultiReader(bytes.NewReader(header[8:12]), bytes.NewReader(data[16:])))
	if err != nil {
		return nil, err
	}
	for {
		id, _, _, err := chunks.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if string(id[:]) == "ALPH" {
			header[20] = 0x10
		}
	}
	return webp.Decode(io.MultiReader(bytes.NewReader(header[:]), bytes.NewReader(data[16:])))
}

// webPUint24 读取 WebP 帧头中的二十四位小端整数
func webPUint24(data []byte) int {
	return int(data[0]) | int(data[1])<<8 | int(data[2])<<16
}
