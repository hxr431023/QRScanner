# Build QRScanner
# Usage: .\build.ps1
$ErrorActionPreference = "Stop"

Set-Location -LiteralPath $PSScriptRoot
$env:GOPROXY = "https://goproxy.cn,direct"

# 1. Resource file: embeds the application manifest (Common Controls 6.0).
#    Without it Walk's tabs/tooltips fail to initialize at runtime.
$rsrc = Join-Path (go env GOPATH) "bin\rsrc.exe"
if (-not (Test-Path -LiteralPath $rsrc)) {
    Write-Host "Installing rsrc..."
    go install github.com/akavel/rsrc@latest
    $rsrc = Join-Path (go env GOPATH) "bin\rsrc.exe"
}
& $rsrc -manifest qrscanner.manifest -o rsrc.syso

# 2. Build as a GUI app: -H windowsgui suppresses the console window.
go build -ldflags "-H windowsgui" -o qrscanner.exe .

Write-Host "Built qrscanner.exe"
