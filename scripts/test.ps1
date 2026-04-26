$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$goExe = "C:\Program Files\Go\bin\go.exe"

$env:GOCACHE = Join-Path $root ".gocache"
$env:GOMODCACHE = Join-Path $root ".gomodcache"
$env:GOPATH = Join-Path $root ".gopath"

& $goExe test ./...
if ($LASTEXITCODE -ne 0) {
    throw "go test failed"
}
