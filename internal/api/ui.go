package api

const indexHTML = layoutTop + `
  <style>
    .dashboard-shell {
      display: grid;
      grid-template-columns: minmax(0, 1fr) 360px;
      gap: 24px;
      align-items: start;
    }
    .dashboard-main {
      min-width: 0;
    }
    .dashboard-side {
      position: sticky;
      top: 24px;
    }
    .dashboard-grid {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 24px;
    }
    .dashboard-section {
      margin-bottom: 0;
    }
    .dashboard-section.full {
      grid-column: 1 / -1;
    }
    .toolbar {
      display: flex;
      justify-content: space-between;
      align-items: center;
      gap: 16px;
      margin-bottom: 24px;
    }
    .status-badge {
      display: inline-flex;
      align-items: center;
      gap: 8px;
      background: rgba(56, 142, 60, 0.15);
      border: 1px solid rgba(56, 142, 60, 0.3);
      color: #9be7a1;
      padding: 8px 12px;
      border-radius: 999px;
      font-size: 13px;
      font-weight: 600;
    }
    .status-badge::before {
      content: '';
      width: 10px;
      height: 10px;
      border-radius: 50%;
      background: currentColor;
      box-shadow: 0 0 8px currentColor;
    }
    .status-badge.error {
      background: rgba(211, 47, 47, 0.15);
      border-color: rgba(211, 47, 47, 0.35);
      color: #ff8a80;
    }
    .grid {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 16px;
    }
    .full {
      grid-column: 1 / -1;
    }
    .actions {
      margin-top: 18px;
      display: flex;
      flex-wrap: wrap;
      gap: 12px;
    }
    .inline-note {
      margin-top: 10px;
      font-size: 12px;
      color: var(--text-muted);
    }
    .console-card {
      margin-bottom: 0;
    }
    .console {
      background: #0c0c0c;
      border: 1px solid var(--border);
      border-radius: 8px;
      padding: 16px;
      height: min(72vh, 760px);
      overflow: auto;
      font-family: Consolas, Monaco, monospace;
      font-size: 13px;
      line-height: 1.5;
      color: #87f59f;
    }
    .console.error-text {
      color: #ff9b9b;
    }
    .console-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 12px;
      color: var(--text-muted);
      font-size: 12px;
      text-transform: uppercase;
      letter-spacing: 0.04em;
    }
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
      background: rgba(30, 30, 30, 0.95);
      border-left: 4px solid var(--primary);
      color: white;
      padding: 14px 18px;
      border-radius: 8px;
      box-shadow: 0 10px 24px rgba(0, 0, 0, 0.35);
      font-size: 14px;
      max-width: 360px;
    }
    .toast.error { border-color: var(--danger); }
    .toast.success { border-color: var(--success); }
    @media (max-width: 1100px) {
      .dashboard-shell {
        grid-template-columns: 1fr;
      }
      .dashboard-side {
        position: static;
      }
      .dashboard-grid {
        grid-template-columns: 1fr;
      }
      .dashboard-section.full {
        grid-column: auto;
      }
      .console {
        height: 320px;
      }
    }
  </style>

  <div class="toast-container" id="toast-container"></div>

  <div class="toolbar">
    <div>
      <h1 style="margin-bottom:8px">Bảng điều khiển</h1>
      <p style="margin:0; color: var(--text-muted)">Theo dõi bridge, kiểm tra kết nối DNSE và thao tác nhanh với dữ liệu thị trường.</p>
    </div>
    <div class="status-badge" id="status-badge">Đang kiểm tra máy chủ...</div>
  </div>

  <div class="dashboard-shell">
    <div class="dashboard-main">
      <div class="dashboard-grid">
    <section class="card dashboard-section">
      <h2>Điều khiển hệ thống</h2>
      <div class="grid">
        <div class="form-group full">
          <label>Chế độ giao dịch</label>
          <select id="sysMode">
            <option value="manual">Thủ công</option>
            <option value="semi_auto">Bán tự động</option>
            <option value="auto">Tự động</option>
          </select>
        </div>
      </div>
      <div class="actions">
        <button class="btn" onclick="ping()">Kiểm tra máy chủ</button>
        <button class="btn secondary" onclick="getStatus()">Xem trạng thái</button>
        <button class="btn secondary" onclick="changeMode()">Đổi chế độ</button>
        <button class="btn danger" onclick="killSwitch(true)">Tắt khẩn cấp</button>
        <button class="btn" style="background:var(--success)" onclick="killSwitch(false)">Bật lại</button>
      </div>
    </section>

    <section class="card dashboard-section">
      <h2>Tài khoản và xác thực</h2>
      <div class="grid">
        <div class="form-group">
          <label>Số tài khoản</label>
          <input id="accountNo" placeholder="Ví dụ: 0001007412">
        </div>
        <div class="form-group">
          <label>Loại OTP</label>
          <select id="otpType">
            <option value="email_otp">email_otp</option>
            <option value="smart_otp">smart_otp</option>
          </select>
        </div>
        <div class="form-group full">
          <label>Mã OTP</label>
          <input id="passcode" placeholder="Nhập tay hoặc lấy OTP mới nhất">
        </div>
      </div>
      <div class="actions">
        <button class="btn" onclick="getAccount()">Lấy thông tin tài khoản</button>
        <button class="btn secondary" onclick="getLatestOTP()">Lấy OTP tự động</button>
        <button class="btn secondary" onclick="sendOtp()">Gửi OTP qua email</button>
        <button class="btn" style="background:var(--success)" onclick="verifyOtp()">Đăng ký token</button>
      </div>
    </section>

    <section class="card dashboard-section">
      <h2>Dữ liệu thị trường và lịch sử</h2>
      <div class="grid">
        <div class="form-group">
          <label>Thời điểm bắt đầu (ms)</label>
          <input id="firstTime" type="number" value="0">
        </div>
        <div class="form-group">
          <label>Thời điểm kết thúc (ms)</label>
          <input id="lastTime" type="number" value="0">
        </div>
        <div class="form-group">
          <label>Số ngày lấy lùi</label>
          <input id="lookbackDays" type="number" value="365" min="1">
        </div>
      </div>
      <div class="actions">
        <button class="btn" onclick="syncHistory()">Đồng bộ lịch sử</button>
        <button class="btn secondary" onclick="fullSyncHistory()">Nạp lại toàn bộ lịch sử</button>
        <button class="btn secondary" onclick="backfillHistory()">Nạp lịch sử đến hết hôm qua</button>
      </div>
      <div class="inline-note">Luồng "đến hết hôm qua" dùng để dựng nền dữ liệu một lần; realtime và dữ liệu hôm nay đi theo luồng riêng.</div>
    </section>

    <section class="card dashboard-section">
      <h2>Sức mua (PPSE)</h2>
      <div class="grid">
        <div class="form-group">
          <label>Mã</label>
          <input id="pkgSymbol" value="VN30F1M">
        </div>
        <div class="form-group">
          <label>Loại thị trường</label>
          <select id="pkgMarketType">
            <option value="DERIVATIVE">DERIVATIVE</option>
            <option value="STOCK">STOCK</option>
          </select>
        </div>
        <div class="form-group">
          <label>Mã gói vay</label>
          <input id="loanPackageId" placeholder="Tự điền">
        </div>
        <div class="form-group">
          <label>Giá</label>
          <input id="ppsePrice" type="number" value="0">
        </div>
        <div class="form-group full">
          <label>Giá realtime</label>
          <div id="latestPriceText" class="inline-note">Chưa có giá realtime.</div>
        </div>
      </div>
      <div class="actions">
        <button class="btn" onclick="getLoanPackages()">Lấy danh sách gói vay</button>
        <button class="btn secondary" onclick="refreshLatestPrice()">Lấy giá realtime</button>
        <button class="btn secondary" onclick="getPpse()">Tính PPSE</button>
      </div>
    </section>

    <section class="card dashboard-section">
      <h2>Vị thế và thông tin lệnh</h2>
      <div class="grid">
        <div class="form-group full">
          <label>Mã lệnh</label>
          <input id="queryOrderId" placeholder="Ví dụ: ord-12345">
        </div>
      </div>
      <div class="actions">
        <button class="btn" onclick="getPositions()">Xem toàn bộ vị thế</button>
        <button class="btn secondary" onclick="getPositionBySymbol()">Xem vị thế theo mã</button>
        <button class="btn secondary" onclick="getOrder()">Xem chi tiết lệnh</button>
        <button class="btn danger" onclick="cancelOrder()">Hủy lệnh</button>
      </div>
    </section>

    <section class="card dashboard-section">
      <h2>Đặt lệnh thủ công</h2>
      <div class="grid">
        <div class="form-group">
          <label>Mã lệnh phía client</label>
          <input id="clientOrderId" value="mt5-test-001">
        </div>
        <div class="form-group">
          <label>Mã</label>
          <input id="symbol" value="VN30F1M">
        </div>
        <div class="form-group">
          <label>Chiều lệnh</label>
          <select id="side">
            <option value="BUY">Mua</option>
            <option value="SELL">Bán</option>
          </select>
        </div>
        <div class="form-group">
          <label>Khối lượng</label>
          <input id="quantity" type="number" min="1" value="1">
        </div>
        <div class="form-group">
          <label>Loại lệnh</label>
          <select id="orderType">
            <option value="MTL">MTL</option>
            <option value="LO">LO</option>
            <option value="MOK">MOK</option>
            <option value="MAK">MAK</option>
            <option value="ATO">ATO</option>
            <option value="ATC">ATC</option>
          </select>
        </div>
        <div class="form-group">
          <label>Giá</label>
          <input id="price" type="number" min="0" value="0">
        </div>
        <div class="form-group">
          <label>Loại thị trường</label>
          <select id="marketType">
            <option value="DERIVATIVE">DERIVATIVE</option>
            <option value="STOCK">STOCK</option>
          </select>
        </div>
        <div class="form-group">
          <label>Nhóm lệnh</label>
          <input id="orderCategory" value="NORMAL">
        </div>
      </div>
      <div class="actions">
        <button class="btn secondary" onclick="refreshLatestPrice()">Lấy giá realtime</button>
        <button class="btn" style="background:var(--success)" onclick="placeOrder()">Gửi lệnh</button>
      </div>
    </section>

    <section class="card dashboard-section">
      <h2>Tín hiệu chờ xử lý</h2>
      <div class="grid">
        <div class="form-group full">
          <label>Mã tín hiệu</label>
          <input id="signalId" placeholder="Ví dụ: sig-12345">
        </div>
      </div>
      <div class="actions">
        <button class="btn" onclick="getPendingSignals()">Tải tín hiệu chờ</button>
        <button class="btn" style="background:var(--success)" onclick="testBotSignal('BUY')">Test bot mua Demo</button>
        <button class="btn danger" onclick="testBotSignal('SELL')">Test bot bán Demo</button>
        <button class="btn" style="background:var(--success)" onclick="confirmSignal()">Xác nhận tín hiệu</button>
        <button class="btn danger" onclick="rejectSignal()">Từ chối tín hiệu</button>
      </div>
    </section>
      </div>
    </div>

    <aside class="dashboard-side">
      <section class="card console-card">
        <div class="console-header">
          <span>Đầu ra hệ thống</span>
          <button class="btn secondary" onclick="clearConsole()">Xóa</button>
        </div>
        <div id="output" class="console">Hệ thống đã sẵn sàng.</div>
      </section>
    </aside>
  </div>

  <script>
    const $ = (id) => document.getElementById(id);

    function showToast(message, type = 'success') {
      const container = $('toast-container');
      const toast = document.createElement('div');
      toast.className = 'toast ' + type;
      toast.textContent = message;
      container.appendChild(toast);
      setTimeout(() => toast.remove(), 3000);
    }

    function printConsole(data, isError = false) {
      const out = $('output');
      out.className = isError ? 'console error-text' : 'console';
      out.textContent = typeof data === 'string' ? data : JSON.stringify(data, null, 2);
    }

    function clearConsole() {
      $('output').textContent = '';
    }

    function setStatus(isOnline) {
      const badge = $('status-badge');
      if (isOnline) {
        badge.className = 'status-badge';
        badge.textContent = 'Bridge đang hoạt động';
      } else {
        badge.className = 'status-badge error';
        badge.textContent = 'Không kết nối được';
      }
    }

    async function request(path, options = {}) {
      try {
        const res = await fetch(path, options);
        const text = await res.text();
        let body;
        try { body = text ? JSON.parse(text) : {}; } catch { body = text; }

        if (!res.ok) {
          printConsole(body, true);
          showToast('Lỗi ' + res.status, 'error');
          setStatus(false);
          throw body;
        }

        printConsole(body, false);
        showToast('Thành công');
        setStatus(true);
        return body;
      } catch (err) {
        if (!err.error) {
          printConsole(err.message || 'Lỗi mạng', true);
          showToast('Lỗi mạng', 'error');
          setStatus(false);
        }
        throw err;
      }
    }

    async function run(fn) {
      try { await fn(); } catch (e) { console.error(e); }
    }

    async function loadLatestPrice(symbol, quiet = false) {
      symbol = (symbol || $('symbol').value || $('pkgSymbol').value || '').trim();
      if (!symbol) throw new Error('Vui lòng nhập mã');
      const res = await fetch('/market/latest?symbol=' + encodeURIComponent(symbol));
      const text = await res.text();
      let body;
      try { body = text ? JSON.parse(text) : {}; } catch { body = text; }
      if (!res.ok) {
        if (!quiet) {
          printConsole(body, true);
          showToast('Chưa có giá realtime cho ' + symbol, 'error');
        }
        throw body;
      }
      const price = Number(body.price || (body.tick && body.tick.last) || 0);
      if (price > 0) {
        $('price').value = price;
        $('ppsePrice').value = price;
        $('latestPriceText').textContent = symbol.toUpperCase() + ': ' + price + ' lúc ' + (body.time || '');
      }
      if (!quiet) {
        printConsole(body, false);
        showToast('Đã lấy giá realtime');
      }
      return price;
    }

    function refreshLatestPrice(symbol) {
      run(() => loadLatestPrice(symbol, false));
    }

    async function ensureOrderPrice() {
      const orderType = $('orderType').value;
      const currentPrice = Number($('price').value);
      if (currentPrice > 0 || orderType !== 'LO') return currentPrice;
      try { return await loadLatestPrice($('symbol').value, true); } catch { return currentPrice; }
    }

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
          showToast('Không tìm thấy OTP hợp lệ', 'error');
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
      run(() => request('/history/today-all', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({})
      }));
    }

    function fullSyncHistory() {
      run(() => request('/history/full-all', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          lookbackDays: Number($('lookbackDays').value) || 365
        })
      }));
    }

    function backfillHistory() {
      run(() => request('/history/backfill-all', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          lookbackDays: Number($('lookbackDays').value) || 365
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
      run(async () => {
        if (Number($('ppsePrice').value) <= 0) {
          try { await loadLatestPrice($('pkgSymbol').value || $('symbol').value, true); } catch {}
        }
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
      if (!id) return showToast('Vui lòng nhập mã lệnh', 'error');
      let path = '/order/' + encodeURIComponent(id);
      if (id.startsWith('client:')) {
        path = '/order/client/' + encodeURIComponent(id.slice(7));
      } else if (id.startsWith('signal-')) {
        path = '/order/client/' + encodeURIComponent(id);
      }
      run(() => request(path));
    }

    function cancelOrder() {
      const id = $('queryOrderId').value;
      if (!id) return showToast('Vui lòng nhập mã lệnh', 'error');
      run(() => request('/cancel', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ orderId: id })
      }));
    }

    function placeOrder() {
      run(async () => {
        await ensureOrderPrice();
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

    function testBotSignal(side) {
      run(async () => {
        await request('/mode', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ mode: 'auto' })
        });
        if (Number($('price').value) <= 0) {
          try { await loadLatestPrice($('symbol').value, true); } catch {}
        }
        const body = {
          accountNo: $('accountNo').value || 'ENTRADE_DEMO',
          symbol: $('symbol').value || 'VN30F1M',
          side: side,
          quantity: Number($('quantity').value) || 1,
          price: Number($('price').value) || 0,
          orderType: $('orderType').value || 'MTL',
          marketType: $('marketType').value || 'DERIVATIVE',
          orderCategory: $('orderCategory').value || 'NORMAL'
        };
        const data = await request('/signal', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(body)
        });
        if (data.signalId) {
          $('signalId').value = data.signalId;
          $('queryOrderId').value = 'client:signal-' + data.signalId;
        }
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
      if (!id) return showToast('Vui lòng nhập mã tín hiệu', 'error');
      run(() => request('/confirm', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ signalId: id })
      }));
    }

    function rejectSignal() {
      const id = $('signalId').value;
      if (!id) return showToast('Vui lòng nhập mã tín hiệu', 'error');
      run(() => request('/reject', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ signalId: id })
      }));
    }

    ping();
  </script>
` + layoutBottom
