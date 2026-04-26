$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$dist = Join-Path $root "dist"
$packageRoot = Join-Path $dist "customer-delivery-log"
$goExe = "C:\Program Files\Go\bin\go.exe"

$env:GOCACHE = Join-Path $root ".gocache"
$env:GOMODCACHE = Join-Path $root ".gomodcache"
$env:GOPATH = Join-Path $root ".gopath"

if (Test-Path $packageRoot) {
    Remove-Item -Recurse -Force $packageRoot
}

New-Item -ItemType Directory -Force -Path $packageRoot | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $packageRoot "web") | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $packageRoot "scripts") | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $packageRoot "customer-delivery-record-system-docs") | Out-Null

& $goExe build -o (Join-Path $packageRoot "customer-delivery-log.exe") .\cmd\server
if ($LASTEXITCODE -ne 0) {
    throw "go build failed"
}

Copy-Item -Recurse -Force (Join-Path $root "web\*") (Join-Path $packageRoot "web")
Copy-Item -Force (Join-Path $root ".env.example") (Join-Path $packageRoot ".env.example")
Copy-Item -Force (Join-Path $root "deploy\nginx.conf") (Join-Path $packageRoot "nginx.conf")
Copy-Item -Force (Join-Path $root "scripts\test.ps1") (Join-Path $packageRoot "scripts\test.ps1")
Copy-Item -Force (Join-Path $root "customer-delivery-record-system-docs\03-database-schema.sql") (Join-Path $packageRoot "database-schema.sql")
Copy-Item -Recurse -Force (Join-Path $root "customer-delivery-record-system-docs\*") (Join-Path $packageRoot "customer-delivery-record-system-docs")
Copy-Item -Force (Join-Path $root "README.md") (Join-Path $packageRoot "README.md")
Copy-Item -Force (Join-Path $root "DEPLOY_CENTOS7.md") (Join-Path $packageRoot "DEPLOY_CENTOS7.md")

$zipPath = Join-Path $dist "customer-delivery-log-package.zip"
if (Test-Path $zipPath) {
    Remove-Item -Force $zipPath
}

Compress-Archive -Path (Join-Path $packageRoot "*") -DestinationPath $zipPath
Write-Host "Package created:" $zipPath
