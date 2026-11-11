//go:build ignore

// 例の文書(examples/md/blocks.bx.md・examples/ir/blocks.json)が使う画像を作る。
// 外の画像は使わない。単純な図形だけの架空の図を、標準ライブラリで PNG・JPEG に書く。
//
//	go run tools/gen_example_images.go    # リポジトリの根で
//
// 同じ画像を examples/md/images/ と examples/ir/images/ の両方に書く(src は文書からの
// 相対パスで、文書のフォルダの外(..)は書けないため)。
package main

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
)

type canvas struct{ *image.RGBA }

func newCanvas(w, h int, bg color.RGBA) canvas {
	c := canvas{image.NewRGBA(image.Rect(0, 0, w, h))}
	c.rect(0, 0, w, h, bg)
	return c
}

func (c canvas) rect(x, y, w, h int, col color.RGBA) {
	for j := y; j < y+h; j++ {
		for i := x; i < x+w; i++ {
			c.SetRGBA(i, j, col)
		}
	}
}

func (c canvas) circle(cx, cy, r int, col color.RGBA) {
	for j := cy - r; j <= cy+r; j++ {
		for i := cx - r; i <= cx+r; i++ {
			if (i-cx)*(i-cx)+(j-cy)*(j-cy) <= r*r {
				c.SetRGBA(i, j, col)
			}
		}
	}
}

var (
	navy  = color.RGBA{0x1F, 0x3A, 0x6B, 0xFF}
	blue  = color.RGBA{0x3D, 0x7D, 0xD8, 0xFF}
	sky   = color.RGBA{0xC9, 0xDD, 0xF7, 0xFF}
	paper = color.RGBA{0xF4, 0xF6, 0xFA, 0xFF}
	green = color.RGBA{0x2E, 0xA0, 0x6E, 0xFF}
	amber = color.RGBA{0xE8, 0xA3, 0x3D, 0xFF}
	rose  = color.RGBA{0xD0, 0x55, 0x5E, 0xFF}
	gray  = color.RGBA{0xA8, 0xB2, 0xC4, 0xFF}
	white = color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}
)

// landscape は横長(16:9)の架空の画面:上の帯・左の欄・棒の並び。
func landscape() image.Image {
	c := newCanvas(960, 540, paper)
	c.rect(0, 0, 960, 64, navy)
	c.circle(40, 32, 14, amber)
	c.rect(0, 64, 200, 476, sky)
	for i := 0; i < 5; i++ {
		c.rect(24, 100+i*56, 152, 28, white)
	}
	heights := []int{180, 260, 220, 330, 300, 380}
	for i, h := range heights {
		c.rect(260+i*110, 470-h, 70, h, blue)
	}
	c.rect(240, 470, 680, 4, gray)
	return c
}

// portrait は縦長(2:3)の架空の手持ちの端末の画面:札の並び。
func portrait() image.Image {
	c := newCanvas(600, 900, navy)
	c.rect(40, 40, 520, 820, paper)
	c.rect(40, 40, 520, 90, blue)
	cols := []color.RGBA{green, amber, rose, blue}
	for i, col := range cols {
		y := 170 + i*170
		c.rect(80, y, 440, 130, white)
		c.circle(140, y+65, 36, col)
		c.rect(200, y+40, 280, 18, gray)
		c.rect(200, y+76, 200, 14, sky)
	}
	return c
}

// step は手順の絵(4:3)。n 個の丸のうち k 番目までを塗る。
func step(k int, accent color.RGBA) image.Image {
	c := newCanvas(480, 360, sky)
	c.rect(30, 30, 420, 300, white)
	for i := 0; i < 3; i++ {
		col := gray
		if i < k {
			col = accent
		}
		c.circle(120+i*120, 150, 40, col)
	}
	c.rect(80, 250, 320*k/3, 24, accent)
	c.rect(80+320*k/3, 250, 320-320*k/3, 24, gray)
	return c
}

func main() {
	files := map[string]func() ([]byte, error){
		"screen-wide.png": func() ([]byte, error) { return encPNG(landscape()) },
		"device-tall.jpg": func() ([]byte, error) { return encJPEG(portrait()) },
		"step-1.png":      func() ([]byte, error) { return encPNG(step(1, green)) },
		"step-2.png":      func() ([]byte, error) { return encPNG(step(2, blue)) },
		"step-3.jpg":      func() ([]byte, error) { return encJPEG(step(3, amber)) },
	}
	for _, dir := range []string{"examples/md/images", "examples/ir/images"} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			panic(err)
		}
		for name, f := range files {
			b, err := f()
			if err != nil {
				panic(err)
			}
			if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
				panic(err)
			}
		}
	}
}

func encPNG(img image.Image) ([]byte, error) {
	var b bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	err := enc.Encode(&b, img)
	return b.Bytes(), err
}

func encJPEG(img image.Image) ([]byte, error) {
	var b bytes.Buffer
	err := jpeg.Encode(&b, img, &jpeg.Options{Quality: 80})
	return b.Bytes(), err
}
