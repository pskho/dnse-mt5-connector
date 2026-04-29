$mt5Path = Join-Path $env:APPDATA "MetaQuotes\Terminal"
$sourceDll = Join-Path $PSScriptRoot "cpp\build\Release\DNSEBridge.dll"
$sourceEa = Join-Path $PSScriptRoot "mql5\DNSE_MarketData_Bridge.mq5"

function Find-MetaEditor {
    $candidates = @()
    $roots = @($env:ProgramFiles, ${env:ProgramFiles(x86)}) | Where-Object { $_ -and (Test-Path $_) }
    foreach ($root in $roots) {
        $candidates += Get-ChildItem -Path $root -Directory -ErrorAction SilentlyContinue |
            Where-Object { $_.Name -like "MetaTrader*" -or $_.Name -like "*MetaEditor*" -or $_.Name -like "*DNSE*" } |
            ForEach-Object {
                @(
                    (Join-Path $_.FullName "metaeditor64.exe"),
                    (Join-Path $_.FullName "metaeditor.exe")
                )
            }
    }

    foreach ($candidate in $candidates | Select-Object -Unique) {
        if (Test-Path $candidate) {
            return $candidate
        }
    }

    foreach ($cmdName in @("metaeditor64.exe", "metaeditor.exe")) {
        $cmd = Get-Command $cmdName -ErrorAction SilentlyContinue
        if ($cmd) {
            return $cmd.Source
        }
    }
    return $null
}

function Compile-EA($metaEditorPath, $eaPath) {
    if (-not $metaEditorPath -or -not (Test-Path $metaEditorPath)) {
        Write-Host "  - MetaEditor not found. Skipping auto-compile." -ForegroundColor Yellow
        return
    }

    $logPath = [System.IO.Path]::ChangeExtension($eaPath, ".compile.log")
    Write-Host "  - Auto-compiling EA with MetaEditor..."
    $args = "/compile:`"$eaPath`" /log:`"$logPath`""
    $proc = Start-Process -FilePath $metaEditorPath -ArgumentList $args -Wait -PassThru -WindowStyle Hidden
    if (Test-Path $logPath) {
        Write-Host "    Compile log: $logPath"
        Get-Content $logPath | Select-Object -Last 5 | ForEach-Object { Write-Host "    $_" }
    } else {
        Write-Host "    Compile finished with exit code $($proc.ExitCode)"
    }
}

Write-Host "==================================================="
Write-Host "  Deploying DNSE Bridge to MT5 Data Folders..."
Write-Host "==================================================="
Write-Host ""

if (-Not (Test-Path $sourceDll)) { Write-Host "[ERROR] DLL not found: $sourceDll. Please build first." -ForegroundColor Red; exit 1 }
if (-Not (Test-Path $sourceEa)) { Write-Host "[ERROR] EA not found: $sourceEa." -ForegroundColor Red; exit 1 }
if (-Not (Test-Path $mt5Path)) { Write-Host "[ERROR] MT5 path not found: $mt5Path" -ForegroundColor Red; exit 1 }

$metaEditor = Find-MetaEditor
if ($metaEditor) {
    Write-Host "Found MetaEditor: $metaEditor"
} else {
    Write-Host "[WARNING] MetaEditor not found. EA files will be copied but not auto-compiled." -ForegroundColor Yellow
}

$deployCount = 0
$folders = Get-ChildItem -Path $mt5Path -Directory
foreach ($folder in $folders) {
    $mql5Path = Join-Path $folder.FullName "MQL5"
    if (Test-Path $mql5Path) {
        Write-Host "Found MT5 Data Folder: $($folder.Name)"
        
        $destLib = Join-Path $mql5Path "Libraries"
        $destEaFolder = Join-Path $mql5Path "Experts\DNSE"
        $legacyEaFiles = @(
            (Join-Path $mql5Path "Experts\DNSE_MarketData_Bridge.mq5"),
            (Join-Path $mql5Path "Experts\DNSE_MarketData_Bridge.ex5")
        )
        
        if (-Not (Test-Path $destEaFolder)) { New-Item -ItemType Directory -Path $destEaFolder | Out-Null }
        
        Write-Host "  - Copying DLL to Libraries..."
        Copy-Item -Path $sourceDll -Destination $destLib -Force
        
        Write-Host "  - Copying EA to Experts\DNSE..."
        $destEa = Join-Path $destEaFolder "DNSE_MarketData_Bridge.mq5"
        Copy-Item -Path $sourceEa -Destination $destEaFolder -Force

        foreach ($legacyEa in $legacyEaFiles) {
            if (Test-Path $legacyEa) {
                Write-Host "  - Removing legacy EA from Experts root: $legacyEa"
                Remove-Item -Path $legacyEa -Force -ErrorAction SilentlyContinue
            }
        }

        Compile-EA $metaEditor $destEa
        
        $deployCount++
        Write-Host ""
    }
}

if ($deployCount -eq 0) {
    Write-Host "[WARNING] No valid MT5 MQL5 folders found in $mt5Path" -ForegroundColor Yellow
} else {
    Write-Host "[SUCCESS] Successfully deployed to $deployCount MT5 instance(s)." -ForegroundColor Green
}
