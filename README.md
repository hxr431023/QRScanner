# QR Scanner

A Windows desktop app written in Go with the [Walk](https://github.com/lxn/walk) GUI toolkit.
Scans QR codes and 1D barcodes, and generates QR codes.

## Download / Run

Get `qrscanner.exe` from the [releases page](https://github.com/hxr431023/QRScanner/releases)
and double-click it. No installer, no runtime, no dependencies.

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

### Logging and caching
If `log.dll` is present (next to the executable, or in the local DLL library), the app
writes operation logs and reuses cached results — scanning the same image twice returns
the cached decode instead of decoding again. If `log.dll` is missing, the app runs
normally with logging disabled.

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

## Dependencies

| Module | Purpose |
|---|---|
| `github.com/lxn/walk` | native Win32 GUI |
| `github.com/lxn/win` | Win32 API bindings |
| `github.com/boombuler/barcode` | QR generation |
| `github.com/makiuchi-d/gozxing` | QR + 1D barcode decoding |

## License

MPL-2.0
