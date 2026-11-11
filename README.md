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
- Pick the symbol type first — **QR**, **Code 128**, **Code 39** or **EAN-13**.
- Type text, or pick a file with **Choose file...** (the file is Base32-encoded first),
  then press **Generate**.
- **Save PNG...** writes the generated symbol to disk.

#### Personalisation (v0.2)
The generator draws the symbol itself instead of scaling a 1bpp bitmap, so every
option below still produces a code a scanner can read. `style_test.go` renders each
combination and decodes it back with gozxing; the test fails if a pretty code stops
being scannable.

| Option | QR | 1D |
|---|---|---|
| Foreground / background colour (`#RRGGBB`, `#RGB`, `RRGGBBAA`) | yes | yes |
| Transparent background (PNG alpha) | yes | yes |
| Module shape: square / rounded / dot / diamond | yes | — |
| Finder eye: square / rounded / circle | yes | — |
| Error correction L / M / Q / H | yes | — |
| Pixels per module, quiet zone | yes | — |
| Bar width / bar height | — | yes |
| Avatar in the centre (optional white plate) | yes | — |
| Avatar size as a fraction of the symbol | yes | — |
| Caption above / below, Chinese and English text | yes | yes |
| Frame: none / thin / thick, with its own colour | yes | yes |

Notes:
- The **circle** eye keeps flat sides and rounds everything else. A pure circle erases
  the straight edges a scanner uses to locate the pattern, and decoding fails.
- A centred avatar covers real modules, so the error correction level is raised to **H**
  automatically when it is lower.
- 1D symbols always keep a quiet zone of at least ten narrow bars on both sides;
  without it a scanner cannot find where the symbol starts.
- Walk has no colour picker, so colours are typed as hex and validated on generate.

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

## Reusable packages

The parts that are not specific to barcodes live in two standalone packages. Copy a
directory into another Go project and it works unchanged — neither of them contains a
single hard-coded absolute path.

| Package | Contents | Reuse it for |
|---|---|---|
| `native/` | DLL loading and isolate lifetime, safe argument marshalling, TMMF settings store, log + result cache, LERF parser | any app that talks to the GraalVM DLL library |
| `paint/` | hex colour parsing, rectangle / rounded rect / circle / ring / diamond / stroke primitives, system TrueType loading with CJK support | any app that draws its own images |

```go
import (
    "qrscanner/native"
    "qrscanner/paint"
)

cfg := native.NewConfig(native.AppDir())   // TMMF settings, nil-safe
log := native.NewLog(native.AppDir())      // log + cache, nil-safe
lang, ok := native.LoadDir(native.AppDir(), "zh", "en")

c, _ := paint.ParseHex("#1E88E5")
paint.RoundRect(img, image.Rect(0, 0, 40, 40), 8, c)
```

Both packages treat a missing DLL or missing font as "feature off" rather than as an
error, so an app keeps running without them.

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
| `github.com/boombuler/barcode` | QR + 1D matrix generation |
| `github.com/makiuchi-d/gozxing` | QR + 1D barcode decoding |
| `golang.org/x/image` | TrueType text rendering (captions) |

## License

MPL-2.0
