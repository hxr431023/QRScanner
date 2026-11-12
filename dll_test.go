package main

// dll_test.go — 本文件为 QRScanner 自己写的代码，不是微软的代码。
// These tests run the real DLLs from the repository directory, so a broken
// export, a changed signature or a wrong spec key shows up here instead of in
// the window. They skip cleanly when the DLLs have not been built yet.

import (
	"image"
	_ "image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the repository root")
	}
	return filepath.Dir(file)
}

func prepare(t *testing.T) string {
	t.Helper()
	root := repoRoot(t)
	baseDirOverride = root
	for _, name := range []string{"qrcore.dll", "qrnative.dll"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Skipf("%s is not built yet: %v", name, err)
		}
	}
	if err := initNative(); err != nil {
		t.Skipf("qrnative.dll: %v", err)
	}
	if err := initCore(); err != nil {
		t.Skipf("qrcore.dll: %v", err)
	}
	return root
}

func TestNativeDLLExports(t *testing.T) {
	prepare(t)

	if !have(capLog) || !have(capConfig) || !have(capTmmf) || !have(capLerf) {
		t.Fatalf("capabilities = %#b, want all four", caps)
	}

	if lang := configGet("language"); lang == "" {
		t.Fatal("configGet(language) returned nothing")
	}

	loadLang()
	if got := tr("1"); got == "" || got == "1" {
		t.Fatalf("translate(1) = %q", got)
	}

	// This export used to take the whole process down; keep it covered.
	setLogEnabled(true)
	setLogEnabled(false)
}

func TestRenderThroughCoreDLL(t *testing.T) {
	root := prepare(t)

	cases := []struct {
		name    string
		content string
		style   map[string]string
	}{
		{
			name:    "qr-styled",
			content: "https://example.com/through-the-dll",
			style: map[string]string{
				"code_kind": "qr",
				"fg_color":  "#112E6A",
				"bg_color":  "#FFF7E3",
				"shape":     "rounded",
				"eye":       "rounded",
				"ecc":       "H",
				"cell":      "12",
				"quiet":     "4",
				"top_text":  "dll render",
			},
		},
		{
			name:    "qr-plain",
			content: "plain payload",
			style: map[string]string{
				"code_kind": "qr",
				"cell":      "10",
			},
		},
	}

	for _, tc := range cases {
		out := filepath.Join(root, ".tmp-"+tc.name+".png")
		if err := renderToFile(specText(tc.style), tc.content, out); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		size := fileSize(t, out)
		if size < 500 {
			t.Fatalf("%s: png is only %d bytes", tc.name, size)
		}
		if tc.style["code_kind"] == "qr" {
			if got := decodePNG(t, out); got != tc.content {
				t.Fatalf("%s: decoded %q, want %q", tc.name, got, tc.content)
			}
		}
		_ = os.Remove(out)
	}
}

func TestOneDRendersThroughCoreDLL(t *testing.T) {
	root := prepare(t)

	out := filepath.Join(root, ".tmp-1d.png")
	style := map[string]string{
		"code_kind":  "code128",
		"fg_color":   "#0A3D91",
		"bg_color":   "#FFFFEE",
		"bar_width":  "3",
		"bar_height": "120",
		"top_text":   "SKU 0001",
		"quiet":      "10",
	}
	if err := renderToFile(specText(style), "QRSCANNER-TEST-0001", out); err != nil {
		t.Fatalf("code128: %v", err)
	}
	if size := fileSize(t, out); size < 500 {
		t.Fatalf("code128 png is only %d bytes", size)
	}
	_ = os.Remove(out)
}

func fileSize(t *testing.T, path string) int64 {
	t.Helper()
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return st.Size()
}

func decodePNG(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		t.Fatalf("bitmap: %v", err)
	}
	res, err := qrcode.NewQRCodeReader().Decode(bmp, nil)
	if err != nil {
		t.Fatalf("gozxing: %v", err)
	}
	return res.GetText()
}
