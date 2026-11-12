package style

// spec_test.go — 本文件为 QRScanner 自己写的代码，不是微软的代码。
// The render request crossing the qrcore.dll boundary is plain text, so the text
// format itself needs a test: a spec that parses wrongly would produce a code that
// still looks fine but no longer decodes.

import (
	"image"
	"image/color"
	"testing"

	"github.com/boombuler/barcode/qr"
	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

func TestParseSpec(t *testing.T) {
	text := "code_kind=code128\r\n" +
		"fg_color=#0A3D91\n" +
		"bg_color=FFFFEE\n" +
		"transparent=1\n" +
		"shape=dot\n" +
		"eye=circle\n" +
		"ecc=H\n" +
		"cell=14\n" +
		"quiet=6\n" +
		"bar_width=3\n" +
		"bar_height=120\n" +
		"top_text=hello\n" +
		"bottom_text=world\n" +
		"border=thick\n" +
		"border_color=#112E6A\n" +
		"logo_ratio=0.28\n" +
		"unknown_key=ignored\n" +
		"# a comment line\n"

	o := ParseSpec(text)
	if o.Kind != "code128" {
		t.Fatalf("kind = %q", o.Kind)
	}
	if o.Foreground != (color.RGBA{0x0A, 0x3D, 0x91, 255}) {
		t.Fatalf("foreground = %v", o.Foreground)
	}
	if o.Background.R != 0xFF || o.Background.G != 0xFF || o.Background.B != 0xEE {
		t.Fatalf("background = %v", o.Background)
	}
	if !o.Transparent {
		t.Fatal("transparent should be set")
	}
	if o.Shape != "dot" || o.Eye != "circle" || o.Border != "thick" {
		t.Fatalf("shape/eye/border = %q/%q/%q", o.Shape, o.Eye, o.Border)
	}
	if o.Cell != 14 || o.Quiet != 6 || o.BarWidth != 3 || o.BarHeight != 120 {
		t.Fatalf("numbers = %d/%d/%d/%d", o.Cell, o.Quiet, o.BarWidth, o.BarHeight)
	}
	if o.TopText != "hello" || o.BottomText != "world" {
		t.Fatalf("captions = %q/%q", o.TopText, o.BottomText)
	}
	if o.LogoRatio != 0.28 {
		t.Fatalf("logo ratio = %v", o.LogoRatio)
	}
}

func TestEmptySpecKeepsDefaults(t *testing.T) {
	o := ParseSpec("")
	d := DefaultStyle()
	if o != d {
		t.Fatalf("empty spec changed defaults: %+v", o)
	}
}

func TestSpecRenderStillDecodes(t *testing.T) {
	text := "code_kind=qr\n" +
		"fg_color=#112E6A\n" +
		"bg_color=#FFF7E3\n" +
		"shape=rounded\n" +
		"eye=rounded\n" +
		"ecc=H\n" +
		"cell=12\n" +
		"quiet=4\n" +
		"top_text=spec round trip\n"

	o := ParseSpec(text)
	payload := "https://example.com/spec-round-trip"
	img, err := RenderCode(payload, o)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	got, err := decodeQRImage(img)
	if err != nil {
		t.Fatalf("does not decode: %v", err)
	}
	if got != payload {
		t.Fatalf("decoded %q, want %q", got, payload)
	}
}

func TestEncodeSpecRoundTrip(t *testing.T) {
	o := DefaultStyle()
	o.Kind = "code39"
	o.Shape = "diamond"
	o.Eye = "circle"
	o.ECC = qr.Q
	o.Cell = 9
	o.TopText = "round trip"
	o.Border = "thin"

	back := ParseSpec(EncodeSpec(o))
	if back.Kind != o.Kind || back.Shape != o.Shape || back.Eye != o.Eye ||
		back.Cell != o.Cell || back.TopText != o.TopText || back.Border != o.Border {
		t.Fatalf("round trip lost fields:\n%+v\n%+v", o, back)
	}
	if back.ECC.String() != o.ECC.String() {
		t.Fatalf("ecc = %v, want %v", back.ECC, o.ECC)
	}
}

func decodeQRImage(img image.Image) (string, error) {
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
