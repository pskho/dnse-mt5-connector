# DNSE MT5 Connector

A high-performance, open-source Windows bridge connecting MetaTrader 5 (MT5) with the DNSE OpenAPI. It provides a robust, low-latency pipeline for realtime market data streaming, historical data synchronization, and automated/semi-automated trading execution.

## 🚀 Features

### Automated MT5 Setup & UI
- **Zero-Touch Configuration**: A built-in web-based Setup Wizard automates MT5 data folder detection, backs up your existing files, and seamlessly installs the required DLL and EA into your MetaTrader 5 terminal.
- **Dependency Auto-Provisioning**: The Go Bridge instantly creates all required `data`, `logs`, and config dependencies if they are missing on startup.
- **Support Export**: In a single click, generate a ready-to-send ZIP file containing your masked configs, logs, and system status to send to support without leaking secrets.

### Market Data Streaming
- **Realtime Ticks**: Streams realtime `VN30F1M` ticks directly from DNSE WebSocket to MT5 via a local TCP socket, bypassing slow CSV import methods.
- **Smart History Synchronization**: Automatically syncs missing historical candles from the DNSE API into MT5 on startup. It uses incremental sync to fetch only missing gaps, avoiding redundant data downloads.
- **Mock Mode**: Fully supports offline development with generated sine-wave ticks and historical data.

### Trading Execution
- **Multi-Mode Operation**: Supports `Manual`, `Semi-Auto` (requires user confirmation), and `Auto` trading modes.
- **Semi-Auto Signal Pipeline**: MT5 generates signals, the Go Bridge caches them, and the operator confirms/rejects them via the Web UI before the order is actually placed on DNSE.
- **Full Order Lifecycle**: Place, track, and cancel `NORMAL`, `MTL`, `LO`, `MOK`, `MAK`, `ATO`, and `ATC` orders.
- **Loan Package Integration**: Automatically queries and selects the optimal derivative or stock loan package (PPSE) for the account prior to placing an order.

### Security & Authentication
- **Email OTP Auto-Fetch**: Securely connects to your Gmail via OAuth2 to auto-read DNSE OTP emails. The bridge intercepts trading token requests, waits for the OTP, and automatically registers the session without any manual intervention.
- **OAuth2 Token Storage**: Securely handles and caches Gmail and DNSE session tokens.
- **Trading Kill Switch**: Instantly block all new order submissions while keeping position and cancel endpoints active.

### Advanced Risk Controls
- Validates maximum open positions to prevent over-exposure.
- Prevents duplicate order submissions using an in-memory idempotency hash (Symbol + Side + Quantity) active for 3 seconds.
- Rejects orders outside of standard trading hours.
- Verifies PPSE limits before routing.

### Built-in Web UI & Dashboard
A premium, dark-mode glassmorphic Web Dashboard is hosted at `http://127.0.0.1:8080/`. The browser will **automatically open** to the setup page on launch. 
- **`/setup`**: An interactive wizard that checks your system, detects MT5 folders, installs DLLs, tests the Gmail OTP connection, and verifies DNSE API access.
- **`/status`**: Real-time operational status (Go Bridge, MT5 Connection, Tokens, Trading mode).
- **`/settings`**: A safe form to update your config settings without dealing with YAML files.
- **`/logs`**: A terminal-like live viewer of the system `app.jsonl` logs.

---

## 🏗️ Architecture

```text
DNSE OpenAPI (REST + WebSocket)
      ↕
[ Go Bridge ] ↔ Gmail API (OAuth2 Auto OTP)
      ↕ (HTTP 8080 - REST/UI)
      ↕ (TCP 9090 - Market Data)
[ C++ DLL (Windows x64) ]
      ↕
[ MQL5 Expert Advisor ]
      ↕
MetaTrader 5 Platform
```

## ⚙️ Configuration

Copy `config/config.yaml.example` to `config/config.yaml` and edit:

```yaml
database_path: "data/connector.db"
log_file: "logs/app.jsonl"

dnse:
  base_url: https://openapi.dnse.com.vn
  api_key: YOUR_API_KEY
  api_secret: YOUR_API_SECRET
  account_no: "0001007412"
  mock: false

risk:
  max_quantity: 10
  max_open_position: 10
  duplicate_window_seconds: 3

market_data:
  enabled: true
  symbol: VN30F1M
  bridge_address: 127.0.0.1:9090
  websocket_url: wss://ws-openapi.dnse.com.vn/v1/stream?encoding=json
  mock: false

history:
  enabled: true
  initial_lookback_days: 365
  incremental_sync: true
  full_rebuild: false

gmail_otp:
  enabled: true
  credentials_file: "config/credentials.json"
  token_file: "config/token.json"
```

## 🛠️ Quick Start

### 1. Start the Go Bridge
Install Go 1.25+ and run:
```powershell
go mod tidy
go run ./cmd
```
*Note: Once started, your default browser will automatically launch and take you to `http://127.0.0.1:8080/setup`.*

### 2. Follow the Setup Wizard
Use the intuitive web UI to automatically configure your environment:
- **System Check**: Automatically verifies ports and dependencies.
- **MT5 Installation**: Click the auto-detect button to automatically find your MT5 installation (e.g. `AppData/Roaming/MetaQuotes/...`) and copy the `DNSEBridge.dll` and `DNSE_MarketData_Bridge.mq5`.
- **API & OTP Testing**: Input your DNSE keys (masked) and click test.

### 3. Attach the EA in MT5
Open MT5, go to `Tools -> Options -> Expert Advisors` and check **Allow DLL imports**. Open a chart and attach the `DNSE_MarketData_Bridge` EA.

### 4. Monitor & Trade
Return to the main Dashboard `http://127.0.0.1:8080/` to test placing orders, managing positions, or confirming algorithmic signals.

---

## 📚 API Endpoints

The Go Bridge exposes a comprehensive REST API on port `8080`.

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/` | `GET` | Web UI Dashboard |
| `/ping` | `GET` | Server health check |
| `/status` | `GET` | Overall system status (APIs, tokens, MT5) |
| `/mode` | `POST` | Switch between `manual`, `semi_auto`, `auto` |
| `/kill-switch` | `POST` | Enable/Disable trading globally |
| `/account` | `GET` | Fetch DNSE account details |
| `/otp/latest` | `GET` | Get the latest cached OTP from Gmail |
| `/registration/send-email-otp`| `POST` | Trigger DNSE to send an OTP |
| `/registration/trading-token` | `POST` | Register a new trading session |
| `/history/sync` | `POST` | Trigger historical data synchronization |
| `/positions` | `GET` | List all open positions |
| `/position/{symbol}`| `GET` | Get position by symbol |
| `/order` | `POST` | Place a new order |
| `/order/{id}` | `GET` | Query order status |
| `/cancel` | `POST` | Cancel an active order |
| `/signals` | `GET` | Get pending MT5 signals |
| `/confirm` | `POST` | Confirm and execute a pending signal |
| `/reject` | `POST` | Reject a pending signal |
| `/loan-packages` | `GET` | Query available loan packages |
| `/ppse` | `GET` | Calculate Purchasing Power |

## 🤝 Contributing
Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the Project
2. Create your Feature Branch (`git checkout -b feature/AmazingFeature`)
3. Commit your Changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the Branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

---

## 🧑‍💻 Developer Notes

### ⚠️ DNSE API Authentication - MUST READ before touching auth code

> **See [`docs/DNSE_API_AUTH.md`](docs/DNSE_API_AUTH.md) for complete details.**

The DNSE OpenAPI uses HTTP Signatures (`x-signature` header). There are **3 critical gotchas** that will silently break authentication:

**1. Go's `http.Header.Set()` canonicalizes header names — DO NOT USE IT for auth headers.**
```go
// ❌ WRONG: Go converts "date" → "Date", "x-signature" → "X-Signature"
req.Header.Set("date", date)
req.Header.Set("x-signature", sig)

// ✅ CORRECT: Use raw map assignment to preserve exact lowercase names
req.Header["date"]        = []string{date}
req.Header["x-signature"] = []string{sig}
```

**2. The base64 signature MUST be URL-encoded.**
```go
raw := base64.StdEncoding.EncodeToString(mac.Sum(nil))
signature := url.QueryEscape(raw)  // "/" → "%2F", "+" → "%2B", "=" → "%3D"
```

**3. The signing string MUST include `nonce` on a 3rd line.**
```
(request-target): get /accounts
date: Wed, 29 Apr 2026 03:35:08 +0000
nonce: 40e22598c132020383c21e627a474353
```

The authoritative reference is the [official Python SDK](https://github.com/dnse-tech/openapi-sdk/tree/main/python) (`python/dnse/common.py`).

---

## 📄 License
This project is open-sourced under the MIT License.
