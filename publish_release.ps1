param(
    [string]$Version = "v0.1.0-trial",
    [string]$Branch = "main",
    [string]$Repo = "",
    [string]$InstallerName = "DNSE-MT5-Connector-VN30F1M-Setup.exe",
    [string]$PackageName = "DNSE-MT5-Connector-VN30F1M-Trial",
    [switch]$SkipGit,
    [switch]$SkipRelease
)

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

function Test-CommandExists {
    param([string]$Name)
    return [bool](Get-Command $Name -ErrorAction SilentlyContinue)
}

function Require-Command {
    param([string]$Name, [string]$Hint)
    if (-not (Test-CommandExists $Name)) {
        throw "$Name khong co trong PATH. $Hint"
    }
}

function Ensure-Remote {
    param([string]$RemoteUrl)
    if (-not $RemoteUrl) {
        return
    }
    $hasOrigin = git remote 2>$null | Select-String -SimpleMatch "origin"
    if ($hasOrigin) {
        git remote set-url origin $RemoteUrl
    } else {
        git remote add origin $RemoteUrl
    }
}

Write-Host "1. Build lai file installer..." -ForegroundColor Cyan
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\build_installer.ps1 -InstallerName $InstallerName -PackageName $PackageName

$installerPath = Join-Path $PSScriptRoot ("dist\" + $InstallerName)
$zipPath = Join-Path $PSScriptRoot ("dist\" + $PackageName + ".zip")
$notesPath = Join-Path $PSScriptRoot "RELEASE_NOTES_v0.1.0-trial.md"

if (-not (Test-Path $installerPath)) {
    throw "Khong tim thay installer: $installerPath"
}
if (-not (Test-Path $zipPath)) {
    throw "Khong tim thay zip: $zipPath"
}
if (-not (Test-Path $notesPath)) {
    throw "Khong tim thay release notes: $notesPath"
}

if (-not $SkipGit) {
    Write-Host "2. Chuan bi git..." -ForegroundColor Cyan
    Require-Command "git" "Hay cai Git truoc khi publish."

    if (-not (Test-Path ".git")) {
        git init
    }

    if ($Repo) {
        Ensure-Remote $Repo
    }

    git add .

    $status = git status --short
    if ($status) {
        git commit -m "release: $Version"
    } else {
        Write-Host "Khong co thay doi moi de commit." -ForegroundColor Yellow
    }

    git branch -M $Branch
    $hasOrigin = git remote 2>$null | Select-String -SimpleMatch "origin"
    if ($hasOrigin) {
        git push -u origin $Branch
    } else {
        Write-Host "Chua co remote origin, bo qua buoc push source." -ForegroundColor Yellow
    }

    $existingTag = git tag --list $Version
    if (-not $existingTag) {
        git tag $Version
    }
    if ($hasOrigin) {
        git push origin $Version
    }
}

if (-not $SkipRelease) {
    Write-Host "3. Tao GitHub Release..." -ForegroundColor Cyan
    Require-Command "gh" "Hay cai GitHub CLI 'gh' va chay 'gh auth login'."

    $releaseArgs = @(
        "release", "create", $Version,
        $installerPath,
        $zipPath,
        "--title", "DNSE MT5 Connector $Version",
        "--notes-file", $notesPath
    )

    if ($Repo) {
        $releaseArgs += @("--repo", $Repo)
    }

    & gh @releaseArgs
}

Write-Host ""
Write-Host "Xong." -ForegroundColor Green
Write-Host "Installer: $installerPath"
Write-Host "Zip      : $zipPath"
Write-Host "Notes    : $notesPath"
