package main

// QR Scanner — Walk GUI over gozxing (decode) and boombuler/barcode (encode).
// UI strings come from LERF language files via lerf.dll (English + Chinese).

import (
	"bytes"
	"encoding/base32"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/oned"
	"github.com/makiuchi-d/gozxing/qrcode"
)

var (
	mw         *walk.MainWindow
	scanResult *walk.TextEdit
	genInput   *walk.LineEdit
	genImg     *walk.ImageView
	genPathLbl *walk.Label
	currentPNG string // last render written by qrcore.dll

	chkLogEnabled *walk.CheckBox
	cmbFormat     *walk.ComboBox
	cmbLang       *walk.ComboBox
	statusLbl     *walk.Label

	// personalisation controls
	cmbKind     *walk.ComboBox
	cmbECC      *walk.ComboBox
	cmbShape    *walk.ComboBox
	cmbEye      *walk.ComboBox
	cmbBorder   *walk.ComboBox
	cmbLogoRate *walk.ComboBox
	edFg        *walk.LineEdit
	edBg        *walk.LineEdit
	edBorderCol *walk.LineEdit
	edCell      *walk.LineEdit
	edQuiet     *walk.LineEdit
	edBarW      *walk.LineEdit
	edBarH      *walk.LineEdit
	edTopText   *walk.LineEdit
	edBotText   *walk.LineEdit
	chkTrans    *walk.CheckBox
	logoLbl     *walk.Label
	logoPath    string
)

func main() {
	// qrnative.dll carries the settings store, the language table and the log;
	// qrcore.dll carries the renderer. Without them the GUI can still run but
	// there is nothing to generate with, so that is reported at startup.
	startErr := initNative()
	if startErr == nil {
		startErr = initCore()
	}
	if startErr != nil {
		showFatal(startErr.Error())
		return
	}
	// settings are read back through qrnative.dll before any control is built
	if v := configGet("log_enabled"); v != "" {
		logEnabled = v == "1" || v == "true"
	}
	setLogEnabled(logEnabled)
	if v := configGet("output_format"); v != "" {
		outputFormat = v
	}
	if v := configGet("language"); v == "zh" {
		currentLang = "zh"
	}
	loadLang()
	defer shutdownAll()

	formats := []string{"plain", "json", "detailed"}
	langs := []string{"English", "中文"}

	win := MainWindow{
		AssignTo: &mw,
		Title:    tr(iAppTitle),
		Size:     Size{Width: 900, Height: 880},
		Layout:   VBox{},
		Children: []Widget{
			TabWidget{
				Pages: []TabPage{
					TabPage{
						Title:  tr(iTabScan),
						Layout: VBox{},
						Children: []Widget{
							Composite{
								Layout: HBox{},
								Children: []Widget{
									PushButton{
										Text: tr(iBtnOpenImage),
										OnClicked: func() {
											dlg := new(walk.FileDialog)
											dlg.Title = tr(iDlgOpenImage)
											dlg.Filter = "Images (*.png;*.jpg;*.jpeg;*.bmp)|*.png;*.jpg;*.jpeg;*.bmp"
											if ok, err := dlg.ShowOpen(mw); err != nil || !ok {
												return
											}
											decodeFile(dlg.FilePath)
										},
									},
									PushButton{
										Text:      tr(iBtnScreenshot),
										OnClicked: func() { decodeScreenshot() },
									},
								},
							},
							Label{Text: tr(iLblResult)},
							TextEdit{
								AssignTo: &scanResult,
								ReadOnly: true,
								MinSize:  Size{Height: 200},
							},
						},
					},
					TabPage{
						Title:  tr(iTabGenerate),
						Layout: VBox{},
						Children: []Widget{
							Label{Text: tr(iLblInput)},
							LineEdit{AssignTo: &genInput},
							Composite{
								Layout: HBox{},
								Children: []Widget{
									PushButton{
										Text: tr(iBtnChooseFile),
										OnClicked: func() {
											dlg := new(walk.FileDialog)
											dlg.Title = tr(iDlgChooseFile)
											if ok, err := dlg.ShowOpen(mw); err != nil || !ok {
												return
											}
											genPathLbl.SetText(dlg.FilePath)
											genInput.SetText(dlg.FilePath)
										},
									},
									PushButton{
										Text:      tr(iBtnGenerateQr),
										OnClicked: func() { generateCode() },
									},
									PushButton{
										Text:      tr(iBtnSavePng),
										OnClicked: func() { saveQR() },
									},
								},
							},
							Label{AssignTo: &genPathLbl, Text: ""},
							Composite{
								Layout: HBox{},
								Children: []Widget{
									Composite{
										Layout: VBox{},
										Children: []Widget{
											Composite{Layout: HBox{}, Children: []Widget{
												Label{Text: tr(iLblKind), MinSize: Size{Width: 110}},
												ComboBox{AssignTo: &cmbKind, Model: kindLabels()},
											}},
											Composite{Layout: HBox{}, Children: []Widget{
												Label{Text: tr(iLblFg), MinSize: Size{Width: 110}},
												LineEdit{AssignTo: &edFg, Text: "#000000", MaxLength: 9},
											}},
											Composite{Layout: HBox{}, Children: []Widget{
												Label{Text: tr(iLblBg), MinSize: Size{Width: 110}},
												LineEdit{AssignTo: &edBg, Text: "#FFFFFF", MaxLength: 9},
											}},
											CheckBox{AssignTo: &chkTrans, Text: tr(iChkTransparent)},
											Composite{Layout: HBox{}, Children: []Widget{
												Label{Text: tr(iLblEcc), MinSize: Size{Width: 110}},
												ComboBox{AssignTo: &cmbECC, Model: []string{"L", "M", "Q", "H"}, CurrentIndex: 1},
											}},
											Composite{Layout: HBox{}, Children: []Widget{
												Label{Text: tr(iLblShape), MinSize: Size{Width: 110}},
												ComboBox{AssignTo: &cmbShape, Model: shapeLabels()},
											}},
											Composite{Layout: HBox{}, Children: []Widget{
												Label{Text: tr(iLblEye), MinSize: Size{Width: 110}},
												ComboBox{AssignTo: &cmbEye, Model: eyeLabels()},
											}},
											Composite{Layout: HBox{}, Children: []Widget{
												Label{Text: tr(iLblCell), MinSize: Size{Width: 110}},
												LineEdit{AssignTo: &edCell, Text: "10", MaxLength: 4},
											}},
											Composite{Layout: HBox{}, Children: []Widget{
												Label{Text: tr(iLblQuiet), MinSize: Size{Width: 110}},
												LineEdit{AssignTo: &edQuiet, Text: "4", MaxLength: 3},
											}},
										},
									},
									Composite{
										Layout: VBox{},
										Children: []Widget{
											Composite{Layout: HBox{}, Children: []Widget{
												Label{Text: tr(iLblBorder), MinSize: Size{Width: 110}},
												ComboBox{AssignTo: &cmbBorder, Model: borderLabels()},
											}},
											Composite{Layout: HBox{}, Children: []Widget{
												Label{Text: tr(iLblBorderColor), MinSize: Size{Width: 110}},
												LineEdit{AssignTo: &edBorderCol, Text: "#000000", MaxLength: 9},
											}},
											Composite{Layout: HBox{}, Children: []Widget{
												PushButton{
													Text: tr(iBtnLogo),
													OnClicked: func() {
														dlg := new(walk.FileDialog)
														dlg.Title = tr(iDlgChooseLogo)
														dlg.Filter = "Images (*.png;*.jpg;*.jpeg;*.bmp)|*.png;*.jpg;*.jpeg;*.bmp"
														if ok, err := dlg.ShowOpen(mw); err != nil || !ok {
															return
														}
														logoPath = dlg.FilePath
														logoLbl.SetText(filepath.Base(logoPath))
													},
												},
												PushButton{
													Text: tr(iBtnLogoClear),
													OnClicked: func() {
														logoPath = ""
														logoLbl.SetText("")
													},
												},
											}},
											Label{AssignTo: &logoLbl, Text: ""},
											Composite{Layout: HBox{}, Children: []Widget{
												Label{Text: tr(iLblLogoRatio), MinSize: Size{Width: 110}},
												ComboBox{AssignTo: &cmbLogoRate, Model: []string{"16%", "22%", "28%", "33%"}, CurrentIndex: 1},
											}},
											Composite{Layout: HBox{}, Children: []Widget{
												Label{Text: tr(iLblTopText), MinSize: Size{Width: 110}},
												LineEdit{AssignTo: &edTopText},
											}},
											Composite{Layout: HBox{}, Children: []Widget{
												Label{Text: tr(iLblBottomText), MinSize: Size{Width: 110}},
												LineEdit{AssignTo: &edBotText},
											}},
											Composite{Layout: HBox{}, Children: []Widget{
												Label{Text: tr(iLblBarWidth), MinSize: Size{Width: 110}},
												LineEdit{AssignTo: &edBarW, Text: "2", MaxLength: 4},
												Label{Text: tr(iLblBarHeight), MinSize: Size{Width: 110}},
												LineEdit{AssignTo: &edBarH, Text: "140", MaxLength: 5},
											}},
										},
									},
								},
							},
							ImageView{
								AssignTo: &genImg,
								MinSize:  Size{Width: 300, Height: 300},
								Mode:     ImageViewModeZoom,
							},
						},
					},
					TabPage{
						Title:  tr(iTabSettings),
						Layout: VBox{},
						Children: []Widget{
							CheckBox{
								AssignTo: &chkLogEnabled,
								Text:     tr(iChkLogEnabled),
								Checked:  logEnabled,
								OnCheckedChanged: func() {
									logEnabled = chkLogEnabled.Checked()
									configSet("log_enabled", boolStr(logEnabled))
									configSave()
								},
							},
							Composite{
								Layout: HBox{},
								Children: []Widget{
									Label{Text: tr(iLblFormat)},
									ComboBox{
										AssignTo:     &cmbFormat,
										Model:        formats,
										CurrentIndex: formatIndex(outputFormat),
										OnCurrentIndexChanged: func() {
											i := cmbFormat.CurrentIndex()
											if i < 0 || i >= len(formats) {
												return
											}
											outputFormat = formats[i]
											configSet("output_format", outputFormat)
											configSave()
										},
									},
								},
							},
							Composite{
								Layout: HBox{},
								Children: []Widget{
									Label{Text: tr(iLblLanguage)},
									ComboBox{
										AssignTo:     &cmbLang,
										Model:        langs,
										CurrentIndex: langIndex(currentLang),
										OnCurrentIndexChanged: func() {
											i := cmbLang.CurrentIndex()
											if i == 0 {
												setLang("en")
											} else if i == 1 {
												setLang("zh")
											}
											walk.MsgBox(mw, tr(iTabSettings), tr(iMsgRestartHint), walk.MsgBoxIconInformation)
										},
									},
								},
							},
							Label{Text: ""},
							Label{AssignTo: &statusLbl, Text: "", MinSize: Size{Height: 110}},
						},
					},
				},
			},
		},
	}

	if err := win.Create(); err != nil {
		showFatal(trf(iMsgCreateFail, err.Error()))
		return
	}

	refreshStatus()
	applySavedStyle()
	mw.Run()
}

func boolStr(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func formatIndex(f string) int {
	switch f {
	case "json":
		return 1
	case "detailed":
		return 2
	}
	return 0
}

func langIndex(l string) int {
	if l == "zh" {
		return 1
	}
	return 0
}

// refreshStatus shows which DLLs are active and where settings are stored.
func refreshStatus() {
	if statusLbl == nil {
		return
	}
	on := func(v bool) string {
		if v {
			return tr(iStatusReady)
		}
		return tr(iStatusMissing)
	}
	statusLbl.SetText(trf(iStatusTemplate,
		appDir(), on(have(capLog)), on(have(capConfig)), on(have(capTmmf)), on(have(capLerf))))
}

// showFatal displays a startup error even when the GUI is unavailable.
func showFatal(msg string) {
	os.WriteFile(filepath.Join(os.TempDir(), "qrscanner_crash.txt"), []byte(msg), 0644)
	walk.MsgBox(nil, tr(iErrStartup), msg, walk.MsgBoxIconError)
}

// ---- Scan logic ----

func decodeFile(path string) {
	// Check cache first
	cached := cacheLookup("scan_image", path)
	if cached != "" {
		scanResult.SetText(tr(iScanCacheHit) + "\r\n" + trf(iScanDecoded, cached))
		go processScanResult(cached)
		logWrite("scan", "cache_hit:"+path)
		return
	}

	f, err := os.Open(path)
	if err != nil {
		walk.MsgBox(mw, tr(iErrTitle), err.Error(), walk.MsgBoxIconError)
		return
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		walk.MsgBox(mw, tr(iErrTitle), trf(iMsgCannotDecode, err.Error()), walk.MsgBoxIconError)
		return
	}
	text := decodeImage(img)
	if text != "" {
		cachePut("scan_image", path, text)
		logWrite("scan", path+" -> "+text)
	}
}

func decodeScreenshot() {
	cmd := exec.Command("powershell", "-NoProfile", "-Command", `
Add-Type -AssemblyName System.Windows.Forms,System.Drawing
$b = [System.Windows.Forms.Screen]::PrimaryScreen.Bounds
$bmp = New-Object System.Drawing.Bitmap $b.Width, $b.Height
$g = [System.Drawing.Graphics]::FromImage($bmp)
$g.CopyFromScreen($b.Location, [System.Drawing.Point]::Empty, $b.Size)
$tmp = [System.IO.Path]::GetTempFileName() + ".png"
$bmp.Save($tmp, [System.Drawing.Imaging.ImageFormat]::Png)
$g.Dispose(); $bmp.Dispose()
Write-Output $tmp
`)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		walk.MsgBox(mw, tr(iErrTitle), trf(iMsgScreenshotFail, err.Error()), walk.MsgBoxIconError)
		return
	}
	tmpPath := strings.TrimSpace(out.String())
	defer os.Remove(tmpPath)
	decodeFile(tmpPath)
}

// decodeImage tries QR then 1D barcodes; returns decoded text or "".
func decodeImage(img image.Image) string {
	bitmap, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		scanResult.SetText(trf(iScanProcessFail, err.Error()))
		return ""
	}
	hints := map[gozxing.DecodeHintType]interface{}{
		gozxing.DecodeHintType_TRY_HARDER: true,
	}

	var result *gozxing.Result
	qrReader := qrcode.NewQRCodeReader()
	if res, err := qrReader.Decode(bitmap, hints); err == nil {
		result = res
	} else {
		onedReaders := []gozxing.Reader{
			oned.NewEAN13Reader(),
			oned.NewEAN8Reader(),
			oned.NewUPCAReader(),
			oned.NewUPCEReader(),
			oned.NewCode128Reader(),
			oned.NewCode39Reader(),
			oned.NewCodaBarReader(),
			oned.NewITFReader(),
		}
		for _, r := range onedReaders {
			if res, err := r.Decode(bitmap, hints); err == nil {
				result = res
				break
			}
		}
	}

	if result == nil {
		scanResult.SetText(tr(iScanNoCode))
		return ""
	}
	text := result.GetText()
	format := result.GetBarcodeFormat()
	scanResult.SetText(trf(iScanFormat, format.String()) + "\r\n" + trf(iScanDecoded, text))
	go processScanResult(text)
	return text
}

func processScanResult(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if strings.HasPrefix(text, "http://") || strings.HasPrefix(text, "https://") {
		openBrowser(text)
		scanResult.SetText(scanResult.Text() + "\r\n" + tr(iScanOpenedBrowser))
		return
	}
	if fi, err := os.Stat(text); err == nil && fi.IsDir() {
		openFolder(text)
		scanResult.SetText(scanResult.Text() + "\r\n" + trf(iScanOpenedFolder, text))
		return
	}
	if decoded, err := tryBase32Decode(text); err == nil && len(decoded) > 0 {
		scanResult.SetText(scanResult.Text() + "\r\n" + tr(iScanBase32) + "\r\n" + string(decoded))
		return
	}
	scanResult.SetText(scanResult.Text() + "\r\n" + tr(iScanPlainText))
}

func tryBase32Decode(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	s = strings.ToUpper(s)
	for len(s)%8 != 0 {
		s += "="
	}
	return base32.StdEncoding.DecodeString(s)
}

func openBrowser(url string) {
	if runtime.GOOS == "windows" {
		exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	}
}

func openFolder(path string) {
	exec.Command("explorer", path).Start()
}

// ---- Generate logic ----

func kindLabels() []string {
	return []string{tr(iKindQR), tr(iKindCode128), tr(iKindCode39), tr(iKindEAN13)}
}

func shapeLabels() []string {
	return []string{tr(iShapeSquare), tr(iShapeRounded), tr(iShapeDot), tr(iShapeDiamond)}
}

func eyeLabels() []string {
	return []string{tr(iEyeSquare), tr(iEyeRounded), tr(iEyeCircle)}
}

func borderLabels() []string {
	return []string{tr(iBorderNone), tr(iBorderThin), tr(iBorderThick)}
}

func comboIndex(c *walk.ComboBox, fallback int) int {
	if c == nil {
		return fallback
	}
	i := c.CurrentIndex()
	if i < 0 {
		return fallback
	}
	return i
}

func atoiOr(le *walk.LineEdit, def int) int {
	if le == nil {
		return def
	}
	v, err := strconv.Atoi(strings.TrimSpace(le.Text()))
	if err != nil {
		return def
	}
	return v
}

func textOr(le *walk.LineEdit) string {
	if le == nil {
		return ""
	}
	return strings.TrimSpace(le.Text())
}

// collectStyle reads every personalisation control into the key=value map that
// qrcore.dll parses. The keys are the same ones written to settings.tmmf, so
// saving and restoring the look needs no second vocabulary.
func collectStyle() map[string]string {
	m := map[string]string{
		"code_kind":    "qr",
		"fg_color":     "#000000",
		"bg_color":     "#FFFFFF",
		"transparent":  "0",
		"shape":        "square",
		"eye":          "square",
		"ecc":          "M",
		"cell":         "10",
		"quiet":        "4",
		"bar_width":    "2",
		"bar_height":   "140",
		"border":       "none",
		"border_color": "#000000",
		"logo_ratio":   "0.22",
	}
	switch comboIndex(cmbKind, 0) {
	case 1:
		m["code_kind"] = "code128"
	case 2:
		m["code_kind"] = "code39"
	case 3:
		m["code_kind"] = "ean13"
	}
	if s := textOr(edFg); s != "" {
		m["fg_color"] = s
	}
	if s := textOr(edBg); s != "" {
		m["bg_color"] = s
	}
	if s := textOr(edBorderCol); s != "" {
		m["border_color"] = s
	}
	if chkTrans != nil && chkTrans.Checked() {
		m["transparent"] = "1"
	}

	switch comboIndex(cmbShape, 0) {
	case 1:
		m["shape"] = "rounded"
	case 2:
		m["shape"] = "dot"
	case 3:
		m["shape"] = "diamond"
	}
	switch comboIndex(cmbEye, 0) {
	case 1:
		m["eye"] = "rounded"
	case 2:
		m["eye"] = "circle"
	}
	switch comboIndex(cmbECC, 1) {
	case 0:
		m["ecc"] = "L"
	case 2:
		m["ecc"] = "Q"
	case 3:
		m["ecc"] = "H"
	}
	switch comboIndex(cmbBorder, 0) {
	case 1:
		m["border"] = "thin"
	case 2:
		m["border"] = "thick"
	}
	switch comboIndex(cmbLogoRate, 1) {
	case 0:
		m["logo_ratio"] = "0.16"
	case 2:
		m["logo_ratio"] = "0.28"
	case 3:
		m["logo_ratio"] = "0.33"
	}
	m["cell"] = strconv.Itoa(atoiOr(edCell, 10))
	m["quiet"] = strconv.Itoa(atoiOr(edQuiet, 4))
	m["bar_width"] = strconv.Itoa(atoiOr(edBarW, 2))
	m["bar_height"] = strconv.Itoa(atoiOr(edBarH, 140))
	m["top_text"] = textOr(edTopText)
	m["bottom_text"] = textOr(edBotText)
	if logoPath != "" {
		m["logo_path"] = logoPath
	}
	return m
}

// specText renders the map as the "key=value" block qrcore.dll expects.
func specText(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		// a newline inside a value would break the line format
		v := strings.NewReplacer("\r", " ", "\n", " ").Replace(m[k])
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(v)
		b.WriteString("\n")
	}
	return b.String()
}

// styleKey is the cache key fragment so a styled result is not confused with a
// plain one.
func styleKey(m map[string]string) string {
	return specText(m)
}

func generateCode() {
	input := strings.TrimSpace(genInput.Text())
	if input == "" {
		walk.MsgBox(mw, tr(iErrTitle), tr(iMsgInputEmpty), walk.MsgBoxIconWarning)
		return
	}

	o := collectStyle()
	if logo := o["logo_path"]; logo != "" && o["code_kind"] != "qr" {
		walk.MsgBox(mw, tr(iErrTitle), tr(iMsgNeedQR), walk.MsgBoxIconWarning)
		delete(o, "logo_path")
	}
	// an avatar covers real modules, so the symbol needs spare capacity
	if o["logo_path"] != "" && (o["ecc"] == "L" || o["ecc"] == "M") {
		walk.MsgBox(mw, tr(iErrTitle), tr(iMsgLogoInUse), walk.MsgBoxIconInformation)
		o["ecc"] = "H"
	}

	key := input + "|" + styleKey(o)
	if cached := cacheLookup("generate", key); cached != "" {
		genPathLbl.SetText(trf(iGenCacheHit, cached))
		logWrite("generate", "cache_hit:"+input)
		renderAndShow(cached, o)
		return
	}

	var data string
	if fi, err := os.Stat(input); err == nil && !fi.IsDir() {
		b, err := os.ReadFile(input)
		if err != nil {
			walk.MsgBox(mw, tr(iErrTitle), trf(iMsgCannotRead, err.Error()), walk.MsgBoxIconError)
			return
		}
		encoded := base32.StdEncoding.EncodeToString(b)
		data = encoded
		genPathLbl.SetText(trf(iGenFileInfo, filepath.Base(input),
			fmt.Sprint(len(b)), fmt.Sprint(len(encoded))))
	} else {
		data = input
	}

	renderAndShow(data, o)
	cachePut("generate", key, data)
	logWrite("generate", input+" -> "+fmt.Sprint(len(data)))
}

// renderAndShow hands the request to qrcore.dll, which writes the PNG itself.
func renderAndShow(data string, o map[string]string) {
	target := filepath.Join(os.TempDir(), "qr_preview.png")
	if err := renderToFile(specText(o), data, target); err != nil {
		walk.MsgBox(mw, tr(iErrTitle), trf(iMsgRenderFail, err.Error()), walk.MsgBoxIconError)
		return
	}
	if err := showImage(target); err != nil {
		walk.MsgBox(mw, tr(iErrTitle), err.Error(), walk.MsgBoxIconError)
		return
	}
	currentPNG = target
	saveStyle(o)
}

// showImage loads the PNG the renderer produced into the image view.
func showImage(path string) error {
	bmp, err := walk.NewBitmapFromFile(path)
	if err != nil {
		return err
	}
	return genImg.SetImage(bmp)
}

// saveStyle keeps the last used look in the TMMF settings, so the next launch
// opens with the same controls.
func saveStyle(o map[string]string) {
	for _, k := range []string{
		"code_kind", "fg_color", "bg_color", "transparent", "shape", "eye", "ecc",
		"cell", "quiet", "bar_width", "bar_height", "top_text", "bottom_text",
		"border", "border_color", "logo_ratio",
	} {
		if v, ok := o[k]; ok {
			configSet(k, v)
		}
	}
	configSave()
}

// applySavedStyle restores the controls from the settings file on startup.
func applySavedStyle() {
	if !have(capConfig) {
		return
	}
	setEdit := func(le *walk.LineEdit, key string) {
		if v := configGet(key); v != "" && le != nil {
			le.SetText(v)
		}
	}
	setCombo := func(c *walk.ComboBox, idx int) {
		if c != nil && idx >= 0 {
			c.SetCurrentIndex(idx)
		}
	}
	kindIdx := 0
	switch configGet("code_kind") {
	case "code128":
		kindIdx = 1
	case "code39":
		kindIdx = 2
	case "ean13":
		kindIdx = 3
	}
	setCombo(cmbKind, kindIdx)

	shapeIdx := 0
	switch configGet("shape") {
	case "rounded":
		shapeIdx = 1
	case "dot":
		shapeIdx = 2
	case "diamond":
		shapeIdx = 3
	}
	setCombo(cmbShape, shapeIdx)

	eyeIdx := 0
	switch configGet("eye") {
	case "rounded":
		eyeIdx = 1
	case "circle":
		eyeIdx = 2
	}
	setCombo(cmbEye, eyeIdx)

	eccIdx := 1
	switch configGet("ecc") {
	case "L":
		eccIdx = 0
	case "Q":
		eccIdx = 2
	case "H":
		eccIdx = 3
	}
	setCombo(cmbECC, eccIdx)

	borderIdx := 0
	switch configGet("border") {
	case "thin":
		borderIdx = 1
	case "thick":
		borderIdx = 2
	}
	setCombo(cmbBorder, borderIdx)

	switch configGet("logo_ratio") {
	case "0.16":
		setCombo(cmbLogoRate, 0)
	case "0.28":
		setCombo(cmbLogoRate, 2)
	case "0.33":
		setCombo(cmbLogoRate, 3)
	}

	setEdit(edFg, "fg_color")
	setEdit(edBg, "bg_color")
	setEdit(edBorderCol, "border_color")
	setEdit(edCell, "cell")
	setEdit(edQuiet, "quiet")
	setEdit(edBarW, "bar_width")
	setEdit(edBarH, "bar_height")
	setEdit(edTopText, "top_text")
	setEdit(edBotText, "bottom_text")
	if chkTrans != nil && configGet("transparent") == "1" {
		chkTrans.SetChecked(true)
	}
}

func saveQR() {
	if currentPNG == "" {
		walk.MsgBox(mw, tr(iErrTitle), tr(iMsgNoQr), walk.MsgBoxIconWarning)
		return
	}
	dlg := new(walk.FileDialog)
	dlg.Title = tr(iDlgSaveQr)
	dlg.Filter = "PNG image (*.png)|*.png"
	dlg.FilePath = "qrcode.png"
	if ok, err := dlg.ShowSave(mw); err != nil || !ok {
		return
	}
	data, err := os.ReadFile(currentPNG)
	if err != nil {
		walk.MsgBox(mw, tr(iErrTitle), err.Error(), walk.MsgBoxIconError)
		return
	}
	if err := os.WriteFile(dlg.FilePath, data, 0644); err != nil {
		walk.MsgBox(mw, tr(iErrTitle), err.Error(), walk.MsgBoxIconError)
		return
	}
	walk.MsgBox(mw, tr(iMsgSaved), trf(iMsgSavedTo, dlg.FilePath), walk.MsgBoxIconInformation)
}
