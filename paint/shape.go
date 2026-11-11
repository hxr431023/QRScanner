// paint/shape.go — 本文件为本项目自己写的代码，不是微软的代码。
// This file is written for this project; no Microsoft code is used here.
//
// Filled primitives drawn straight onto an *image.RGBA. Edges are hard (no anti
// aliasing) on purpose: a symbol stays crisp and a decoder samples the centre of
// each cell, so a soft edge only costs contrast.
package paint

import (
	"image"
	"image/color"
	"image/draw"
)

// Rect paints a rectangle. A colour with alpha 0 clears the pixels instead,
// which is how a transparent background is produced.
func Rect(dst *image.RGBA, r image.Rectangle, c color.RGBA) {
	r = r.Intersect(dst.Bounds())
	if r.Empty() {
		return
	}
	if c.A == 0 {
		draw.Draw(dst, r, image.Transparent, image.Point{}, draw.Src)
		return
	}
	draw.Draw(dst, r, image.NewUniform(c), image.Point{}, draw.Src)
}

// Circle fills a circle of diameter d centred on (cx, cy).
func Circle(dst *image.RGBA, cx, cy, d int, c color.RGBA) {
	rad := float64(d) / 2
	for y := cy - d/2; y <= cy+d/2; y++ {
		for x := cx - d/2; x <= cx+d/2; x++ {
			dx := float64(x) + 0.5 - float64(cx)
			dy := float64(y) + 0.5 - float64(cy)
			if dx*dx+dy*dy <= rad*rad {
				dst.SetRGBA(x, y, c)
			}
		}
	}
}

// Ring fills the band between two diameters.
func Ring(dst *image.RGBA, cx, cy, outer, inner int, c color.RGBA) {
	for y := cy - outer; y <= cy+outer; y++ {
		for x := cx - outer; x <= cx+outer; x++ {
			dx := float64(x) + 0.5 - float64(cx)
			dy := float64(y) + 0.5 - float64(cy)
			dd := dx*dx + dy*dy
			if dd <= float64(outer*outer) && dd >= float64(inner*inner) {
				dst.SetRGBA(x, y, c)
			}
		}
	}
}

// Diamond fills the rectangle with its corners cut to a diamond.
func Diamond(dst *image.RGBA, r image.Rectangle, c color.RGBA) {
	cx := (r.Min.X + r.Max.X) / 2
	cy := (r.Min.Y + r.Max.Y) / 2
	hw := float64(r.Dx()) / 2
	hh := float64(r.Dy()) / 2
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			dx := float64(x) + 0.5 - float64(cx)
			dy := float64(y) + 0.5 - float64(cy)
			if abs(dx)/hw+abs(dy)/hh <= 1.0 {
				dst.SetRGBA(x, y, c)
			}
		}
	}
}

// RoundRect fills a rectangle with rounded corners. A radius larger than half
// the shorter side is clamped, so callers cannot produce a broken shape.
func RoundRect(dst *image.RGBA, r image.Rectangle, radius float64, c color.RGBA) {
	r = r.Intersect(dst.Bounds())
	if r.Empty() {
		return
	}
	w := float64(r.Dx())
	h := float64(r.Dy())
	if radius <= 0 || radius > w/2 || radius > h/2 {
		radius = 0
	}
	cx0 := float64(r.Min.X) + radius
	cy0 := float64(r.Min.Y) + radius
	cx1 := float64(r.Max.X) - radius
	cy1 := float64(r.Max.Y) - radius
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			px := float64(x) + 0.5
			py := float64(y) + 0.5
			if radius == 0 || (px >= cx0 && px <= cx1) || (py >= cy0 && py <= cy1) {
				dst.SetRGBA(x, y, c)
				continue
			}
			var ccx, ccy float64
			switch {
			case px < cx0 && py < cy0:
				ccx, ccy = cx0, cy0
			case px > cx1 && py < cy0:
				ccx, ccy = cx1, cy0
			case px < cx0:
				ccx, ccy = cx0, cy1
			default:
				ccx, ccy = cx1, cy1
			}
			dx := px - ccx
			dy := py - ccy
			if dx*dx+dy*dy <= radius*radius {
				dst.SetRGBA(x, y, c)
			}
		}
	}
}

// StrokeRect draws a rectangle outline of width w.
func StrokeRect(dst *image.RGBA, r image.Rectangle, w int, c color.RGBA) {
	if w <= 0 {
		return
	}
	Rect(dst, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+w), c)
	Rect(dst, image.Rect(r.Min.X, r.Max.Y-w, r.Max.X, r.Max.Y), c)
	Rect(dst, image.Rect(r.Min.X, r.Min.Y, r.Min.X+w, r.Max.Y), c)
	Rect(dst, image.Rect(r.Max.X-w, r.Min.Y, r.Max.X, r.Max.Y), c)
}

// ClearRect paints the area with the given colour, treating alpha 0 as
// "make transparent".
func ClearRect(dst *image.RGBA, r image.Rectangle, c color.RGBA) { Rect(dst, r, c) }

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
