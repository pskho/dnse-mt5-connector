param(
    [string]$Output = "bridge.exe"
)

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

$goCmd = Get-Command go -ErrorAction SilentlyContinue
if (-not $goCmd) {
    throw "Không tìm thấy Go trong PATH. Hãy cài Go hoặc mở PowerShell mới sau khi cài."
}

Write-Host "Đang build bridge từ source hiện tại..." -ForegroundColor Cyan
& $goCmd.Source build -o $Output ./cmd

if ($LASTEXITCODE -ne 0) {
    throw "Build bridge thất bại."
}

if (-not (Test-Path $Output)) {
    throw "Không tạo được file $Output."
}

Write-Host "Bridge sẵn sàng:" -ForegroundColor Green
Write-Host "  $(Join-Path $PSScriptRoot $Output)"
