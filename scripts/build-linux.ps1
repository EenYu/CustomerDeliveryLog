$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$dist = Join-Path $root "dist"
$stageRoot = Join-Path $dist "customer-delivery-log-linux-amd64"
$packageName = "customer-delivery-log"
$packageRoot = Join-Path $stageRoot $packageName
$goExe = "C:\Program Files\Go\bin\go.exe"

$env:GOCACHE = Join-Path $root ".gocache"
$env:GOMODCACHE = Join-Path $root ".gomodcache"
$env:GOPATH = Join-Path $root ".gopath"
$env:GOPROXY = "https://goproxy.cn,direct"
$env:CGO_ENABLED = "0"
$env:GOOS = "linux"
$env:GOARCH = "amd64"

if (Test-Path $stageRoot) {
    Remove-Item -Recurse -Force $stageRoot
}

$dirs = @(
    $packageRoot,
    (Join-Path $packageRoot "bin"),
    (Join-Path $packageRoot "config"),
    (Join-Path $packageRoot "config\nginx"),
    (Join-Path $packageRoot "config\sql"),
    (Join-Path $packageRoot "docs"),
    (Join-Path $packageRoot "docs\product-specs"),
    (Join-Path $packageRoot "logs"),
    (Join-Path $packageRoot "run"),
    (Join-Path $packageRoot "uploads"),
    (Join-Path $packageRoot "web")
)

foreach ($dir in $dirs) {
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
}

& $goExe build -trimpath -ldflags "-s -w" -o (Join-Path $packageRoot "bin\customer-delivery-log") .\cmd\server
if ($LASTEXITCODE -ne 0) {
    throw "go build failed"
}

Copy-Item -Recurse -Force (Join-Path $root "web\*") (Join-Path $packageRoot "web")
Copy-Item -Force (Join-Path $root "deploy\linux\start.sh") (Join-Path $packageRoot "start.sh")
Copy-Item -Force (Join-Path $root "deploy\linux\stop.sh") (Join-Path $packageRoot "stop.sh")
Copy-Item -Force (Join-Path $root "deploy\linux\restart.sh") (Join-Path $packageRoot "restart.sh")
Copy-Item -Force (Join-Path $root "deploy\linux\app.env.example") (Join-Path $packageRoot "config\app.env.example")
Copy-Item -Force (Join-Path $root "deploy\nginx.conf") (Join-Path $packageRoot "config\nginx\customer-delivery-log.conf")
Copy-Item -Force (Join-Path $root "customer-delivery-record-system-docs\03-database-schema.sql") (Join-Path $packageRoot "config\sql\database-schema.sql")
Copy-Item -Recurse -Force (Join-Path $root "customer-delivery-record-system-docs\*") (Join-Path $packageRoot "docs\product-specs")
Copy-Item -Force (Join-Path $root "README.md") (Join-Path $packageRoot "docs\README.md")
Copy-Item -Force (Join-Path $root "DEPLOY_CENTOS7.md") (Join-Path $packageRoot "docs\DEPLOY_CENTOS7.md")

$tarPath = Join-Path $dist "customer-delivery-log-linux-amd64.tar.gz"
if (Test-Path $tarPath) {
    Remove-Item -Force $tarPath
}

tar -czf $tarPath -C $stageRoot $packageName
if ($LASTEXITCODE -ne 0) {
    throw "tar packaging failed"
}

Write-Host "Linux package created:" $tarPath
