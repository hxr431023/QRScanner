package main

// style.go — 本文件为 QRScanner 自己写的代码，不是微软的代码，也不含任何微软源码。
// style.go — written for this project; no Microsoft code is contained here.
//
// This file renders the "personalised" symbols: custom colours, module shapes,
// styled finder eyes, a centred avatar/logo, captions above and below the symbol,
// and an optional frame. The bit matrix itself still comes from boombuler/barcode —
// only the drawing is done here, so every symbol stays decodable.
//
// Colours, primitives and text now live in the reusable `paint` package; this
// file keeps only the symbol-specific layout and composition.

import (
	"errors"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"github.com/boombuler/barcode/code39"
	"github.com/boombuler/barcode/ean"
	"github.com/boombuler/barcode/qr"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"

	"qrscanner/paint"
)

// StyleOptions collects every personalisation knob for one rendered symbol.
type StyleOptions struct {
	Kind string // qr | code128 | code39 | ean13

	Foreground color.RGBA
	Background color.RGBA
	Transparent bool // ignore Background and keep the pixels transparent

	Shape string // square | rounded | dot | diamond  (QR only)
	Eye   string // square | rounded | circle        (QR only)
	ECC   qr.ErrorCorrectionLevel

	Cell  int // pixels per QR module
	Quiet int // quiet zone in modules

	BarWidth  int // pixels per narrow bar unit (1D)
	BarHeight int // bar height in pixels (1D)

	TopText    string
	BottomText string

	Border      string // none | thin | thick
	BorderColor color.RGBA

	LogoPath  string
	LogoRatio float64 // logo width as a fraction of the symbol width
}

// DefaultStyle is the plain black-on-white look the app shipped with.
func DefaultStyle() StyleOptions {
	return StyleOptions{
		Kind:        "qr",
		Foreground:  color.RGBA{0, 0, 0, 255},
		Background:  color.RGBA{255, 255, 255, 255},
		Shape:       "square",
		Eye:         "square",
		ECC:         qr.M,
		Cell:        10,
		Quiet:       4,
		BarWidth:    2,
		BarHeight:   140,
		Border:      "none",
		BorderColor: color.RGBA{0, 0, 0, 255},
		LogoRatio:   0.22,
	}
}

// ParseHexColor accepts "#RRGGBB", "RRGGBB", "#RGB" and "RRGGBBAA".
// It is a thin alias so the rest of the app keeps one colour entry point.
func ParseHexColor(s string) (color.RGBA, error) { return paint.ParseHex(s) }

// RenderCode builds the final image for the given content and style.
func RenderCode(content string, o StyleOptions) (image.Image, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, errors.New("empty content")
	}
	if o.Cell <= 0 {
		o.Cell = 10
	}
	if o.Quiet < 0 {
		o.Quiet = 4
	}
	if o.BarWidth <= 0 {
		o.BarWidth = 2
	}
	if o.BarHeight <= 0 {
		o.BarHeight = 140
	}
	// An avatar covers real modules, so the code needs spare capacity to survive.
	if o.LogoPath != "" && o.ECC < qr.Q {
		o.ECC = qr.H
	}
	if o.Kind == "" || o.Kind == "qr" {
		return renderQR(content, o)
	}
	return render1D(content, o)
}

// ---- QR ----

func renderQR(content string, o StyleOptions) (image.Image, error) {
	bc, err := qr.Encode(content, o.ECC, qr.Auto)
	if err != nil {
		return nil, err
	}
	dim := bc.Bounds().Dx()
	dark := func(x, y int) bool {
		r, g, b, _ := bc.At(x, y).RGBA()
		return r < 0x8000 && g < 0x8000 && b < 0x8000
	}

	cell := o.Cell
	codeW := (dim + 2*o.Quiet) * cell

	lay := layoutFor(o, codeW, codeW)
	img := newCanvas(lay.width, lay.height, o)

	// quiet zone is part of the symbol, so it is filled with the background colour
	fillRect(img, lay.code, bgColor(o))

	for y := 0; y < dim; y++ {
		for x := 0; x < dim; x++ {
			if !dark(x, y) {
				continue
			}
			px := lay.code.Min.X + (o.Quiet+x)*cell
			py := lay.code.Min.Y + (o.Quiet+y)*cell
			drawModule(img, px, py, cell, o.Shape, o.Foreground)
		}
	}

	drawEyes(img, lay.code, dim, o)
	drawLogo(img, lay.code, o)
	drawCaptions(img, lay, o)
	return img, nil
}

// drawModule paints one dark module in the requested shape.
func drawModule(img *image.RGBA, x, y, cell int, shape string, c color.RGBA) {
	switch shape {
	case "dot":
		fillCircle(img, x+cell/2, y+cell/2, cell, c)
	case "rounded":
		fillRoundRect(img, image.Rect(x, y, x+cell, y+cell), float64(cell)/4, c)
	case "diamond":
		fillDiamond(img, image.Rect(x, y, x+cell, y+cell), c)
	default:
		fillRect(img, image.Rect(x, y, x+cell, y+cell), c)
	}
}

// drawEyes redraws the three finder patterns so they can be rounded or circular.
// The inner pattern is cleared first, otherwise the square modules underneath
// would show through the round shape.
func drawEyes(img *image.RGBA, code image.Rectangle, dim int, o StyleOptions) {
	if o.Eye == "" || o.Eye == "square" {
		return
	}
	cell := o.Cell
	corners := [][2]int{{0, 0}, {dim - 7, 0}, {0, dim - 7}}
	for _, c := range corners {
		x0 := code.Min.X + (o.Quiet+c[0])*cell
		y0 := code.Min.Y + (o.Quiet+c[1])*cell
		area := image.Rect(x0, y0, x0+7*cell, y0+7*cell)
		fillRect(img, area, bgColor(o))

		outer := image.Rect(x0, y0, x0+7*cell, y0+7*cell)
		inner := image.Rect(x0+cell, y0+cell, x0+6*cell, y0+6*cell)
		core := image.Rect(x0+2*cell, y0+2*cell, x0+5*cell, y0+5*cell)

		switch o.Eye {
		case "circle":
			// A pure circle erases the straight edges a scanner uses to locate the
			// pattern, so this keeps flat sides and rounds everything else.
			fillRoundRect(img, outer, float64(2*cell), o.Foreground)
			fillRoundRect(img, inner, float64(cell), bgColor(o))
			fillRoundRect(img, core, float64(cell)/2, o.Foreground)
		case "rounded":
			fillRoundRect(img, outer, float64(3*cell)/2, o.Foreground)
			fillRoundRect(img, inner, float64(cell), bgColor(o))
			fillRoundRect(img, core, float64(cell)/2, o.Foreground)
		default:
			fillRect(img, outer, o.Foreground)
			fillRect(img, inner, bgColor(o))
			fillRect(img, core, o.Foreground)
		}
	}
}

// drawLogo stamps an avatar in the middle, on a plate that keeps the surrounding
// modules readable.
func drawLogo(img *image.RGBA, code image.Rectangle, o StyleOptions) {
	if strings.TrimSpace(o.LogoPath) == "" {
		return
	}
	logo, err := loadImageFile(o.LogoPath)
	if err != nil {
		return
	}
	ratio := o.LogoRatio
	if ratio <= 0 {
		ratio = 0.22
	}
	if ratio > 0.35 {
		ratio = 0.35
	}
	box := int(float64(code.Dx()) * ratio)
	if box < 16 {
		box = 16
	}
	pad := box / 10
	cx := (code.Min.X + code.Max.X) / 2
	cy := (code.Min.Y + code.Max.Y) / 2

	plate := image.Rect(cx-box/2-pad, cy-box/2-pad, cx+box/2+pad, cy+box/2+pad)
	plateColor := o.Background
	if o.Transparent || plateColor.A == 0 {
		plateColor = color.RGBA{255, 255, 255, 255}
	}
	fillRoundRect(img, plate, float64(pad+box/8), plateColor)

	dst := image.Rect(cx-box/2, cy-box/2, cx+box/2, cy+box/2)
	xdraw.CatmullRom.Scale(img, dst, logo, logo.Bounds(), draw.Over, nil)
}

// ---- 1D ----

func render1D(content string, o StyleOptions) (image.Image, error) {
	var (
		bc  barcode.Barcode
		err error
	)
	switch o.Kind {
	case "code39":
		bc, err = code39.Encode(content, true, true)
	case "ean13":
		bc, err = ean.Encode(content)
	default:
		bc, err = code128.Encode(content)
	}
	if err != nil {
		return nil, err
	}

	bits := bc.Bounds().Dx()
	barW := o.BarWidth
	// A 1D symbol needs a quiet zone of at least ten narrow bars on both sides,
	// otherwise a scanner cannot find where the symbol starts.
	qz := o.Quiet
	if qz < 10 {
		qz = 10
	}
	codeW := (bits + 2*qz) * barW
	codeH := o.BarHeight

	lay := layoutFor(o, codeW, codeH)
	img := newCanvas(lay.width, lay.height, o)
	fillRect(img, lay.code, bgColor(o))

	y0 := lay.code.Min.Y
	x0 := lay.code.Min.X + qz*barW
	for x := 0; x < bits; x++ {
		r, g, b, _ := bc.At(x, 0).RGBA()
		if r >= 0x8000 || g >= 0x8000 || b >= 0x8000 {
			continue
		}
		fillRect(img, image.Rect(x0+x*barW, y0,
			x0+(x+1)*barW, y0+codeH), o.Foreground)
	}

	drawCaptions(img, lay, o)
	return img, nil
}

// ---- layout ----

type layout struct {
	width, height int
	code          image.Rectangle // where the symbol itself is drawn
	topY          int             // top caption baseline, 0 when unused
	botY          int
	captionSize   int
}

const (
	canvasMargin = 8
	captionGap   = 8
)

// layoutFor reserves room for the frame and the two captions.
func layoutFor(o StyleOptions, codeW, codeH int) layout {
	border := 0
	switch o.Border {
	case "thin":
		border = 10
	case "thick":
		border = 22
	}

	lay := layout{}

	captionSize := codeW / 22
	if captionSize < 12 {
		captionSize = 12
	}
	if captionSize > 40 {
		captionSize = 40
	}
	lay.captionSize = captionSize

	capH := 0
	if strings.TrimSpace(o.TopText) != "" || strings.TrimSpace(o.BottomText) != "" {
		face := faceFor(captionSize)
		capH = paint.Height(face) + captionGap
	}

	innerW := codeW + 2*border
	innerH := codeH + 2*border + 2*capH
	lay.width = innerW + 2*canvasMargin
	lay.height = innerH + 2*canvasMargin

	x0 := canvasMargin + border
	y0 := canvasMargin + border
	if strings.TrimSpace(o.TopText) != "" {
		lay.topY = y0 + capH/2
		y0 += capH
	}
	if strings.TrimSpace(o.BottomText) != "" {
		lay.botY = y0 + codeH + capH/2
	}
	lay.code = image.Rect(x0, y0, x0+codeW, y0+codeH)
	return lay
}

// newCanvas allocates the image and paints the background and the frame.
func newCanvas(w, h int, o StyleOptions) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	fillRect(img, img.Bounds(), bgColor(o))
	if o.Border == "thin" || o.Border == "thick" {
		frame := img.Bounds().Inset(canvasMargin)
		w := 0
		if o.Border == "thin" {
			w = 4
		} else {
			w = 8
		}
		strokeRect(img, frame, w, o.BorderColor)
	}
	return img
}

// ---- captions ----

func drawCaptions(img *image.RGBA, lay layout, o StyleOptions) {
	drawCaption(img, lay, lay.topY, strings.TrimSpace(o.TopText), o.Foreground)
	drawCaption(img, lay, lay.botY, strings.TrimSpace(o.BottomText), o.Foreground)
}

func drawCaption(img *image.RGBA, lay layout, baseline int, text string, c color.RGBA) {
	if text == "" || baseline == 0 {
		return
	}
	face := faceFor(lay.captionSize)
	w := paint.Width(face, text)
	x := (lay.width - w) / 2
	if x < 0 {
		x = 0
	}
	paint.Draw(img, face, text, x, baseline+lay.captionSize/3, c)
}

// ---- fonts ----

// faceFor returns a face at the requested pixel size. The real loading lives in
// the reusable paint package; this keeps the call sites in this file short.
func faceFor(size int) font.Face { return paint.Face(size) }

// ---- primitives (thin wrappers over the reusable paint package) ----

func fillRect(dst *image.RGBA, r image.Rectangle, c color.RGBA) {
	paint.Rect(dst, r, c)
}

func fillCircle(dst *image.RGBA, cx, cy, d int, c color.RGBA) {
	paint.Circle(dst, cx, cy, d, c)
}

func fillDiamond(dst *image.RGBA, r image.Rectangle, c color.RGBA) {
	paint.Diamond(dst, r, c)
}

func fillRoundRect(dst *image.RGBA, r image.Rectangle, radius float64, c color.RGBA) {
	paint.RoundRect(dst, r, radius, c)
}

func strokeRect(dst *image.RGBA, r image.Rectangle, w int, c color.RGBA) {
	paint.StrokeRect(dst, r, w, c)
}

// bgColor resolves the paint used for "empty" pixels: the background colour, or
// a fully transparent pixel when the user asked for a transparent background.
func bgColor(o StyleOptions) color.RGBA {
	if o.Transparent {
		return color.RGBA{0, 0, 0, 0}
	}
	return o.Background
}

func loadImageFile(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}
