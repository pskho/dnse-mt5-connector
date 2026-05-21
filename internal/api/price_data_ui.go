package api

const priceDataHTML = layoutTop + `
  <style>
    .price-shell {
      display: grid;
      grid-template-columns: minmax(0, 1fr) 360px;
      gap: 24px;
      align-items: start;
    }
    .price-main { min-width: 0; }
    .price-side { position: sticky; top: 24px; }
    .price-toolbar {
      display: flex;
      justify-content: space-between;
      align-items: flex-start;
      gap: 16px;
      margin-bottom: 24px;
    }
    .actions {
      display: flex;
      flex-wrap: wrap;
      gap: 12px;
      margin-top: 18px;
    }
    .inline-note {
      margin-top: 10px;
      font-size: 12px;
      color: var(--text-muted);
      line-height: 1.6;
    }
    .symbol-panel {
      max-height: 360px;
      overflow: auto;
      align-content: flex-start;
      padding-right: 6px;
    }
    .selected-strip {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
      max-height: 170px;
      overflow: auto;
      padding: 12px;
      border: 1px solid var(--border);
      border-radius: 8px;
      background: #171717;
    }
    .mini-chip {
      border: 1px solid var(--border);
      border-radius: 999px;
      padding: 6px 10px;
      color: var(--text);
      background: #111;
      font-size: 13px;
    }
    .console {
      background: #0c0c0c;
      border: 1px solid var(--border);
      border-radius: 8px;
      padding: 16px;
      height: min(68vh, 720px);
      overflow: auto;
      font-family: Consolas, Monaco, monospace;
      font-size: 13px;
      line-height: 1.5;
      color: #87f59f;
      white-space: pre-wrap;
    }
    .console.error-text { color: #ff9b9b; }
    @media (max-width: 1100px) {
      .price-shell { grid-template-columns: 1fr; }
      .price-side { position: static; }
      .console { height: 320px; }
    }
  </style>

  <div class="price-toolbar">
    <div>
      <h1 style="margin-bottom:8px">Dữ liệu giá</h1>
      <p class="muted" style="margin:0">Quản lý danh sách mã và đồng bộ giá lịch sử để MT5 có đủ dữ liệu khi mở giữa phiên.</p>
    </div>
    <button class="btn secondary" type="button" onclick="loadSettings()">Tải lại</button>
  </div>

  <div class="price-shell">
    <div class="price-main">
      <section class="card">
        <h2>Đồng bộ giá lịch sử</h2>
        <div class="grid-2">
          <div class="form-group">
            <label>Phạm vi đồng bộ</label>
            <select id="historyScope" onchange="renderSelectedSymbols()">
              <option value="single">Một mã</option>
              <option value="all">Tất cả mã đã chọn</option>
            </select>
          </div>
          <div class="form-group">
            <label>Mã mặc định</label>
            <input id="historySymbol" value="VN30F1M">
          </div>
          <div class="form-group">
            <label>Loại thị trường</label>
            <select id="historyMarketType">
              <option value="DERIVATIVE">DERIVATIVE</option>
              <option value="STOCK">STOCK</option>
              <option value="INDEX">INDEX</option>
            </select>
          </div>
          <div class="form-group">
            <label>Khung dữ liệu</label>
            <select id="historyResolution">
              <option value="1">M1</option>
              <option value="5">M5</option>
              <option value="15">M15</option>
              <option value="30">M30</option>
              <option value="60">H1</option>
            </select>
          </div>
          <div class="form-group">
            <label>Số ngày lấy lùi</label>
            <input id="historyLookbackDays" type="number" value="365" min="1" max="3650">
          </div>
          <div class="form-group">
            <label>Trạng thái danh sách</label>
            <div id="selectedSummary" class="inline-note">Đang tải danh sách mã...</div>
          </div>
        </div>
        <div class="actions">
          <button class="btn" type="button" onclick="syncHistoricalPrices()">Đồng bộ giá lịch sử</button>
          <button class="btn secondary" type="button" onclick="syncMaximumHistoricalPrices()">Đồng bộ tối đa có thể</button>
          <button class="btn secondary" type="button" onclick="syncTodayPrices()">Vá riêng hôm nay</button>
          <button class="btn secondary" type="button" onclick="saveSymbols()">Lưu danh sách mã</button>
        </div>
        <div class="inline-note">Đồng bộ giá lịch sử sẽ lấy dữ liệu theo số ngày đã nhập. Nút tối đa có thể chạy ngầm cho một mã, đi lùi từng lớp nhỏ cho tới khi API không còn dữ liệu cũ hơn. Nếu MT5 đang mở, bridge sẽ nhận snapshot mới và import vào custom symbol.</div>
      </section>

      <section class="card">
        <h2>Danh sách mã</h2>
        <div class="grid-2">
          <div class="form-group">
            <label>Mã chính</label>
            <select id="primarySymbol"></select>
          </div>
          <div class="form-group">
            <label>Tìm nhanh mã</label>
            <input type="text" id="symbolSearch" placeholder="Ví dụ: HPG, FPT, VN30..." oninput="renderSymbolChoices()">
          </div>
        </div>
        <div class="actions" style="margin-top:0">
          <button class="btn secondary" type="button" onclick="loadInstrumentSymbols()">Tải toàn bộ danh sách mã từ DNSE</button>
          <button class="btn secondary" type="button" onclick="toggleExchange('HOSE')">HOSE</button>
          <button class="btn secondary" type="button" onclick="toggleExchange('HNX')">HNX</button>
          <button class="btn secondary" type="button" onclick="toggleExchange('UPCOM')">UPCOM</button>
          <button class="btn secondary" type="button" onclick="toggleExchange('INDEX')">Chỉ số</button>
          <button class="btn secondary" type="button" onclick="toggleExchange('DERIVATIVE')">Phái sinh</button>
        </div>
        <p id="instrumentLoadResult" class="inline-note">Danh sách mã đang chọn sẽ được dùng cho bridge, MT5 và nút đồng bộ tất cả.</p>
        <div id="symbolChoices" class="chip-list symbol-panel"></div>
      </section>
    </div>

    <aside class="price-side">
      <section class="card">
        <h2>Các mã đã chọn</h2>
        <div id="selectedSymbols" class="selected-strip">Đang tải...</div>
      </section>
      <section class="card">
        <div style="display:flex; justify-content:space-between; align-items:center; gap:12px; margin-bottom:12px;">
          <h2 style="margin:0">Kết quả</h2>
          <button class="btn secondary" type="button" onclick="clearOutput()">Xóa</button>
        </div>
        <div id="output" class="console">Sẵn sàng đồng bộ dữ liệu giá.</div>
      </section>
    </aside>
  </div>

  <script>
    const $ = (id) => document.getElementById(id);
    const defaultSymbols = ['VN30F1M'];
    let settings = null;
    let availableSymbols = [...defaultSymbols];
    let selectedSymbols = new Set(defaultSymbols);
    let selectedExchanges = new Set(['HOSE', 'HNX', 'UPCOM', 'INDEX', 'DERIVATIVE']);

    function printOutput(data, isError = false) {
      const out = $('output');
      out.className = isError ? 'console error-text' : 'console';
      out.textContent = typeof data === 'string' ? data : JSON.stringify(data, null, 2);
    }

    function clearOutput() {
      $('output').textContent = '';
    }

    async function requestJSON(path, options = {}) {
      const res = await fetch(path, options);
      const text = await res.text();
      let body = {};
      if (text.trim()) {
        try { body = JSON.parse(text); }
        catch { body = { success: false, error: text }; }
      }
      if (!res.ok) {
        printOutput(body.error || body, true);
        throw body;
      }
      printOutput(body, false);
      return body;
    }

    function mergeSymbols(symbols) {
      const seen = new Set(availableSymbols.map((value) => value.toUpperCase()));
      (symbols || []).forEach((symbol) => {
        symbol = String(symbol || '').trim().toUpperCase();
        if (symbol && !seen.has(symbol)) {
          seen.add(symbol);
          availableSymbols.push(symbol);
        }
      });
      availableSymbols.sort();
    }

    async function loadProfiles() {
      try {
        const data = await requestJSON('/symbols/profiles');
        mergeSymbols((data.profiles || []).map((profile) => profile.Symbol || profile.symbol || ''));
      } catch {}
    }

    async function loadSettings() {
      const data = await requestJSON('/api/settings');
      settings = data;
      const configured = (data.marketData && data.marketData.symbols && data.marketData.symbols.length) ? data.marketData.symbols : defaultSymbols;
      selectedSymbols = new Set(configured.map((value) => String(value || '').trim().toUpperCase()).filter(Boolean));
      mergeSymbols(configured);
      $('historySymbol').value = (data.marketData && data.marketData.symbol) || 'VN30F1M';
      renderPrimaryOptions((data.marketData && data.marketData.symbol) || configured[0] || 'VN30F1M');
      renderSymbolChoices();
      renderSelectedSymbols();
    }

    function renderPrimaryOptions(selected) {
      const select = $('primarySymbol');
      const symbols = Array.from(new Set([...selectedSymbols, ...availableSymbols])).sort();
      select.innerHTML = symbols.map((symbol) => '<option value="' + symbol + '">' + symbol + '</option>').join('');
      select.value = selected || symbols[0] || 'VN30F1M';
      select.onchange = () => { $('historySymbol').value = select.value || 'VN30F1M'; };
    }

    function renderSymbolChoices() {
      const box = $('symbolChoices');
      const keyword = ($('symbolSearch').value || '').trim().toUpperCase();
      const visible = availableSymbols.filter((symbol) => !keyword || symbol.includes(keyword));
      box.innerHTML = visible.map((symbol) => {
        const checked = selectedSymbols.has(symbol) ? 'checked' : '';
        return '<label class="chip"><input type="checkbox" value="' + symbol + '" ' + checked + ' onchange="toggleSymbol(this.value, this.checked)"><span>' + symbol + '</span></label>';
      }).join('') || '<span class="muted">Không tìm thấy mã phù hợp.</span>';
      renderPrimaryOptions($('primarySymbol').value || $('historySymbol').value || 'VN30F1M');
    }

    function renderSelectedSymbols() {
      const list = Array.from(selectedSymbols).sort();
      $('selectedSummary').textContent = list.length + ' mã đang được chọn' + ($('historyScope').value === 'all' ? ' cho đồng bộ tất cả.' : '.');
      $('selectedSymbols').innerHTML = list.length
        ? list.map((symbol) => '<span class="mini-chip">' + symbol + '</span>').join('')
        : '<span class="muted">Chưa chọn mã nào.</span>';
    }

    function toggleSymbol(symbol, checked) {
      symbol = String(symbol || '').trim().toUpperCase();
      if (!symbol) return;
      if (checked) selectedSymbols.add(symbol);
      else selectedSymbols.delete(symbol);
      renderPrimaryOptions($('primarySymbol').value || symbol);
      renderSelectedSymbols();
    }

    function toggleExchange(exchange) {
      exchange = exchange.toUpperCase();
      if (selectedExchanges.has(exchange)) selectedExchanges.delete(exchange);
      else selectedExchanges.add(exchange);
      loadInstrumentSymbols();
    }

    async function loadInstrumentSymbols() {
      const note = $('instrumentLoadResult');
      const exchanges = Array.from(selectedExchanges);
      note.textContent = 'Đang tải danh sách mã từ DNSE...';
      try {
        const data = await requestJSON('/symbols/tickers?refresh=1');
        const filtered = (data.data || []).filter((item) => {
          const exchange = String(item.exchange || item.Exchange || item.marketType || item.MarketType || '').toUpperCase();
          const marketType = String(item.marketType || item.MarketType || '').toUpperCase();
          if (!exchanges.length) return true;
          return exchanges.includes(exchange) || exchanges.includes(marketType);
        });
        mergeSymbols(filtered.map((item) => item.symbol || item.Symbol || ''));
        renderSymbolChoices();
        note.textContent = 'Đã tải ' + filtered.length + ' / ' + (data.total || 0) + ' mã. Bộ lọc: ' + exchanges.join(', ') + '.';
      } catch (e) {
        note.textContent = (e && e.error) || 'Không tải được danh sách mã từ DNSE.';
      }
    }

    function settingsPayload() {
      const list = Array.from(selectedSymbols).sort();
      return {
        apiKey: settings?.dnse?.apiKey || '',
        accountNo: settings?.dnse?.accountNo || '',
        mock: settings?.dnse?.mock === true,
        symbols: list,
        primarySymbol: $('primarySymbol').value || list[0] || 'VN30F1M',
        entradeEnabled: settings?.entrade?.enabled === true,
        entradeMock: settings?.entrade?.mock === true,
        entradeEnvironment: settings?.entrade?.environment || 'real',
        entradeDefaultAccountNos: settings?.entrade?.defaultAccountNos || [],
        entradeAccounts: settings?.entrade?.accounts || [],
        tradingGroups: settings?.trading?.groups || [],
        tradingRoutes: settings?.trading?.routes || {}
      };
    }

    async function saveSymbols() {
      if (!selectedSymbols.size) {
        printOutput('Hãy chọn ít nhất một mã.', true);
        return;
      }
      await requestJSON('/api/settings', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(settingsPayload())
      });
      await loadSettings();
    }

    function historyPayload() {
      return {
        symbol: ($('historySymbol').value || 'VN30F1M').trim().toUpperCase(),
        marketType: $('historyMarketType').value || 'DERIVATIVE',
        resolution: Number($('historyResolution').value) || 1,
        lookbackDays: Number($('historyLookbackDays').value) || 365
      };
    }

    async function syncHistoricalPrices() {
      await syncHistoricalPricesWithLookback(historyPayload().lookbackDays, 'Đồng bộ giá lịch sử');
    }

    async function syncMaximumHistoricalPrices() {
      const all = $('historyScope').value === 'all';
      const target = all ? Array.from(selectedSymbols).sort().length + ' mã đã chọn' : (($('historySymbol').value || 'VN30F1M').trim().toUpperCase());
      if (all) {
        printOutput('Đồng bộ tối đa có thể chỉ nên chạy cho từng mã để tránh chạm rate limit DNSE. Hãy chọn phạm vi Một mã, hoặc dùng Đồng bộ giá lịch sử với số ngày cụ thể cho tất cả mã.', true);
        return;
      }
      const ok = window.confirm('Đồng bộ tối đa có thể cho ' + target + '. Tác vụ sẽ chạy ngầm, tự nghỉ giữa các request và tự chờ khi DNSE rate limit. Tiếp tục?');
      if (!ok) return;
      const payload = historyPayload();
      printOutput('Đã đưa ' + payload.symbol + ' vào hàng đợi đồng bộ tối đa. Có thể tiếp tục dùng realtime trong lúc hệ thống chạy nền...');
      await requestJSON('/history/max-background', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      });
      setTimeout(loadBackgroundHistoryJobs, 2000);
    }

    async function loadBackgroundHistoryJobs() {
      try {
        const data = await requestJSON('/history/max-background');
        const jobs = data.jobs || [];
        const running = jobs.some((job) => job.status === 'running');
        if (running) setTimeout(loadBackgroundHistoryJobs, 10000);
      } catch {}
    }

    async function syncHistoricalPricesWithLookback(lookbackDays, label) {
      const payload = historyPayload();
      payload.lookbackDays = Math.max(1, Number(lookbackDays) || 365);
      const all = $('historyScope').value === 'all';
      printOutput('Đang chạy ' + label + ' cho ' + (all ? Array.from(selectedSymbols).sort().length + ' mã đã chọn' : payload.symbol) + ', lookbackDays=' + payload.lookbackDays + '...');
      await requestJSON(all ? '/history/full-all' : '/history/full', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(all ? { lookbackDays: payload.lookbackDays, symbols: Array.from(selectedSymbols).sort() } : payload)
      });
    }

    async function syncTodayPrices() {
      const payload = historyPayload();
      const all = $('historyScope').value === 'all';
      printOutput('Đang vá riêng dữ liệu hôm nay cho ' + (all ? Array.from(selectedSymbols).sort().length + ' mã đã chọn' : payload.symbol) + '...');
      await requestJSON(all ? '/history/today-all' : '/history/today', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(all ? { symbols: Array.from(selectedSymbols).sort() } : payload)
      });
    }

    loadProfiles().then(loadSettings);
  </script>
` + layoutBottom
