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
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>DNSE MT5 Connector</title>
  <style>` + layoutCSS + `</style>
</head>
<body>
  <nav>
    <div class="brand">DNSE Connector</div>
    <a href="/" id="nav-dash">Dashboard</a>
    <a href="/setup" id="nav-setup">Setup Wizard</a>
    <a href="/status" id="nav-status">System Status</a>
    <a href="/settings" id="nav-settings">Settings</a>
    <a href="/logs" id="nav-logs">Logs</a>
    <div style="margin-top: auto; padding: 20px;">
      <button class="btn secondary" style="width: 100%" onclick="window.location.href='/support/export'">Export Support Zip</button>
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
  <h1>Setup Wizard</h1>
  <p style="color: var(--text-muted)">Welcome to the DNSE MT5 Connector. This wizard will help you configure everything.</p>
  
  <div class="card">
    <div class="step done">
      <h3>Step 1: System Check</h3>
      <div id="sys-check-res">Checking...</div>
    </div>
    
    <div class="step" id="step-2">
      <h3>Step 2: MT5 Installation</h3>
      <p>We need to install the bridging DLL and MQL5 EA into your MetaTrader 5 data folder.</p>
      <button class="btn" onclick="detectMT5()">Auto-Detect & Install</button>
      <pre id="mt5-res" style="background: var(--bg); padding: 10px; border-radius: 4px; margin-top: 10px; display: none;"></pre>
    </div>

    <div class="step" id="step-3">
      <h3>Step 3: Gmail OTP</h3>
      <p>For fully automated trading without entering OTPs manually, authorize Gmail access.</p>
      <button class="btn secondary" onclick="checkGmail()">Check Gmail Status</button>
      <p id="gmail-res" style="color: var(--text-muted);"></p>
    </div>

    <div class="step" id="step-4">
      <h3>Step 4: DNSE API Test</h3>
      <button class="btn secondary" onclick="testDNSE()">Test Connection</button>
      <pre id="dnse-res" style="background: var(--bg); padding: 10px; border-radius: 4px; margin-top: 10px; display: none;"></pre>
    </div>

    <div class="step" id="step-5" style="border-left-color: transparent;">
      <h3>Step 5: Completion</h3>
      <p>Once everything is green, head to the <a href="/" style="color: var(--primary)">Dashboard</a> to monitor your system.</p>
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
      out.innerText = 'Installing files...';
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
        el.innerHTML = '<span class="status-indicator status-ok"></span> Gmail Authorized';
        document.getElementById('step-3').classList.add('done');
      } else {
        el.innerHTML = '<span class="status-indicator status-err"></span> Gmail not authorized. Check terminal logs for auth URL.';
      }
    }

    async function testDNSE() {
      const out = document.getElementById('dnse-res');
      out.style.display = 'block';
      out.innerText = 'Testing...';
      const {ok, data} = await r('/account');
      if (ok && !data.error) {
        out.innerText = "Connection successful!\nAccount Info loaded.";
        document.getElementById('step-4').classList.add('done');
        document.getElementById('step-5').classList.add('done');
      } else {
        out.innerText = "Error: " + JSON.stringify(data, null, 2);
      }
    }

    // Run sys check
    r('/status').then(({ok, data}) => {
      let html = '<ul style="margin:0; padding-left:20px; color: var(--text-muted)">';
      html += '<li>Go Bridge: ' + (ok ? '<span style="color:var(--success)">OK</span>' : 'Error') + '</li>';
      html += '<li>TCP Port 9090: ' + (data.market_data_ok ? '<span style="color:var(--success)">OK</span>' : 'Error') + '</li>';
      html += '</ul>';
      document.getElementById('sys-check-res').innerHTML = html;
    });
  </script>
` + layoutBottom

const settingsHTML = layoutTop + `
  <h1>Settings</h1>
  <div class="card">
    <div class="form-group">
      <label>DNSE API Key</label>
      <input type="text" id="apiKey" placeholder="Leave empty to keep unchanged">
    </div>
    <div class="form-group">
      <label>DNSE API Secret</label>
      <input type="password" id="apiSecret" placeholder="*** MASKED *** (Leave empty to keep unchanged)">
    </div>
    <div class="form-group">
      <label>DNSE Account No</label>
      <input type="text" id="accountNo">
    </div>
    <div class="form-group">
      <label>Mock Mode</label>
      <select id="mockMode">
        <option value="true">True (Offline Testing)</option>
        <option value="false">False (Live API)</option>
      </select>
    </div>
    <button class="btn" onclick="saveSettings()">Save Configuration</button>
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
        document.getElementById('save-res').innerText = 'Saved successfully! Please restart the Go Bridge to apply.';
        setTimeout(() => document.getElementById('save-res').innerText = '', 5000);
      }
    }
    
    loadSettings();
  </script>
` + layoutBottom

const logsHTML = layoutTop + `
  <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px;">
    <h1 style="margin:0">System Logs</h1>
    <button class="btn secondary" onclick="loadLogs()">Refresh Logs</button>
  </div>
  <div class="card" style="padding: 0; overflow: hidden;">
    <pre id="log-viewer" style="margin: 0; padding: 20px; height: 600px; overflow-y: auto; background: #000; color: #00ff00; font-family: monospace; font-size: 12px;"></pre>
  </div>

  <script>
    async function loadLogs() {
      const out = document.getElementById('log-viewer');
      out.innerText = 'Loading...';
      try {
        const res = await fetch('/api/logs/raw');
        const text = await res.text();
        out.innerText = text;
        out.scrollTop = out.scrollHeight;
      } catch(e) {
        out.innerText = 'Failed to load logs.';
      }
    }
    loadLogs();
    setInterval(loadLogs, 5000);
  </script>
` + layoutBottom

const systemStatusHTML = layoutTop + `
  <h1>System Status</h1>
  <div class="grid-2" id="status-grid">
    Loading...
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
        { name: 'MT5 Connection', ok: data.mt5_connected },
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
            '<span class="status-indicator ' + (item.ok ? 'status-ok' : 'status-err') + '"></span>' +
            (item.ok ? 'Operational' : 'Error / Not Found') +
          '</div>';
        grid.appendChild(card);
      });
    }
    loadStatus();
    setInterval(loadStatus, 15000);
  </script>
` + layoutBottom
