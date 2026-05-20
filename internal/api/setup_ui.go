package api

const layoutCSS = `
:root {
  --bg: #121212;
  --surface: #1e1e1e;
  --surface-hover: #2c2c2c;
  --primary: #d32f2f;
  --primary-hover: #b71c1c;
  --text: #ffffff;
  --text-muted: #9e9e9e;
  --border: #333333;
  --success: #388e3c;
  --warning: #f57c00;
  --danger: #d32f2f;
}
* { box-sizing: border-box; }
body {
  margin: 0;
  font-family: "Segoe UI", Arial, sans-serif;
  background: var(--bg);
  color: var(--text);
  display: flex;
  min-height: 100vh;
}
nav {
  width: 250px;
  background: var(--surface);
  border-right: 1px solid var(--border);
  padding: 20px 0;
  display: flex;
  flex-direction: column;
}
.brand {
  padding: 0 20px 20px;
  font-size: 18px;
  font-weight: 700;
  border-bottom: 1px solid var(--border);
  margin-bottom: 20px;
  color: var(--primary);
}
nav a {
  padding: 12px 20px;
  color: var(--text-muted);
  text-decoration: none;
  font-weight: 500;
  transition: all 0.2s;
}
nav a:hover, nav a.active {
  background: rgba(211, 47, 47, 0.12);
  color: var(--primary);
  border-right: 3px solid var(--primary);
}
main {
  flex: 1;
  padding: 40px;
  max-width: 1180px;
  width: 100%;
  margin: 0 auto;
}
.card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 24px;
  margin-bottom: 24px;
}
h1, h2, h3 { margin-top: 0; }
.btn {
  background: var(--primary);
  color: white;
  border: none;
  padding: 10px 16px;
  border-radius: 6px;
  cursor: pointer;
  font-weight: 600;
  font-size: 14px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}
.btn:hover { background: var(--primary-hover); }
.btn.secondary {
  background: transparent;
  border: 1px solid var(--border);
  color: var(--text);
}
.btn.secondary:hover { background: var(--surface-hover); }
.form-group { margin-bottom: 16px; }
label {
  display: block;
  margin-bottom: 6px;
  color: var(--text-muted);
  font-size: 14px;
}
input, select, textarea {
  width: 100%;
  background: var(--bg);
  border: 1px solid var(--border);
  color: var(--text);
  padding: 10px 12px;
  border-radius: 6px;
  font-family: inherit;
}
.status-indicator {
  display: inline-block;
  width: 10px;
  height: 10px;
  border-radius: 50%;
  margin-right: 8px;
}
.status-ok { background: var(--success); box-shadow: 0 0 8px var(--success); }
.status-err { background: var(--danger); box-shadow: 0 0 8px var(--danger); }
.status-warn { background: var(--warning); box-shadow: 0 0 8px var(--warning); }
.grid-2 { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 20px; }
.grid-3 { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 16px; }
.step {
  border-left: 2px solid var(--border);
  padding-left: 20px;
  margin-left: 10px;
  padding-bottom: 30px;
  position: relative;
}
.step::before {
  content: "";
  position: absolute;
  left: -11px;
  top: 0;
  width: 20px;
  height: 20px;
  background: var(--surface);
  border: 2px solid var(--primary);
  border-radius: 50%;
}
.step.done::before {
  background: var(--success);
  border-color: var(--success);
}
.chip-list {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}
.chip {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-radius: 999px;
  border: 1px solid var(--border);
  background: #171717;
}
.chip input { width: auto; margin: 0; }
.inline-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 18px;
}
.account-list {
  display: grid;
  gap: 10px;
  margin-top: 16px;
}
.account-row {
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 12px;
  background: #171717;
}
.account-head {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: flex-start;
}
.account-meta {
  color: var(--text-muted);
  font-size: 13px;
  line-height: 1.6;
}
.mini-btn {
  padding: 7px 10px;
  font-size: 12px;
}
.muted {
  color: var(--text-muted);
}
.section-title {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
}
pre {
  white-space: pre-wrap;
  word-wrap: break-word;
}
code {
  font-family: Consolas, monospace;
}
@media (max-width: 960px) {
  body { display: block; }
  nav { width: 100%; }
  main { padding: 24px; }
  .grid-2, .grid-3 { grid-template-columns: 1fr; }
}
`

const layoutTop = `<!DOCTYPE html>
<html lang="vi">
<head>
  <meta charset="UTF-8">
  <title>DNSE MT5 Connector</title>
  <style>` + layoutCSS + `</style>
</head>
<body>
  <nav>
    <div class="brand">DNSE Connector</div>
    <a href="/" id="nav-dash">Bảng điều khiển</a>
    <a href="/setup" id="nav-setup">Bắt đầu sử dụng</a>
    <a href="/status" id="nav-status">Trạng thái hệ thống</a>
    <a href="/settings" id="nav-settings">Cấu hình</a>
    <a href="/logs" id="nav-logs">Nhật ký</a>
    <div style="margin-top:auto; padding:20px;">
      <button class="btn secondary" style="width:100%" onclick="window.location.href='/support/export'">Xuất gói hỗ trợ</button>
    </div>
  </nav>
  <main>
`

const layoutBottom = `
  </main>
  <script>
    const path = window.location.pathname;
    if(path === '/') document.getElementById('nav-dash').classList.add('active');
    else if(path.startsWith('/setup')) document.getElementById('nav-setup').classList.add('active');
    else if(path.startsWith('/status')) document.getElementById('nav-status').classList.add('active');
    else if(path.startsWith('/settings')) document.getElementById('nav-settings').classList.add('active');
    else if(path.startsWith('/logs')) document.getElementById('nav-logs').classList.add('active');
  </script>
</body>
</html>
`

const setupHTML = layoutTop + `
  <h1>Bắt đầu sử dụng</h1>
  <p class="muted">Thiết lập nhanh tài khoản DNSE, chọn bộ mã cần theo dõi và cài tự động vào MetaTrader 5. Ở lần chạy đầu, hệ thống sẽ tự nạp lịch sử nền để khách không phải thao tác thêm trên bảng điều khiển.</p>

  <div class="card">
    <div class="step done">
      <h3>Bước 1: Kiểm tra hệ thống</h3>
      <div id="sys-check-res">Đang kiểm tra...</div>
    </div>

    <div class="step" id="step-2">
      <h3>Bước 2: Cấu hình thông tin DNSE và danh sách mã</h3>
      <p class="muted">Có thể quản lý chi tiết hơn tại trang <a href="/settings" style="color: var(--primary)">Cấu hình</a>.</p>
      <button class="btn secondary" onclick="window.location.href='/settings'">Mở trang cấu hình</button>
    </div>

    <div class="step" id="step-3">
      <h3>Bước 3: Cài vào MT5</h3>
      <p>Hệ thống sẽ tự chép DLL và Expert Advisor vào thư mục dữ liệu MetaTrader 5.</p>
      <button class="btn" onclick="detectMT5()">Tự dò và cài đặt</button>
      <pre id="mt5-res" style="background: var(--bg); padding: 10px; border-radius: 4px; margin-top: 10px; display: none;"></pre>
    </div>

    <div class="step" id="step-4">
      <h3>Bước 4: Kiểm tra kết nối DNSE</h3>
      <button class="btn secondary" onclick="testDNSE()">Kiểm tra kết nối</button>
      <pre id="dnse-res" style="background: var(--bg); padding: 10px; border-radius: 4px; margin-top: 10px; display: none;"></pre>
    </div>

    <div class="step" id="step-5" style="border-left-color: transparent;">
      <h3>Bước 5: Hoàn tất</h3>
      <p>Sau khi lưu cấu hình, bridge sẽ dùng mã chính để nạp lịch sử nền trong lần chạy đầu. Khi mở MT5, các custom symbol dạng <code>_DNSE</code> sẽ được tạo theo danh sách mã đã chọn.</p>
    </div>
  </div>

  <script>
    async function requestJSON(url, opts) {
      try {
        const res = await fetch(url, opts);
        const data = await res.json();
        return { ok: res.ok, data };
      } catch (e) {
        return { ok: false, data: { error: e.message } };
      }
    }

    async function detectMT5() {
      const out = document.getElementById('mt5-res');
      out.style.display = 'block';
      out.innerText = 'Đang cài các tệp vào MT5...';
      const { data } = await requestJSON('/api/setup/install', { method: 'POST' });
      if (data.success) {
        out.innerText = data.logs.join('\n');
        document.getElementById('step-3').classList.add('done');
      } else {
        out.innerText = data.message + '\n' + (data.logs ? data.logs.join('\n') : '');
      }
    }

    async function testDNSE() {
      const out = document.getElementById('dnse-res');
      out.style.display = 'block';
      out.innerText = 'Đang kiểm tra...';
      const { ok, data } = await requestJSON('/account');
      if (ok && !data.error) {
        out.innerText = 'Kết nối thành công.\nĐã tải thông tin tài khoản.';
        document.getElementById('step-4').classList.add('done');
        document.getElementById('step-5').classList.add('done');
      } else {
        out.innerText = 'Lỗi: ' + JSON.stringify(data, null, 2);
      }
    }

    requestJSON('/status').then(({ ok, data }) => {
      let html = '<ul style="margin:0; padding-left:20px; color: var(--text-muted)">';
      html += '<li>Go Bridge: ' + (ok ? '<span style="color:var(--success)">OK</span>' : 'Lỗi') + '</li>';
      html += '<li>Cổng TCP 9090: ' + (data.market_data_ok ? '<span style="color:var(--success)">OK</span>' : 'Lỗi') + '</li>';
      html += '<li>Khách MT5 đang kết nối: ' + (data.market_data_active_clients || 0) + '</li>';
      html += '</ul>';
      document.getElementById('sys-check-res').innerHTML = html;
    });
  </script>
` + layoutBottom

const settingsHTML = layoutTop + `
  <div class="section-title">
    <div>
      <h1 style="margin-bottom:8px;">Cấu hình</h1>
      <p class="muted" style="margin:0;">Chọn tài khoản DNSE, danh sách mã muốn theo dõi và mã chính để MT5 ưu tiên nạp dữ liệu ngay từ lần chạy đầu.</p>
    </div>
  </div>

  <div class="grid-2">
    <section class="card">
      <h2>Thông tin DNSE</h2>
      <div class="form-group">
        <label>Khóa API DNSE</label>
        <input type="text" id="apiKey" placeholder="Để trống nếu không muốn thay đổi">
      </div>
      <div class="form-group">
        <label>Mã bí mật API DNSE</label>
        <input type="password" id="apiSecret" placeholder="Để trống nếu không muốn thay đổi">
      </div>
      <div class="form-group">
        <label>Số tài khoản DNSE</label>
        <input type="text" id="accountNo" placeholder="Ví dụ: 0001007412">
      </div>
      <div class="form-group">
        <label>Chế độ mô phỏng</label>
        <select id="mockMode">
          <option value="false">Tắt (API thật)</option>
          <option value="true">Bật (kiểm thử offline)</option>
        </select>
      </div>
      <div class="form-group">
        <label>Kết nối đặt lệnh</label>
        <select id="tradingProvider" onchange="renderTradingProviderHint()">
          <option value="dnse">Chỉ DNSE</option>
          <option value="entrade">DNSE + Entrade</option>
        </select>
        <p id="tradingProviderHint" class="muted" style="margin-top:10px;"></p>
      </div>
      <div style="margin-top:22px;">
        <h3 style="font-size:16px;">Tài khoản DNSE có thể đặt lệnh</h3>
        <div id="dnseAccountsList" class="account-list"></div>
      </div>
      <p class="muted">Nếu bật DNSE + Entrade, server có thể route lệnh tới cả hai loại tài khoản theo nhóm đặt lệnh ở phần bên dưới.</p>
    </section>

    <section class="card">
      <h2>Li&ecirc;n k&#7871;t t&agrave;i kho&#7843;n Entrade</h2>
      <p class="muted">Kh&aacute;ch h&agrave;ng ch&#7881; c&#7847;n nh&#7853;p username/password Entrade. H&#7879; th&#7889;ng s&#7869; t&#7921; &#273;&#259;ng nh&#7853;p, l&#7845;y th&ocirc;ng tin t&agrave;i kho&#7843;n master v&agrave; g&oacute;i vay ph&aacute;i sinh.</p>
      <div id="entradeLinkStatus" class="muted" style="margin-bottom:16px;">Ch&#432;a li&ecirc;n k&#7871;t t&agrave;i kho&#7843;n Entrade.</div>
      <div class="form-group">
        <label>Username Entrade</label>
        <input type="text" id="entradeUsername" autocomplete="username" placeholder="V&iacute; d&#7909;: 1000000001">
      </div>
      <div class="form-group">
        <label>Password Entrade</label>
        <input type="password" id="entradePassword" autocomplete="current-password" placeholder="Nh&#7853;p password Entrade">
      </div>
      <div class="inline-actions">
        <button class="btn" type="button" onclick="linkEntradeAccount()">Li&ecirc;n k&#7871;t t&agrave;i kho&#7843;n</button>
      </div>
      <p id="entradeLinkResult" class="muted" style="margin-top:16px;"></p>
      <div style="margin-top:22px;">
        <h3 style="font-size:16px;">Tài khoản đã liên kết</h3>
        <p class="muted">Chọn các tài khoản trong nhóm đặt lệnh. Khi bot gửi tín hiệu không chỉ rõ tài khoản, bridge sẽ gửi lệnh tới toàn bộ nhóm này.</p>
        <div id="entradeAccountsList" class="account-list"></div>
      </div>
    </section>

    <section class="card">
      <h2>Quản lý đặt lệnh</h2>
      <p class="muted">Tạo nhóm đặt lệnh rồi gán từng luồng sử dụng vào nhóm phù hợp. MT5 và bot có thể để trống tài khoản; server sẽ tự chọn theo cấu hình này.</p>
      <div class="notice" style="margin:14px 0; padding:14px; border:1px solid var(--border); border-radius:8px;">
        <strong>Hướng dẫn nhanh cho khách hàng</strong>
        <div class="muted" style="margin-top:8px; line-height:1.7;">
          1. Chọn tài khoản DNSE/Entrade vào từng nhóm.<br>
          2. Bấm <strong>Sửa</strong> trong nhóm để đặt loại lệnh, khối lượng mặc định, trần khối lượng và danh sách mã được phép. Để trống danh sách mã nếu muốn nhận mã từ chart/bot.<br>
          3. Gán Dashboard, nút BUY/SELL MT5, SuperTrend Bot hoặc API vào nhóm cần dùng.<br>
          4. Nếu code bot riêng, gửi POST <code>/signal</code> với <code>symbol</code> là mã trên chart, <code>source</code> là <code>supertrend</code>, <code>mt5_manual</code>, <code>dashboard</code> hoặc <code>signal_api</code>. Có thể bỏ <code>quantity</code> để dùng khối lượng mặc định của nhóm, hoặc gửi thêm <code>routeGroupId</code> để chỉ định nhóm cụ thể.<br>
          5. Muốn test an toàn, tạo nhóm chỉ gồm tài khoản Entrade Demo/giấy rồi gán bot vào nhóm đó.
        </div>
      </div>
      <div id="executionGroupsList" class="account-list"></div>
      <div class="inline-actions" style="margin:14px 0;">
        <button class="btn secondary" type="button" onclick="addExecutionGroup()">Thêm nhóm đặt lệnh</button>
      </div>
      <h3 style="font-size:16px;">Gán luồng sử dụng</h3>
      <div class="grid-2" style="grid-template-columns:1fr 1fr; gap:14px;">
        <div class="form-group"><label>Dashboard đặt lệnh</label><select id="routeDashboard"></select></div>
        <div class="form-group"><label>Nút BUY/SELL trên MT5</label><select id="routeMT5Manual"></select></div>
        <div class="form-group"><label>SuperTrend Bot</label><select id="routeSuperTrend"></select></div>
        <div class="form-group"><label>API /signal mặc định</label><select id="routeSignalApi"></select></div>
        <div class="form-group"><label>API /order mặc định</label><select id="routeOrderApi"></select></div>
      </div>
    </section>

    <section class="card">
      <h2>Danh sách mã</h2>
      <div class="form-group">
        <label>Mã chính dùng để ưu tiên hiển thị và nạp dữ liệu đầu tiên</label>
        <select id="primarySymbol"></select>
      </div>
      <div class="form-group">
        <label>Tải danh sách mã theo sàn khi cần</label>
        <div class="inline-actions" style="margin-top:0">
          <button class="btn secondary" type="button" onclick="loadInstrumentSymbols()">Tải toàn bộ danh sách mã từ DNSE</button>
          <button class="btn secondary" type="button" onclick="toggleExchange('HOSE')">HOSE</button>
          <button class="btn secondary" type="button" onclick="toggleExchange('HNX')">HNX</button>
          <button class="btn secondary" type="button" onclick="toggleExchange('UPCOM')">UPCOM</button>
          <button class="btn secondary" type="button" onclick="toggleExchange('INDEX')">Chỉ số</button>
          <button class="btn secondary" type="button" onclick="toggleExchange('DERIVATIVE')">Phái sinh</button>
        </div>
        <p id="instrument-load-res" class="muted" style="margin-top:10px;">Danh sách mã được tải một lần từ ticker API của DNSE, lưu lại để dùng tiếp cho bridge và MT5.</p>
      </div>
      <div class="form-group">
        <label>Tìm nhanh mã</label>
        <input type="text" id="symbolSearch" placeholder="Ví dụ: HPG, FPT, VN30..." oninput="renderSymbolChoices()">
      </div>
      <div class="form-group">
        <label>Chọn các mã cần theo dõi</label>
        <div id="symbolChoices" class="chip-list" style="max-height:320px; overflow:auto; align-content:flex-start;"></div>
      </div>
      <div class="muted">MT5 sẽ tạo custom symbol cho toàn bộ danh sách này. Các mã phái sinh hiện dùng chuẩn <code>V100F*</code> cho bộ VN100.</div>
    </section>
  </div>

  <div class="card">
    <div class="inline-actions">
      <button class="btn" onclick="saveSettings()">Lưu cấu hình</button>
      <button class="btn secondary" onclick="selectDefaultSet()">Chọn bộ mặc định</button>
      <button class="btn secondary" onclick="selectAllSymbols()">Chọn tất cả</button>
      <button class="btn secondary" onclick="clearSymbols()">Bỏ chọn tất cả</button>
    </div>
    <p id="save-res" class="muted" style="margin-top:16px;"></p>
  </div>

  <script>
    const defaultSymbols = ["VN30F1M","VN30F2M","VN30F1Q","VN30F2Q","V100F1M","V100F2M","V100F1Q","V100F2Q","VNINDEX","VN30","HNX","HNX30","VN100","UPCOM","VNXALLSHARE"];
    let availableSymbols = [];
    let selectedExchanges = new Set(["HOSE", "HNX", "UPCOM", "INDEX", "DERIVATIVE"]);
    let selectedSymbolsState = new Set(defaultSymbols);
    let currentEntrade = {};
    let currentTrading = { groups: [], routes: {} };

    function mergeSymbols(symbols) {
      const seen = new Set(availableSymbols);
      symbols.forEach((symbol) => {
        symbol = (symbol || '').trim().toUpperCase();
        if (symbol && !seen.has(symbol)) {
          seen.add(symbol);
          availableSymbols.push(symbol);
        }
      });
      availableSymbols.sort((a, b) => a.localeCompare(b));
    }

    async function loadProfiles() {
      const res = await fetch('/symbols/profiles');
      const data = await res.json();
      availableSymbols = [];
      mergeSymbols((data.profiles || []).map((profile) => profile.Symbol || ''));
      mergeSymbols(defaultSymbols);
      renderSymbolChoices();
    }

    async function loadInstrumentSymbols() {
      const note = document.getElementById('instrument-load-res');
      const exchanges = Array.from(selectedExchanges);
      note.textContent = 'Đang tải toàn bộ metadata mã từ DNSE...';
      try {
        const res = await fetch('/symbols/tickers?refresh=1');
        const data = await res.json();
        if (!res.ok) {
          note.textContent = data.error || 'Không tải được danh sách mã từ DNSE.';
          return;
        }
        const filtered = (data.data || []).filter((item) => {
          const exchange = (item.exchange || item.Exchange || '').toString().trim().toUpperCase();
          return !exchanges.length || selectedExchanges.has(exchange);
        });
        mergeSymbols(filtered.map((item) => item.symbol || item.Symbol || ''));
        renderSymbolChoices(getSelectedSymbols());
        note.textContent = 'Đã nạp ' + filtered.length + ' / ' + (data.total || 0) + ' mã từ ticker API. Nhóm đang lọc: ' + exchanges.join(', ') + '.';
      } catch (e) {
        note.textContent = 'Không tải được danh sách mã từ DNSE.';
      }
    }

    function toggleExchange(exchange) {
      if (selectedExchanges.has(exchange)) selectedExchanges.delete(exchange);
      else selectedExchanges.add(exchange);
      document.getElementById('instrument-load-res').textContent = 'Sàn đang chọn: ' + Array.from(selectedExchanges).join(', ');
    }

    function renderSymbolChoices(selected = null) {
      if (selected) {
        selectedSymbolsState = new Set(selected.map((value) => (value || '').trim().toUpperCase()).filter(Boolean));
      }
      const box = document.getElementById('symbolChoices');
      const primary = document.getElementById('primarySymbol');
      const currentSelected = new Set(getSelectedSymbols());
      const keyword = (document.getElementById('symbolSearch')?.value || '').trim().toUpperCase();
      box.innerHTML = '';
      primary.innerHTML = '';

      availableSymbols.forEach((symbol) => {
        if (keyword && symbol.indexOf(keyword) < 0) return;
        const id = 'sym-' + symbol;
        const wrap = document.createElement('label');
        wrap.className = 'chip';
        wrap.innerHTML = '<input type="checkbox" id="' + id + '" value="' + symbol + '"><span>' + symbol + '</span>';
        box.appendChild(wrap);
        const input = wrap.querySelector('input');
        input.checked = currentSelected.has(symbol);
        input.addEventListener('change', () => {
          if (input.checked) selectedSymbolsState.add(symbol);
          else selectedSymbolsState.delete(symbol);
          syncPrimaryOptions();
        });

        const option = document.createElement('option');
        option.value = symbol;
        option.textContent = symbol;
        primary.appendChild(option);
      });

      syncPrimaryOptions();
    }

    function getSelectedSymbols() {
      return Array.from(selectedSymbolsState);
    }

    function syncPrimaryOptions() {
      const selected = getSelectedSymbols();
      const primary = document.getElementById('primarySymbol');
      const current = primary.value;

      Array.from(primary.options).forEach((option) => {
        option.hidden = selected.indexOf(option.value) < 0;
      });

      if (!selected.length) {
        primary.value = '';
        return;
      }
      if (selected.indexOf(current) >= 0) {
        primary.value = current;
      } else {
        primary.value = selected[0];
      }
    }

    function selectDefaultSet() {
      selectedSymbolsState = new Set(defaultSymbols);
      renderSymbolChoices();
    }

    function selectAllSymbols() {
      selectedSymbolsState = new Set(availableSymbols);
      renderSymbolChoices();
    }

    function clearSymbols() {
      selectedSymbolsState = new Set();
      renderSymbolChoices();
    }

    function escapeHtml(value) {
      return String(value || '').replace(/[&<>"']/g, (ch) => ({
        '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'
      }[ch]));
    }

    function entradeAccounts() {
      return (currentEntrade && Array.isArray(currentEntrade.accounts)) ? currentEntrade.accounts : [];
    }

    function entradeDefaultSet() {
      return new Set(((currentEntrade && currentEntrade.defaultAccountNos) || []).map((value) => String(value || '').trim().toUpperCase()).filter(Boolean));
    }

    async function loadDNSEAccounts() {
      const box = document.getElementById('dnseAccountsList');
      if (!box) return;
      box.innerHTML = '<div class="muted">Đang tải tài khoản DNSE...</div>';
      try {
        const res = await fetch('/api/dnse/orderable-accounts');
        const data = await res.json();
        if (!res.ok) {
          box.innerHTML = '<div class="muted">Chưa tải được danh sách tài khoản DNSE.</div>';
          return;
        }
        const accounts = data.accounts || [];
        if (!accounts.length) {
          box.innerHTML = '<div class="muted">Chưa có tài khoản DNSE khả dụng.</div>';
          return;
        }
        box.innerHTML = accounts.map((account) => {
          const accountNo = account.accountNo || account.AccountNo || '';
          const status = account.derivativeAccountStatus || account.DerivativeAccountStatus || 'UNKNOWN';
          const configured = accountNo && accountNo === document.getElementById('accountNo').value.trim();
          return '<div class="account-row">' +
            '<div style="font-weight:700">' + escapeHtml(accountNo || '-') + (configured ? ' <span class="muted">(đang chọn)</span>' : '') + '</div>' +
            '<div class="account-meta">Trạng thái phái sinh: <code>' + escapeHtml(status) + '</code></div>' +
          '</div>';
        }).join('');
      } catch (e) {
        box.innerHTML = '<div class="muted">Chưa tải được danh sách tài khoản DNSE.</div>';
      }
    }

    function renderEntradeStatus() {
      const status = document.getElementById('entradeLinkStatus');
      const username = document.getElementById('entradeUsername');
      const enabled = currentEntrade && currentEntrade.enabled;
      const accountNo = currentEntrade && currentEntrade.accountNo;
      const investorId = currentEntrade && currentEntrade.investorId;
      const accounts = entradeAccounts();
      username.value = (currentEntrade && currentEntrade.username) || '';
      if (accounts.length) {
        if (enabled) {
          status.innerHTML = '<span class="status-indicator status-ok"></span>Đã liên kết ' + accounts.length + ' tài khoản Entrade. Server đang bật route DNSE + Entrade.';
        } else {
          status.innerHTML = '<span class="status-indicator status-warn"></span>Đã liên kết ' + accounts.length + ' tài khoản Entrade, nhưng hiện chỉ bật route DNSE.';
        }
        return;
      }
      if (enabled && investorId) {
        status.innerHTML = '<span class="status-indicator status-ok"></span>Đ&atilde; li&ecirc;n k&#7871;t Entrade: investorId <code>' + investorId + '</code>' + (accountNo ? ', master account <code>' + accountNo + '</code>' : '') + '.';
      } else {
        status.innerHTML = '<span class="status-indicator status-warn"></span>Ch&#432;a li&ecirc;n k&#7871;t t&agrave;i kho&#7843;n Entrade.';
      }
    }

    function renderEntradeAccounts() {
      const box = document.getElementById('entradeAccountsList');
      if (!box) return;
      const accounts = entradeAccounts();
      const defaults = entradeDefaultSet();
      if (!accounts.length) {
        box.innerHTML = '<div class="muted">Chưa có tài khoản Entrade nào.</div>';
        return;
      }
      box.innerHTML = accounts.map((account, index) => {
        const id = String(account.id || '').toUpperCase();
        const inGroup = defaults.has(id);
        const loanID = Number(account.loanPackageId || 0);
        return '<div class="account-row">' +
          '<div class="account-head">' +
            '<div>' +
              '<div style="font-weight:700">' + escapeHtml(id) + (account.enabled === false ? ' <span class="muted">(đang tắt)</span>' : '') + '</div>' +
              '<div class="account-meta">' +
                'Username: <code>' + escapeHtml(account.username) + '</code><br>' +
                'InvestorId: <code>' + escapeHtml(account.investorId || '-') + '</code><br>' +
                'Master account: <code>' + escapeHtml(account.accountNo || '-') + '</code><br>' +
                'Loan package ID dùng đặt lệnh: <code>' + (loanID > 0 ? loanID : 'tự lấy') + '</code>' +
              '</div>' +
            '</div>' +
            '<div class="inline-actions" style="margin-top:0; justify-content:flex-end;">' +
              '<button class="btn secondary mini-btn" type="button" onclick="editEntradeAccount(' + index + ')">Sửa</button>' +
              '<button class="btn secondary mini-btn" type="button" onclick="deleteEntradeAccount(' + index + ')">Xóa</button>' +
            '</div>' +
          '</div>' +
          '<label class="chip" style="margin-top:10px; width:max-content;">' +
            '<input type="checkbox" ' + (inGroup ? 'checked' : '') + ' onchange="toggleEntradeDefault(' + index + ', this.checked)">' +
            '<span>Đặt lệnh trong nhóm mặc định</span>' +
          '</label>' +
        '</div>';
      }).join('');
    }

    function renderTradingProviderHint() {
      const hint = document.getElementById('tradingProviderHint');
      if (!hint) return;
      const provider = document.getElementById('tradingProvider').value;
      const entradeCount = entradeAccounts().length;
      const defaultCount = (currentEntrade.defaultAccountNos || []).length;
      if (provider === 'entrade') {
        if (!entradeCount) {
          hint.innerHTML = '<span class="status-indicator status-warn"></span>Bạn chưa liên kết Entrade. Các nhóm có tài khoản Entrade sẽ chưa dùng được cho tới khi liên kết và lưu cấu hình.';
        } else if (!defaultCount) {
          hint.innerHTML = '<span class="status-indicator status-warn"></span>Hãy tick ít nhất một tài khoản Entrade vào nhóm mặc định. MT5 có thể để trống account, server sẽ tự chọn nhóm này.';
        } else {
          hint.innerHTML = '<span class="status-indicator status-ok"></span>MT5 có thể để trống account. Server sẽ route theo nhóm đặt lệnh, bao gồm cả DNSE và Entrade nếu nhóm có cả hai.';
        }
        return;
      }
      hint.innerHTML = '<span class="status-indicator status-ok"></span>MT5 có thể để trống account. Server chỉ dùng các nhóm có tài khoản DNSE.';
    }

    function allOrderAccounts() {
      const out = [];
      const dnseAccount = document.getElementById('accountNo').value.trim();
      if (dnseAccount) out.push({ id: dnseAccount, label: 'DNSE ' + dnseAccount });
      entradeAccounts().forEach((account) => {
        const id = String(account.id || '').toUpperCase();
        if (id) out.push({ id, label: id + ' / Entrade' });
      });
      return out;
    }

    function defaultTradingGroups() {
      const groups = [];
      const dnseAccount = document.getElementById('accountNo').value.trim();
      if (dnseAccount) {
        groups.push({
          id: 'dnse-main', name: 'DNSE chính', enabled: true, accountNos: [dnseAccount],
          defaultQuantity: 1, maxQuantity: 1, orderType: 'MTL', marketType: 'DERIVATIVE', orderCategory: 'NORMAL',
          allowBuy: true, allowSell: true, symbols: []
        });
      }
      const entradeDefaults = Array.from(entradeDefaultSet());
      if (entradeDefaults.length) {
        groups.push({
          id: 'entrade-default', name: 'Entrade mặc định', enabled: true, accountNos: entradeDefaults,
          defaultQuantity: 1, maxQuantity: 1, orderType: 'MTL', marketType: 'DERIVATIVE', orderCategory: 'NORMAL',
          allowBuy: true, allowSell: true, symbols: ['VN30F1M']
        });
      }
      if (!groups.length) {
        groups.push({ id: 'default', name: 'Nhóm mặc định', enabled: true, accountNos: [], defaultQuantity: 1, maxQuantity: 1, orderType: 'MTL', marketType: 'DERIVATIVE', orderCategory: 'NORMAL', allowBuy: true, allowSell: true, symbols: [] });
      }
      return groups;
    }

    function ensureTradingConfig() {
      if (!currentTrading) currentTrading = {};
      if (!Array.isArray(currentTrading.groups) || !currentTrading.groups.length) {
        currentTrading.groups = defaultTradingGroups();
      }
      if (!currentTrading.routes) currentTrading.routes = {};
      const first = currentTrading.groups[0]?.id || '';
      ['dashboard', 'mt5Manual', 'superTrend', 'signalApi', 'orderApi'].forEach((key) => {
        if (!currentTrading.routes[key]) currentTrading.routes[key] = first;
      });
    }

    function renderExecutionGroups() {
      ensureTradingConfig();
      const box = document.getElementById('executionGroupsList');
      if (!box) return;
      const accounts = allOrderAccounts();
      box.innerHTML = currentTrading.groups.map((group, index) => {
        const selected = new Set((group.accountNos || []).map((value) => String(value || '').toUpperCase()));
        const accountChecks = accounts.map((account) => {
          const checked = selected.has(account.id.toUpperCase()) ? 'checked' : '';
          return '<label class="chip"><input type="checkbox" ' + checked + ' onchange="toggleGroupAccount(' + index + ', \'' + escapeHtml(account.id) + '\', this.checked)"><span>' + escapeHtml(account.label) + '</span></label>';
        }).join('');
        return '<div class="account-row">' +
          '<div class="account-head">' +
            '<div><strong>' + escapeHtml(group.name || group.id) + '</strong><div class="account-meta"><code>' + escapeHtml(group.id) + '</code></div></div>' +
            '<div class="inline-actions" style="margin-top:0; justify-content:flex-end;">' +
              '<button class="btn secondary mini-btn" type="button" onclick="editExecutionGroup(' + index + ')">Sửa</button>' +
              '<button class="btn secondary mini-btn" type="button" onclick="removeExecutionGroup(' + index + ')">Xóa</button>' +
            '</div>' +
          '</div>' +
          '<label class="chip" style="margin-top:10px; width:max-content;"><input type="checkbox" ' + (group.enabled === false ? '' : 'checked') + ' onchange="currentTrading.groups[' + index + '].enabled=this.checked"><span>Bật nhóm</span></label>' +
          '<div class="chip-list" style="margin-top:10px;">' + (accountChecks || '<span class="muted">Chưa có tài khoản để chọn.</span>') + '</div>' +
          '<div class="account-meta" style="margin-top:10px;">Loại lệnh: <code>' + escapeHtml(group.orderType || 'MTL') + '</code>, thị trường: <code>' + escapeHtml(group.marketType || 'DERIVATIVE') + '</code>, default qty: <code>' + escapeHtml(group.defaultQuantity || 0) + '</code>, max qty: <code>' + escapeHtml(group.maxQuantity || 0) + '</code>, mã giới hạn: <code>' + escapeHtml((group.symbols || []).join(', ') || 'tất cả') + '</code></div>' +
        '</div>';
      }).join('');
      renderRouteSelectors();
    }

    function renderRouteSelectors() {
      ensureTradingConfig();
      const options = currentTrading.groups.map((group) => '<option value="' + escapeHtml(group.id) + '">' + escapeHtml(group.name || group.id) + '</option>').join('');
      const fields = [
        ['routeDashboard', 'dashboard'],
        ['routeMT5Manual', 'mt5Manual'],
        ['routeSuperTrend', 'superTrend'],
        ['routeSignalApi', 'signalApi'],
        ['routeOrderApi', 'orderApi']
      ];
      fields.forEach(([id, key]) => {
        const select = document.getElementById(id);
        if (!select) return;
        select.innerHTML = options;
        select.value = currentTrading.routes[key] || currentTrading.groups[0]?.id || '';
        select.onchange = () => { currentTrading.routes[key] = select.value; };
      });
    }

    function toggleGroupAccount(index, accountId, checked) {
      const group = currentTrading.groups[index];
      if (!group) return;
      const set = new Set((group.accountNos || []).map((value) => String(value || '').toUpperCase()));
      if (checked) set.add(accountId.toUpperCase());
      else set.delete(accountId.toUpperCase());
      group.accountNos = Array.from(set);
      renderExecutionGroups();
    }

    function addExecutionGroup() {
      ensureTradingConfig();
      const id = window.prompt('Mã nhóm, ví dụ: bot-demo, copy-all');
      if (!id) return;
      const name = window.prompt('Tên nhóm hiển thị', id) || id;
      currentTrading.groups.push({ id: id.trim().toLowerCase().replaceAll('_', '-').replaceAll(' ', '-'), name, enabled: true, accountNos: [], defaultQuantity: 1, maxQuantity: 1, orderType: 'MTL', marketType: 'DERIVATIVE', orderCategory: 'NORMAL', allowBuy: true, allowSell: true, symbols: [] });
      renderExecutionGroups();
    }

    function editExecutionGroup(index) {
      const group = currentTrading.groups[index];
      if (!group) return;
      const name = window.prompt('Tên nhóm', group.name || group.id);
      if (name === null) return;
      const defaultQuantity = window.prompt('Khối lượng mặc định khi bot/MT5 không truyền quantity. 0 = bắt buộc bot truyền.', group.defaultQuantity || 0);
      if (defaultQuantity === null) return;
      const maxQuantity = window.prompt('Khối lượng tối đa mỗi lệnh. 0 = không giới hạn riêng.', group.maxQuantity || 0);
      if (maxQuantity === null) return;
      const orderType = window.prompt('Loại lệnh mặc định', group.orderType || 'MTL');
      if (orderType === null) return;
      const symbols = window.prompt('Mã được phép, phân tách bằng dấu phẩy. Để trống = tất cả.', (group.symbols || []).join(','));
      if (symbols === null) return;
      group.name = name.trim() || group.id;
      group.defaultQuantity = Number(defaultQuantity || 0);
      group.maxQuantity = Number(maxQuantity || 0);
      group.orderType = (orderType || 'MTL').trim().toUpperCase();
      group.marketType = (group.marketType || 'DERIVATIVE').toUpperCase();
      group.orderCategory = (group.orderCategory || 'NORMAL').toUpperCase();
      group.symbols = symbols.split(',').map((value) => value.trim().toUpperCase()).filter(Boolean);
      renderExecutionGroups();
    }

    function removeExecutionGroup(index) {
      if (currentTrading.groups.length <= 1) {
        window.alert('Cần giữ ít nhất một nhóm đặt lệnh.');
        return;
      }
      const group = currentTrading.groups[index];
      if (!group || !window.confirm('Xóa nhóm ' + group.name + '?')) return;
      currentTrading.groups.splice(index, 1);
      const fallback = currentTrading.groups[0]?.id || '';
      Object.keys(currentTrading.routes || {}).forEach((key) => {
        if (currentTrading.routes[key] === group.id) currentTrading.routes[key] = fallback;
      });
      renderExecutionGroups();
    }

    function editEntradeAccount(index) {
      const account = entradeAccounts()[index];
      if (!account) return;
      const accountNo = window.prompt('Master account / investorAccountId', account.accountNo || '');
      if (accountNo === null) return;
      const loanPackageId = window.prompt('Loan package ID / bankMarginPortfolioId. Để trống nếu muốn tự lấy gói đầu tiên.', account.loanPackageId || '');
      if (loanPackageId === null) return;
      account.accountNo = accountNo.trim();
      account.loanPackageId = Number(loanPackageId || 0);
      renderEntradeAccounts();
      renderExecutionGroups();
    }

    function deleteEntradeAccount(index) {
      const account = entradeAccounts()[index];
      if (!account) return;
      if (!window.confirm('Xóa tài khoản ' + account.id + ' khỏi danh sách liên kết?')) return;
      currentEntrade.accounts.splice(index, 1);
      const removedID = String(account.id || '').toUpperCase();
      currentEntrade.defaultAccountNos = ((currentEntrade.defaultAccountNos || []).filter((id) => String(id || '').toUpperCase() !== removedID));
      if (!currentEntrade.accounts.length) currentEntrade.enabled = false;
      renderEntradeStatus();
      renderEntradeAccounts();
      renderExecutionGroups();
    }

    function toggleEntradeDefault(index, checked) {
      const account = entradeAccounts()[index];
      if (!account) return;
      const id = String(account.id || '').toUpperCase();
      const defaults = entradeDefaultSet();
      if (checked) defaults.add(id);
      else defaults.delete(id);
      currentEntrade.defaultAccountNos = Array.from(defaults);
      renderEntradeAccounts();
      renderExecutionGroups();
    }

    async function linkEntradeAccount() {
      const result = document.getElementById('entradeLinkResult');
      const username = document.getElementById('entradeUsername').value.trim();
      const password = document.getElementById('entradePassword').value.trim();
      if (!username || !password) {
        result.textContent = 'Hãy nhập username và password Entrade.';
        return;
      }
      result.textContent = 'Đang liên kết Entrade...';
      try {
        const res = await fetch('/api/entrade/link', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ username, password, environment: 'real' })
        });
        const data = await res.json();
        if (!res.ok || !data.success) {
          result.textContent = data.error || 'Không thể liên kết Entrade.';
          return;
        }
        const account = data.account || {};
        const packages = account.loanPackages || [];
        result.textContent = 'Đã liên kết thành công. Trạng thái: ' + (account.status || 'UNKNOWN') + '. Gói vay: ' + packages.length + '.';
        result.textContent = 'Đã liên kết thành công Real/Demo. Tổng profile Entrade: ' + ((data.accounts || []).length || 1) + '. Loan package ID Real: ' + ((packages[0] || {}).id || 'tự lấy') + '.';
        document.getElementById('entradePassword').value = '';
        await loadSettings();
      } catch (e) {
        result.textContent = 'Không thể liên kết Entrade.';
      }
    }

    async function loadSettings() {
      const res = await fetch('/api/settings');
      const data = await res.json();
      document.getElementById('apiKey').value = data.dnse.apiKey || '';
      document.getElementById('accountNo').value = data.dnse.accountNo || '';
      document.getElementById('mockMode').value = data.dnse.mock ? 'true' : 'false';
      currentEntrade = data.entrade || {};
      currentTrading = data.trading || { groups: [], routes: {} };
      document.getElementById('tradingProvider').value = currentEntrade.enabled ? 'entrade' : 'dnse';
      renderEntradeStatus();
      renderEntradeAccounts();
      renderTradingProviderHint();
      renderExecutionGroups();
      loadDNSEAccounts();

      const selectedSymbols = (data.marketData && data.marketData.symbols && data.marketData.symbols.length)
        ? data.marketData.symbols
        : defaultSymbols;

      mergeSymbols(selectedSymbols);
      renderSymbolChoices(selectedSymbols);
      document.getElementById('primarySymbol').value = (data.marketData && data.marketData.symbol) || selectedSymbols[0] || '';
      syncPrimaryOptions();
    }

    async function saveSettings() {
      const selectedSymbols = getSelectedSymbols();
      const saveRes = document.getElementById('save-res');
      if (!selectedSymbols.length) {
        saveRes.textContent = 'Hãy chọn ít nhất một mã để theo dõi.';
        return;
      }

      const body = {
        apiKey: document.getElementById('apiKey').value,
        apiSecret: document.getElementById('apiSecret').value,
        accountNo: document.getElementById('accountNo').value,
        mock: document.getElementById('mockMode').value === 'true',
        symbols: selectedSymbols,
        primarySymbol: document.getElementById('primarySymbol').value,
        entradeEnabled: document.getElementById('tradingProvider').value === 'entrade' && entradeAccounts().length > 0,
        entradeMock: currentEntrade.mock === true,
        entradeEnvironment: currentEntrade.environment || 'real',
        entradeDefaultAccountNos: currentEntrade.defaultAccountNos || [],
        entradeAccounts: currentEntrade.accounts || [],
        tradingGroups: currentTrading.groups || [],
        tradingRoutes: currentTrading.routes || {}
      };

      const res = await fetch('/api/settings', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body)
      });

      if (res.ok) {
        saveRes.textContent = 'Đã lưu thành công. Hãy khởi động lại bridge để áp dụng toàn bộ danh sách mã và mã chính.';
      } else {
        const data = await res.json();
        saveRes.textContent = data.error || 'Không thể lưu cấu hình.';
      }
    }

    function buildSettingsPayload() {
      const selectedSymbols = getSelectedSymbols();
      if (!selectedSymbols.length) return null;
      return {
        apiKey: document.getElementById('apiKey').value,
        apiSecret: document.getElementById('apiSecret').value,
        accountNo: document.getElementById('accountNo').value,
        mock: document.getElementById('mockMode').value === 'true',
        symbols: selectedSymbols,
        primarySymbol: document.getElementById('primarySymbol').value,
        entradeEnabled: document.getElementById('tradingProvider').value === 'entrade' && entradeAccounts().length > 0,
        entradeMock: currentEntrade.mock === true,
        entradeEnvironment: currentEntrade.environment || 'real',
        entradeDefaultAccountNos: currentEntrade.defaultAccountNos || [],
        entradeAccounts: currentEntrade.accounts || [],
        tradingGroups: currentTrading.groups || [],
        tradingRoutes: currentTrading.routes || {}
      };
    }

    async function persistSettings(message) {
      const saveRes = document.getElementById('save-res');
      const body = buildSettingsPayload();
      if (!body) {
        if (saveRes) saveRes.textContent = 'Hãy chọn ít nhất một mã để theo dõi.';
        return false;
      }
      const res = await fetch('/api/settings', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body)
      });
      if (res.ok) {
        if (saveRes) saveRes.textContent = message || 'Đã lưu thành công. Hãy khởi động lại bridge để áp dụng toàn bộ cấu hình.';
        return true;
      }
      const data = await res.json();
      if (saveRes) saveRes.textContent = data.error || 'Không thể lưu cấu hình.';
      return false;
    }

    async function saveSettings() {
      await persistSettings();
    }

    async function editEntradeAccount(index) {
      const account = entradeAccounts()[index];
      if (!account) return;
      const accountNo = window.prompt('Master account / investorAccountId', account.accountNo || '');
      if (accountNo === null) return;
      const loanPackageId = window.prompt('Loan package ID / bankMarginPortfolioId. Để trống nếu muốn tự lấy gói đầu tiên.', account.loanPackageId || '');
      if (loanPackageId === null) return;
      account.accountNo = accountNo.trim();
      account.loanPackageId = Number(loanPackageId || 0);
      renderEntradeAccounts();
      await persistSettings('Đã lưu thay đổi tài khoản Entrade.');
    }

    async function deleteEntradeAccount(index) {
      const account = entradeAccounts()[index];
      if (!account) return;
      if (!window.confirm('Xóa tài khoản ' + account.id + ' khỏi danh sách liên kết?')) return;
      currentEntrade.accounts.splice(index, 1);
      const removedID = String(account.id || '').toUpperCase();
      currentEntrade.defaultAccountNos = ((currentEntrade.defaultAccountNos || []).filter((id) => String(id || '').toUpperCase() !== removedID));
      if (!currentEntrade.accounts.length) currentEntrade.enabled = false;
      renderEntradeStatus();
      renderEntradeAccounts();
      renderTradingProviderHint();
      renderExecutionGroups();
      await persistSettings('Đã xóa tài khoản Entrade khỏi cấu hình.');
    }

    async function toggleEntradeDefault(index, checked) {
      const account = entradeAccounts()[index];
      if (!account) return;
      const id = String(account.id || '').toUpperCase();
      const defaults = entradeDefaultSet();
      if (checked) defaults.add(id);
      else defaults.delete(id);
      currentEntrade.defaultAccountNos = Array.from(defaults);
      renderEntradeAccounts();
      renderTradingProviderHint();
      renderExecutionGroups();
      await persistSettings('Đã cập nhật nhóm đặt lệnh Entrade.');
    }

    loadProfiles().then(loadSettings);
  </script>
` + layoutBottom

const logsHTML = layoutTop + `
  <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:20px;">
    <h1 style="margin:0">Nhật ký hệ thống</h1>
    <button class="btn secondary" onclick="loadLogs()">Tải lại nhật ký</button>
  </div>
  <p style="margin-top:0; color: var(--text-muted)">Trang này chỉ hiển thị phần nhật ký gần nhất để hệ thống luôn nhẹ. Nhật ký cũ sẽ được tự động tách sang thư mục <code>logs/archive</code>.</p>
  <div class="card" style="padding:0; overflow:hidden;">
    <pre id="log-viewer" style="margin:0; padding:20px; height:600px; overflow-y:auto; background:#000; color:#00ff99; font-family:Consolas, monospace; font-size:12px;"></pre>
  </div>

  <script>
    async function loadLogs() {
      const out = document.getElementById('log-viewer');
      out.innerText = 'Đang tải...';
      try {
        const res = await fetch('/api/logs/raw');
        const text = await res.text();
        out.innerText = text;
        out.scrollTop = out.scrollHeight;
      } catch (e) {
        out.innerText = 'Không thể tải nhật ký.';
      }
    }
    loadLogs();
    setInterval(loadLogs, 5000);
  </script>
` + layoutBottom

const systemStatusHTML = layoutTop + `
  <h1>Trạng thái hệ thống</h1>
  <div class="grid-2" id="status-grid">
    Đang tải...
  </div>

  <script>
    async function loadStatus() {
      const res = await fetch('/status');
      const data = await res.json();
      const grid = document.getElementById('status-grid');
      grid.innerHTML = '';

      const items = [
        { name: 'Go Bridge API', ok: data.api_ok },
        { name: 'Xác thực DNSE', ok: data.token_valid },
        { name: 'TCP Market Data (9090)', ok: data.market_data_ok },
        { name: 'EA / DLL MT5', ok: data.mt5_connected, text: data.mt5_connected ? 'Đã kết nối' : 'Đang chờ EA', statusClass: data.mt5_connected ? 'status-ok' : 'status-warn' },
        { name: 'Gmail Auto OTP', ok: data.gmail_ok },
        { name: 'Kill Switch', ok: data.system_enabled, text: data.system_enabled ? 'Đang hoạt động' : 'Đang tắt khẩn cấp', statusClass: data.system_enabled ? 'status-ok' : 'status-err' }
      ];

      items.forEach((item) => {
        const card = document.createElement('div');
        card.className = 'card';
        card.style.marginBottom = '0';
        card.innerHTML =
          '<h3 style="margin:0 0 10px; color: var(--text-muted); font-size:14px;">' + item.name + '</h3>' +
          '<div style="font-size:18px; font-weight:700; display:flex; align-items:center;">' +
            '<span class="status-indicator ' + (item.statusClass || (item.ok ? 'status-ok' : 'status-err')) + '"></span>' +
            (item.text || (item.ok ? 'Đang hoạt động' : 'Lỗi / chưa tìm thấy')) +
          '</div>';
        grid.appendChild(card);
      });
    }
    loadStatus();
    setInterval(loadStatus, 15000);
  </script>
` + layoutBottom
