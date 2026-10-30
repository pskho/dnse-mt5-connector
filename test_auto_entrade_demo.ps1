param(
    [string]$BridgeUrl = "http://127.0.0.1:8080",
    [string]$Side = "BUY",
    [string]$Symbol = "VN30F1M",
    [int]$Quantity = 1,
    [string]$AccountNo = "",
    [string]$OrderType = "MTL",
    [double]$Price = 0
)

$ErrorActionPreference = "Stop"

$Side = $Side.ToUpperInvariant()
if ($Side -ne "BUY" -and $Side -ne "SELL") {
    throw "Side must be BUY or SELL."
}

Write-Host "Checking bridge at $BridgeUrl..."
Invoke-RestMethod -Method Get -Uri "$BridgeUrl/ping" | ConvertTo-Json -Depth 8

Write-Host "Enabling order flow and switching signal mode to auto..."
Invoke-RestMethod -Method Post -Uri "$BridgeUrl/kill-switch" -ContentType "application/json" -Body (@{
    enabled = $true
} | ConvertTo-Json -Compress) | ConvertTo-Json -Depth 8

Invoke-RestMethod -Method Post -Uri "$BridgeUrl/mode" -ContentType "application/json" -Body (@{
    mode = "auto"
} | ConvertTo-Json -Compress) | ConvertTo-Json -Depth 8

$body = @{
    accountNo = $AccountNo
    symbol = $Symbol.ToUpperInvariant()
    source = "signal_api"
    side = $Side
    quantity = $Quantity
    price = $Price
    orderType = $OrderType.ToUpperInvariant()
    marketType = "DERIVATIVE"
    orderCategory = "NORMAL"
}
if ([string]::IsNullOrWhiteSpace($AccountNo)) {
    $body.Remove("accountNo")
}

if ([string]::IsNullOrWhiteSpace($AccountNo)) {
    Write-Host "Sending $Side signal to the server default account/group..."
} else {
    Write-Host "Sending $Side signal to $AccountNo..."
}
$signal = Invoke-RestMethod -Method Post -Uri "$BridgeUrl/signal" -ContentType "application/json" -Body ($body | ConvertTo-Json -Compress)
$signal | ConvertTo-Json -Depth 8

if ($signal.clientOrderId) {
    Start-Sleep -Seconds 1
    Write-Host "Querying order by clientOrderId $($signal.clientOrderId)..."
    Invoke-RestMethod -Method Get -Uri "$BridgeUrl/order/client/$($signal.clientOrderId)" | ConvertTo-Json -Depth 8
} else {
    Write-Host "No clientOrderId returned. Signal mode may not be auto or order submission failed before acceptance."
}
