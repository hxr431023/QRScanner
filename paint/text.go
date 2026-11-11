// paint/text.go — 本文件为本项目自己写的代码，不是微软的代码。
// This file is written for this project; no Microsoft code is used here.
//
// Text rendering on Windows without a third-party font engine: a TrueType face
// is loaded once from the system font directory and drawn with golang.org/x/image
// font.Drawer, which gives anti-aliased glyphs. A Chinese-capable face is tried
// first so captions can be Chinese.
package paint

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

var (
	mu       sync.Mutex
	faces    = map[int]font.Face{}
	once     sync.Once
	ttf      *sfnt.Font
	faceErr  error
)

// candidates are tried in order. simhei ships with every Simplified Chinese
// Windows install, so Chinese text works out of the box.
var candidates = []string{"simhei.ttf", "msyh.ttc", "simsun.ttc", "arial.ttf", "segoeui.ttf"}

// Face returns a face for the requested pixel size. When no TrueType font can
// be loaded it falls back to the built-in bitmap face, so text still renders.
func Face(size int) font.Face {
	if size < 8 {
		size = 8
	}
	mu.Lock()
	defer mu.Unlock()
	if f, ok := faces[size]; ok {
		return f
	}
	once.Do(func() {
		root := os.Getenv("SystemRoot")
		if root == "" {
			root = `C:\Windows`
		}
		dir := filepath.Join(root, "Fonts")
		for _, name := range candidates {
			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				continue
			}
			if strings.EqualFold(filepath.Ext(name), ".ttc") {
				col, err := sfnt.ParseCollection(data)
				if err != nil || col.NumFonts() == 0 {
					continue
				}
				if f, err := col.Font(0); err == nil {
					ttf = f
					return
				}
				continue
			}
			if f, err := sfnt.Parse(data); err == nil {
				ttf = f
				return
			}
		}
	})
	if ttf == nil {
		f := basicfont.Face7x13
		faces[size] = f
		return f
	}
	f, err := opentype.NewFace(ttf, &opentype.FaceOptions{
		Size:    float64(size),
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		faceErr = err
		faces[size] = basicfont.Face7x13
		return basicfont.Face7x13
	}
	faces[size] = f
	return f
}

// Width measures a string in pixels.
func Width(face font.Face, s string) int {
	return font.MeasureString(face, s).Round()
}

// Height is the vertical space a line needs.
func Height(face font.Face) int {
	return face.Metrics().Height.Round()
}

// Draw writes text with its baseline at y, starting at x.
func Draw(dst *image.RGBA, face font.Face, s string, x, y int, c color.RGBA) {
	if s == "" {
		return
	}
	d := &font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(c),
		Face: face,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(s)
}

// DrawBlock draws a single line centred horizontally inside r, vertically
// centred as well. It returns the line height used.
func DrawBlock(dst *image.RGBA, face font.Face, s string, r image.Rectangle, c color.RGBA) int {
	h := Height(face)
	x := r.Min.X + (r.Dx()-Width(face, s))/2
	baseline := r.Min.Y + (r.Dy()-h)/2 + face.Metrics().Ascent.Round()
	Draw(dst, face, s, x, baseline, c)
	return h
}
