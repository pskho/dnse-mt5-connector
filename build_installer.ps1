param(
    [string]$OutputRoot = "dist",
    [string]$PackageName = "DNSE-MT5-Connector-VN30F1M-Trial",
    [string]$InstallerName = "DNSE-MT5-Connector-VN30F1M-Setup.exe",
    [string]$TelemetryMeasurementID = $(if ($env:DNSE_GA4_MEASUREMENT_ID) { $env:DNSE_GA4_MEASUREMENT_ID } else { "G-C0J6H1FF81" }),
    [string]$TelemetryAPISecret = $(if ($env:DNSE_GA4_API_SECRET) { $env:DNSE_GA4_API_SECRET } else { "j7WPYh2gRGyOFZu-kWs2xg" }),
    [switch]$AllowMissingTelemetry
)

$ErrorActionPreference = "Stop"

function Find-CSharpCompiler {
    $candidates = @(
        "C:\Windows\Microsoft.NET\Framework64\v4.0.30319\csc.exe",
        "C:\Windows\Microsoft.NET\Framework\v4.0.30319\csc.exe",
        "C:\Windows\Microsoft.NET\Framework64\v3.5\csc.exe",
        "C:\Windows\Microsoft.NET\Framework\v3.5\csc.exe"
    )

    foreach ($candidate in $candidates) {
        if (Test-Path $candidate) {
            return $candidate
        }
    }

    throw "Khong tim thay csc.exe de build installer."
}

$root = $PSScriptRoot
$bundleRoot = Join-Path $root $OutputRoot
$stageRoot = Join-Path $bundleRoot "installer-src"
$packageZip = Join-Path $bundleRoot ($PackageName + ".zip")
$installerPath = Join-Path $bundleRoot $InstallerName
$bootstrapSource = Join-Path $root "installer_bootstrap.cs"

$packageArgs = @(
    "-NoProfile",
    "-ExecutionPolicy", "Bypass",
    "-File", (Join-Path $root "package_trial.ps1"),
    "-OutputRoot", $OutputRoot,
    "-PackageName", $PackageName
)
if (-not [string]::IsNullOrWhiteSpace($TelemetryMeasurementID)) {
    $packageArgs += @("-TelemetryMeasurementID", $TelemetryMeasurementID)
}
if (-not [string]::IsNullOrWhiteSpace($TelemetryAPISecret)) {
    $packageArgs += @("-TelemetryAPISecret", $TelemetryAPISecret)
}
if ($AllowMissingTelemetry) {
    $packageArgs += "-AllowMissingTelemetry"
}

powershell.exe @packageArgs
if ($LASTEXITCODE -ne 0) {
    throw "Khong the tao goi trial moi. Package build that bai."
}

if (-not (Test-Path $packageZip)) {
    throw "Package zip not found: $packageZip"
}
if (-not (Test-Path $bootstrapSource)) {
    throw "installer_bootstrap.cs not found."
}

if (Test-Path $stageRoot) {
    Remove-Item -Path $stageRoot -Recurse -Force
}
New-Item -ItemType Directory -Path $stageRoot | Out-Null

$resourceZip = Join-Path $stageRoot "payload.zip"
Copy-Item $packageZip $resourceZip -Force

if (Test-Path $installerPath) {
    try {
        Remove-Item $installerPath -Force
    } catch {
        throw "Khong the ghi de $installerPath. Hay dong file installer dang mo, hoac build voi -InstallerName ten-khac.exe"
    }
}

$csc = Find-CSharpCompiler

$refs = @(
    "/r:System.dll",
    "/r:System.Core.dll",
    "/r:System.Windows.Forms.dll",
    "/r:System.IO.Compression.dll",
    "/r:System.IO.Compression.FileSystem.dll"
)

$args = @(
    "/nologo",
    "/target:winexe",
    "/platform:anycpu",
    "/out:$installerPath",
    "/resource:$resourceZip,payload.zip"
) + $refs + @($bootstrapSource)

& $csc $args

if ($LASTEXITCODE -ne 0) {
    throw "Build installer that bai."
}

if (-not (Test-Path $installerPath)) {
    throw "Installer build failed: $installerPath was not created."
}

Write-Host "Installer ready:" -ForegroundColor Green
Write-Host "  $installerPath"
