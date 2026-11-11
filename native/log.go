package native

// native/log.go — 本文件为本项目自己写的代码，不是微软的代码。
// This file is written for this project; no Microsoft code is used here.
//
// log.dll does two things: it appends operations to a log file under <dir>/log,
// and it keeps a result cache under <dir>/cache keyed by (operation, params).
// A cache hit lets the caller skip the expensive work that produced the result.
//
// Both lookups return a status code rather than a length, so the result is read
// from the output buffer by scanning to the NUL terminator.

// Log wraps log.dll plus its result cache. A nil *Log is valid and silent.
type Log struct {
	d       *DLL
	Enabled bool
}

var logExports = []string{
	"graal_create_isolate", "graal_detach_all_threads_and_tear_down_isolate",
	"log_init", "log_write", "log_write_end", "log_shutdown",
	"log_set_enabled", "log_is_enabled", "log_size", "log_clear_all",
	"log_cache_lookup", "log_cache_put",
}

// NewLog loads log.dll from dir and points it at that directory.
func NewLog(dir string) *Log {
	d := Load(dir, "log.dll", logExports)
	if !d.Valid() {
		return nil
	}
	d.Invoke("log_init", Str(dir))
	return &Log{d: d, Enabled: true}
}

// Valid reports whether logging is usable.
func (l *Log) Valid() bool { return l != nil && l.d.Valid() }

// SetEnabled turns logging and caching on or off, in memory and in the DLL.
func (l *Log) SetEnabled(on bool) {
	if l == nil {
		return
	}
	l.Enabled = on
	if !l.d.Valid() {
		return
	}
	v := Num(0)
	if on {
		v = Num(1)
	}
	l.d.Invoke("log_set_enabled", v)
}

// Write appends one operation record.
func (l *Log) Write(tag, op, result string) {
	if !l.Valid() || !l.Enabled {
		return
	}
	l.d.Invoke("log_write", Str(tag), Str(op), Str(result))
}

// Lookup returns the cached result for (op, params), or "" on a miss.
func (l *Log) Lookup(op, params string) string {
	if !l.Valid() {
		return ""
	}
	buf := make([]byte, 65536)
	out := Buf(buf)
	l.d.Invoke("log_cache_lookup", Str(op), Str(params), out, Num(uintptr(len(buf))))
	return out.Out()
}

// Put stores a result in the cache.
func (l *Log) Put(op, params, result string) {
	if !l.Valid() || !l.Enabled {
		return
	}
	l.d.Invoke("log_cache_put", Str(op), Str(params), Str(result))
}

// Close shuts the log down and tears the isolate down.
func (l *Log) Close() {
	if l == nil {
		return
	}
	l.d.Invoke("log_shutdown")
	l.d.Close()
}
