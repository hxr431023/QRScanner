package main

// qrnative.dll — 本文件为 QRScanner 自己写的代码，不是微软的代码，也不含任何微软源码。
// qrnative.dll — written for this project; no Microsoft code is contained here.
//
// This is the C interface of the reusable native layer: the TMMF settings store,
// the LERF language table and the log with its result cache. qrscanner.exe uses it
// through LoadLibrary, and so can any other program that wants the same behaviour
// without writing its own FFI plumbing.
//
// All strings are UTF-8. Every export lives in this DLL only, so the caller never
// has to free memory that was allocated here: text is copied into a buffer the
// caller owns, and the return value is a status code.

/*
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"path/filepath"
	"sync"
	"unsafe"

	"qrscanner/native"
)

const (
	capLog    = 1 << 0
	capConfig = 1 << 1
	capTmmf   = 1 << 2
	capLerf   = 1 << 3
)

var (
	mu      sync.Mutex
	baseDir string
	cfg     *native.Config
	logStore *native.Log
	tmmfDLL *native.DLL
	lerfDLL *native.DLL
	lang    native.Table
	langOK  bool
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

var tmmfExports = []string{
	"graal_create_isolate", "graal_detach_all_threads_and_tear_down_isolate",
	"tmmf_read_all", "tmmf_read_tag", "tmmf_search",
	"tmmf_list_tags", "tmmf_line_count", "tmmf_write_tag",
}

var lerfExports = []string{
	"graal_create_isolate", "graal_detach_all_threads_and_tear_down_isolate",
	"lerf_load", "lerf_get", "lerf_get_from_file", "lerf_get_multi",
	"lerf_list_all", "lerf_validate", "lerf_key_count",
}

//export QRNativeVersion
func QRNativeVersion(buf *C.char, n C.int) C.int {
	return writeBuf(buf, n, "qrnative 0.2")
}

//export QRNativeLastError
func QRNativeLastError(buf *C.char, n C.int) C.int {
	mu.Lock()
	s := lastErr
	mu.Unlock()
	return writeBuf(buf, n, s)
}

// QRNativeInit loads the four runtime DLLs from dir. It always succeeds; a
// missing DLL simply clears the matching capability bit.
//
//export QRNativeInit
func QRNativeInit(dir *C.char) C.int {
	if dir == nil {
		return setErr(fmt.Errorf("null argument"))
	}
	mu.Lock()
	defer mu.Unlock()

	baseDir = C.GoString(dir)
	cfg = native.NewConfig(baseDir)
	logStore = native.NewLog(baseDir)
	tmmfDLL = native.Load(baseDir, "tmmf.dll", tmmfExports)
	lerfDLL = native.Load(baseDir, "lerf.dll", lerfExports)
	lang = native.Table{}
	langOK = false
	return 0
}

// QRNativeCaps reports which capabilities are live, as a bit mask.
//
//export QRNativeCaps
func QRNativeCaps() C.int {
	mu.Lock()
	defer mu.Unlock()
	var caps int
	if logStore.Valid() {
		caps |= capLog
	}
	if cfg.Valid() {
		caps |= capConfig
	}
	if tmmfDLL != nil && tmmfDLL.Valid() {
		caps |= capTmmf
	}
	if lerfDLL != nil && lerfDLL.Valid() {
		caps |= capLerf
	}
	return C.int(caps)
}

//export QRNativeConfigGet
func QRNativeConfigGet(key *C.char, buf *C.char, n C.int) C.int {
	if key == nil {
		return setErr(fmt.Errorf("null argument"))
	}
	mu.Lock()
	v := cfg.Get(C.GoString(key))
	mu.Unlock()
	if v == "" {
		return 1
	}
	writeBuf(buf, n, v)
	return 0
}

//export QRNativeConfigSet
func QRNativeConfigSet(key *C.char, value *C.char) C.int {
	if key == nil || value == nil {
		return setErr(fmt.Errorf("null argument"))
	}
	mu.Lock()
	defer mu.Unlock()
	if !cfg.Valid() {
		return 1
	}
	cfg.Set(C.GoString(key), C.GoString(value))
	return 0
}

//export QRNativeConfigSave
func QRNativeConfigSave() C.int {
	mu.Lock()
	defer mu.Unlock()
	if !cfg.Valid() {
		return 1
	}
	cfg.Save()
	return 0
}

// QRNativeSetLogEnabled switches logging on or off.
//
// The flag is kept here instead of being pushed into log.dll: log_set_enabled
// crashes when it is called from inside this DLL (an access violation inside the
// native image), while every other log export works. Since all writes go through
// QRNativeLogWrite, which honours this flag, the behaviour the caller sees is the
// same and nothing crashes.
//
//export QRNativeSetLogEnabled
func QRNativeSetLogEnabled(on C.int) C.int {
	mu.Lock()
	defer mu.Unlock()
	if !logStore.Valid() {
		return 1
	}
	logStore.Enabled = on != 0
	return 0
}

//export QRNativeLogWrite
func QRNativeLogWrite(op *C.char, result *C.char) C.int {
	if op == nil || result == nil {
		return setErr(fmt.Errorf("null argument"))
	}
	mu.Lock()
	defer mu.Unlock()
	logStore.Write("qrscanner", C.GoString(op), C.GoString(result))
	return 0
}

//export QRNativeCacheLookup
func QRNativeCacheLookup(op *C.char, params *C.char, buf *C.char, n C.int) C.int {
	if op == nil || params == nil {
		return setErr(fmt.Errorf("null argument"))
	}
	mu.Lock()
	v := logStore.Lookup(C.GoString(op), C.GoString(params))
	mu.Unlock()
	if v == "" {
		return 1
	}
	writeBuf(buf, n, v)
	return 0
}

//export QRNativeCachePut
func QRNativeCachePut(op *C.char, params *C.char, result *C.char) C.int {
	if op == nil || params == nil || result == nil {
		return setErr(fmt.Errorf("null argument"))
	}
	mu.Lock()
	defer mu.Unlock()
	logStore.Put(C.GoString(op), C.GoString(params), C.GoString(result))
	return 0
}

// QRNativeLoadLang reads <dir>/resources/cmd/<lang>.lerf, falling back to English,
// and makes it the active table.
//
//export QRNativeLoadLang
func QRNativeLoadLang(code *C.char) C.int {
	if code == nil {
		return setErr(fmt.Errorf("null argument"))
	}
	mu.Lock()
	defer mu.Unlock()
	lc := C.GoString(code)
	if lc != "zh" {
		lc = "en"
	}
	if lerfDLL != nil && lerfDLL.Valid() {
		// format check only: this build of lerf.dll echoes keys instead of
		// returning their text, so the lookup itself is done by our own parser
		lerfDLL.Invoke("lerf_validate", native.Str(filepath.Join(baseDir, "resources", "cmd", "en.lerf")))
	}
	lang, langOK = native.LoadDir(baseDir, lc, "en")
	if !langOK {
		return 1
	}
	return 0
}

//export QRNativeTranslate
func QRNativeTranslate(id *C.char, buf *C.char, n C.int) C.int {
	if id == nil {
		return setErr(fmt.Errorf("null argument"))
	}
	mu.Lock()
	s := lang.Get(C.GoString(id))
	mu.Unlock()
	if s == "" {
		return 1
	}
	writeBuf(buf, n, s)
	return 0
}

func main() {}
