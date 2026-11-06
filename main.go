package main

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
	"syscall"
	"unsafe"

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
)

// ---- log.dll binding ----
var (
	logDll                  *syscall.DLL
	fnCreateIsolate        uintptr
	fnDetachIsolate        uintptr
	fnLogInit              uintptr
	fnLogWrite             uintptr
	fnLogShutdown          uintptr
	fnLogCacheLookup       uintptr
	fnLogCachePut          uintptr
	logThread              uintptr
)

func initLog() {
	dllPath := `C:\wpjc\md\native\DLL\core\log.dll`
	if _, err := os.Stat(dllPath); err != nil {
		// log.dll not found, run without logging
		return
	}
	logDll = syscall.MustLoadDLL(dllPath)
	fnCreateIsolate = logDll.MustFindProc("graal_create_isolate").Addr()
	fnDetachIsolate = logDll.MustFindProc("graal_detach_all_threads_and_tear_down_isolate").Addr()
	fnLogInit = logDll.MustFindProc("log_init").Addr()
	fnLogWrite = logDll.MustFindProc("log_write").Addr()
	fnLogShutdown = logDll.MustFindProc("log_shutdown").Addr()
	fnLogCacheLookup = logDll.MustFindProc("log_cache_lookup").Addr()
	fnLogCachePut = logDll.MustFindProc("log_cache_put").Addr()

	var isolate, thread uintptr
	ret, _, _ := syscall.SyscallN(fnCreateIsolate, 0,
		uintptr(unsafe.Pointer(&isolate)),
		uintptr(unsafe.Pointer(&thread)))
	if ret != 0 {
		return
	}
	logThread = thread
	baseDir, _ := filepath.Abs(".")
	syscall.SyscallN(fnLogInit, logThread,
		uintptr(unsafe.Pointer(syscall.StringBytePtr(baseDir))))
}

func shutdownLog() {
	if logThread == 0 {
		return
	}
	syscall.SyscallN(fnLogShutdown, logThread)
	syscall.SyscallN(fnDetachIsolate, logThread)
}

func logWrite(op, result string) {
	if logThread == 0 {
		return
	}
	syscall.SyscallN(fnLogWrite, logThread,
		uintptr(unsafe.Pointer(syscall.StringBytePtr("qrscanner"))),
		uintptr(unsafe.Pointer(syscall.StringBytePtr(op))),
		uintptr(unsafe.Pointer(syscall.StringBytePtr(result))))
}

// cacheLookup returns cached result or empty string on miss
func cacheLookup(op, params string) string {
	if logThread == 0 {
		return ""
	}
	buf := make([]byte, 65536)
	ret, _, _ := syscall.SyscallN(fnLogCacheLookup, logThread,
		uintptr(unsafe.Pointer(syscall.StringBytePtr(op))),
		uintptr(unsafe.Pointer(syscall.StringBytePtr(params))),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)))
	if ret < 0 {
		return ""
	}
	return string(buf[:ret])
}

func cachePut(op, params, result string) {
	if logThread == 0 {
		return
	}
	syscall.SyscallN(fnLogCachePut, logThread,
		uintptr(unsafe.Pointer(syscall.StringBytePtr(op))),
		uintptr(unsafe.Pointer(syscall.StringBytePtr(params))),
		uintptr(unsafe.Pointer(syscall.StringBytePtr(result))))
}

func main() {
	initLog()
	defer shutdownLog()

	MainWindow{
		AssignTo: &mw,
		Title:    "QR Scanner",
		Size:     Size{Width: 600, Height: 500},
		Layout:   VBox{},
		Children: []Widget{
			TabWidget{
				Pages: []TabPage{
					TabPage{
						Title:  "Scan",
						Layout: VBox{},
						Children: []Widget{
							Composite{
								Layout: HBox{},
								Children: []Widget{
									PushButton{
										Text: "Open image file...",
										OnClicked: func() {
											dlg := new(walk.FileDialog)
											dlg.Title = "Select QR / barcode image"
											dlg.Filter = "Images (*.png;*.jpg;*.jpeg;*.bmp)|*.png;*.jpg;*.jpeg;*.bmp"
											if ok, err := dlg.ShowOpen(mw); err != nil || !ok {
												return
											}
											decodeFile(dlg.FilePath)
										},
									},
									PushButton{
										Text: "Screenshot and decode",
										OnClicked: func() {
											decodeScreenshot()
										},
									},
								},
							},
							Label{Text: "Result:"},
							TextEdit{
								AssignTo: &scanResult,
								ReadOnly: true,
								MinSize:  Size{Height: 200},
							},
						},
					},
					TabPage{
						Title:  "Generate",
						Layout: VBox{},
						Children: []Widget{
							Label{Text: "Text or file to encode:"},
							LineEdit{
								AssignTo: &genInput,
							},
							Composite{
								Layout: HBox{},
								Children: []Widget{
									PushButton{
										Text: "Choose file...",
										OnClicked: func() {
											dlg := new(walk.FileDialog)
											dlg.Title = "Select file to encode"
											if ok, err := dlg.ShowOpen(mw); err != nil || !ok {
												return
											}
											genPathLbl.SetText("File: " + dlg.FilePath)
											genInput.SetText(dlg.FilePath)
										},
									},
									PushButton{
										Text: "Generate QR",
										OnClicked: func() {
											generateQR(genInput.Text())
										},
									},
									PushButton{
										Text: "Save PNG...",
										OnClicked: func() {
											saveQR()
										},
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
				},
			},
		},
	}.Create()

	mw.Run()
}

// ---- Scan logic ----

func decodeFile(path string) {
	// Check cache first
	cached := cacheLookup("scan_image", path)
	if cached != "" {
		scanResult.SetText("(cache hit)\nDecoded: " + cached + "\n")
		go processScanResult(cached)
		logWrite("scan", "cache_hit:"+path)
		return
	}

	f, err := os.Open(path)
	if err != nil {
		walk.MsgBox(mw, "Error", err.Error(), walk.MsgBoxIconError)
		return
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		walk.MsgBox(mw, "Error", "Cannot decode image: "+err.Error(), walk.MsgBoxIconError)
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
		walk.MsgBox(mw, "Error", "Screenshot failed: "+err.Error(), walk.MsgBoxIconError)
		return
	}
	tmpPath := strings.TrimSpace(out.String())
	defer os.Remove(tmpPath)
	decodeFile(tmpPath)
}

// decodeImage tries QR then 1D barcodes; returns decoded text or ""
func decodeImage(img image.Image) string {
	bitmap, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		scanResult.SetText("Failed to process image: " + err.Error())
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
		scanResult.SetText("No barcode found (tried QR + EAN13/EAN8/UPC-A/UPC-E/Code128/Code39/Codabar/ITF)")
		return ""
	}
	text := result.GetText()
	format := result.GetBarcodeFormat()
	scanResult.SetText(fmt.Sprintf("Format: %s\nDecoded: %s\n", format.String(), text))
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
		scanResult.SetText(scanResult.Text() + "→ Opened in browser\n")
		return
	}
	if fi, err := os.Stat(text); err == nil && fi.IsDir() {
		openFolder(text)
		scanResult.SetText(scanResult.Text() + "→ Opened folder: " + text + "\n")
		return
	}
	if decoded, err := tryBase32Decode(text); err == nil && len(decoded) > 0 {
		scanResult.SetText(scanResult.Text() + "→ Base32 decoded:\n" + string(decoded) + "\n")
		return
	}
	scanResult.SetText(scanResult.Text() + "→ Plain text\n")
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
		walk.MsgBox(mw, "Error", "Input is empty", walk.MsgBoxIconWarning)
		return
	}

	// Check cache
	cached := cacheLookup("generate", input)
	if cached != "" {
		genPathLbl.SetText("(cache hit: " + cached + ")")
		logWrite("generate", "cache_hit:"+input)
		// still regenerate image from cached text
		doGenerateQR(cached, true)
		return
	}

	var data string
	if fi, err := os.Stat(input); err == nil && !fi.IsDir() {
		b, err := os.ReadFile(input)
		if err != nil {
			walk.MsgBox(mw, "Error", "Cannot read file: "+err.Error(), walk.MsgBoxIconError)
			return
		}
		encoded := base32.StdEncoding.EncodeToString(b)
		data = encoded
		genPathLbl.SetText(fmt.Sprintf("File: %s (%d bytes → base32 %d chars)", filepath.Base(input), len(b), len(encoded)))
	} else {
		data = input
	}

	doGenerateQR(data, false)
	cachePut("generate", input, data)
	logWrite("generate", input+" -> "+fmt.Sprintf("%d chars", len(data)))
}

func doGenerateQR(data string, isCached bool) {
	qrCode, err := qr.Encode(data, qr.M, qr.Auto)
	if err != nil {
		walk.MsgBox(mw, "Error", "QR encode failed: "+err.Error(), walk.MsgBoxIconError)
		return
	}
	scaled, err := barcode.Scale(qrCode, 4, 4)
	if err != nil {
		walk.MsgBox(mw, "Error", err.Error(), walk.MsgBoxIconError)
		return
	}
	currentQR = scaled

	tmp := filepath.Join(os.TempDir(), "qr_preview.png")
	f, err := os.Create(tmp)
	if err != nil {
		walk.MsgBox(mw, "Error", err.Error(), walk.MsgBoxIconError)
		return
	}
	if err := png.Encode(f, scaled); err != nil {
		f.Close()
		walk.MsgBox(mw, "Error", err.Error(), walk.MsgBoxIconError)
		return
	}
	f.Close()

	bmp, err := walk.NewBitmapFromFile(tmp)
	if err != nil {
		walk.MsgBox(mw, "Error", err.Error(), walk.MsgBoxIconError)
		return
	}
	genImg.SetImage(bmp)
}

func saveQR() {
	if currentQR == nil {
		walk.MsgBox(mw, "Error", "No QR code generated yet", walk.MsgBoxIconWarning)
		return
	}
	dlg := new(walk.FileDialog)
	dlg.Title = "Save QR code as PNG"
	dlg.Filter = "PNG image (*.png)|*.png"
	dlg.FilePath = "qrcode.png"
	if ok, err := dlg.ShowSave(mw); err != nil || !ok {
		return
	}
	f, err := os.Create(dlg.FilePath)
	if err != nil {
		walk.MsgBox(mw, "Error", err.Error(), walk.MsgBoxIconError)
		return
	}
	defer f.Close()
	png.Encode(f, currentQR)
	walk.MsgBox(mw, "Saved", "Saved to: "+dlg.FilePath, walk.MsgBoxIconInformation)
}
