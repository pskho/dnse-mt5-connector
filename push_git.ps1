param(
    [string]$RemoteUrl = "",
    [string]$Branch = "main",
    [string]$CommitMessage = "feat: package VN30F1M trial and start Phase A multi-symbol backend"
)

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

function Require-Git {
    $cmd = Get-Command git -ErrorAction SilentlyContinue
    if (-not $cmd) {
        throw "Git is not installed or not available in PATH."
    }
}

Require-Git

if (-not (Test-Path ".git")) {
    Write-Host "Initializing git repository..."
    git init
}

if (-not $RemoteUrl) {
    $existingRemote = git remote get-url origin 2>$null
    if (-not $existingRemote) {
        $RemoteUrl = Read-Host "Nhap git remote URL (vd: https://github.com/you/repo.git)"
    }
}

if ($RemoteUrl) {
    $hasOrigin = git remote 2>$null | Select-String -SimpleMatch "origin"
    if ($hasOrigin) {
        git remote set-url origin $RemoteUrl
    } else {
        git remote add origin $RemoteUrl
    }
}

$name = git config user.name 2>$null
if (-not $name) {
    Write-Host "Git user.name chua duoc dat."
}
$email = git config user.email 2>$null
if (-not $email) {
    Write-Host "Git user.email chua duoc dat."
}

git add .

$status = git status --short
if (-not $status) {
    Write-Host "Khong co thay doi moi de commit."
} else {
    git commit -m $CommitMessage
}

if ($RemoteUrl -or (git remote get-url origin 2>$null)) {
    git branch -M $Branch
    git push -u origin $Branch
    Write-Host "Push xong len origin/$Branch" -ForegroundColor Green
} else {
    Write-Host "Da commit local, nhung chua co remote origin de push." -ForegroundColor Yellow
}
