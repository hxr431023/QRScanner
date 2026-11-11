package main

// dlls.go — thin application layer over the reusable `native` package.
//
// The FFI plumbing (isolate handling, argument lifetime, status-code buffers),
// the TMMF settings store, the log with its result cache and the LERF parser all
// live in `qrscanner/native` so another project can copy that directory and get
// the same behaviour. This file only binds them to this app.
//
// Every DLL is loaded from the directory that contains qrscanner.exe. Nothing is
// hard-coded to an absolute path, so a cloned copy of this repo runs as-is.
// Missing DLLs never abort startup: each capability simply degrades to "off".

import (
	"os"
	"path/filepath"

	"qrscanner/native"
)

// appDir is the directory holding qrscanner.exe — the base dir for all DLL
// lookups and for the log/cache/config files they write.
func appDir() string { return native.AppDir() }

func appPath(name string) string { return filepath.Join(appDir(), name) }

// ---- tmmf.dll ----

var tmmfDLL *native.DLL

var tmmfExports = []string{
	"graal_create_isolate", "graal_detach_all_threads_and_tear_down_isolate",
	"tmmf_read_all", "tmmf_read_tag", "tmmf_search",
	"tmmf_list_tags", "tmmf_line_count", "tmmf_write_tag",
}

func initTmmf() { tmmfDLL = native.Load(appDir(), "tmmf.dll", tmmfExports) }

// tmmfSearch returns every entry on the line matching keyword, or "".
func tmmfSearch(file, keyword string) string {
	if !tmmfDLL.Valid() {
		return ""
	}
	if _, err := os.Stat(appPath(file)); err != nil {
		return ""
	}
	buf := make([]byte, 4096)
	out := native.Buf(buf)
	tmmfDLL.Invoke("tmmf_search", native.Str(appPath(file)), native.Str(keyword), out, native.Num(uintptr(len(buf))))
	return out.Out()
}

func shutdownTmmf() {
	if tmmfDLL != nil {
		tmmfDLL.Close()
	}
}

// ---- lerf.dll ----

var lerfDLL *native.DLL

var lerfExports = []string{
	"graal_create_isolate", "graal_detach_all_threads_and_tear_down_isolate",
	"lerf_load", "lerf_get", "lerf_get_from_file", "lerf_get_multi",
	"lerf_list_all", "lerf_validate", "lerf_key_count",
}

func initLerf() {
	lerfDLL = native.Load(appDir(), "lerf.dll", lerfExports)
	if lerfDLL.Valid() {
		// format check only; entry lookup is done by the native parser because
		// this build of lerf.dll echoes the key instead of returning its text
		lerfDLL.Invoke("lerf_validate", native.Str(langPath("en")))
	}
}

func shutdownLerf() {
	if lerfDLL != nil {
		lerfDLL.Close()
	}
}

// ---- log.dll ----

var logStore *native.Log

var (
	logEnabled   = true
	outputFormat = "plain" // plain | json | detailed
)

func initLog() {
	logStore = native.NewLog(appDir())
	if logStore == nil {
		return
	}
	logStore.SetEnabled(logEnabled)
}

func shutdownLog() {
	if logStore != nil {
		logStore.Close()
	}
}

func logWrite(op, result string) {
	if !logEnabled || logStore == nil {
		return
	}
	logStore.Write("qrscanner", op, result)
}

// cacheLookup returns the cached result, or "" on miss / unavailable.
func cacheLookup(op, params string) string {
	if logStore == nil {
		return ""
	}
	return logStore.Lookup(op, params)
}

func cachePut(op, params, result string) {
	if !logEnabled || logStore == nil {
		return
	}
	logStore.Put(op, params, result)
}

// ---- config.dll (settings persisted in TMMF format) ----

var cfgStore *native.Config

func initConfig() {
	cfgStore = native.NewConfig(appDir())
	if cfgStore == nil {
		return
	}
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
	if cfgStore == nil {
		return ""
	}
	return cfgStore.Get(key)
}

func configSet(key, value string) {
	if cfgStore == nil {
		return
	}
	cfgStore.Set(key, value)
}

func configSave() {
	if cfgStore == nil {
		return
	}
	cfgStore.Save()
}

func shutdownConfig() {
	if cfgStore != nil {
		cfgStore.Close()
	}
}

// ---- i18n via LERF ----
//
// Language files are standard LERF documents (`<id>text<id>` entries) stored under
// resources/cmd/. The parser lives in the native package so other projects reuse it.

var (
	currentLang = "en"                // en | zh
	langTable   = native.Table{}      // id -> text
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

	// personalisation
	iLblKind        = "45"
	iKindQR         = "46"
	iKindCode128    = "47"
	iKindCode39     = "48"
	iKindEAN13      = "49"
	iLblFg          = "50"
	iLblBg          = "51"
	iChkTransparent = "52"
	iLblShape       = "53"
	iShapeSquare    = "54"
	iShapeRounded   = "55"
	iShapeDot       = "56"
	iShapeDiamond   = "57"
	iLblEye         = "58"
	iEyeSquare      = "59"
	iEyeRounded     = "60"
	iEyeCircle      = "61"
	iLblEcc         = "62"
	iLblCell        = "63"
	iLblQuiet       = "64"
	iLblBarWidth    = "65"
	iLblBarHeight   = "66"
	iBtnLogo        = "67"
	iBtnLogoClear   = "68"
	iLblLogoRatio   = "69"
	iLblTopText     = "70"
	iLblBottomText  = "71"
	iLblBorder      = "72"
	iBorderNone     = "73"
	iBorderThin     = "74"
	iBorderThick    = "75"
	iLblBorderColor = "76"
	iMsgBadColor    = "77"
	iMsgRenderFail  = "78"
	iDlgChooseLogo  = "79"
	iMsgLogoInUse   = "80"
	iMsgNeedQR      = "81"
)

func langPath(lang string) string {
	return appPath(filepath.Join("resources", "cmd", lang+".lerf"))
}

// loadLang reloads the active language table, falling back to English.
func loadLang() {
	langTable, langValid = native.LoadDir(appDir(), currentLang, "en")
}

// tr returns the localized text for an entry id, or the id itself when missing.
func tr(id string) string { return langTable.Get(id) }

// trf is tr with {0}, {1}, ... placeholders substituted.
func trf(id string, args ...string) string { return langTable.Getf(id, args...) }

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
