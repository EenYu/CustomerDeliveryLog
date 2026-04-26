param(
    [Parameter(Mandatory = $true)]
    [ValidateSet("sync-scripts", "start", "stop", "restart", "status", "health", "logs", "nginx-test", "nginx-reload")]
    [string]$Action,

    [string]$RemoteHost,
    [int]$Port,
    [string]$User,
    [string]$Password = $env:REMOTE_PASSWORD,
    [string]$AppDir,
    [string]$RunAs = $env:REMOTE_RUN_AS,
    [string]$PythonExe = "python"
)

$ErrorActionPreference = "Stop"

$scriptPath = Join-Path $PSScriptRoot "remote_ops.py"
$rootDir = Split-Path -Parent $PSScriptRoot

if (-not $RemoteHost) {
    $RemoteHost = if ($env:REMOTE_HOST) { $env:REMOTE_HOST } else { "192.168.203.131" }
}

if (-not $PSBoundParameters.ContainsKey("Port")) {
    $Port = if ($env:REMOTE_PORT) { [int]$env:REMOTE_PORT } else { 22 }
}

if (-not $User) {
    $User = if ($env:REMOTE_USER) { $env:REMOTE_USER } else { "root" }
}

if (-not $AppDir) {
    $AppDir = if ($env:REMOTE_APP_DIR) { $env:REMOTE_APP_DIR } else { "/home/gxp/customer-delivery-log" }
}

$arguments = @(
    $scriptPath,
    $Action,
    "--host", $RemoteHost,
    "--port", $Port,
    "--user", $User,
    "--app-dir", $AppDir,
    "--root-dir", $rootDir
)

if ($Password) {
    $arguments += @("--password", $Password)
}

if ($RunAs) {
    $arguments += @("--run-as", $RunAs)
}

& $PythonExe @arguments
exit $LASTEXITCODE
