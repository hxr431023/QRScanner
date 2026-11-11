package paint

// paint_test.go — 本文件为本项目自己写的代码，不是微软的代码。
// Proves the reusable drawing package works on its own, with no app context.

import (
	"image"
	"image/color"
	"testing"
)

func TestParseHex(t *testing.T) {
	cases := map[string]color.RGBA{
		"#FF0000":   {255, 0, 0, 255},
		"00ff00":    {0, 255, 0, 255},
		"#123":      {0x11, 0x22, 0x33, 255},
		"#00000000": {0, 0, 0, 0},
	}
	for in, want := range cases {
		got, err := ParseHex(in)
		if err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if got != want {
			t.Fatalf("%s: got %v want %v", in, got, want)
		}
	}
	if _, err := ParseHex("nope"); err == nil {
		t.Fatal("garbage should fail")
	}
	if Hex(color.RGBA{0x12, 0x34, 0x56, 255}) != "#123456" {
		t.Fatal("Hex round trip failed")
	}
}

func TestPrimitivesStayInsideTheImage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 20, 20))
	Rect(img, image.Rect(-50, -50, 500, 500), color.RGBA{255, 0, 0, 255})
	Circle(img, 10, 10, 8, color.RGBA{0, 0, 255, 255})
	RoundRect(img, image.Rect(2, 2, 18, 18), 6, color.RGBA{0, 255, 0, 255})
	Diamond(img, image.Rect(2, 2, 18, 18), color.RGBA{255, 255, 0, 255})
	StrokeRect(img, image.Rect(1, 1, 19, 19), 1, color.RGBA{0, 0, 0, 255})
	if img.Bounds() != image.Rect(0, 0, 20, 20) {
		t.Fatal("bounds changed")
	}
}

func TestFaceIsUsable(t *testing.T) {
	f := Face(16)
	if f == nil {
		t.Fatal("no face")
	}
	if Width(f, "Hello") <= 0 || Height(f) <= 0 {
		t.Fatal("face reports zero metrics")
	}
}
