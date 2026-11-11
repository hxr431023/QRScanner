package main

// QR Scanner — Walk GUI over gozxing (decode) and boombuler/barcode (encode).
// UI strings come from LERF language files via lerf.dll (English + Chinese).

import (
	"bytes"
	"encoding/base32"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/oned"
	"github.com/makiuchi-d/gozxing/qrcode"
	"github.com/boombuler/barcode/qr"
)

var (
	mw         *walk.MainWindow
	scanResult *walk.TextEdit
	genInput   *walk.LineEdit
	genImg     *walk.ImageView
	genPathLbl *walk.Label
	currentQR  image.Image

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
	initLog()
	initConfig()
	initTmmf()
	initLerf()
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
		appDir(), on(logStore != nil), on(cfgStore != nil), on(tmmfDLL.Valid()), on(lerfDLL.Valid())))
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

// collectStyle reads every personalisation control into one StyleOptions.
func collectStyle() StyleOptions {
	o := DefaultStyle()
	switch comboIndex(cmbKind, 0) {
	case 1:
		o.Kind = "code128"
	case 2:
		o.Kind = "code39"
	case 3:
		o.Kind = "ean13"
	default:
		o.Kind = "qr"
	}
	if c, err := ParseHexColor(textOr(edFg)); err == nil {
		o.Foreground = c
	}
	if c, err := ParseHexColor(textOr(edBg)); err == nil {
		o.Background = c
	}
	if c, err := ParseHexColor(textOr(edBorderCol)); err == nil {
		o.BorderColor = c
	}
	o.Transparent = chkTrans != nil && chkTrans.Checked()

	switch comboIndex(cmbShape, 0) {
	case 1:
		o.Shape = "rounded"
	case 2:
		o.Shape = "dot"
	case 3:
		o.Shape = "diamond"
	}
	switch comboIndex(cmbEye, 0) {
	case 1:
		o.Eye = "rounded"
	case 2:
		o.Eye = "circle"
	}
	switch comboIndex(cmbECC, 1) {
	case 0:
		o.ECC = qr.L
	case 2:
		o.ECC = qr.Q
	case 3:
		o.ECC = qr.H
	default:
		o.ECC = qr.M
	}
	switch comboIndex(cmbBorder, 0) {
	case 1:
		o.Border = "thin"
	case 2:
		o.Border = "thick"
	}
	switch comboIndex(cmbLogoRate, 1) {
	case 0:
		o.LogoRatio = 0.16
	case 2:
		o.LogoRatio = 0.28
	case 3:
		o.LogoRatio = 0.33
	default:
		o.LogoRatio = 0.22
	}
	o.Cell = atoiOr(edCell, 10)
	o.Quiet = atoiOr(edQuiet, 4)
	o.BarWidth = atoiOr(edBarW, 2)
	o.BarHeight = atoiOr(edBarH, 140)
	o.TopText = textOr(edTopText)
	o.BottomText = textOr(edBotText)
	o.LogoPath = logoPath
	return o
}

// styleKey is the cache key fragment so a styled result is not confused with a
// plain one.
func styleKey(o StyleOptions) string {
	return fmt.Sprintf("%s|%v|%v|%v|%s|%s|%s|%d|%d|%d|%d|%s|%s|%s|%v|%s|%.2f",
		o.Kind, o.Foreground, o.Background, o.Transparent, o.Shape, o.Eye, o.ECC.String(),
		o.Cell, o.Quiet, o.BarWidth, o.BarHeight, o.TopText, o.BottomText,
		o.Border, o.BorderColor, o.LogoPath, o.LogoRatio)
}

func generateCode() {
	input := strings.TrimSpace(genInput.Text())
	if input == "" {
		walk.MsgBox(mw, tr(iErrTitle), tr(iMsgInputEmpty), walk.MsgBoxIconWarning)
		return
	}

	o := collectStyle()
	if o.LogoPath != "" && o.Kind != "qr" {
		walk.MsgBox(mw, tr(iErrTitle), tr(iMsgNeedQR), walk.MsgBoxIconWarning)
		o.LogoPath = ""
	}
	if o.LogoPath != "" && o.ECC < qr.Q {
		walk.MsgBox(mw, tr(iErrTitle), tr(iMsgLogoInUse), walk.MsgBoxIconInformation)
		o.ECC = qr.H
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

func renderAndShow(data string, o StyleOptions) {
	img, err := RenderCode(data, o)
	if err != nil {
		walk.MsgBox(mw, tr(iErrTitle), trf(iMsgRenderFail, err.Error()), walk.MsgBoxIconError)
		return
	}
	currentQR = img
	if err := showImage(img); err != nil {
		walk.MsgBox(mw, tr(iErrTitle), err.Error(), walk.MsgBoxIconError)
		return
	}
	saveStyle(o)
}

// showImage writes the render to a temp PNG and hands it to the image view.
func showImage(img image.Image) error {
	tmp := filepath.Join(os.TempDir(), "qr_preview.png")
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	f.Close()

	bmp, err := walk.NewBitmapFromFile(tmp)
	if err != nil {
		return err
	}
	return genImg.SetImage(bmp)
}

func hexOf(c color.RGBA) string {
	return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
}

// saveStyle keeps the last used look in the TMMF settings, so the next launch
// opens with the same controls.
func saveStyle(o StyleOptions) {
	if cfgStore == nil {
		return
	}
	configSet("code_kind", o.Kind)
	configSet("fg_color", hexOf(o.Foreground))
	configSet("bg_color", hexOf(o.Background))
	configSet("transparent", boolStr(o.Transparent))
	configSet("shape", o.Shape)
	configSet("eye", o.Eye)
	configSet("ecc", o.ECC.String())
	configSet("cell", strconv.Itoa(o.Cell))
	configSet("quiet", strconv.Itoa(o.Quiet))
	configSet("bar_width", strconv.Itoa(o.BarWidth))
	configSet("bar_height", strconv.Itoa(o.BarHeight))
	configSet("top_text", o.TopText)
	configSet("bottom_text", o.BottomText)
	configSet("border", o.Border)
	configSet("border_color", hexOf(o.BorderColor))
	configSet("logo_ratio", fmt.Sprintf("%.2f", o.LogoRatio))
	configSave()
}

// applySavedStyle restores the controls from the settings file on startup.
func applySavedStyle() {
	if cfgStore == nil {
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
	if currentQR == nil {
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
	f, err := os.Create(dlg.FilePath)
	if err != nil {
		walk.MsgBox(mw, tr(iErrTitle), err.Error(), walk.MsgBoxIconError)
		return
	}
	defer f.Close()
	if err := png.Encode(f, currentQR); err != nil {
		walk.MsgBox(mw, tr(iErrTitle), err.Error(), walk.MsgBoxIconError)
		return
	}
	walk.MsgBox(mw, tr(iMsgSaved), trf(iMsgSavedTo, dlg.FilePath), walk.MsgBoxIconInformation)
}
