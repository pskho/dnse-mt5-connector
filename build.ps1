param(
    [switch]$Deploy
)

$cppDir = Join-Path $PSScriptRoot "cpp"
Set-Location $cppDir

$cmakeCmd = "cmake"
if (Test-Path "C:\Program Files\CMake\bin\cmake.exe") {
    $cmakeCmd = "& `"C:\Program Files\CMake\bin\cmake.exe`""
}

Write-Host "1. Cleaning old build cache..." -ForegroundColor Cyan
Remove-Item -Recurse -Force build -ErrorAction SilentlyContinue

Write-Host "2. Configuring CMake (Visual Studio 2022 x64)..." -ForegroundColor Cyan
Invoke-Expression "$cmakeCmd -S . -B build -G `"Visual Studio 17 2022`" -A x64"

if ($LASTEXITCODE -ne 0) {
    Write-Host "ERROR: CMake configuration failed!" -ForegroundColor Red
    exit $LASTEXITCODE
}

Write-Host "3. Building DLL (Release mode)..." -ForegroundColor Cyan
Invoke-Expression "$cmakeCmd --build build --config Release"

if ($LASTEXITCODE -ne 0) {
    Write-Host "ERROR: Build failed!" -ForegroundColor Red
    exit $LASTEXITCODE
}

$dllPath = Join-Path $cppDir "build\Release\DNSEBridge.dll"
Write-Host "SUCCESS: Build complete! DLL is ready at:" -ForegroundColor Green
Write-Host $dllPath -ForegroundColor Yellow

# Optional deploy step if -Deploy flag is passed
if ($Deploy) {
    $mt5LibPath = Read-Host "Enter the path to your MQL5\Libraries folder (e.g., C:\Users\Admin\AppData\Roaming\MetaQuotes\Terminal\...\MQL5\Libraries)"
    if (Test-Path $mt5LibPath) {
        Copy-Item $dllPath -Destination $mt5LibPath -Force
        Write-Host "Copied to $mt5LibPath successfully!" -ForegroundColor Green
    } else {
        Write-Host "Path does not exist, skipping copy step." -ForegroundColor Yellow
    }
}

Set-Location $PSScriptRoot
