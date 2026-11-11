// Package native holds the parts of this project that every app in the suite
// needs: loading the GraalVM DLLs, calling them safely, reading TMMF settings,
// writing the log and its result cache, and parsing LERF language files.
//
// native/ffi.go — 本文件为本项目自己写的代码，不是微软的代码。
// This file is written for this project; no Microsoft code is used here.
//
// Drop this package into another project and it works unchanged: everything is
// resolved relative to a directory the caller passes in, never to an absolute
// path of the machine that built the binary.
package native

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

// Arg keeps the Go memory backing a C argument alive for the whole call.
// Passing a bare uintptr would let the garbage collector move or free the
// string mid-call, which is why every argument goes through this type.
type Arg struct {
	sp  *byte
	buf []byte
	num uintptr
}

// Str wraps a C string argument.
func Str(s string) *Arg {
	p, _ := syscall.BytePtrFromString(s)
	return &Arg{sp: p}
}

// Buf wraps an output buffer the callee writes a NUL-terminated string into.
func Buf(b []byte) *Arg { return &Arg{buf: b} }

// Num wraps a plain integer argument.
func Num(n uintptr) *Arg { return &Arg{num: n} }

// Ptr returns the raw pointer handed to the DLL.
func (a *Arg) Ptr() uintptr {
	switch {
	case a.sp != nil:
		return uintptr(unsafe.Pointer(a.sp))
	case len(a.buf) > 0:
		return uintptr(unsafe.Pointer(&a.buf[0]))
	default:
		return a.num
	}
}

// Out reads the NUL-terminated string the callee wrote into the buffer.
// The DLLs return a status code, not a length, so the buffer is scanned to its
// first NUL instead of being sliced by the return value. A zero-initialised
// buffer therefore yields "" on failure.
func (a *Arg) Out() string {
	if len(a.buf) == 0 {
		return ""
	}
	for i := 0; i < len(a.buf); i++ {
		if a.buf[i] == 0 {
			return string(a.buf[:i])
		}
	}
	return string(a.buf)
}

// DLL is one loaded native image plus the isolate thread its exports need.
type DLL struct {
	h      *syscall.DLL
	thread uintptr
	proc   map[string]uintptr
	name   string
}

// Load loads name from dir, resolves the listed exports and creates an isolate.
// It returns nil when the file is missing or unusable: callers treat that as
// "feature off" rather than as a fatal error.
func Load(dir, name string, exports []string) *DLL {
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	h, err := syscall.LoadDLL(path)
	if err != nil {
		return nil
	}
	d := &DLL{h: h, name: name, proc: map[string]uintptr{}}
	for _, n := range exports {
		if p, err := h.FindProc(n); err == nil {
			d.proc[n] = p.Addr()
		}
	}
	create := d.proc["graal_create_isolate"]
	if create == 0 {
		return nil
	}
	var isolate, thread uintptr
	if ret, _, _ := syscall.SyscallN(create, 0,
		uintptr(unsafe.Pointer(&isolate)),
		uintptr(unsafe.Pointer(&thread))); ret != 0 || thread == 0 {
		return nil
	}
	d.thread = thread
	return d
}

// Valid reports whether the DLL loaded and has a live isolate.
func (d *DLL) Valid() bool { return d != nil && d.thread != 0 }

// Has reports whether an export was resolved.
func (d *DLL) Has(name string) bool {
	return d.Valid() && d.proc[name] != 0
}

// Invoke calls an export with the isolate thread prepended, which is the ABI
// these native images use. Unknown exports and dead isolates return 0.
func (d *DLL) Invoke(name string, args ...*Arg) uintptr {
	if !d.Valid() {
		return 0
	}
	addr := d.proc[name]
	if addr == 0 {
		return 0
	}
	all := make([]uintptr, 0, len(args)+1)
	all = append(all, d.thread)
	for _, a := range args {
		all = append(all, a.Ptr())
	}
	r1, _, _ := syscall.SyscallN(addr, all...)
	// The backing memory of every argument has to outlive the call.
	runtime.KeepAlive(args)
	return r1
}

// Close tears the isolate down. Safe to call on a nil receiver.
func (d *DLL) Close() {
	if !d.Valid() {
		return
	}
	d.Invoke("graal_detach_all_threads_and_tear_down_isolate")
	d.thread = 0
}

// AppDir returns the directory of the running executable. It is the base for
// every DLL lookup and for the files those DLLs write.
func AppDir() string {
	p, err := os.Executable()
	if err != nil {
		if wd, e := os.Getwd(); e == nil {
			return wd
		}
		return "."
	}
	return filepath.Dir(p)
}

// TrimNUL removes one trailing NUL if a DLL wrote a C string into a Go string.
func TrimNUL(s string) string { return strings.TrimRight(s, "\x00") }
