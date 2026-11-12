package main

// dlls.go — 本文件为 QRScanner 自己写的代码，不是微软的代码，也不含任何微软源码。
// dlls.go — written for this project; no Microsoft code is contained here.
//
// This file only *talks to* the two reusable DLLs; it does not implement anything.
//
//   qrnative.dll  TMMF settings, LERF language table, log + result cache
//   qrcore.dll    the personalised QR / 1D renderer
//
// Both are loaded from the directory that holds qrscanner.exe, never from an
// absolute path, so a downloaded copy of this repo runs as-is. Every call passes
// UTF-8 C strings and a caller-owned buffer; nothing allocated inside the DLL is
// ever freed here, because each side owns its own memory.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"qrscanner/native"
)

// capability bits reported by qrnative.dll
const (
	capLog    = 1 << 0
	capConfig = 1 << 1
	capTmmf   = 1 << 2
	capLerf   = 1 << 3
)

var (
	nativeDLL *syscall.DLL
	coreDLL   *syscall.DLL

	procInit        *syscall.Proc
	procCaps        *syscall.Proc
	procCfgGet      *syscall.Proc
	procCfgSet      *syscall.Proc
	procCfgSave     *syscall.Proc
	procSetLogOn    *syscall.Proc
	procLogWrite    *syscall.Proc
	procCacheLook   *syscall.Proc
	procCachePut    *syscall.Proc
	procLoadLang    *syscall.Proc
	procTranslate   *syscall.Proc
	procCoreRender  *syscall.Proc
	procCoreErr     *syscall.Proc

	caps int
)

// baseDirOverride lets a caller (or a test) point the DLL lookup at an explicit
// directory instead of the directory of the running executable.
var baseDirOverride string

// appDir is the directory holding qrscanner.exe and the DLLs it loads.
func appDir() string {
	if baseDirOverride != "" {
		return baseDirOverride
	}
	return native.AppDir()
}

func appPath(name string) string { return filepath.Join(appDir(), name) }

// bytePtr returns a NUL-terminated UTF-8 copy of s that stays alive for the call.
func bytePtr(s string) []byte {
	b := make([]byte, len(s)+1)
	copy(b, s)
	return b
}

func ptr(b []byte) uintptr {
	if len(b) == 0 {
		return 0
	}
	return uintptr(unsafe.Pointer(&b[0]))
}

func loadProc(d *syscall.DLL, name string) *syscall.Proc {
	if d == nil {
		return nil
	}
	p, err := d.FindProc(name)
	if err != nil {
		return nil
	}
	return p
}

// initNative loads qrnative.dll and points it at this program's directory.
func initNative() error {
	path := appPath("qrnative.dll")
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	d, err := syscall.LoadDLL(path)
	if err != nil {
		return err
	}
	nativeDLL = d
	procInit = loadProc(d, "QRNativeInit")
	procCaps = loadProc(d, "QRNativeCaps")
	procCfgGet = loadProc(d, "QRNativeConfigGet")
	procCfgSet = loadProc(d, "QRNativeConfigSet")
	procCfgSave = loadProc(d, "QRNativeConfigSave")
	procSetLogOn = loadProc(d, "QRNativeSetLogEnabled")
	procLogWrite = loadProc(d, "QRNativeLogWrite")
	procCacheLook = loadProc(d, "QRNativeCacheLookup")
	procCachePut = loadProc(d, "QRNativeCachePut")
	procLoadLang = loadProc(d, "QRNativeLoadLang")
	procTranslate = loadProc(d, "QRNativeTranslate")
	if procInit == nil || procCaps == nil {
		return fmt.Errorf("qrnative.dll: expected exports are missing")
	}

	dir := bytePtr(appDir())
	procInit.Call(ptr(dir))

	r, _, _ := procCaps.Call()
	caps = int(r)
	return nil
}

// initCore loads qrcore.dll, the renderer.
func initCore() error {
	path := appPath("qrcore.dll")
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	d, err := syscall.LoadDLL(path)
	if err != nil {
		return err
	}
	coreDLL = d
	procCoreRender = loadProc(d, "QRCoreRender")
	procCoreErr = loadProc(d, "QRCoreLastError")
	if procCoreRender == nil {
		return fmt.Errorf("qrcore.dll: QRCoreRender is missing")
	}
	return nil
}

func have(cap int) bool { return caps&cap != 0 }

// coreError returns the renderer's last error text.
func coreError() string {
	if procCoreErr == nil {
		return ""
	}
	buf := make([]byte, 4096)
	r, _, _ := procCoreErr.Call(ptr(buf), uintptr(len(buf)))
	_ = r
	return cString(buf)
}

// renderToFile asks qrcore.dll to draw the symbol and write it as PNG.
func renderToFile(spec, content, outPath string) error {
	if procCoreRender == nil {
		return fmt.Errorf("qrcore.dll is not available")
	}
	sp := bytePtr(spec)
	ct := bytePtr(content)
	op := bytePtr(outPath)
	r, _, _ := procCoreRender.Call(ptr(sp), ptr(ct), ptr(op))
	if r != 0 {
		if msg := coreError(); msg != "" {
			return fmt.Errorf("%s", msg)
		}
		return fmt.Errorf("render failed")
	}
	return nil
}

func cString(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

// ---- settings (TMMF, through qrnative.dll) ----

func configGet(key string) string {
	if procCfgGet == nil {
		return ""
	}
	k := bytePtr(key)
	buf := make([]byte, 4096)
	r, _, _ := procCfgGet.Call(ptr(k), ptr(buf), uintptr(len(buf)))
	if r != 0 {
		return ""
	}
	return cString(buf)
}

func configSet(key, value string) {
	if procCfgSet == nil {
		return
	}
	k := bytePtr(key)
	v := bytePtr(value)
	procCfgSet.Call(ptr(k), ptr(v))
}

func configSave() {
	if procCfgSave == nil {
		return
	}
	procCfgSave.Call()
}

// ---- log and cache ----

var (
	logEnabled   = true
	outputFormat = "plain"
)

func setLogEnabled(on bool) {
	logEnabled = on
	if procSetLogOn == nil {
		return
	}
	v := uintptr(0)
	if on {
		v = 1
	}
	procSetLogOn.Call(v)
}

func logWrite(op, result string) {
	if !logEnabled || procLogWrite == nil {
		return
	}
	a := bytePtr(op)
	b := bytePtr(result)
	procLogWrite.Call(ptr(a), ptr(b))
}

func cacheLookup(op, params string) string {
	if procCacheLook == nil {
		return ""
	}
	o := bytePtr(op)
	p := bytePtr(params)
	buf := make([]byte, 65536)
	r, _, _ := procCacheLook.Call(ptr(o), ptr(p), ptr(buf), uintptr(len(buf)))
	if r != 0 {
		return ""
	}
	return cString(buf)
}

func cachePut(op, params, result string) {
	if !logEnabled || procCachePut == nil {
		return
	}
	o := bytePtr(op)
	p := bytePtr(params)
	r := bytePtr(result)
	procCachePut.Call(ptr(o), ptr(p), ptr(r))
}

// ---- language (LERF, through qrnative.dll) ----

func loadLang() {
	if procLoadLang == nil {
		return
	}
	l := bytePtr(currentLang)
	procLoadLang.Call(ptr(l))
}

// tr returns the text for an entry id, or the id itself when it is missing.
func tr(id string) string {
	if procTranslate == nil {
		return id
	}
	i := bytePtr(id)
	buf := make([]byte, 8192)
	r, _, _ := procTranslate.Call(ptr(i), ptr(buf), uintptr(len(buf)))
	if r != 0 {
		return id
	}
	return cString(buf)
}

// trf substitutes {0}, {1}, ... in the translated text.
func trf(id string, args ...string) string {
	s := tr(id)
	for i, a := range args {
		s = strings.ReplaceAll(s, fmt.Sprintf("{%d}", i), a)
	}
	return s
}

func setLang(lang string) {
	if lang != "zh" {
		lang = "en"
	}
	currentLang = lang
	loadLang()
	configSet("language", currentLang)
	configSave()
}

func shutdownAll() {
	if nativeDLL != nil {
		nativeDLL.Release()
	}
	if coreDLL != nil {
		coreDLL.Release()
	}
}
