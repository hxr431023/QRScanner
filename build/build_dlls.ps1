#Requires -Version 5.1
<#
.SYNOPSIS
    Builds the two reusable DLLs and the GUI executable of QRScanner.

.DESCRIPTION
    qrcore.dll   - the symbol renderer (QR + Code128 / Code39 / EAN-13).
    qrnative.dll - the general layer (GraalVM DLLs, TMMF settings, LERF text,
                   logging and the result cache).
    qrscanner.exe - the Walk GUI. It keeps the window, tabs and message loop in
                   the process, because Walk needs COM and the message pump on the
                   main thread; everything reusable lives in the two DLLs.

    Both DLLs are cgo builds, so a MinGW-w64 gcc is required. If the build
    machine has none, pass -GccPath. Nothing else has to be installed: the DLLs
    only import KERNEL32 and the UCRT, so no MinGW runtime has to be shipped.

.EXAMPLE
    .\build.ps1

.EXAMPLE
    .\build_dlls.ps1 -GccPath "C:\wpjc\mingw64\bin\gcc.exe"

.EXAMPLE
    .\build_dlls.ps1 -SkipExe          # only rebuild the DLLs
#>
param(
    [string]$GccPath = "",
    [string]$GoExe = "",
    [switch]$SkipExe
)

$ErrorActionPreference = "Stop"

# The repository root is the parent of this script's folder.
$root = Split-Path -Parent $PSScriptRoot
Set-Location -LiteralPath $root

function Resolve-Tool {
    param([string]$Explicit, [string]$Name, [string[]]$Fallbacks)

    if ($Explicit) {
        if (-not (Test-Path -LiteralPath $Explicit)) {
            throw "$Name not found at $Explicit"
        }
        return $Explicit
    }
    $cmd = Get-Command $Name -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }
    foreach ($f in $Fallbacks) {
        if (Test-Path -LiteralPath $f) { return $f }
    }
    throw "$Name was not found. Install it, or pass an explicit path."
}

$go = Resolve-Tool -Explicit $GoExe -Name "go" -Fallbacks @("C:\wpjc\go\bin\go.exe")
$gcc = Resolve-Tool -Explicit $GccPath -Name "gcc" -Fallbacks @(
    "C:\wpjc\mingw64\bin\gcc.exe",
    "C:\mingw64\bin\gcc.exe"
)

# cgo needs gcc reachable through CC, and its own bin directory on PATH so the
# driver can find cc1 / collect2.
$env:CGO_ENABLED = "1"
$env:CC = $gcc
$env:PATH = (Split-Path -Parent $gcc) + ";" + $env:PATH

Write-Host "go  : $go"
Write-Host "gcc : $gcc"

& $gcc --version | Select-Object -First 1 | ForEach-Object { Write-Host "      $_" }

function Invoke-Build {
    param([string]$Label, [string[]]$BuildArgs)

    Write-Host "-> $Label"
    & $go @BuildArgs
    if ($LASTEXITCODE -ne 0) {
        throw "go build failed for $Label (exit $LASTEXITCODE)"
    }
}

# 1. The renderer. Pure Go, no GraalVM dependency: any language can load it,
#    hand it a key=value spec and get a PNG back.
Invoke-Build "qrcore.dll" @(
    "build", "-buildmode=c-shared", "-o", "qrcore.dll", "./cmd/qrcore"
)

# 2. The general layer: isolate handling, settings, language, log and cache.
Invoke-Build "qrnative.dll" @(
    "build", "-buildmode=c-shared", "-o", "qrnative.dll", "./cmd/qrnative"
)

# c-shared writes a C header next to the DLL; other languages need it.
foreach ($h in @("qrcore.h", "qrnative.h")) {
    if (-not (Test-Path -LiteralPath (Join-Path $root $h))) {
        throw "$h was not generated next to the DLLs"
    }
}

# 3. The GUI. -H windowsgui hides the console window; the manifest in rsrc.syso
#    (Common Controls 6.0) is required or Walk cannot create its widgets.
if (-not $SkipExe) {
    $gopath = (& $go env GOPATH).Trim()
    $rsrc = Join-Path $gopath "bin\rsrc.exe"
    if (-not (Test-Path -LiteralPath $rsrc)) {
        Write-Host "-> installing rsrc (manifest generator)"
        & $go install github.com/akavel/rsrc@latest
        if ($LASTEXITCODE -ne 0) {
            throw "go install rsrc failed (exit $LASTEXITCODE)"
        }
    }
    & $rsrc -manifest qrscanner.manifest -o rsrc.syso
    if ($LASTEXITCODE -ne 0) {
        throw "rsrc failed (exit $LASTEXITCODE)"
    }

    Invoke-Build "qrscanner.exe" @(
        "build", "-ldflags", "-H windowsgui", "-o", "qrscanner.exe", "."
    )
}

Write-Host ""
Write-Host "Built:"
foreach ($f in @("qrcore.dll", "qrnative.dll", "qrscanner.exe")) {
    $p = Join-Path $root $f
    if (Test-Path -LiteralPath $p) {
        $size = (Get-Item -LiteralPath $p).Length
        Write-Host ("  {0,-14} {1,12:N0} bytes" -f $f, $size)
    }
}
