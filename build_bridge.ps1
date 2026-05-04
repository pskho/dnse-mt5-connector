param(
    [string]$Output = "bridge.exe"
)

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

$goCmd = Get-Command go -ErrorAction SilentlyContinue
if (-not $goCmd) {
    throw "Khong tim thay Go trong PATH. Hay cai Go hoac mo PowerShell moi sau khi cai."
}

$finalOutput = if ([System.IO.Path]::IsPathRooted($Output)) {
    $Output
} else {
    Join-Path $PSScriptRoot $Output
}

$tempOutput = Join-Path $env:TEMP ("dnse-bridge-" + [guid]::NewGuid().ToString("N") + ".exe")
$goCache = Join-Path $PSScriptRoot ".gocache"
$goModCache = Join-Path $PSScriptRoot ".gomodcache"

Write-Host "Dang build bridge tu source hien tai..." -ForegroundColor Cyan

try {
    New-Item -ItemType Directory -Path $goCache -Force | Out-Null
    New-Item -ItemType Directory -Path $goModCache -Force | Out-Null

    if (Test-Path $tempOutput) {
        Remove-Item $tempOutput -Force
    }

    $env:GOCACHE = $goCache
    $env:GOMODCACHE = $goModCache

    & $goCmd.Source build -o $tempOutput ./cmd
    $buildExitCode = $LASTEXITCODE

    if ($buildExitCode -ne 0 -and -not (Test-Path $tempOutput)) {
        throw "Build bridge that bai."
    }

    if (-not (Test-Path $tempOutput)) {
        throw "Khong tao duoc file tam $tempOutput."
    }

    if ($buildExitCode -ne 0) {
        Write-Host "Go build tra ve canh bao sau khi da tao file output. Tiep tuc dung ban build moi." -ForegroundColor Yellow
    }

    if (Test-Path $finalOutput) {
        Remove-Item $finalOutput -Force
    }

    Move-Item $tempOutput $finalOutput -Force
}
finally {
    if (Test-Path $tempOutput) {
        Remove-Item $tempOutput -Force -ErrorAction SilentlyContinue
    }
}

if (-not (Test-Path $finalOutput)) {
    throw "Khong tao duoc file $finalOutput."
}

Write-Host "Bridge san sang:" -ForegroundColor Green
Write-Host "  $finalOutput"
