# QR Scanner

A Windows desktop app written in Go with the [Walk](https://github.com/lxn/walk) GUI toolkit.
Scans QR codes and 1D barcodes, and generates QR codes and 1D barcodes with full
personalisation.

The app is split into a small GUI executable and two reusable DLLs, so the drawing
engine and the GraalVM/TMMF/LERF plumbing can be used by other projects without
carrying a GUI along.

## Download / Run

Get `qrscanner.exe`, `qrcore.dll` and `qrnative.dll` from the
[releases page](https://github.com/hxr431023/QRScanner/releases) and put them in one
folder together with `config.dll`, `lerf.dll`, `log.dll`, `tmmf.dll` and the
`resources\` folder, then double-click the exe. No installer and no runtime required.

All DLLs are loaded **only** from the directory containing `qrscanner.exe`. Absolute
paths are never hard-coded, so you can clone this repository anywhere and run it —
and a copy you download and unpack somewhere works the same way.

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
option below still produces a code a scanner can read. `style/spec_test.go` renders
each combination and decodes it back with gozxing; the test fails if a pretty code
stops being scannable.

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

Settings files are per-machine and are not committed; the LERF language files are
part of the repository.

### Logging and caching
`log.dll` records operations and caches results: scanning the same image twice returns
the cached decode instead of decoding again. Cache entries are matched on operation name
plus identical parameters and live in the daily log.

## How the app is split

`qrscanner.exe` keeps only what must own a window: the Walk tabs, the controls, the
dialogs and the message loop. That part cannot move into a DLL — Walk needs COM and
the Windows message pump on the process's main thread, and a DLL has no main thread of
its own.

Everything reusable lives in two DLLs next to the exe:

| File | Contents | Reuse it for |
|---|---|---|
| `qrcore.dll` | the symbol renderer: colours, module shapes, finder eyes, ECC, avatar, captions, frame, QR + Code 128 / Code 39 / EAN-13 | any app that has to draw a styled, still-scannable code |
| `qrnative.dll` | GraalVM DLL loading and isolate lifetime, TMMF settings, LERF language tables, logging and the result cache | any app from this suite that talks to the GraalVM DLL library |

Both are compiled by Go in `c-shared` mode and expose a plain C ABI, so they can be
loaded by C, C++, C#, Rust, Python, Node or another Go program. They only import
`KERNEL32` and the UCRT — no MinGW runtime has to be shipped next to them.

### Calling qrcore.dll

The ABI cannot carry a Go struct, so a render request crosses the boundary as plain
text: one `key=value` per line. The keys are the same ones `settings.tmmf` already
uses, so a caller can reuse the text it stores and does not have to learn a second
vocabulary.

```
Key           Meaning                                  Default
code_kind     qr, code128, code39, ean13               qr
fg_color      #RRGGBB / #RGB / RRGGBBAA                #000000
bg_color      same syntax                              #FFFFFF
transparent   1 or 0 (PNG alpha background)            0
shape         square, rounded, dot, diamond            square
eye           square, rounded, circle                  square
ecc           L, M, Q, H                               M
cell          pixels per module                        8
quiet         quiet zone in modules                    4
bar_width     narrow bar width in pixels (1D)           2
bar_height    bar height in pixels (1D)                80
top_text      caption above the symbol                 (empty)
bottom_text   caption below the symbol                 (empty)
border        none, thin, thick                        none
border_color  same colour syntax                      #000000
logo_path     PNG file to place in the centre (QR)     (empty)
logo_ratio    avatar side as a fraction of the symbol  0.22
```

Unknown keys are ignored, a missing key falls back to the default, `#` starts a comment
line, and CRLF is accepted. `style/spec.go` has `ParseSpec` / `EncodeSpec`, which are
inverse operations and are covered by round-trip tests.

Exports:

```c
int QRCoreRender(const char *spec, const char *content, const char *outPath);
int QRCoreRenderSize(const char *spec, const char *content, int *w, int *h);
int QRCoreVersion(char *buf, int size);
int QRCoreLastError(char *buf, int size);
```

Every call writes its result into a buffer the caller owns and returns a status code
(0 = ok). Nothing allocated inside the DLL is handed back, so there is no matching
free call to get wrong.

```python
# Python, with the DLLs in the same folder as the script
import ctypes
core = ctypes.CDLL("qrcore.dll")
spec = "code_kind=qr\nfg_color=#112E6A\nshape=rounded\necc=H\ncell=12\ntop_text=hello\n"
rc = core.QRCoreRender(spec.encode(), b"https://example.com", b"out.png")
```

### Calling qrnative.dll

```c
int QRNativeInit(const char *dir);          /* once, with the folder holding the DLLs */
int QRNativeCaps(void);                     /* bit 1 log, 2 config, 4 tmmf, 8 lerf */
int QRNativeConfigGet(const char *key, char *buf, int size);
int QRNativeConfigSet(const char *key, const char *value);
int QRNativeConfigSave(void);
int QRNativeSetLogEnabled(int on);
int QRNativeLogWrite(const char *op, const char *result);
int QRNativeCacheLookup(const char *op, const char *params, char *buf, int size);
int QRNativeCachePut(const char *op, const char *params, const char *result);
int QRNativeLoadLang(const char *code);     /* "en" or "zh" */
int QRNativeTranslate(const char *id, char *buf, int size);
int QRNativeVersion(char *buf, int size);
int QRNativeLastError(char *buf, int size);
```

A missing `config.dll`, `lerf.dll`, `log.dll` or `tmmf.dll` clears the matching
capability bit instead of failing — callers check `QRNativeCaps` and switch the feature
off. `QRNativeSetLogEnabled` keeps the switch inside the DLL rather than pushing it into
`log.dll`: that one export crashes when called from inside this DLL, while every other
log export works.

## Reusable packages

Two Go packages are importable as-is, and one is a package the DLL is built from.
Copy a directory into another Go project and it works unchanged — none of them contains
a hard-coded absolute path.

| Package | Contents | Reuse it for |
|---|---|---|
| `native/` | DLL loading and isolate lifetime, safe argument marshalling, TMMF settings store, log + result cache, LERF parser | any app that talks to the GraalVM DLL library |
| `paint/` | hex colour parsing, rectangle / rounded rect / circle / ring / diamond / stroke primitives, system TrueType loading with CJK support | any app that draws its own images |
| `style/` | `StyleOptions`, `RenderCode` and the `key=value` spec parser | drawing a styled, scannable QR or 1D symbol |

```go
import (
    "qrscanner/native"
    "qrscanner/paint"
    "qrscanner/style"
)

cfg := native.NewConfig(native.AppDir())   // TMMF settings, nil-safe
log := native.NewLog(native.AppDir())      // log + cache, nil-safe
lang, ok := native.LoadDir(native.AppDir(), "zh", "en")

c, _ := paint.ParseHex("#1E88E5")
paint.RoundRect(img, image.Rect(0, 0, 40, 40), 8, c)

o := style.ParseSpec("code_kind=qr\necc=H\nshape=rounded\n")
img, _ := style.RenderCode("https://example.com", o)
```

`native/` and `style/` treat a missing DLL or font as "feature off" rather than as an
error, so an app keeps running without them.

## Native DLLs (all included in this repository)

| File | Role | Built from |
|---|---|---|
| `qrcore.dll` | symbol renderer | `cmd/qrcore` |
| `qrnative.dll` | settings / language / log / cache layer | `cmd/qrnative` |
| `log.dll` | operation log + result cache | GraalVM native-image |
| `config.dll` | settings, persisted in TMMF format | GraalVM native-image |
| `tmmf.dll` | Tag-Multiple-Match format parser | GraalVM native-image |
| `lerf.dll` | LERF key-value / language file parser | GraalVM native-image |

The four GraalVM DLLs expose a plain C ABI and are loaded with `LoadLibrary` +
`GetProcAddress` (see `native/`). Each creates its own isolate; a missing DLL turns the
corresponding feature off and the app still starts.

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

`{0}`, `{1}` are placeholders filled at runtime. `lerf.dll` validates the format;
entry lookup is done in code because this build of `lerf.dll` echoes the requested key
instead of returning the stored text.

## Build from source

Requirements: Go 1.21+ and a MinGW-w64 gcc, because the two DLLs are cgo builds.
Nothing else — the DLLs do not need the MinGW runtime at run time.

```powershell
.\build.ps1
```

That builds `qrcore.dll`, `qrnative.dll` and `qrscanner.exe` in order. To rebuild only
the DLLs:

```powershell
.\build.ps1 -SkipExe
```

If gcc is not on `PATH`, pass `-GccPath C:\path\to\gcc.exe`; `-GoExe` does the same for
Go. The script lives in `build\build_dlls.ps1`.

The same steps by hand:

```powershell
$env:CGO_ENABLED = "1"
$env:CC = "C:\wpjc\mingw64\bin\gcc.exe"

go build -buildmode=c-shared -o qrcore.dll   ./cmd/qrcore
go build -buildmode=c-shared -o qrnative.dll ./cmd/qrnative

# manifest (Common Controls 6.0), then the GUI
go install github.com/akavel/rsrc@latest
rsrc -manifest qrscanner.manifest -o rsrc.syso
go build -ldflags "-H windowsgui" -o qrscanner.exe .
```

`c-shared` also writes `qrcore.h` and `qrnative.h`; they are committed because a
caller in C, C++ or C# needs them.

The manifest is **required**. Without Common Controls 6.0 the Walk widgets fail to
initialize and the app exits before showing a window.

## Tests

```powershell
go test ./...
```

- `style/` renders every shape, eye, colour, avatar and caption combination and
  decodes each one back with gozxing.
- `native/`, `paint/` cover the settings store, the LERF parser and the drawing
  primitives.
- the root package loads the real `qrcore.dll` and `qrnative.dll` from the repository
  and renders through them, so a changed export or a broken spec key fails the test
  rather than the window. Those tests skip when the DLLs have not been built.

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
