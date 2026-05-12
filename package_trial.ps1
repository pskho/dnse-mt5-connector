param(
    [string]$OutputRoot = "dist",
    [string]$PackageName = "DNSE-MT5-Connector-VN30F1M-Trial",
    [string]$TelemetryMeasurementID = $(if ($env:DNSE_GA4_MEASUREMENT_ID) { $env:DNSE_GA4_MEASUREMENT_ID } else { "G-C0J6H1FF81" }),
    [string]$TelemetryAPISecret = $(if ($env:DNSE_GA4_API_SECRET) { $env:DNSE_GA4_API_SECRET } else { "j7WPYh2gRGyOFZu-kWs2xg" }),
    [switch]$AllowMissingTelemetry,
    [switch]$SkipBridgeBuild
)

$ErrorActionPreference = "Stop"

$root = $PSScriptRoot
$bundleRoot = Join-Path $root $OutputRoot
$packageRoot = Join-Path $bundleRoot $PackageName
$zipPath = Join-Path $bundleRoot ($PackageName + ".zip")
$bridgePath = Join-Path $root "bridge.exe"
$configExamplePath = Join-Path $root "config\config.yaml.example"

function Escape-YamlValue([string]$Value) {
    return ($Value -replace '\\', '\\' -replace '"', '\"')
}

function Set-TelemetryValue([string]$Path, [string]$Key, [string]$Value) {
    $escaped = Escape-YamlValue $Value
    $lines = Get-Content -Path $Path
    $inTelemetry = $false
    for ($i = 0; $i -lt $lines.Count; $i++) {
        $trimmed = $lines[$i].Trim()
        if ($trimmed -eq "telemetry:") {
            $inTelemetry = $true
            continue
        }
        if ($inTelemetry -and $trimmed -ne "" -and -not $lines[$i].StartsWith("  ")) {
            $inTelemetry = $false
        }
        if ($inTelemetry -and $lines[$i] -match "^  $Key`:") {
            $lines[$i] = "  ${Key}: `"$escaped`""
        }
    }
    Set-Content -Path $Path -Value $lines -Encoding UTF8
}

if (([string]::IsNullOrWhiteSpace($TelemetryMeasurementID) -or [string]::IsNullOrWhiteSpace($TelemetryAPISecret)) -and -not $AllowMissingTelemetry) {
    throw "Telemetry bat buoc cho ban phat hanh. Hay truyen -TelemetryMeasurementID va -TelemetryAPISecret, hoac dat bien moi truong DNSE_GA4_MEASUREMENT_ID / DNSE_GA4_API_SECRET."
}

if (-not $SkipBridgeBuild) {
    powershell.exe -NoProfile -ExecutionPolicy Bypass -File (Join-Path $root "build_bridge.ps1") -Output $bridgePath
    if ($LASTEXITCODE -ne 0) {
        throw "Build bridge that bai. Dung dong goi de tranh su dung bridge.exe cu."
    }
}

$requiredFiles = @(
    $bridgePath,
    (Join-Path $root "deploy_mt5.ps1"),
    (Join-Path $root "deploy_mt5.bat"),
    (Join-Path $root "launch_dnse_mt5.bat"),
    (Join-Path $root "mql5\DNSE_MarketData_Bridge.mq5"),
    (Join-Path $root "mql5\DNSE_SuperTrend_Bot.mq5"),
    (Join-Path $root "cpp\build\Release\DNSEBridge.dll")
)

foreach ($file in $requiredFiles) {
    if (-not (Test-Path $file)) {
        throw "Missing required file: $file"
    }
}

if (Test-Path $packageRoot) {
    Remove-Item -Path $packageRoot -Recurse -Force
}
if (-not (Test-Path $bundleRoot)) {
    New-Item -ItemType Directory -Path $bundleRoot | Out-Null
}

Get-ChildItem -Path $bundleRoot -File -ErrorAction SilentlyContinue |
    Where-Object { $_.Name -like "*upload*" -or $_.Name -like "~DNSE-*" } |
    Remove-Item -Force -ErrorAction SilentlyContinue

New-Item -ItemType Directory -Path $packageRoot | Out-Null
New-Item -ItemType Directory -Path (Join-Path $packageRoot "config") | Out-Null
New-Item -ItemType Directory -Path (Join-Path $packageRoot "logs") | Out-Null
New-Item -ItemType Directory -Path (Join-Path $packageRoot "data") | Out-Null
New-Item -ItemType Directory -Path (Join-Path $packageRoot "mql5") | Out-Null
New-Item -ItemType Directory -Path (Join-Path $packageRoot "cpp\build\Release") -Force | Out-Null

Copy-Item $bridgePath (Join-Path $packageRoot "bridge.exe") -Force
Copy-Item (Join-Path $root "deploy_mt5.ps1") (Join-Path $packageRoot "deploy_mt5.ps1") -Force
Copy-Item (Join-Path $root "deploy_mt5.bat") (Join-Path $packageRoot "deploy_mt5.bat") -Force
Copy-Item (Join-Path $root "launch_dnse_mt5.bat") (Join-Path $packageRoot "launch_dnse_mt5.bat") -Force
Copy-Item (Join-Path $root "mql5\DNSE_MarketData_Bridge.mq5") (Join-Path $packageRoot "mql5\DNSE_MarketData_Bridge.mq5") -Force
Copy-Item (Join-Path $root "mql5\DNSE_SuperTrend_Bot.mq5") (Join-Path $packageRoot "mql5\DNSE_SuperTrend_Bot.mq5") -Force
Copy-Item (Join-Path $root "cpp\build\Release\DNSEBridge.dll") (Join-Path $packageRoot "cpp\build\Release\DNSEBridge.dll") -Force
Copy-Item (Join-Path $root "README_TRIAL_VN30F1M.md") (Join-Path $packageRoot "README_TRIAL_VN30F1M.md") -Force
Copy-Item (Join-Path $root "start_trial.bat") (Join-Path $packageRoot "start_trial.bat") -Force
Copy-Item (Join-Path $root "stop_trial.bat") (Join-Path $packageRoot "stop_trial.bat") -Force
Copy-Item (Join-Path $root "open_setup.bat") (Join-Path $packageRoot "open_setup.bat") -Force
Copy-Item (Join-Path $root "run_bridge_console.bat") (Join-Path $packageRoot "run_bridge_console.bat") -Force
if (Test-Path $configExamplePath) {
    Copy-Item $configExamplePath (Join-Path $packageRoot "config\config.yaml.example") -Force
    Copy-Item $configExamplePath (Join-Path $packageRoot "config\config.yaml") -Force
} else {
    $configContent = @'
host: "127.0.0.1"
port: 8080
database_path: "data/connector.db"
log_file: "logs/app.jsonl"

risk:
  max_quantity: 10
  max_open_position: 10
  duplicate_window_seconds: 3

dnse:
  base_url: "https://openapi.dnse.com.vn"
  api_key: "PASTE_DNSE_API_KEY_HERE"
  api_secret: "PASTE_DNSE_API_SECRET_HERE"
  account_no: "PASTE_ACCOUNT_NO_HERE"
  mock: false

market_data:
  enabled: true
  symbol: "VN30F1M"
  symbols: ["VN30F1M"]
  bridge_address: "127.0.0.1:9090"
  websocket_url: "wss://ws-openapi.dnse.com.vn/v1/stream?encoding=json"
  channels: ["trades.json", "quotes.json"]
  mock: false
  reconnect_seconds: 5

history:
  enabled: true
  symbol: "VN30F1M"
  market_type: "DERIVATIVE"
  resolution: 1
  initial_lookback_days: 365
  incremental_sync: true
  full_rebuild: false
  max_batch_days: 30

gmail_otp:
  enabled: false
  credentials_file: "config/credentials.json"
  token_file: "config/token.json"
  poll_interval_seconds: 3
  email_domain_filter: "dnse.com.vn"

telemetry:
  enabled: true
  provider: "ga4"
  measurement_id: ""
  api_secret: ""
  client_id_file: "data/client_id"
  app_version: "0.1.0-trial"
'@

    Set-Content -Path (Join-Path $packageRoot "config\config.yaml.example") -Value $configContent -Encoding UTF8
    Set-Content -Path (Join-Path $packageRoot "config\config.yaml") -Value $configContent -Encoding UTF8
}

if (-not [string]::IsNullOrWhiteSpace($TelemetryMeasurementID) -and -not [string]::IsNullOrWhiteSpace($TelemetryAPISecret)) {
    foreach ($targetConfig in @(
        (Join-Path $packageRoot "config\config.yaml"),
        (Join-Path $packageRoot "config\config.yaml.example")
    )) {
        Set-TelemetryValue -Path $targetConfig -Key "measurement_id" -Value $TelemetryMeasurementID
        Set-TelemetryValue -Path $targetConfig -Key "api_secret" -Value $TelemetryAPISecret
    }
}

if (Test-Path $zipPath) {
    Remove-Item -Path $zipPath -Force
}

$tarCmd = Get-Command tar.exe -ErrorAction SilentlyContinue
if (-not $tarCmd) {
    throw "Khong tim thay tar.exe de dong goi file zip."
}

& $tarCmd.Source -a -c -f $zipPath -C $packageRoot .
if ($LASTEXITCODE -ne 0 -or -not (Test-Path $zipPath)) {
    throw "Khong tao duoc file zip: $zipPath"
}

Write-Host "Package ready:" -ForegroundColor Green
Write-Host "  Folder: $packageRoot"
Write-Host "  Zip   : $zipPath"
