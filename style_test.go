package main

// style_test.go — 本文件为 QRScanner 自己写的代码，不是微软的代码。
// It renders a few personalised symbols and decodes them again with gozxing,
// so a pretty code is only accepted when a scanner can still read it.

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/oned"
	"github.com/makiuchi-d/gozxing/qrcode"
)

func decodeAsQR(img image.Image) (string, error) {
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return "", err
	}
	res, err := qrcode.NewQRCodeReader().Decode(bmp, map[gozxing.DecodeHintType]interface{}{
		gozxing.DecodeHintType_TRY_HARDER: true,
	})
	if err != nil {
		return "", err
	}
	return res.GetText(), nil
}

func decodeAsCode128(img image.Image) (string, error) {
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return "", err
	}
	res, err := oned.NewCode128Reader().Decode(bmp, map[gozxing.DecodeHintType]interface{}{
		gozxing.DecodeHintType_TRY_HARDER: true,
	})
	if err != nil {
		return "", err
	}
	return res.GetText(), nil
}

func TestStyledQRStillDecodes(t *testing.T) {
	plain := DefaultStyle()

	coloured := DefaultStyle()
	coloured.Foreground = color.RGBA{0x11, 0x2E, 0x6A, 255}
	coloured.Background = color.RGBA{0xFF, 0xF7, 0xE3, 255}
	coloured.Cell = 12

	dots := coloured
	dots.Shape = "dot"
	dots.Eye = "circle"

	rounded := coloured
	rounded.Shape = "rounded"
	rounded.Eye = "rounded"

	diamond := coloured
	diamond.Shape = "diamond"
	diamond.Eye = "rounded"

	framed := rounded
	framed.Border = "thin"
	framed.TopText = "Test caption"
	framed.BottomText = "second line"

	cases := []struct {
		name string
		opt  StyleOptions
		text string
	}{
		{"plain", plain, "https://example.com/plain"},
		{"coloured", coloured, "https://example.com/colour"},
		{"dots+circle", dots, "https://example.com/dots"},
		{"rounded", rounded, "https://example.com/rounded"},
		{"diamond", diamond, "https://example.com/diamond"},
		{"captions+frame", framed, "https://example.com/framed"},
	}

	for _, c := range cases {
		img, err := RenderCode(c.text, c.opt)
		if err != nil {
			t.Fatalf("%s: render failed: %v", c.name, err)
		}
		got, err := decodeAsQR(img)
		if err != nil {
			t.Fatalf("%s: does not decode any more: %v", c.name, err)
		}
		if got != c.text {
			t.Fatalf("%s: decoded %q, want %q", c.name, got, c.text)
		}
		t.Logf("%s ok (%dx%d)", c.name, img.Bounds().Dx(), img.Bounds().Dy())
	}
}

func TestLogoQRStillDecodes(t *testing.T) {
	dir := t.TempDir()
	logoPath := filepath.Join(dir, "avatar.png")
	f, err := os.Create(logoPath)
	if err != nil {
		t.Fatal(err)
	}
	avatar := image.NewRGBA(image.Rect(0, 0, 64, 64))
	draw.Draw(avatar, avatar.Bounds(), image.NewUniform(color.RGBA{200, 60, 60, 255}), image.Point{}, draw.Src)
	draw.Draw(avatar, image.Rect(12, 12, 52, 52), image.NewUniform(color.RGBA{255, 255, 255, 255}), image.Point{}, draw.Src)
	if err := png.Encode(f, avatar); err != nil {
		t.Fatal(err)
	}
	f.Close()

	o := DefaultStyle()
	o.LogoPath = logoPath
	o.LogoRatio = 0.22
	o.Shape = "rounded"
	o.Eye = "rounded"

	text := "https://example.com/with-avatar"
	img, err := RenderCode(text, o)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	got, err := decodeAsQR(img)
	if err != nil {
		t.Fatalf("avatar QR does not decode: %v", err)
	}
	if got != text {
		t.Fatalf("decoded %q, want %q", got, text)
	}
}

func TestColoured1DStillDecodes(t *testing.T) {
	o := DefaultStyle()
	o.Kind = "code128"
	o.Foreground = color.RGBA{0x0A, 0x3D, 0x91, 255}
	o.Background = color.RGBA{0xFF, 0xFF, 0xEE, 255}
	o.BarWidth = 3
	o.BarHeight = 120
	o.TopText = "SKU 0001"
	o.BottomText = "made in test"
	o.Border = "thin"

	text := "QRSCANNER-TEST-0001"
	img, err := RenderCode(text, o)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	got, err := decodeAsCode128(img)
	if err != nil {
		t.Fatalf("1D does not decode: %v", err)
	}
	if got != text {
		t.Fatalf("decoded %q, want %q", got, text)
	}
}

func TestHexColourParsing(t *testing.T) {
	cases := map[string]color.RGBA{
		"#FF0000": {255, 0, 0, 255},
		"00ff00":  {0, 255, 0, 255},
		"#123":    {0x11, 0x22, 0x33, 255},
		"#00000000": {0, 0, 0, 0},
	}
	for in, want := range cases {
		got, err := ParseHexColor(in)
		if err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if got != want {
			t.Fatalf("%s: got %v, want %v", in, got, want)
		}
	}
	if _, err := ParseHexColor("nope"); err == nil {
		t.Fatal("garbage colour should fail")
	}
}
