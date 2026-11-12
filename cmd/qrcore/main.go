package main

// qrcore.dll — 本文件为 QRScanner 自己写的代码，不是微软的代码，也不含任何微软源码。
// qrcore.dll — written for this project; no Microsoft code is contained here.
//
// This is the C interface of the rendering engine. qrscanner.exe no longer draws
// anything itself: it calls these three exports through LoadLibrary/GetProcAddress,
// and so can any other program (Python ctypes, C#, Rust, ...).
//
// The whole request travels as text so nothing Go-specific crosses the boundary:
//
//	qrcore_render(specText, content, outPngPath) -> 0 on success
//	specText is "key=value" lines, see style.ParseSpec.
//
// Every export writes its result into a caller-provided buffer, so no memory
// allocated by this DLL is ever handed out to be freed elsewhere.

/*
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"image/png"
	"os"
	"sync"
	"unsafe"

	"qrscanner/style"
)

var (
	mu      sync.Mutex
	lastErr string
)

func setErr(err error) C.int {
	mu.Lock()
	defer mu.Unlock()
	if err == nil {
		lastErr = ""
		return 0
	}
	lastErr = err.Error()
	return 1
}

// writeBuf copies s into the caller's NUL-terminated buffer and returns its byte length.
func writeBuf(buf *C.char, n C.int, s string) C.int {
	if buf == nil || n <= 0 {
		return 0
	}
	limit := int(n) - 1
	if len(s) > limit {
		s = s[:limit]
	}
	dst := unsafe.Slice((*byte)(unsafe.Pointer(buf)), int(n))
	copy(dst, s)
	dst[len(s)] = 0
	return C.int(len(s))
}

//export QRCoreVersion
func QRCoreVersion(buf *C.char, n C.int) C.int {
	return writeBuf(buf, n, "qrcore 0.2")
}

//export QRCoreLastError
func QRCoreLastError(buf *C.char, n C.int) C.int {
	mu.Lock()
	s := lastErr
	mu.Unlock()
	return writeBuf(buf, n, s)
}

//export QRCoreRender
func QRCoreRender(spec *C.char, content *C.char, outPath *C.char) C.int {
	if spec == nil || content == nil || outPath == nil {
		return setErr(fmt.Errorf("null argument"))
	}
	o := style.ParseSpec(C.GoString(spec))
	img, err := style.RenderCode(C.GoString(content), o)
	if err != nil {
		return setErr(err)
	}
	f, err := os.Create(C.GoString(outPath))
	if err != nil {
		return setErr(err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return setErr(err)
	}
	if err := f.Close(); err != nil {
		return setErr(err)
	}
	return setErr(nil)
}

//export QRCoreRenderSize
func QRCoreRenderSize(spec *C.char, content *C.char, w *C.int, h *C.int) C.int {
	if spec == nil || content == nil || w == nil || h == nil {
		return setErr(fmt.Errorf("null argument"))
	}
	img, err := style.RenderCode(C.GoString(content), style.ParseSpec(C.GoString(spec)))
	if err != nil {
		return setErr(err)
	}
	b := img.Bounds()
	*w = C.int(b.Dx())
	*h = C.int(b.Dy())
	return setErr(nil)
}

func main() {}
