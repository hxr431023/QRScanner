# QR Scanner

A Windows desktop app written in Go with the [Walk](https://github.com/lxn/walk) GUI toolkit.
Scans QR codes and 1D barcodes, and generates QR codes.

## Download / Run

Get `qrscanner.exe` from the [releases page](https://github.com/hxr431023/QRScanner/releases)
and put it in a folder together with this repository's `*.dll` files and `resources\`
folder, then double-click it. No installer and no runtime required.

All native DLLs are loaded **only** from the directory containing `qrscanner.exe`.
Absolute paths are never hard-coded, so you can clone this repository anywhere and run it.

## Features

### Scan tab
- **Open image file...** — decode a QR code or 1D barcode from a PNG/JPG/BMP file.
- **Screenshot and decode** — capture the current primary screen and decode it.
- Supported formats:
  - QR Code
  - EAN-13, EAN-8, UPC-A, UPC-E
  - Code 128, Code 39, Codabar, ITF
- The detected format is shown above the decoded text.
- Decoded results are handled automatically:
  - `http://` / `https://` → opens in the default browser
  - an existing local directory → opens in Explorer
  - valid Base32 → decoded and shown
  - anything else → shown as plain text

### Generate tab
- Type text, or pick a file with **Choose file...** (the file is Base32-encoded first),
  then press **Generate QR**.
- **Save PNG...** writes the generated QR code to disk.

### Settings tab
- **Enable logging and result cache** — turns `log.dll` logging on or off.
- **Output format** — `plain`, `json`, or `detailed`.
- **Language** — English or 中文. Applied on the next start.
- Settings are stored by `config.dll` in **TMMF** format at
  `resources\cmd\settings.tmmf`.

### Logging and caching
`log.dll` records operations and caches results: scanning the same image twice returns
the cached decode instead of decoding again. Cache entries are matched on operation name
plus identical parameters and live in the daily log.

## Native DLLs (all included in this repository)

| File | Role |
|---|---|
| `log.dll` | operation log + result cache |
| `config.dll` | settings, persisted in TMMF format |
| `tmmf.dll` | Tag-Multiple-Match format parser |
| `lerf.dll` | LERF key-value / language file parser |

Each of them is a GraalVM native-image build that exposes a plain C ABI, so they are
loaded with `LoadLibrary` + `GetProcAddress` (see `dlls.go`). Every DLL create its own
isolate. If a DLL is missing the corresponding feature silently switches off — the app
still starts and works.

## Languages

UI text is not hard-coded. It lives in two LERF files:

- `resources\cmd\en.lerf` — English
- `resources\cmd\zh.lerf` — Chinese

LERF is a small resource format:

```
LERF.utf8
QRScanner.V1.0.0

# comment
<1>QR Scanner<1>
<2>Scan<2>
```

`{0}`, `{1}` are placeholders filled at runtime. `lerf.dll` is used to validate the
format; entry lookup is performed in code because this build of `lerf.dll` echoes the
requested key instead of returning the stored text.

## Build from source

Requirements: Go 1.21+ on Windows.

```powershell
# 1. generate the resource file (embeds the application manifest)
go install github.com/akavel/rsrc@latest
rsrc -manifest qrscanner.manifest -o rsrc.syso

# 2. build
go build -ldflags "-H windowsgui" -o qrscanner.exe .
```

The manifest is **required**. Without Common Controls 6.0 the Walk widgets fail to
initialize and the app exits before showing a window.

Or just run the helper script:

```powershell
.\build.ps1
```

## Go dependencies

| Module | Purpose |
|---|---|
| `github.com/lxn/walk` | native Win32 GUI |
| `github.com/lxn/win` | Win32 API bindings |
| `github.com/boombuler/barcode` | QR generation |
| `github.com/makiuchi-d/gozxing` | QR + 1D barcode decoding |

## License

MPL-2.0
