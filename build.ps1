# Build QRScanner: the two reusable DLLs plus the GUI executable.
# Usage: .\build.ps1            (all three)
#        .\build.ps1 -SkipExe   (only qrcore.dll and qrnative.dll)
#
# The real work lives in build\build_dlls.ps1, which locates go and gcc, sets
# CGO_ENABLED / CC and builds in the right order. This wrapper exists so the
# command people already know keeps working.
param(
    [string]$GccPath = "",
    [string]$GoExe = "",
    [switch]$SkipExe
)

$ErrorActionPreference = "Stop"
& (Join-Path $PSScriptRoot "build\build_dlls.ps1") `
    -GccPath $GccPath -GoExe $GoExe -SkipExe:$SkipExe
