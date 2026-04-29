param(
    [string]$OutputRoot = "dist",
    [string]$PackageName = "DNSE-MT5-Connector-VN30F1M-Trial",
    [switch]$SkipBridgeBuild
)

$ErrorActionPreference = "Stop"

$root = $PSScriptRoot
$bundleRoot = Join-Path $root $OutputRoot
$packageRoot = Join-Path $bundleRoot $PackageName
$zipPath = Join-Path $bundleRoot ($PackageName + ".zip")
$bridgePath = Join-Path $root "bridge.exe"

if (-not $SkipBridgeBuild) {
    powershell.exe -NoProfile -ExecutionPolicy Bypass -File (Join-Path $root "build_bridge.ps1") -Output $bridgePath
}

$requiredFiles = @(
    $bridgePath,
    (Join-Path $root "deploy_mt5.ps1"),
    (Join-Path $root "deploy_mt5.bat"),
    (Join-Path $root "mql5\DNSE_MarketData_Bridge.mq5"),
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

New-Item -ItemType Directory -Path $packageRoot | Out-Null
New-Item -ItemType Directory -Path (Join-Path $packageRoot "config") | Out-Null
New-Item -ItemType Directory -Path (Join-Path $packageRoot "logs") | Out-Null
New-Item -ItemType Directory -Path (Join-Path $packageRoot "data") | Out-Null
New-Item -ItemType Directory -Path (Join-Path $packageRoot "mql5") | Out-Null
New-Item -ItemType Directory -Path (Join-Path $packageRoot "cpp\build\Release") -Force | Out-Null

Copy-Item $bridgePath (Join-Path $packageRoot "bridge.exe") -Force
Copy-Item (Join-Path $root "deploy_mt5.ps1") (Join-Path $packageRoot "deploy_mt5.ps1") -Force
Copy-Item (Join-Path $root "deploy_mt5.bat") (Join-Path $packageRoot "deploy_mt5.bat") -Force
Copy-Item (Join-Path $root "mql5\DNSE_MarketData_Bridge.mq5") (Join-Path $packageRoot "mql5\DNSE_MarketData_Bridge.mq5") -Force
Copy-Item (Join-Path $root "cpp\build\Release\DNSEBridge.dll") (Join-Path $packageRoot "cpp\build\Release\DNSEBridge.dll") -Force
Copy-Item (Join-Path $root "README_TRIAL_VN30F1M.md") (Join-Path $packageRoot "README_TRIAL_VN30F1M.md") -Force
Copy-Item (Join-Path $root "start_trial.bat") (Join-Path $packageRoot "start_trial.bat") -Force
Copy-Item (Join-Path $root "stop_trial.bat") (Join-Path $packageRoot "stop_trial.bat") -Force
Copy-Item (Join-Path $root "open_setup.bat") (Join-Path $packageRoot "open_setup.bat") -Force
if (Test-Path (Join-Path $root "config\config.yaml.example")) {
    Copy-Item (Join-Path $root "config\config.yaml.example") (Join-Path $packageRoot "config\config.yaml.example") -Force
}

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
  account_no: ""
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
'@

Set-Content -Path (Join-Path $packageRoot "config\config.yaml") -Value $configContent -Encoding UTF8

if (Test-Path $zipPath) {
    Remove-Item -Path $zipPath -Force
}
Compress-Archive -Path (Join-Path $packageRoot "*") -DestinationPath $zipPath -CompressionLevel Optimal

Write-Host "Package ready:" -ForegroundColor Green
Write-Host "  Folder: $packageRoot"
Write-Host "  Zip   : $zipPath"
