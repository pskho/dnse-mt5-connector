package api

const indexHTML = `<!doctype html>
<html lang="vi">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>DNSE MT5 Connector Dashboard</title>
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap" rel="stylesheet">
  <style>
    :root {
      --bg: #0f172a;
      --card-bg: rgba(30, 41, 59, 0.7);
      --card-border: rgba(255, 255, 255, 0.1);
      --text: #f8fafc;
      --text-muted: #94a3b8;
      --primary: #3b82f6;
      --primary-hover: #2563eb;
      --danger: #ef4444;
      --danger-hover: #dc2626;
      --success: #10b981;
      --success-hover: #059669;
      --warning: #f59e0b;
      --input-bg: rgba(15, 23, 42, 0.6);
      --input-border: rgba(255, 255, 255, 0.15);
      --input-focus: #3b82f6;
    }

    * { box-sizing: border-box; }
    
    body {
      margin: 0;
      font-family: 'Inter', sans-serif;
      background: var(--bg);
      background-image: 
        radial-gradient(circle at 15% 50%, rgba(59, 130, 246, 0.15), transparent 25%),
        radial-gradient(circle at 85% 30%, rgba(16, 185, 129, 0.15), transparent 25%);
      color: var(--text);
      min-height: 100vh;
      display: flex;
      flex-direction: column;
    }

    header {
      padding: 20px 30px;
      background: rgba(15, 23, 42, 0.8);
      backdrop-filter: blur(12px);
      border-bottom: 1px solid var(--card-border);
      position: sticky;
      top: 0;
      z-index: 10;
      display: flex;
      justify-content: space-between;
      align-items: center;
    }

    h1 {
      font-size: 24px;
      font-weight: 700;
      margin: 0;
      background: linear-gradient(to right, #60a5fa, #34d399);
      -webkit-background-clip: text;
      -webkit-text-fill-color: transparent;
    }

    .status-badge {
      padding: 6px 12px;
      border-radius: 9999px;
      font-size: 13px;
      font-weight: 600;
      background: rgba(16, 185, 129, 0.2);
      color: var(--success);
      border: 1px solid rgba(16, 185, 129, 0.3);
      display: flex;
      align-items: center;
      gap: 6px;
    }
    
    .status-badge.error {
      background: rgba(239, 68, 68, 0.2);
      color: var(--danger);
      border-color: rgba(239, 68, 68, 0.3);
    }

    .status-badge::before {
      content: '';
      width: 8px;
      height: 8px;
      border-radius: 50%;
      background: currentColor;
      box-shadow: 0 0 8px currentColor;
    }

    main {
      flex: 1;
      max-width: 1400px;
      margin: 0 auto;
      padding: 30px;
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(350px, 1fr));
      gap: 24px;
      width: 100%;
    }

    section {
      background: var(--card-bg);
      backdrop-filter: blur(16px);
      border: 1px solid var(--card-border);
      border-radius: 16px;
      padding: 24px;
      box-shadow: 0 10px 30px -10px rgba(0, 0, 0, 0.5);
      transition: transform 0.2s, box-shadow 0.2s;
    }

    section:hover {
      transform: translateY(-2px);
      box-shadow: 0 20px 40px -15px rgba(0, 0, 0, 0.6);
      border-color: rgba(255, 255, 255, 0.2);
    }

    h2 {
      font-size: 18px;
      font-weight: 600;
      margin: 0 0 20px;
      padding-bottom: 12px;
      border-bottom: 1px solid var(--card-border);
      display: flex;
      align-items: center;
      justify-content: space-between;
    }

    .grid {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 16px;
    }

    .full {
      grid-column: 1 / -1;
    }

    label {
      display: flex;
      flex-direction: column;
      gap: 8px;
      font-size: 13px;
      font-weight: 500;
      color: var(--text-muted);
    }

    input, select {
      height: 40px;
      background: var(--input-bg);
      border: 1px solid var(--input-border);
      border-radius: 8px;
      padding: 0 12px;
      font-size: 14px;
      color: var(--text);
      font-family: inherit;
      transition: all 0.2s;
    }

    input:focus, select:focus {
      outline: none;
      border-color: var(--input-focus);
      box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.2);
    }

    button {
      height: 40px;
      background: var(--primary);
      color: white;
      border: none;
      border-radius: 8px;
      padding: 0 16px;
      font-size: 14px;
      font-weight: 600;
      cursor: pointer;
      transition: all 0.2s;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      gap: 8px;
    }

    button:hover {
      background: var(--primary-hover);
      transform: translateY(-1px);
    }

    button:active {
      transform: translateY(1px);
    }

    button.secondary {
      background: rgba(255, 255, 255, 0.1);
      color: var(--text);
    }

    button.secondary:hover {
      background: rgba(255, 255, 255, 0.15);
    }

    button.danger {
      background: var(--danger);
    }

    button.danger:hover {
      background: var(--danger-hover);
    }
    
    button.success {
      background: var(--success);
    }

    button.success:hover {
      background: var(--success-hover);
    }

    .actions {
      margin-top: 20px;
      display: flex;
      flex-wrap: wrap;
      gap: 12px;
    }

    .console-wrapper {
      grid-column: 1 / -1;
      position: sticky;
      bottom: 20px;
      z-index: 100;
    }

    .console {
      background: #020617;
      border: 1px solid var(--card-border);
      border-radius: 12px;
      padding: 16px;
      height: 250px;
      overflow: auto;
      font-family: 'Consolas', 'Monaco', monospace;
      font-size: 13px;
      line-height: 1.5;
      color: #34d399;
      box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5);
    }

    .console.error-text {
      color: #f87171;
    }

    .console-header {
      display: flex;
      justify-content: space-between;
      margin-bottom: 12px;
      font-family: 'Inter', sans-serif;
      color: var(--text-muted);
      font-size: 12px;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.05em;
    }

    /* Toasts */
    .toast-container {
      position: fixed;
      top: 20px;
      right: 20px;
      display: flex;
      flex-direction: column;
      gap: 10px;
      z-index: 1000;
    }

    .toast {
      background: rgba(30, 41, 59, 0.9);
      backdrop-filter: blur(8px);
      border-left: 4px solid var(--primary);
      color: white;
      padding: 16px 20px;
      border-radius: 8px;
      box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.3);
      font-size: 14px;
      font-weight: 500;
      animation: slideIn 0.3s ease forwards;
      max-width: 350px;
    }

    .toast.error { border-color: var(--danger); }
    .toast.success { border-color: var(--success); }

    @keyframes slideIn {
      from { transform: translateX(100%); opacity: 0; }
      to { transform: translateX(0); opacity: 1; }
    }

    @keyframes fadeOut {
      to { opacity: 0; transform: translateY(-10px); }
    }
    
    .signals-list, .positions-list {
      display: flex;
      flex-direction: column;
      gap: 10px;
      margin-top: 10px;
    }
    
    .list-item {
      background: rgba(0,0,0,0.2);
      border: 1px solid var(--card-border);
      padding: 12px;
      border-radius: 8px;
      font-size: 13px;
    }

  </style>
</head>
<body>

  <div class="toast-container" id="toast-container"></div>

  <header>
    <h1>DNSE MT5 Connector</h1>
    <div class="status-badge" id="status-badge">Checking Server...</div>
  </header>

  <main>
    <!-- Section 1: System Control -->
    <section>
      <h2>System Control</h2>
      <div class="grid">
        <label class="full">Trading Mode
          <select id="sysMode">
            <option value="manual">Manual</option>
            <option value="semi_auto">Semi-Auto</option>
            <option value="auto">Auto</option>
          </select>
        </label>
      </div>
      <div class="actions">
        <button onclick="ping()">Ping Server</button>
        <button class="secondary" onclick="getStatus()">Server Status</button>
        <button class="secondary" onclick="changeMode()">Set Mode</button>
        <button class="danger" onclick="killSwitch(true)">KILL SWITCH</button>
        <button class="success" onclick="killSwitch(false)">Unkill</button>
      </div>
    </section>

    <!-- Section 2: Account & Auth -->
    <section>
      <h2>Account & Auth</h2>
      <div class="grid">
        <label>Account No
          <input id="accountNo" placeholder="e.g. 0001007412">
        </label>
        <label>OTP Type
          <select id="otpType">
            <option value="email_otp">email_otp</option>
            <option value="smart_otp">smart_otp</option>
          </select>
        </label>
        <label class="full">Passcode
          <input id="passcode" placeholder="Enter OTP manually or fetch latest">
        </label>
      </div>
      <div class="actions">
        <button onclick="getAccount()">Get Account</button>
        <button class="secondary" onclick="getLatestOTP()">Fetch Auto OTP</button>
        <button class="secondary" onclick="sendOtp()">Send Email OTP</button>
        <button class="success" onclick="verifyOtp()">Register Token</button>
      </div>
    </section>

    <!-- Section 3: Market Data & History -->
    <section>
      <h2>Market Data & History</h2>
      <div class="grid">
        <label>First Time (ms)
          <input id="firstTime" type="number" value="0">
        </label>
        <label>Last Time (ms)
          <input id="lastTime" type="number" value="0">
        </label>
        <label>Lookback Days
          <input id="lookbackDays" type="number" value="365" min="1">
        </label>
      </div>
        <div class="actions">
          <button onclick="syncHistory()">Sync History Data</button>
          <button class="secondary" onclick="fullSyncHistory()">Full History Rebuild</button>
          <button class="secondary" onclick="backfillHistory()">Backfill <= Yesterday</button>
        </div>
        <div style="margin-top:10px; font-size:12px; color:var(--text-muted)">
          Note: Backfill <= Yesterday dung de nap mot lan phan lich su nen; realtime va du lieu hom nay de luong khac xu ly.
        </div>
    </section>

    <!-- Section 4: Loan & PPSE -->
    <section>
      <h2>Purchasing Power (PPSE)</h2>
      <div class="grid">
        <label>Symbol
          <input id="pkgSymbol" value="VN30F1M">
        </label>
        <label>Market Type
          <select id="pkgMarketType">
            <option value="DERIVATIVE">DERIVATIVE</option>
            <option value="STOCK">STOCK</option>
          </select>
        </label>
        <label>Loan Package ID
          <input id="loanPackageId" placeholder="Auto-filled">
        </label>
        <label>Price
          <input id="ppsePrice" type="number" value="0">
        </label>
      </div>
      <div class="actions">
        <button onclick="getLoanPackages()">Get Loan Packages</button>
        <button class="secondary" onclick="getPpse()">Calculate PPSE</button>
      </div>
    </section>

    <!-- Section 5: Position & Orders -->
    <section>
      <h2>Positions & Order Info</h2>
      <div class="grid">
        <label class="full">Order ID (to query/cancel)
          <input id="queryOrderId" placeholder="e.g. ord-12345">
        </label>
      </div>
      <div class="actions">
        <button onclick="getPositions()">All Positions</button>
        <button class="secondary" onclick="getPositionBySymbol()">Pos by Symbol</button>
        <button class="secondary" onclick="getOrder()">Get Order Info</button>
        <button class="danger" onclick="cancelOrder()">Cancel Order</button>
      </div>
    </section>

    <!-- Section 6: Place Order -->
    <section>
      <h2>Place Order (Manual)</h2>
      <div class="grid">
        <label>Client Order ID
          <input id="clientOrderId" value="mt5-test-001">
        </label>
        <label>Symbol
          <input id="symbol" value="VN30F1M">
        </label>
        <label>Side
          <select id="side">
            <option value="BUY">BUY</option>
            <option value="SELL">SELL</option>
          </select>
        </label>
        <label>Quantity
          <input id="quantity" type="number" min="1" value="1">
        </label>
        <label>Order Type
          <select id="orderType">
            <option value="MTL">MTL</option>
            <option value="LO">LO</option>
            <option value="MOK">MOK</option>
            <option value="MAK">MAK</option>
            <option value="ATO">ATO</option>
            <option value="ATC">ATC</option>
          </select>
        </label>
        <label>Price
          <input id="price" type="number" min="0" value="0">
        </label>
        <label>Market Type
          <select id="marketType">
            <option value="DERIVATIVE">DERIVATIVE</option>
            <option value="STOCK">STOCK</option>
          </select>
        </label>
        <label>Order Category
          <input id="orderCategory" value="NORMAL">
        </label>
      </div>
      <div class="actions">
        <button class="success" onclick="placeOrder()">Submit Order</button>
      </div>
    </section>

    <!-- Section 7: Signals Management -->
    <section>
      <h2>Pending Signals (Semi-Auto)</h2>
      <div class="grid">
        <label class="full">Signal ID (to confirm/reject)
          <input id="signalId" placeholder="e.g. sig-12345">
        </label>
      </div>
      <div class="actions">
        <button onclick="getPendingSignals()">Fetch Signals</button>
        <button class="success" onclick="confirmSignal()">Confirm Signal</button>
        <button class="danger" onclick="rejectSignal()">Reject Signal</button>
      </div>
    </section>

    <!-- Global Console -->
    <div class="console-wrapper">
      <div class="console">
        <div class="console-header">
          <span>Terminal Output</span>
          <span style="cursor:pointer" onclick="clearConsole()">Clear</span>
        </div>
        <div id="output">System initialized. Ready for requests.</div>
      </div>
    </div>
  </main>

  <script>
    const $ = (id) => document.getElementById(id);

    // Toast System
    function showToast(message, type = 'success') {
      const container = $('toast-container');
      const toast = document.createElement('div');
      toast.className = 'toast ' + type;
      toast.textContent = message;
      container.appendChild(toast);
      setTimeout(() => {
        toast.style.animation = 'fadeOut 0.3s ease forwards';
        setTimeout(() => toast.remove(), 300);
      }, 3000);
    }

    // Console System
    function printConsole(data, isError = false) {
      const out = $('output');
      out.className = isError ? 'error-text' : '';
      out.textContent = typeof data === 'string' ? data : JSON.stringify(data, null, 2);
    }
    
    function clearConsole() {
      $('output').textContent = '';
    }

    // Set Status Badge
    function setStatus(isOnline) {
      const badge = $('status-badge');
      if (isOnline) {
        badge.className = 'status-badge';
        badge.textContent = 'Server Online';
      } else {
        badge.className = 'status-badge error';
        badge.textContent = 'Connection Error';
      }
    }

    // API Wrapper
    async function request(path, options = {}) {
      try {
        const res = await fetch(path, options);
        const text = await res.text();
        let body;
        try { body = text ? JSON.parse(text) : {}; } catch { body = text; }
        
        if (!res.ok) {
          printConsole(body, true);
          showToast('Error: ' + res.status, 'error');
          setStatus(false);
          throw body;
        }
        
        printConsole(body, false);
        showToast('Success');
        setStatus(true);
        return body;
      } catch (err) {
        if (!err.error) { // Network error
          printConsole(err.message || 'Network error', true);
          showToast('Network error', 'error');
          setStatus(false);
        }
        throw err;
      }
    }

    async function run(fn) {
      try { await fn(); } catch (e) { console.error(e); }
    }

    // API Functions
    function ping() { run(() => request('/ping')); }
    
    function getStatus() { run(() => request('/status')); }

    function changeMode() {
      run(() => request('/mode', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ mode: $('sysMode').value })
      }));
    }

    function killSwitch(enabled) {
      run(() => request('/kill-switch', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ enabled: enabled })
      }));
    }

    function getAccount() {
      run(async () => {
        const data = await request('/account');
        if (data.accounts && data.accounts[0]) $('accountNo').value = data.accounts[0].accountNo;
      });
    }

    function getLatestOTP() {
      run(async () => {
        const data = await request('/otp/latest');
        if (data.valid && data.otp) {
          $('passcode').value = data.otp;
        } else {
          showToast('No valid OTP found', 'error');
        }
      });
    }

    function sendOtp() {
      run(() => request('/registration/send-email-otp', { method: 'POST' }));
    }

    function verifyOtp() {
      run(() => request('/registration/trading-token', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ passcode: $('passcode').value, otpType: $('otpType').value })
      }));
    }

    function syncHistory() {
      run(() => request('/history/sync', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ 
          firstTime: Number($('firstTime').value), 
          lastTime: Number($('lastTime').value) 
        })
      }));
    }

    function fullSyncHistory() {
      run(() => request('/history/full', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          lookbackDays: Number($('lookbackDays').value) || 365
        })
      }));
    }

    function backfillHistory() {
      run(() => request('/history/backfill', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          lookbackDays: Number($('lookbackDays').value) || 365,
          symbol: $('symbol').value || 'VN30F1M',
          marketType: 'DERIVATIVE',
          resolution: 1
        })
      }));
    }

    function getLoanPackages() {
      run(async () => {
        const params = new URLSearchParams({
          accountNo: $('accountNo').value,
          symbol: $('pkgSymbol').value,
          marketType: $('pkgMarketType').value
        });
        const data = await request('/loan-packages?' + params.toString());
        if (data.loanPackages && data.loanPackages[0]) $('loanPackageId').value = data.loanPackages[0].id;
      });
    }

    function getPpse() {
      run(() => {
        const params = new URLSearchParams({
          accountNo: $('accountNo').value,
          symbol: $('pkgSymbol').value,
          marketType: $('pkgMarketType').value,
          loanPackageId: $('loanPackageId').value,
          price: $('ppsePrice').value
        });
        return request('/ppse?' + params.toString());
      });
    }

    function getPositions() { run(() => request('/positions')); }
    
    function getPositionBySymbol() { 
      const sym = $('pkgSymbol').value || $('symbol').value;
      run(() => request('/position/' + sym)); 
    }

    function getOrder() { 
      const id = $('queryOrderId').value;
      if (!id) return showToast('Please enter Order ID', 'error');
      run(() => request('/order/' + id)); 
    }

    function cancelOrder() {
      const id = $('queryOrderId').value;
      if (!id) return showToast('Please enter Order ID', 'error');
      run(() => request('/cancel', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ orderId: id })
      }));
    }

    function placeOrder() {
      run(() => {
        const loanPackage = $('loanPackageId').value.trim();
        const body = {
          clientOrderId: $('clientOrderId').value,
          accountNo: $('accountNo').value,
          symbol: $('symbol').value,
          side: $('side').value,
          quantity: Number($('quantity').value),
          price: Number($('price').value),
          orderType: $('orderType').value,
          marketType: $('marketType').value,
          orderCategory: $('orderCategory').value || 'NORMAL'
        };
        if (loanPackage) body.loanPackageId = Number(loanPackage);
        return request('/order', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(body)
        });
      });
    }

    function getPendingSignals() {
      run(async () => {
        const data = await request('/signals');
        if (data.signals && data.signals.length > 0) {
          $('signalId').value = data.signals[data.signals.length - 1].id;
        }
      });
    }

    function confirmSignal() {
      const id = $('signalId').value;
      if (!id) return showToast('Please enter Signal ID', 'error');
      run(() => request('/confirm', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ signalId: id })
      }));
    }

    function rejectSignal() {
      const id = $('signalId').value;
      if (!id) return showToast('Please enter Signal ID', 'error');
      run(() => request('/reject', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ signalId: id })
      }));
    }

    // Initial check
    ping();
  </script>
</body>
</html>`

