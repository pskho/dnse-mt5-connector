param()

$ErrorActionPreference = "Stop"

function New-Shortcut {
    param(
        [string]$Path,
        [string]$TargetPath,
        [string]$Arguments = "",
        [string]$WorkingDirectory = ""
    )

    $shell = New-Object -ComObject WScript.Shell
    $shortcut = $shell.CreateShortcut($Path)
    $shortcut.TargetPath = $TargetPath
    if ($Arguments) { $shortcut.Arguments = $Arguments }
    if ($WorkingDirectory) { $shortcut.WorkingDirectory = $WorkingDirectory }
    $shortcut.Save()
}

function Copy-DirectorySafe {
    param(
        [string]$Source,
        [string]$Destination
    )

    Get-ChildItem -Path $Source -Force | ForEach-Object {
        $target = Join-Path $Destination $_.Name
        if ($_.PSIsContainer) {
            if (-not (Test-Path $target)) {
                New-Item -ItemType Directory -Path $target | Out-Null
            }
            Copy-DirectorySafe -Source $_.FullName -Destination $target
        } else {
            if ($_.Name -ieq "config.yaml" -and (Split-Path $_.DirectoryName -Leaf) -ieq "config" -and (Test-Path $target)) {
                return
            }
            Copy-Item $_.FullName $target -Force
        }
    }
}

$sourceDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$payloadZip = Join-Path $sourceDir "payload.zip"
if (-not (Test-Path $payloadZip)) {
    throw "payload.zip not found next to installer script."
}

$installDir = Join-Path $env:LOCALAPPDATA "DNSE-MT5-Connector"
$tempDir = Join-Path $env:TEMP ("dnse-mt5-install-" + [guid]::NewGuid().ToString("N"))
$desktop = [Environment]::GetFolderPath("Desktop")

Write-Host "Dang cai DNSE MT5 Connector vao: $installDir" -ForegroundColor Cyan

if (-not (Test-Path $tempDir)) {
    New-Item -ItemType Directory -Path $tempDir | Out-Null
}
if (-not (Test-Path $installDir)) {
    New-Item -ItemType Directory -Path $installDir | Out-Null
}

Expand-Archive -Path $payloadZip -DestinationPath $tempDir -Force
Copy-DirectorySafe -Source $tempDir -Destination $installDir

$configExample = Join-Path $installDir "config\config.yaml.example"
$configPath = Join-Path $installDir "config\config.yaml"
if (-not (Test-Path $configPath) -and (Test-Path $configExample)) {
    Copy-Item $configExample $configPath -Force
}

$deployScript = Join-Path $installDir "deploy_mt5.ps1"
if (Test-Path $deployScript) {
    Write-Host "Dang cai file vao MT5..." -ForegroundColor Cyan
    powershell.exe -NoProfile -ExecutionPolicy Bypass -File $deployScript
}

$startBat = Join-Path $installDir "start_trial.bat"
$stopBat = Join-Path $installDir "stop_trial.bat"
$openBat = Join-Path $installDir "open_setup.bat"

New-Shortcut -Path (Join-Path $desktop "DNSE MT5 Connector.lnk") -TargetPath $startBat -WorkingDirectory $installDir
New-Shortcut -Path (Join-Path $desktop "DNSE MT5 Setup.lnk") -TargetPath $openBat -WorkingDirectory $installDir
New-Shortcut -Path (Join-Path $desktop "DNSE MT5 Stop.lnk") -TargetPath $stopBat -WorkingDirectory $installDir

Write-Host "Dang khoi dong bridge..." -ForegroundColor Cyan
Start-Process -FilePath $startBat -WorkingDirectory $installDir

Start-Sleep -Seconds 2
Start-Process "http://127.0.0.1:8080/setup"

Write-Host ""
Write-Host "Cai dat xong." -ForegroundColor Green
Write-Host "Buoc tiep theo: dien DNSE API key trong trang setup/settings neu chua co." -ForegroundColor Yellow

Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
