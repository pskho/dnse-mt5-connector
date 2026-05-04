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
  font-family: 'Segoe UI', Arial, sans-serif;
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
  font-weight: bold;
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
  background: rgba(211, 47, 47, 0.1);
  color: var(--primary);
  border-right: 3px solid var(--primary);
}
main {
  flex: 1;
  padding: 40px;
  max-width: 1000px;
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
  gap: 8px;
}
.btn:hover { background: var(--primary-hover); }
.btn.secondary {
  background: transparent;
  border: 1px solid var(--border);
  color: var(--text);
}
.btn.secondary:hover { background: var(--surface-hover); }
.form-group {
  margin-bottom: 16px;
}
label {
  display: block;
  margin-bottom: 6px;
  color: var(--text-muted);
  font-size: 14px;
}
input, select {
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
.grid-2 { display: grid; grid-template-columns: 1fr 1fr; gap: 20px; }
.step {
  border-left: 2px solid var(--border);
  padding-left: 20px;
  margin-left: 10px;
  padding-bottom: 30px;
  position: relative;
}
.step::before {
  content: '';
  position: absolute;
  left: -11px;
  top: 0;
  width: 20px;
  height: 20px;
  background: var(--surface);
  border: 2px solid var(--primary);
  border-radius: 50%;
}
}
.step.done::before { background: var(--success); border-color: var(--success); }
pre { white-space: pre-wrap; word-wrap: break-word; }
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
    <div style="margin-top: auto; padding: 20px;">
      <button class="btn secondary" style="width: 100%" onclick="window.location.href='/support/export'">Xuất gói hỗ trợ</button>
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
  <p style="color: var(--text-muted)">Thiết lập nhanh để DNSE MT5 Connector có thể kết nối, cài vào MT5 và tự nạp dữ liệu lịch sử nền cho lần chạy đầu tiên.</p>
  
  <div class="card">
    <div class="step done">
      <h3>Bước 1: Kiểm tra hệ thống</h3>
      <div id="sys-check-res">Đang kiểm tra...</div>
    </div>
    
    <div class="step" id="step-2">
      <h3>Bước 2: Cài vào MT5</h3>
      <p>Hệ thống sẽ tự chép DLL và Expert Advisor vào thư mục dữ liệu MetaTrader 5.</p>
      <button class="btn" onclick="detectMT5()">Tự dò và cài đặt</button>
      <pre id="mt5-res" style="background: var(--bg); padding: 10px; border-radius: 4px; margin-top: 10px; display: none;"></pre>
    </div>

    <div class="step" id="step-3">
      <h3>Bước 3: OTP tự động</h3>
      <p>Nếu muốn nhận OTP tự động từ email, kiểm tra trạng thái Gmail tại đây.</p>
      <button class="btn secondary" onclick="checkGmail()">Kiểm tra Gmail</button>
      <p id="gmail-res" style="color: var(--text-muted);"></p>
    </div>

    <div class="step" id="step-4">
      <h3>Bước 4: Kiểm tra kết nối DNSE</h3>
      <button class="btn secondary" onclick="testDNSE()">Kiểm tra kết nối</button>
      <pre id="dnse-res" style="background: var(--bg); padding: 10px; border-radius: 4px; margin-top: 10px; display: none;"></pre>
    </div>

    <div class="step" id="step-5" style="border-left-color: transparent;">
      <h3>Bước 5: Hoàn tất</h3>
      <p>Sau khi cấu hình xong, hệ thống sẽ tự đồng bộ dữ liệu lịch sử nền cho mã chính ở lần chạy đầu. Sau đó anh chỉ cần vào <a href="/" style="color: var(--primary)">Bảng điều khiển</a> để theo dõi.</p>
    </div>
  </div>

  <script>
    async function r(url, opts) {
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
      const {ok, data} = await r('/api/setup/install', {method: 'POST'});
      if (data.success) {
        out.innerText = data.logs.join('\n');
        document.getElementById('step-2').classList.add('done');
      } else {
        out.innerText = data.message + '\n' + (data.logs ? data.logs.join('\n') : '');
      }
    }

    async function checkGmail() {
      const {ok, data} = await r('/status');
      const el = document.getElementById('gmail-res');
      if(data.gmail_ok) {
        el.innerHTML = '<span class="status-indicator status-ok"></span> Gmail đã sẵn sàng';
        document.getElementById('step-3').classList.add('done');
      } else {
        el.innerHTML = '<span class="status-indicator status-err"></span> Gmail chưa được xác thực. Hãy xem log terminal để lấy liên kết xác thực.';
      }
    }

    async function testDNSE() {
      const out = document.getElementById('dnse-res');
      out.style.display = 'block';
      out.innerText = 'Đang kiểm tra...';
      const {ok, data} = await r('/account');
      if (ok && !data.error) {
        out.innerText = "Kết nối thành công.\nĐã tải thông tin tài khoản.";
        document.getElementById('step-4').classList.add('done');
        document.getElementById('step-5').classList.add('done');
      } else {
        out.innerText = "Lỗi: " + JSON.stringify(data, null, 2);
      }
    }

    // Run sys check
    r('/status').then(({ok, data}) => {
      let html = '<ul style="margin:0; padding-left:20px; color: var(--text-muted)">';
      html += '<li>Go Bridge: ' + (ok ? '<span style="color:var(--success)">OK</span>' : 'Lỗi') + '</li>';
      html += '<li>Cổng TCP 9090: ' + (data.market_data_ok ? '<span style="color:var(--success)">OK</span>' : 'Lỗi') + '</li>';
      html += '</ul>';
      document.getElementById('sys-check-res').innerHTML = html;
    });
  </script>
` + layoutBottom

const settingsHTML = layoutTop + `
  <h1>Cấu hình</h1>
  <div class="card">
    <div class="form-group">
      <label>Khóa API DNSE</label>
      <input type="text" id="apiKey" placeholder="Để trống nếu không muốn thay đổi">
    </div>
    <div class="form-group">
      <label>Mã bí mật API DNSE</label>
      <input type="password" id="apiSecret" placeholder="*** ĐÃ ẨN *** - để trống nếu không muốn thay đổi">
    </div>
    <div class="form-group">
      <label>Số tài khoản DNSE</label>
      <input type="text" id="accountNo">
    </div>
    <div class="form-group">
      <label>Chế độ mô phỏng</label>
      <select id="mockMode">
        <option value="true">Bật (kiểm thử offline)</option>
        <option value="false">Tắt (API thật)</option>
      </select>
    </div>
    <button class="btn" onclick="saveSettings()">Lưu cấu hình</button>
    <span id="save-res" style="margin-left: 15px; color: var(--success);"></span>
  </div>

  <script>
    async function loadSettings() {
      const res = await fetch('/api/settings');
      const data = await res.json();
      document.getElementById('apiKey').value = data.dnse.apiKey || '';
      document.getElementById('accountNo').value = data.dnse.accountNo || '';
      document.getElementById('mockMode').value = data.dnse.mock ? 'true' : 'false';
    }
    
    async function saveSettings() {
      const body = {
        apiKey: document.getElementById('apiKey').value,
        apiSecret: document.getElementById('apiSecret').value,
        accountNo: document.getElementById('accountNo').value,
        mock: document.getElementById('mockMode').value === 'true'
      };
      
      const res = await fetch('/api/settings', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(body)
      });
      
      if(res.ok) {
        document.getElementById('save-res').innerText = 'Đã lưu thành công. Vui lòng khởi động lại bridge để áp dụng.';
        setTimeout(() => document.getElementById('save-res').innerText = '', 5000);
      }
    }
    
    loadSettings();
  </script>
` + layoutBottom

const logsHTML = layoutTop + `
  <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px;">
    <h1 style="margin:0">Nhật ký hệ thống</h1>
    <button class="btn secondary" onclick="loadLogs()">Tải lại nhật ký</button>
  </div>
  <p style="margin-top:0; color: var(--text-muted)">Trang này chỉ hiển thị phần nhật ký gần nhất để hệ thống luôn nhẹ. Nhật ký cũ sẽ được tự động tách sang thư mục <code>logs/archive</code>.</p>
  <div class="card" style="padding: 0; overflow: hidden;">
    <pre id="log-viewer" style="margin: 0; padding: 20px; height: 600px; overflow-y: auto; background: #000; color: #00ff00; font-family: monospace; font-size: 12px;"></pre>
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
      } catch(e) {
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
        { name: 'DNSE Authentication', ok: data.token_valid },
        { name: 'TCP Market Data (9090)', ok: data.market_data_ok },
        { name: 'EA / DLL MT5', ok: data.mt5_connected, text: data.mt5_connected ? 'Đã kết nối' : 'Đang chờ EA', statusClass: data.mt5_connected ? 'status-ok' : 'status-warn' },
        { name: 'Gmail Auto OTP', ok: data.gmail_ok },
        { name: 'Trading Active (Kill Switch)', ok: data.system_enabled }
      ];
      
      items.forEach(item => {
        const card = document.createElement('div');
        card.className = 'card';
        card.style.marginBottom = '0';
        card.innerHTML = 
          '<h3 style="margin: 0 0 10px; color: var(--text-muted); font-size: 14px;">' + item.name + '</h3>' +
          '<div style="font-size: 18px; font-weight: bold; display: flex; align-items: center;">' +
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
