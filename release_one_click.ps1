param(
    [string]$Version = "v0.1.0-trial",
    [string]$Branch = "main",
    [string]$Repo = "https://github.com/pskho/dnse-mt5-connector.git",
    [string]$GhRepo = "pskho/dnse-mt5-connector",
    [string]$InstallerName = "DNSE-MT5-Connector-VN30F1M-Setup.exe",
    [string]$PackageName = "DNSE-MT5-Connector-VN30F1M-Trial",
    [switch]$SkipDeployMt5,
    [switch]$SkipGit,
    [switch]$SkipRelease
)

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot
$env:GOCACHE = Join-Path $PSScriptRoot ".gocache"

function Require-Command {
    param(
        [string]$Name,
        [string]$Hint
    )
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "$Name khong co trong PATH. $Hint"
    }
}

function Ensure-Remote {
    param([string]$RemoteUrl)
    if (-not $RemoteUrl) {
        return $false
    }
    $hasOrigin = git remote 2>$null | Select-String -SimpleMatch "origin"
    if ($hasOrigin) {
        git remote set-url origin $RemoteUrl | Out-Null
    } else {
        git remote add origin $RemoteUrl | Out-Null
    }
    return $true
}

function Get-TrackedSourcePaths {
    return @(
        ".gitignore",
        "README.md",
        "README_TRIAL_VN30F1M.md",
        "RELEASE_NOTES_v0.1.0-trial.md",
        "build.ps1",
        "build_bridge.ps1",
        "build_one_click.bat",
        "build_installer.ps1",
        "cmd",
        "config\config.yaml.example",
        "cpp\CMakeLists.txt",
        "cpp\DNSEBridge.cpp",
        "deploy_mt5.bat",
        "deploy_mt5.ps1",
        "docs",
        "go.mod",
        "go.sum",
        "install_trial.ps1",
        "installer_bootstrap.cs",
        "internal",
        "mql5\DNSE_MarketData_Bridge.mq5",
        "mql5\DNSE_SuperTrend_Bot.mq5",
        "open_setup.bat",
        "package_trial.ps1",
        "publish_release.ps1",
        "push_git.ps1",
        "release_one_click.ps1",
        "release_one_click.bat",
        "run_bridge_console.bat",
        "start_trial.bat",
        "stop_trial.bat",
        "test_signals.bat"
    )
}

function Stage-ReleaseSources {
    $paths = Get-TrackedSourcePaths
    foreach ($path in $paths) {
        if (Test-Path $path) {
            git add -- $path
        }
    }
}

function Ensure-TagPushed {
    param(
        [string]$TagName,
        [bool]$HasOrigin
    )
    $existingTag = git tag --list $TagName
    if (-not $existingTag) {
        git tag $TagName
    }
    if ($HasOrigin) {
        git push origin $TagName
    }
}

function Ensure-ReleaseAssets {
    param(
        [string]$TagName,
        [string]$InstallerPath,
        [string]$ZipPath,
        [string]$NotesPath,
        [string]$ReleaseRepo
    )

    $viewArgs = @("release", "view", $TagName, "--repo", $ReleaseRepo)
    & gh @viewArgs *> $null
    $exists = ($LASTEXITCODE -eq 0)

    if (-not $exists) {
        $createArgs = @(
            "release", "create", $TagName,
            $InstallerPath,
            $ZipPath,
            "--title", "DNSE MT5 Connector $TagName",
            "--notes-file", $NotesPath,
            "--repo", $ReleaseRepo
        )
        & gh @createArgs
        if ($LASTEXITCODE -ne 0) {
            throw "Khong tao duoc GitHub Release."
        }
        return
    }

    $uploadArgs = @(
        "release", "upload", $TagName,
        $InstallerPath,
        $ZipPath,
        "--clobber",
        "--repo", $ReleaseRepo
    )
    & gh @uploadArgs
    if ($LASTEXITCODE -ne 0) {
        throw "Khong upload duoc release assets."
    }
}

Write-Host "1. Don dep build cu..." -ForegroundColor Cyan
if (Test-Path ".\bridge.exe") { Remove-Item ".\bridge.exe" -Force }
if (Test-Path ".\cpp\build") { Remove-Item ".\cpp\build" -Recurse -Force }
if (Test-Path ".\dist") { Remove-Item ".\dist" -Recurse -Force }

Write-Host "2. Build bridge.exe..." -ForegroundColor Cyan
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\build_bridge.ps1 -Output .\bridge.exe
if ($LASTEXITCODE -ne 0) {
    throw "Build bridge.exe that bai."
}

Write-Host "3. Build DNSEBridge.dll..." -ForegroundColor Cyan
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\build.ps1
if ($LASTEXITCODE -ne 0) {
    throw "Build DNSEBridge.dll that bai."
}

if (-not $SkipDeployMt5) {
    Write-Host "4. Copy DLL/EA vao MT5 va auto-compile EA neu co MetaEditor..." -ForegroundColor Cyan
    powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\deploy_mt5.ps1
    if ($LASTEXITCODE -ne 0) {
        throw "Deploy/compile EA that bai."
    }
}

Write-Host "5. Dong goi zip trial..." -ForegroundColor Cyan
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\package_trial.ps1 -PackageName $PackageName
if ($LASTEXITCODE -ne 0) {
    throw "Dong goi zip that bai."
}

Write-Host "6. Tao installer .exe..." -ForegroundColor Cyan
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\build_installer.ps1 -InstallerName $InstallerName -PackageName $PackageName
if ($LASTEXITCODE -ne 0) {
    throw "Build installer that bai."
}

$installerPath = Join-Path $PSScriptRoot ("dist\" + $InstallerName)
$zipPath = Join-Path $PSScriptRoot ("dist\" + $PackageName + ".zip")
$notesPath = Join-Path $PSScriptRoot "RELEASE_NOTES_v0.1.0-trial.md"

if (-not (Test-Path $installerPath)) { throw "Khong tim thay installer: $installerPath" }
if (-not (Test-Path $zipPath)) { throw "Khong tim thay zip: $zipPath" }
if (-not (Test-Path $notesPath)) { throw "Khong tim thay release notes: $notesPath" }

if (-not $SkipGit) {
    Write-Host "7. Commit va push source code..." -ForegroundColor Cyan
    Require-Command "git" "Hay cai Git truoc khi publish."

    if (-not (Test-Path ".git")) {
        git init | Out-Null
    }

    $hasOrigin = Ensure-Remote $Repo

    Stage-ReleaseSources
    $staged = git diff --cached --name-only
    if ($staged) {
        git commit -m "release: $Version"
    } else {
        Write-Host "Khong co thay doi source moi de commit." -ForegroundColor Yellow
    }

    git branch -M $Branch
    if ($hasOrigin) {
        git push -u origin $Branch
    } else {
        Write-Host "Chua co remote origin, bo qua push source." -ForegroundColor Yellow
    }

    Ensure-TagPushed -TagName $Version -HasOrigin:$hasOrigin
}

if (-not $SkipRelease) {
    Write-Host "8. Cap nhat GitHub Release..." -ForegroundColor Cyan
    Require-Command "gh" "Hay cai GitHub CLI 'gh' va chay 'gh auth login'."
    Ensure-ReleaseAssets -TagName $Version -InstallerPath $installerPath -ZipPath $zipPath -NotesPath $notesPath -ReleaseRepo $GhRepo
}

Write-Host ""
Write-Host "Hoan tat." -ForegroundColor Green
Write-Host "Installer: $installerPath"
Write-Host "Zip      : $zipPath"
Write-Host "Notes    : $notesPath"
