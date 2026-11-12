package style

// spec.go — 本文件为 QRScanner 自己写的代码，不是微软的代码，也不含任何微软源码。
//
// The C interface of qrcore.dll cannot pass a Go struct across the boundary, so a
// render request travels as plain text: one "key=value" pair per line, key names
// identical to the settings keys the app already writes to settings.tmmf. Every
// language can build that string with its own string formatting, which keeps the
// DLL reusable outside this project.

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/boombuler/barcode/qr"
	"qrscanner/paint"
)

// ParseSpec turns a "key=value" text block into a StyleOptions. Unknown keys are
// ignored and every value falls back to DefaultStyle when it is missing or bad,
// so a partial spec is legal.
func ParseSpec(text string) StyleOptions {
	o := DefaultStyle()
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		switch k {
		case "code_kind", "kind":
			o.Kind = v
		case "fg_color", "fg":
			if c, err := paint.ParseHex(v); err == nil {
				o.Foreground = c
			}
		case "bg_color", "bg":
			if c, err := paint.ParseHex(v); err == nil {
				o.Background = c
			}
		case "border_color":
			if c, err := paint.ParseHex(v); err == nil {
				o.BorderColor = c
			}
		case "transparent":
			o.Transparent = v == "1" || strings.EqualFold(v, "true")
		case "shape":
			o.Shape = v
		case "eye":
			o.Eye = v
		case "ecc":
			o.ECC = parseECC(v)
		case "cell":
			o.Cell = atoiOr(v, o.Cell)
		case "quiet":
			o.Quiet = atoiOr(v, o.Quiet)
		case "bar_width":
			o.BarWidth = atoiOr(v, o.BarWidth)
		case "bar_height":
			o.BarHeight = atoiOr(v, o.BarHeight)
		case "top_text":
			o.TopText = v
		case "bottom_text":
			o.BottomText = v
		case "border":
			o.Border = v
		case "logo_path":
			o.LogoPath = v
		case "logo_ratio":
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				o.LogoRatio = f
			}
		}
	}
	return o
}

// EncodeSpec is the inverse of ParseSpec, useful for logging and cache keys.
func EncodeSpec(o StyleOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "code_kind=%s\n", o.Kind)
	fmt.Fprintf(&b, "fg_color=%s\n", paint.Hex(o.Foreground))
	fmt.Fprintf(&b, "bg_color=%s\n", paint.Hex(o.Background))
	fmt.Fprintf(&b, "transparent=%d\n", boolInt(o.Transparent))
	fmt.Fprintf(&b, "shape=%s\n", o.Shape)
	fmt.Fprintf(&b, "eye=%s\n", o.Eye)
	fmt.Fprintf(&b, "ecc=%s\n", o.ECC.String())
	fmt.Fprintf(&b, "cell=%d\n", o.Cell)
	fmt.Fprintf(&b, "quiet=%d\n", o.Quiet)
	fmt.Fprintf(&b, "bar_width=%d\n", o.BarWidth)
	fmt.Fprintf(&b, "bar_height=%d\n", o.BarHeight)
	fmt.Fprintf(&b, "top_text=%s\n", o.TopText)
	fmt.Fprintf(&b, "bottom_text=%s\n", o.BottomText)
	fmt.Fprintf(&b, "border=%s\n", o.Border)
	fmt.Fprintf(&b, "border_color=%s\n", paint.Hex(o.BorderColor))
	fmt.Fprintf(&b, "logo_ratio=%.2f\n", o.LogoRatio)
	if o.LogoPath != "" {
		fmt.Fprintf(&b, "logo_path=%s\n", o.LogoPath)
	}
	return b.String()
}

func parseECC(v string) qr.ErrorCorrectionLevel {
	switch strings.ToUpper(strings.TrimSpace(v)) {
	case "L":
		return qr.L
	case "Q":
		return qr.Q
	case "H":
		return qr.H
	}
	return qr.M
}

func atoiOr(s string, fallback int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return n
	}
	return fallback
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
