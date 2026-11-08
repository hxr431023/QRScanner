package main

// QR Scanner — Walk GUI over gozxing (decode) and boombuler/barcode (encode).
// UI strings come from LERF language files via lerf.dll (English + Chinese).

import (
	"bytes"
	"encoding/base32"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/qr"
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
	currentQR  barcode.Barcode

	chkLogEnabled *walk.CheckBox
	cmbFormat     *walk.ComboBox
	cmbLang       *walk.ComboBox
	statusLbl     *walk.Label
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
		Size:     Size{Width: 620, Height: 520},
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
										OnClicked: func() { generateQR(genInput.Text()) },
									},
									PushButton{
										Text:      tr(iBtnSavePng),
										OnClicked: func() { saveQR() },
									},
								},
							},
							Label{AssignTo: &genPathLbl, Text: ""},
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
		appDir(), on(logDLL != nil), on(cfgDLL != nil), on(tmmfDLL != nil), on(lerfDLL != nil)))
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

func generateQR(input string) {
	input = strings.TrimSpace(input)
	if input == "" {
		walk.MsgBox(mw, tr(iErrTitle), tr(iMsgInputEmpty), walk.MsgBoxIconWarning)
		return
	}

	cached := cacheLookup("generate", input)
	if cached != "" {
		genPathLbl.SetText(trf(iGenCacheHit, cached))
		logWrite("generate", "cache_hit:"+input)
		doGenerateQR(cached)
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

	doGenerateQR(data)
	cachePut("generate", input, data)
	logWrite("generate", input+" -> "+fmt.Sprint(len(data)))
}

func doGenerateQR(data string) {
	qrCode, err := qr.Encode(data, qr.M, qr.Auto)
	if err != nil {
		walk.MsgBox(mw, tr(iErrTitle), trf(iMsgQrEncodeFail, err.Error()), walk.MsgBoxIconError)
		return
	}
	scaled, err := barcode.Scale(qrCode, 4, 4)
	if err != nil {
		walk.MsgBox(mw, tr(iErrTitle), err.Error(), walk.MsgBoxIconError)
		return
	}
	currentQR = scaled

	tmp := filepath.Join(os.TempDir(), "qr_preview.png")
	f, err := os.Create(tmp)
	if err != nil {
		walk.MsgBox(mw, tr(iErrTitle), err.Error(), walk.MsgBoxIconError)
		return
	}
	if err := png.Encode(f, scaled); err != nil {
		f.Close()
		walk.MsgBox(mw, tr(iErrTitle), err.Error(), walk.MsgBoxIconError)
		return
	}
	f.Close()

	bmp, err := walk.NewBitmapFromFile(tmp)
	if err != nil {
		walk.MsgBox(mw, tr(iErrTitle), err.Error(), walk.MsgBoxIconError)
		return
	}
	genImg.SetImage(bmp)
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
	png.Encode(f, currentQR)
	walk.MsgBox(mw, tr(iMsgSaved), trf(iMsgSavedTo, dlg.FilePath), walk.MsgBoxIconInformation)
}
