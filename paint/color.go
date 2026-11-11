// Package paint is the drawing part of this project: colours, primitives and
// text. It has no knowledge of QR codes or barcodes, so any project that has to
// put a custom image on screen can reuse it.
//
// paint/color.go — 本文件为本项目自己写的代码，不是微软的代码。
// This file is written for this project; no Microsoft code is used here.
package paint

import (
	"fmt"
	"image/color"
	"strings"
)

// ParseHex accepts "#RRGGBB", "RRGGBB", "#RGB" and "RRGGBBAA".
func ParseHex(s string) (color.RGBA, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "#")
	switch len(s) {
	case 3:
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]}) + "FF"
	case 6:
		s = s + "FF"
	case 8:
	default:
		return color.RGBA{}, fmt.Errorf("bad colour %q", s)
	}
	var r, g, b, a uint8
	if _, err := fmt.Sscanf(strings.ToUpper(s), "%02X%02X%02X%02X", &r, &g, &b, &a); err != nil {
		return color.RGBA{}, fmt.Errorf("bad colour %q", s)
	}
	return color.RGBA{R: r, G: g, B: b, A: a}, nil
}

// Hex renders a colour as "#RRGGBB".
func Hex(c color.RGBA) string {
	return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
}
