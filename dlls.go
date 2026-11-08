package main

// dlls.go — bindings for the local DLL library (log / config / tmmf / lerf).
//
// Every DLL is loaded from the directory that contains qrscanner.exe. Nothing is
// hard-coded to an absolute path, so a cloned copy of this repo runs as-is.
// Missing DLLs never abort startup: each capability simply degrades to "off".
//
// FFI note: arguments are wrapped in cArg so their backing memory stays reachable
// until after the syscall returns (runtime.KeepAlive). Passing a bare uintptr
// would let the GC reclaim the string mid-call.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

// appDir is the directory holding qrscanner.exe — the base dir for all DLL
// lookups and for the log/cache/config files they write.
func appDir() string {
	p, err := os.Executable()
	if err != nil {
		if wd, e := os.Getwd(); e == nil {
			return wd
		}
		return "."
	}
	return filepath.Dir(p)
}

func appPath(name string) string { return filepath.Join(appDir(), name) }

// ---- FFI argument handling ----

type cArg struct {
	sp  *byte  // C string
	buf []byte // output buffer
	num uintptr
}

func aStr(s string) *cArg {
	p, _ := syscall.BytePtrFromString(s)
	return &cArg{sp: p}
}

func aBuf(b []byte) *cArg { return &cArg{buf: b} }
func aNum(n uintptr) *cArg { return &cArg{num: n} }

func (a *cArg) ptr() uintptr {
	switch {
	case a.sp != nil:
		return uintptr(unsafe.Pointer(a.sp))
	case len(a.buf) > 0:
		return uintptr(unsafe.Pointer(&a.buf[0]))
	default:
		return a.num
	}
}

// graalDLL is a loaded GraalVM native-image DLL plus its isolate thread handle.
type graalDLL struct {
	dll    *syscall.DLL
	thread uintptr
	proc   map[string]uintptr
}

var exportNames = map[string][]string{
	"log.dll": {
		"graal_create_isolate", "graal_detach_all_threads_and_tear_down_isolate",
		"log_init", "log_write", "log_write_end", "log_shutdown",
		"log_set_enabled", "log_is_enabled", "log_size", "log_clear_all",
		"log_cache_lookup", "log_cache_put",
	},
	"config.dll": {
		"graal_create_isolate", "graal_detach_all_threads_and_tear_down_isolate",
		"config_init", "config_load_settings", "config_save_settings",
		"config_get_setting", "config_set_setting", "config_has_setting",
		"config_remove_setting", "config_get_thread_mode", "config_set_thread_mode",
	},
	"tmmf.dll": {
		"graal_create_isolate", "graal_detach_all_threads_and_tear_down_isolate",
		"tmmf_read_all", "tmmf_read_tag", "tmmf_search",
		"tmmf_list_tags", "tmmf_line_count", "tmmf_write_tag",
	},
	"lerf.dll": {
		"graal_create_isolate", "graal_detach_all_threads_and_tear_down_isolate",
		"lerf_load", "lerf_get", "lerf_get_from_file", "lerf_get_multi",
		"lerf_list_all", "lerf_validate", "lerf_key_count",
	},
}

// loadGraalDLL loads <name> from the app directory and creates its isolate.
func loadGraalDLL(name string) *graalDLL {
	path := appPath(name)
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	dll, err := syscall.LoadDLL(path)
	if err != nil {
		return nil
	}
	g := &graalDLL{dll: dll, proc: map[string]uintptr{}}
	for _, n := range exportNames[name] {
		if p, err := dll.FindProc(n); err == nil {
			g.proc[n] = p.Addr()
		}
	}
	create := g.proc["graal_create_isolate"]
	if create == 0 {
		return nil
	}
	var isolate, thread uintptr
	if ret, _, _ := syscall.SyscallN(create, 0,
		uintptr(unsafe.Pointer(&isolate)),
		uintptr(unsafe.Pointer(&thread))); ret != 0 {
		return nil
	}
	g.thread = thread
	return g
}

// invoke calls a DLL export with the isolate thread as the first argument.
func (g *graalDLL) invoke(name string, as ...*cArg) uintptr {
	if g == nil || g.thread == 0 {
		return 0
	}
	addr := g.proc[name]
	if addr == 0 {
		return 0
	}
	args := make([]uintptr, 0, len(as)+1)
	args = append(args, g.thread)
	for _, a := range as {
		args = append(args, a.ptr())
	}
	r1, _, _ := syscall.SyscallN(addr, args...)
	// Backing memory of every argument must outlive the call.
	runtime.KeepAlive(as)
	return r1
}

func (g *graalDLL) close() {
	if g == nil || g.thread == 0 {
		return
	}
	g.invoke("graal_detach_all_threads_and_tear_down_isolate")
	g.thread = 0
}

// out reads a NUL-terminated string written into buf by a DLL call.
// These DLLs return a status code, not a length, so the buffer is scanned to its
// terminator. The buffer is zero-initialised, so a failed call yields "".
func (a *cArg) out() string {
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

// ---- log.dll ----

var logDLL *graalDLL

var (
	logEnabled   = true
	outputFormat = "plain" // plain | json | detailed
)

func initLog() {
	logDLL = loadGraalDLL("log.dll")
	if logDLL == nil {
		return
	}
	logDLL.invoke("log_init", aStr(appDir()))
}

func shutdownLog() {
	if logDLL == nil {
		return
	}
	logDLL.invoke("log_shutdown")
	logDLL.close()
}

func logWrite(op, result string) {
	if !logEnabled {
		return
	}
	logDLL.invoke("log_write", aStr("qrscanner"), aStr(op), aStr(result))
}

// cacheLookup returns the cached result, or "" on miss / unavailable.
func cacheLookup(op, params string) string {
	if logDLL == nil {
		return ""
	}
	buf := make([]byte, 65536)
	out := aBuf(buf)
	logDLL.invoke("log_cache_lookup", aStr(op), aStr(params), out, aNum(uintptr(len(buf))))
	return out.out()
}

func cachePut(op, params, result string) {
	if !logEnabled {
		return
	}
	logDLL.invoke("log_cache_put", aStr(op), aStr(params), aStr(result))
}

// ---- config.dll (settings persisted in TMMF format) ----

var cfgDLL *graalDLL

func initConfig() {
	cfgDLL = loadGraalDLL("config.dll")
	if cfgDLL == nil {
		return
	}
	cfgDLL.invoke("config_init", aStr(appDir()))
	cfgDLL.invoke("config_load_settings")
	if v := configGet("log_enabled"); v != "" {
		logEnabled = v == "1" || v == "true"
	}
	if v := configGet("output_format"); v != "" {
		outputFormat = v
	}
	if v := configGet("language"); v == "zh" {
		currentLang = "zh"
	}
}

func configGet(key string) string {
	if cfgDLL == nil {
		return ""
	}
	buf := make([]byte, 4096)
	out := aBuf(buf)
	cfgDLL.invoke("config_get_setting", aStr(key), out, aNum(uintptr(len(buf))))
	return out.out()
}

func configSet(key, value string) {
	cfgDLL.invoke("config_set_setting", aStr(key), aStr(value))
}

func configSave() { cfgDLL.invoke("config_save_settings") }

func shutdownConfig() {
	if cfgDLL != nil {
		cfgDLL.close()
	}
}

// ---- tmmf.dll ----

var tmmfDLL *graalDLL

func initTmmf() { tmmfDLL = loadGraalDLL("tmmf.dll") }

// tmmfSearch returns every entry on the line matching keyword, or "".
func tmmfSearch(file, keyword string) string {
	if tmmfDLL == nil {
		return ""
	}
	if _, err := os.Stat(appPath(file)); err != nil {
		return ""
	}
	buf := make([]byte, 4096)
	out := aBuf(buf)
	tmmfDLL.invoke("tmmf_search", aStr(appPath(file)), aStr(keyword), out, aNum(uintptr(len(buf))))
	return out.out()
}

func shutdownTmmf() {
	if tmmfDLL != nil {
		tmmfDLL.close()
	}
}

// ---- lerf.dll ----

var lerfDLL *graalDLL

func initLerf() {
	lerfDLL = loadGraalDLL("lerf.dll")
	if lerfDLL != nil {
		// format check only; entry lookup is done by parseLerf
		lerfDLL.invoke("lerf_validate", aStr(langPath("en")))
	}
}

// lerfGetFromFile reads a single key from a LERF file ("" when unavailable).
func lerfGetFromFile(file, key string) string {
	if lerfDLL == nil {
		return ""
	}
	if _, err := os.Stat(appPath(file)); err != nil {
		return ""
	}
	buf := make([]byte, 1024)
	out := aBuf(buf)
	lerfDLL.invoke("lerf_get_from_file", aStr(appPath(file)), aStr(key), out, aNum(uintptr(len(buf))))
	return out.out()
}

func shutdownLerf() {
	if lerfDLL != nil {
		lerfDLL.close()
	}
}

// ---- i18n via LERF ----
//
// Language files are standard LERF documents (`<id>text<id>` entries) stored under
// resources/cmd/. lerf.dll is loaded and its lerf_validate used as a format check;
// entry lookup itself is done here because this build of lerf.dll only echoes the
// requested key back instead of the stored text.

var (
	currentLang = "en"                    // en | zh
	langTable   = map[string]string{}     // id -> text
	langValid   bool
)

const (
	iAppTitle          = "1"
	iTabScan           = "2"
	iTabGenerate       = "3"
	iTabSettings       = "4"
	iBtnOpenImage      = "5"
	iBtnScreenshot     = "6"
	iBtnChooseFile     = "7"
	iBtnGenerateQr     = "8"
	iBtnSavePng        = "9"
	iLblResult         = "10"
	iLblInput          = "11"
	iLblFormat         = "12"
	iLblLanguage       = "13"
	iChkLogEnabled     = "14"
	iDlgOpenImage      = "15"
	iDlgChooseFile     = "16"
	iDlgSaveQr         = "17"
	iErrTitle          = "18"
	iErrStartup        = "19"
	iMsgSaved          = "20"
	iMsgSavedTo        = "21"
	iMsgInputEmpty     = "22"
	iMsgNoQr           = "23"
	iMsgScreenshotFail = "24"
	iMsgCannotDecode   = "25"
	iMsgCannotRead     = "26"
	iMsgQrEncodeFail   = "27"
	iMsgBadQrType      = "28"
	iMsgCreateFail     = "29"
	iMsgRestartHint    = "30"
	iScanNoCode        = "31"
	iScanFormat        = "32"
	iScanDecoded       = "33"
	iScanCacheHit      = "34"
	iScanOpenedBrowser = "35"
	iScanOpenedFolder  = "36"
	iScanBase32        = "37"
	iScanPlainText     = "38"
	iScanProcessFail   = "39"
	iGenFileInfo       = "40"
	iGenCacheHit       = "41"
	iStatusReady       = "42"
	iStatusMissing     = "43"
	iStatusTemplate    = "44"
)

func langPath(lang string) string {
	return appPath(filepath.Join("resources", "cmd", lang+".lerf"))
}

// parseLerf reads a LERF document into an id -> text table.
// Supported line forms: `<id>text<id>`, `key=value`, `# comment`, blank.
func parseLerf(path string) (map[string]string, bool) {
	table := map[string]string{}
	data, err := os.ReadFile(path)
	if err != nil {
		return table, false
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "<") {
			close := strings.Index(line, ">")
			if close < 0 {
				continue
			}
			id := line[1:close]
			body := line[close+1:]
			if end := strings.LastIndex(body, "<"+id+">"); end >= 0 {
				body = body[:end]
			}
			table[id] = body
			continue
		}
		if eq := strings.Index(line, "="); eq > 0 {
			table[line[:eq]] = line[eq+1:]
		}
	}
	return table, len(table) > 0
}

// loadLang reloads the active language table, falling back to English.
func loadLang() {
	var ok bool
	langTable, ok = parseLerf(langPath(currentLang))
	langValid = ok
	if !ok && currentLang != "en" {
		langTable, langValid = parseLerf(langPath("en"))
	}
}

// tr returns the localized text for an entry id, or the id itself when missing.
func tr(id string) string {
	if v, ok := langTable[id]; ok {
		return v
	}
	return id
}

// trf is tr with {0}, {1}, ... placeholders substituted.
func trf(id string, args ...string) string {
	s := tr(id)
	for i, a := range args {
		s = strings.ReplaceAll(s, "{"+itoa(i)+"}", a)
	}
	return s
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

// setLang switches the active language and persists the choice in settings.
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
	shutdownLog()
	shutdownConfig()
	shutdownTmmf()
	shutdownLerf()
}
