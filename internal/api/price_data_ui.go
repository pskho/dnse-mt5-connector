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
    .price-tabs {
      display: flex;
      gap: 8px;
      border-bottom: 1px solid var(--border);
      margin-bottom: 18px;
      overflow-x: auto;
    }
    .price-tab {
      background: transparent;
      border: 0;
      border-bottom: 3px solid transparent;
      color: var(--text-muted);
      cursor: pointer;
      padding: 12px 14px;
      font-weight: 700;
      white-space: nowrap;
    }
    .price-tab.active {
      color: var(--text);
      border-bottom-color: var(--primary);
    }
    .price-panel { display: none; }
    .price-panel.active {
      display: grid;
      gap: 18px;
    }
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
    .readiness-grid {
      display: grid;
      grid-template-columns: repeat(4, minmax(0, 1fr));
      gap: 12px;
      margin-bottom: 18px;
    }
    .readiness-card {
      border: 1px solid var(--border);
      border-radius: 8px;
      background: var(--surface);
      padding: 14px;
      min-height: 86px;
    }
    .readiness-label {
      color: var(--text-muted);
      font-size: 12px;
      margin-bottom: 8px;
    }
    .readiness-value {
      font-size: 17px;
      font-weight: 700;
      overflow-wrap: anywhere;
    }
    .recommendation {
      border: 1px solid rgba(245, 124, 0, 0.42);
      border-radius: 8px;
      padding: 14px;
      background: rgba(245, 124, 0, 0.08);
      color: var(--text);
      line-height: 1.6;
    }
    .recommendation.ok {
      border-color: rgba(56, 142, 60, 0.5);
      background: rgba(56, 142, 60, 0.08);
    }
    .recommendation.bad {
      border-color: rgba(211, 47, 47, 0.52);
      background: rgba(211, 47, 47, 0.09);
    }
    .check-table {
      width: 100%;
      border-collapse: collapse;
      table-layout: fixed;
    }
    .check-table th,
    .check-table td {
      border-bottom: 1px solid var(--border);
      padding: 11px 8px;
      text-align: left;
      vertical-align: top;
      font-size: 13px;
    }
    .check-table th {
      color: var(--text-muted);
      font-size: 12px;
    }
    .status-badge {
      display: inline-flex;
      align-items: center;
      border: 1px solid var(--border);
      border-radius: 999px;
      padding: 4px 8px;
      font-size: 12px;
      color: var(--text-muted);
    }
    .status-badge.ok { border-color: rgba(56, 142, 60, 0.55); color: #9bd79e; }
    .status-badge.warn { border-color: rgba(245, 124, 0, 0.65); color: #ffc47f; }
    .status-badge.bad { border-color: rgba(211, 47, 47, 0.65); color: #ff9b9b; }
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
    .data-check-head {
      display: flex;
      justify-content: space-between;
      align-items: flex-start;
      gap: 16px;
      margin-bottom: 16px;
    }
    .coverage-list {
      display: grid;
      grid-template-columns: repeat(3, minmax(0, 1fr));
      gap: 12px;
      margin: 14px 0 16px;
    }
    .coverage-box {
      text-align: left;
      color: var(--text);
      border: 1px solid var(--border);
      border-radius: 8px;
      background: #171717;
      padding: 12px;
      min-height: 112px;
      cursor: pointer;
      font-family: inherit;
    }
    .coverage-box strong {
      display: block;
      margin-bottom: 8px;
      font-size: 14px;
    }
    .coverage-box div {
      color: var(--text-muted);
      font-size: 12px;
      line-height: 1.6;
    }
    .history-chart-wrap {
      border: 1px solid var(--border);
      border-radius: 8px;
      background: #0c0c0c;
      padding: 12px;
    }
    .history-chart {
      width: 100%;
      height: 320px;
      display: block;
    }
    button.mini-chip {
      cursor: pointer;
      font-family: inherit;
    }
    button.mini-chip.active {
      border-color: var(--primary);
      color: #fff;
      background: rgba(211, 47, 47, 0.16);
    }
    @media (max-width: 1100px) {
      .price-shell { grid-template-columns: 1fr; }
      .price-side { position: static; }
      .console { height: 320px; }
      .coverage-list, .readiness-grid { grid-template-columns: 1fr; }
      .check-table { table-layout: auto; }
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
      <div class="readiness-grid">
        <div class="readiness-card"><div class="readiness-label">Kết nối MT5</div><div id="mt5StatusValue" class="readiness-value">Đang kiểm tra...</div></div>
        <div class="readiness-card"><div class="readiness-label">Dữ liệu mã đang xem</div><div id="cacheStatusValue" class="readiness-value">Đang kiểm tra...</div></div>
        <div class="readiness-card"><div class="readiness-label">Hành động nên làm</div><div id="nextActionValue" class="readiness-value">...</div></div>
        <div class="readiness-card"><div class="readiness-label">Phạm vi đang chọn</div><div id="scopeStatusValue" class="readiness-value">...</div></div>
      </div>

      <div id="readinessRecommendation" class="recommendation">Đang kiểm tra dữ liệu để gợi ý cách nạp ít tốn API nhất.</div>

      <div class="price-tabs" role="tablist" aria-label="Dữ liệu giá">
        <button class="price-tab active" type="button" data-tab="check" onclick="showPriceTab('check')">Kiểm tra MT5</button>
        <button class="price-tab" type="button" data-tab="sync" onclick="showPriceTab('sync')">Đồng bộ</button>
        <button class="price-tab" type="button" data-tab="symbols" onclick="showPriceTab('symbols')">Danh sách mã</button>
        <button class="price-tab" type="button" data-tab="detail" onclick="showPriceTab('detail')">Chi tiết cache</button>
      </div>

      <div id="tab-check" class="price-panel active">
      <section class="card">
        <div class="data-check-head">
          <div>
            <h2 style="margin-bottom:8px">Kiểm tra dữ liệu cho MT5</h2>
            <p class="muted" style="margin:0">Bridge kiểm tra kết nối MT5 và cache lịch sử trước khi gợi ý có cần gọi API DNSE hay không.</p>
          </div>
          <div class="actions" style="margin-top:0">
            <button class="btn secondary" type="button" onclick="runMT5DataCheck()">Kiểm tra mã đang xem</button>
            <button class="btn secondary" type="button" onclick="runSelectedDataCheck()">Kiểm tra các mã đã chọn</button>
          </div>
        </div>
        <div class="grid-2">
          <div class="form-group">
            <label>Mã cần kiểm tra</label>
            <input id="checkSymbol" value="VN30F1M" onkeydown="if(event.key==='Enter') runMT5DataCheck()">
          </div>
          <div class="form-group">
            <label>Khung kiểm tra</label>
            <select id="checkResolution" onchange="runMT5DataCheck()">
              <option value="1">M1</option>
              <option value="5">M5</option>
              <option value="15">M15</option>
              <option value="30">M30</option>
              <option value="60">H1</option>
              <option value="1440">D1</option>
              <option value="10080">W1</option>
            </select>
          </div>
        </div>
        <div id="mt5CheckSummary" class="inline-note">Chưa có kết quả kiểm tra.</div>
        <div id="mt5CheckTable"></div>
      </section>
      </div>

      <div id="tab-sync" class="price-panel">
      <section class="card">
        <h2>Đồng bộ giá lịch sử</h2>
        <div class="grid-2">
          <div class="form-group">
            <label>Phạm vi đồng bộ</label>
            <select id="historyScope" onchange="renderSelectedSymbols(); runMT5DataCheck()">
              <option value="single">Một mã</option>
              <option value="all">Tất cả mã đã chọn</option>
            </select>
          </div>
          <div class="form-group">
            <label>Mã mặc định</label>
            <input id="historySymbol" value="VN30F1M" oninput="updateHistoryMarketHint()">
          </div>
          <div class="form-group">
            <label>Khung dữ liệu</label>
            <select id="historyResolution">
              <option value="1">M1</option>
              <option value="5">M5</option>
              <option value="15">M15</option>
              <option value="30">M30</option>
              <option value="60">H1</option>
              <option value="1440">D1</option>
              <option value="10080">W1</option>
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
          <div class="form-group">
            <label>Nhận diện thị trường</label>
            <div id="historyMarketHint" class="inline-note">Loại thị trường sẽ được tự nhận diện theo mã.</div>
          </div>
        </div>
        <div class="actions">
          <button class="btn secondary" type="button" onclick="runMT5DataCheck()">Kiểm tra trước khi đồng bộ</button>
          <button class="btn" type="button" onclick="syncMaximumHistoricalPrices()">Đồng bộ giá lịch sử</button>
          <button class="btn secondary" type="button" onclick="syncHistoricalPrices()">Đồng bộ theo số ngày</button>
          <button class="btn secondary" type="button" onclick="syncTodayPrices()">Vá riêng hôm nay</button>
          <button class="btn secondary" type="button" onclick="saveSymbols()">Lưu danh sách mã</button>
        </div>
        <div class="inline-note">Đồng bộ giá lịch sử sẽ chạy ngầm, tự đi lùi từng lớp nhỏ cho tới khi API không còn dữ liệu cũ hơn. Đồng bộ theo số ngày dùng khi cần vá nhanh một khoảng cụ thể. Nếu MT5 đang mở, bridge sẽ nhận snapshot mới và import vào custom symbol.</div>
      </section>
      </div>

      <div id="tab-symbols" class="price-panel">
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

      <div id="tab-detail" class="price-panel">
      <section class="card">
        <div class="data-check-head">
          <div>
            <h2 style="margin-bottom:8px">Kiểm tra dữ liệu đã tải</h2>
            <p class="muted" style="margin:0">Bấm vào mã đã chọn để xem cache lịch sử đang có trong phần mềm và chart kiểm tra nhanh.</p>
          </div>
          <button class="btn secondary" type="button" onclick="loadSymbolHistory(activeDetailSymbol || $('historySymbol').value)">Tải lại</button>
        </div>
        <div class="grid-2">
          <div class="form-group">
            <label>Mã đang xem</label>
            <input id="detailSymbol" value="VN30F1M" onkeydown="if(event.key==='Enter') loadSymbolHistory(this.value)">
          </div>
          <div class="form-group">
            <label>Khung chart</label>
            <select id="detailResolution" onchange="loadSymbolHistory(activeDetailSymbol || $('detailSymbol').value)">
              <option value="1">M1</option>
              <option value="5">M5</option>
              <option value="15">M15</option>
              <option value="30">M30</option>
              <option value="60">H1</option>
              <option value="1440">D1</option>
              <option value="10080">W1</option>
            </select>
          </div>
        </div>
        <div id="historyDetailSummary" class="inline-note">Chưa chọn mã để kiểm tra.</div>
        <div id="historyCoverageStats" class="coverage-list"></div>
        <div class="history-chart-wrap">
          <canvas id="historyChart" class="history-chart"></canvas>
        </div>
      </section>
      </div>
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
    let activeDetailSymbol = 'VN30F1M';
    let lastHistoryCandles = [];
    let lastReadiness = null;

    function showPriceTab(tab) {
      document.querySelectorAll('.price-tab').forEach((button) => {
        button.classList.toggle('active', button.dataset.tab === tab);
      });
      document.querySelectorAll('.price-panel').forEach((panel) => {
        panel.classList.toggle('active', panel.id === 'tab-' + tab);
      });
      if (tab === 'detail') setTimeout(() => drawHistoryChart(lastHistoryCandles), 60);
    }

    function setText(id, value) {
      const el = $(id);
      if (el) el.textContent = value;
    }

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

    async function fetchJSONQuiet(path, options = {}) {
      const res = await fetch(path, options);
      const text = await res.text();
      let body = {};
      if (text.trim()) {
        try { body = JSON.parse(text); }
        catch { body = { success: false, error: text }; }
      }
      if (!res.ok) throw body;
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
      $('checkSymbol').value = $('historySymbol').value;
      $('checkResolution').value = $('historyResolution').value;
      renderPrimaryOptions((data.marketData && data.marketData.symbol) || configured[0] || 'VN30F1M');
      renderSymbolChoices();
      renderSelectedSymbols();
      loadSymbolHistory($('historySymbol').value || configured[0] || 'VN30F1M');
      runMT5DataCheck();
    }

    function renderPrimaryOptions(selected) {
      const select = $('primarySymbol');
      const symbols = Array.from(new Set([...selectedSymbols, ...availableSymbols])).sort();
      select.innerHTML = symbols.map((symbol) => '<option value="' + symbol + '">' + symbol + '</option>').join('');
      select.value = selected || symbols[0] || 'VN30F1M';
      select.onchange = () => {
        $('historySymbol').value = select.value || 'VN30F1M';
        $('checkSymbol').value = $('historySymbol').value;
        updateHistoryMarketHint();
        runMT5DataCheck();
      };
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
      setText('scopeStatusValue', $('historyScope').value === 'all' ? list.length + ' mã' : '1 mã');
      $('selectedSymbols').innerHTML = list.length
        ? list.map((symbol) => '<button class="mini-chip' + (symbol === activeDetailSymbol ? ' active' : '') + '" type="button" onclick="selectDetailSymbol(\'' + symbol + '\')">' + symbol + '</button>').join('')
        : '<span class="muted">Chưa chọn mã nào.</span>';
      updateHistoryMarketHint();
    }

    function selectDetailSymbol(symbol) {
      symbol = String(symbol || '').trim().toUpperCase();
      if (!symbol) return;
      activeDetailSymbol = symbol;
      $('historySymbol').value = symbol;
      $('checkSymbol').value = symbol;
      $('detailSymbol').value = symbol;
      if ([...$('primarySymbol').options].some((option) => option.value === symbol)) {
        $('primarySymbol').value = symbol;
      }
      updateHistoryMarketHint();
      renderSelectedSymbols();
      loadSymbolHistory(symbol);
      runMT5DataCheck();
    }

    function toggleSymbol(symbol, checked) {
      symbol = String(symbol || '').trim().toUpperCase();
      if (!symbol) return;
      if (checked) selectedSymbols.add(symbol);
      else selectedSymbols.delete(symbol);
      renderPrimaryOptions($('primarySymbol').value || symbol);
      renderSelectedSymbols();
      runMT5DataCheck();
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
      const symbol = ($('historySymbol').value || 'VN30F1M').trim().toUpperCase();
      return {
        symbol,
        marketType: inferMarketType(symbol),
        resolution: Number($('historyResolution').value) || 1,
        lookbackDays: Number($('historyLookbackDays').value) || 365
      };
    }

    function inferMarketType(symbol) {
      symbol = String(symbol || '').trim().toUpperCase();
      if (['VNINDEX', 'VN30', 'VN100', 'VNXALLSHARE', 'HNX', 'HNX30', 'UPCOM'].includes(symbol)) return 'INDEX';
      if (symbol.startsWith('VN30F') || symbol.startsWith('V100F') || /^\d\dI\d[A-Z0-9]*[GC][A-Z0-9]*$/.test(symbol)) return 'DERIVATIVE';
      return 'STOCK';
    }

    function updateHistoryMarketHint() {
      const hint = $('historyMarketHint');
      if (!hint) return;
      const symbol = ($('historySymbol').value || 'VN30F1M').trim().toUpperCase();
      hint.textContent = 'Tự nhận diện: ' + inferMarketType(symbol) + ' cho ' + symbol + '.';
      if ($('checkSymbol')) $('checkSymbol').value = symbol;
    }

    function resolutionLabel(resolution) {
      const labels = { 1: 'M1', 5: 'M5', 15: 'M15', 30: 'M30', 60: 'H1', 1440: 'D1', 10080: 'W1' };
      return labels[Number(resolution)] || String(resolution);
    }

    function formatNumber(value) {
      return Number(value || 0).toLocaleString('vi-VN');
    }

    function formatDateTime(value) {
      if (!value) return '-';
      const date = new Date(value);
      if (Number.isNaN(date.getTime())) return String(value);
      return date.toLocaleString('vi-VN');
    }

    function coverageForResolution(coverages, resolution) {
      return (coverages || []).find((item) => Number(item.resolution) === Number(resolution)) || null;
    }

    function expectedCandlesForDays(days, resolution) {
      const res = Math.max(1, Number(resolution) || 1);
      if (res >= 1440) return Math.max(1, Math.ceil(Number(days || 0)));
      return Math.max(1, Math.ceil((Number(days || 0) || 0) * 240 / res));
    }

    function classifyCoverage(coverage, resolution, lookbackDays) {
      if (!coverage || !coverage.count) return { level: 'bad', label: 'Chưa có dữ liệu', action: 'Nạp lịch sử' };
      const expected = expectedCandlesForDays(lookbackDays, resolution);
      const ratio = coverage.count / expected;
      const lastMS = Number(coverage.lastTimeMs || 0);
      const ageHours = lastMS > 0 ? (Date.now() - lastMS) / 3600000 : 9999;
      if (ratio >= 0.82 && ageHours <= 36) return { level: 'ok', label: 'Đủ dùng', action: 'Không cần gọi API' };
      if (ratio >= 0.82) return { level: 'warn', label: 'Thiếu dữ liệu mới', action: 'Vá hôm nay' };
      return { level: 'bad', label: 'Thiếu lịch sử', action: 'Nạp lịch sử' };
    }

    function setRecommendation(text, level = 'warn') {
      const box = $('readinessRecommendation');
      if (!box) return;
      box.className = 'recommendation' + (level ? ' ' + level : '');
      box.textContent = text;
    }

    async function runMT5DataCheck() {
      const symbol = String(($('checkSymbol')?.value || $('historySymbol').value || 'VN30F1M')).trim().toUpperCase();
      const resolution = Number($('checkResolution')?.value || $('historyResolution').value || 1);
      const lookbackDays = Math.max(1, Number($('historyLookbackDays').value) || 365);
      if (!symbol) return;
      $('checkSymbol').value = symbol;
      $('historySymbol').value = symbol;
      $('detailSymbol').value = symbol;
      activeDetailSymbol = symbol;
      setText('mt5StatusValue', 'Đang kiểm tra...');
      setText('cacheStatusValue', 'Đang kiểm tra...');
      setText('nextActionValue', '...');
      $('mt5CheckSummary').textContent = 'Đang kiểm tra kết nối MT5 và cache lịch sử cho ' + symbol + '...';

      try {
        const [status, cache] = await Promise.all([
          fetchJSONQuiet('/status'),
          fetchJSONQuiet('/history/cache?symbol=' + encodeURIComponent(symbol) + '&marketType=' + encodeURIComponent(inferMarketType(symbol)) + '&resolution=' + resolution + '&limit=20')
        ]);
        const coverage = coverageForResolution(cache.coverages, resolution);
        const verdict = classifyCoverage(coverage, resolution, lookbackDays);
        const mt5Connected = Number(status.market_data_active_clients || 0) > 0;
        const realtimeSymbols = (status.market_data_symbols || []).filter((item) => String(item.symbol || '').toUpperCase() === symbol || String(item.symbol || '').toUpperCase() === cache.symbol);
        const live = realtimeSymbols.some((item) => Number(item.staleSeconds || 999999) <= 90);
        const apiAvoided = verdict.level === 'ok';
        lastReadiness = { symbol, resolution, lookbackDays, coverage, verdict, mt5Connected, live, cache };

        setText('mt5StatusValue', mt5Connected ? 'Đã kết nối' : 'Chưa thấy MT5');
        setText('cacheStatusValue', verdict.label);
        setText('nextActionValue', verdict.action);
        setText('scopeStatusValue', $('historyScope').value === 'all' ? selectedSymbols.size + ' mã' : symbol);

        const rows = [
          ['Kết nối MT5', mt5Connected ? 'ok' : 'warn', mt5Connected ? 'MT5 đang nhận dữ liệu từ bridge.' : 'Chưa thấy EA/DLL MT5 kết nối. Có thể kiểm tra cache trước, rồi mở MT5 để import.'],
          ['Realtime', live ? 'ok' : 'warn', live ? 'Có dữ liệu realtime gần đây cho mã này.' : 'Chưa có realtime gần đây cho mã này. Nếu đang ngoài phiên, đây có thể là bình thường.'],
          ['Cache ' + resolutionLabel(resolution), verdict.level, coverage ? formatNumber(coverage.count) + ' nến, đến ' + (coverage.lastTime || '-') : 'Chưa có nến trong cache.'],
          ['Gợi ý API', apiAvoided ? 'ok' : verdict.level, apiAvoided ? 'Không nên gọi API lịch sử lúc này.' : (verdict.action === 'Vá hôm nay' ? 'Chỉ cần vá hôm nay, tránh nạp lại toàn bộ.' : 'Cần nạp lịch sử cho mã/khung này.')]
        ];
        $('mt5CheckTable').innerHTML = '<table class="check-table"><thead><tr><th>Hạng mục</th><th>Trạng thái</th><th>Diễn giải</th></tr></thead><tbody>' +
          rows.map((row) => '<tr><td>' + row[0] + '</td><td><span class="status-badge ' + row[1] + '">' + (row[1] === 'ok' ? 'OK' : row[1] === 'bad' ? 'Thiếu' : 'Cần chú ý') + '</span></td><td>' + row[2] + '</td></tr>').join('') +
          '</tbody></table>';

        if (!mt5Connected) {
          setRecommendation('Chưa thấy MT5 kết nối. Hãy mở MT5/EA trước khi kết luận thiếu dữ liệu trong MT5. Nếu cache bên dưới đã đủ, không cần gọi API thêm.', 'warn');
        } else if (verdict.level === 'ok') {
          setRecommendation('Cache đã đủ mới cho ' + symbol + '. MT5 đang kết nối nên bridge có thể import dữ liệu này; nên tránh gọi API lịch sử thêm.', 'ok');
        } else if (verdict.action === 'Vá hôm nay') {
          setRecommendation('Dữ liệu lịch sử đã có nền, chỉ thiếu phần mới. Nên dùng "Vá riêng hôm nay" thay vì đồng bộ tối đa.', 'warn');
        } else {
          setRecommendation('Cache còn thiếu cho ' + symbol + '. Nên đồng bộ lịch sử cho mã này trước, sau đó kiểm tra lại để tránh gọi API lặp.', 'bad');
        }
        $('mt5CheckSummary').textContent = symbol + ' · ' + resolutionLabel(resolution) + ' · lần kiểm tra ' + formatDateTime(new Date().toISOString()) + '.';
      } catch (e) {
        setText('mt5StatusValue', 'Không kiểm tra được');
        setText('cacheStatusValue', 'Không rõ');
        setText('nextActionValue', 'Mở lại bridge');
        $('mt5CheckSummary').textContent = (e && e.error) || 'Không kiểm tra được dữ liệu MT5/cache.';
        $('mt5CheckTable').innerHTML = '';
        setRecommendation('Chưa đọc được trạng thái. Không nên bấm đồng bộ nhiều mã cho tới khi bridge trả trạng thái bình thường.', 'bad');
      }
    }

    async function shouldSkipApiSync(action) {
      await runMT5DataCheck();
      if (!lastReadiness || lastReadiness.verdict.level !== 'ok') return false;
      const all = $('historyScope').value === 'all';
      if (all) return false;
      printOutput('Kiểm tra cho thấy ' + lastReadiness.symbol + ' đã đủ dữ liệu cache cho MT5. Đã bỏ qua "' + action + '" để tránh gọi API DNSE thừa.');
      return true;
    }

    async function runSelectedDataCheck() {
      const symbols = Array.from(selectedSymbols).sort();
      if (!symbols.length) {
        $('mt5CheckSummary').textContent = 'Chưa chọn mã nào để kiểm tra.';
        return;
      }
      const resolution = Number($('checkResolution')?.value || $('historyResolution').value || 1);
      const lookbackDays = Math.max(1, Number($('historyLookbackDays').value) || 365);
      $('mt5CheckSummary').textContent = 'Đang kiểm tra cache cho ' + symbols.length + ' mã đã chọn...';
      try {
        const status = await fetchJSONQuiet('/status');
        const mt5Connected = Number(status.market_data_active_clients || 0) > 0;
        const rows = [];
        let okCount = 0;
        let todayCount = 0;
        for (const symbol of symbols) {
          const cache = await fetchJSONQuiet('/history/cache?symbol=' + encodeURIComponent(symbol) + '&marketType=' + encodeURIComponent(inferMarketType(symbol)) + '&resolution=' + resolution + '&limit=1');
          const coverage = coverageForResolution(cache.coverages, resolution);
          const verdict = classifyCoverage(coverage, resolution, lookbackDays);
          if (verdict.level === 'ok') okCount++;
          if (verdict.action === 'Vá hôm nay') todayCount++;
          rows.push([symbol, verdict.level, verdict.label, coverage ? formatNumber(coverage.count) + ' nến, đến ' + (coverage.lastTime || '-') : 'Chưa có cache', verdict.action]);
        }
        const missing = symbols.length - okCount;
        setText('mt5StatusValue', mt5Connected ? 'Đã kết nối' : 'Chưa thấy MT5');
        setText('cacheStatusValue', okCount + '/' + symbols.length + ' đủ');
        setText('nextActionValue', missing === 0 ? 'Không gọi API' : (todayCount === missing ? 'Vá hôm nay' : 'Nạp mã thiếu'));
        $('mt5CheckTable').innerHTML = '<table class="check-table"><thead><tr><th>Mã</th><th>Trạng thái</th><th>Cache</th><th>Gợi ý</th></tr></thead><tbody>' +
          rows.map((row) => '<tr><td><code>' + row[0] + '</code></td><td><span class="status-badge ' + row[1] + '">' + row[2] + '</span></td><td>' + row[3] + '</td><td>' + row[4] + '</td></tr>').join('') +
          '</tbody></table>';
        $('mt5CheckSummary').textContent = 'Đã kiểm tra ' + symbols.length + ' mã ở khung ' + resolutionLabel(resolution) + '.';
        if (missing === 0) setRecommendation('Tất cả mã đã chọn đã đủ cache cho MT5. Không nên đồng bộ lịch sử thêm để tránh gọi API DNSE thừa.', 'ok');
        else setRecommendation('Có ' + missing + ' mã chưa đủ cache. Chỉ nên đồng bộ các mã thiếu hoặc vá hôm nay nếu cột Gợi ý báo như vậy.', 'warn');
      } catch (e) {
        $('mt5CheckSummary').textContent = (e && e.error) || 'Không kiểm tra được danh sách mã đã chọn.';
        setRecommendation('Không đọc được cache cho danh sách mã. Hãy kiểm tra bridge trước khi đồng bộ nhiều mã.', 'bad');
      }
    }

    async function loadSymbolHistory(symbol) {
      symbol = String(symbol || '').trim().toUpperCase();
      if (!symbol) return;
      activeDetailSymbol = symbol;
      $('detailSymbol').value = symbol;
      renderSelectedSymbols();
      const resolution = Number($('detailResolution').value) || 1;
      const marketType = inferMarketType(symbol);
      $('historyDetailSummary').textContent = 'Đang đọc dữ liệu đã tải cho ' + symbol + '...';
      try {
        const data = await fetchJSONQuiet('/history/cache?symbol=' + encodeURIComponent(symbol) + '&marketType=' + encodeURIComponent(marketType) + '&resolution=' + resolution + '&limit=900');
        renderHistoryDetail(data);
      } catch (e) {
        $('historyDetailSummary').textContent = (e && e.error) || 'Không đọc được dữ liệu đã tải cho ' + symbol + '.';
        $('historyCoverageStats').innerHTML = '';
        drawHistoryChart([]);
      }
    }

    function renderHistoryDetail(data) {
      const coverages = data.coverages || [];
      lastHistoryCandles = data.candles || [];
      const selectedResolution = Number(data.resolution || $('detailResolution').value || 1);
      if ([...$('detailResolution').options].some((option) => Number(option.value) === selectedResolution)) {
        $('detailResolution').value = String(selectedResolution);
      }
      if (!coverages.length) {
        $('historyDetailSummary').textContent = 'Chưa có dữ liệu lịch sử trong cache cho ' + data.symbol + '.';
        $('historyCoverageStats').innerHTML = '';
        drawHistoryChart([]);
        return;
      }
      $('historyDetailSummary').textContent = data.symbol + ' / ' + data.marketType + ': chart đang dùng ' + resolutionLabel(selectedResolution) + ', ' + formatNumber(data.sampledCandles) + '/' + formatNumber(data.totalCandles) + ' nến' + (data.sampled ? ' được lấy mẫu để vẽ nhanh.' : '.');
      $('historyCoverageStats').innerHTML = coverages.map((item) => {
        const active = Number(item.resolution) === selectedResolution ? ' style="border-color:var(--primary)"' : '';
        return '<button class="coverage-box" type="button"' + active + ' onclick="setDetailResolution(' + Number(item.resolution) + ')">' +
          '<strong>' + resolutionLabel(item.resolution) + ' · ' + formatNumber(item.count) + ' nến</strong>' +
          '<div>Từ: ' + (item.firstTime || '-') + '</div>' +
          '<div>Đến: ' + (item.lastTime || '-') + '</div>' +
          '<div>Cập nhật: ' + (item.updatedAt || '-') + '</div>' +
        '</button>';
      }).join('');
      drawHistoryChart(lastHistoryCandles);
    }

    function setDetailResolution(resolution) {
      $('detailResolution').value = String(resolution);
      loadSymbolHistory(activeDetailSymbol || $('detailSymbol').value);
    }

    function drawHistoryChart(candles) {
      const canvas = $('historyChart');
      const ctx = canvas.getContext('2d');
      const rect = canvas.getBoundingClientRect();
      const ratio = window.devicePixelRatio || 1;
      canvas.width = Math.max(1, Math.floor(rect.width * ratio));
      canvas.height = Math.max(1, Math.floor(rect.height * ratio));
      ctx.setTransform(ratio, 0, 0, ratio, 0, 0);
      const width = rect.width;
      const height = rect.height;
      ctx.clearRect(0, 0, width, height);
      ctx.fillStyle = '#0c0c0c';
      ctx.fillRect(0, 0, width, height);
      ctx.strokeStyle = '#242424';
      ctx.lineWidth = 1;
      for (let i = 1; i < 5; i++) {
        const y = (height / 5) * i;
        ctx.beginPath();
        ctx.moveTo(0, y);
        ctx.lineTo(width, y);
        ctx.stroke();
      }
      if (!candles || candles.length < 2) {
        ctx.fillStyle = '#9e9e9e';
        ctx.font = '13px Segoe UI, Arial';
        ctx.fillText('Chưa đủ dữ liệu để vẽ chart.', 16, 28);
        return;
      }
      const values = candles.flatMap((item) => [Number(item.high || item.close), Number(item.low || item.close)]).filter((value) => value > 0);
      const min = Math.min(...values);
      const max = Math.max(...values);
      const span = Math.max(0.0001, max - min);
      const pad = { left: 48, right: 12, top: 16, bottom: 28 };
      const chartW = Math.max(1, width - pad.left - pad.right);
      const chartH = Math.max(1, height - pad.top - pad.bottom);
      const xAt = (i) => pad.left + chartW * (i / Math.max(1, candles.length - 1));
      const yAt = (value) => pad.top + chartH * (1 - ((value - min) / span));
      ctx.strokeStyle = '#5cc8ff';
      ctx.lineWidth = 1.5;
      ctx.beginPath();
      candles.forEach((item, i) => {
        const x = xAt(i);
        const y = yAt(Number(item.close || 0));
        if (i === 0) ctx.moveTo(x, y);
        else ctx.lineTo(x, y);
      });
      ctx.stroke();
      ctx.fillStyle = '#d8d8d8';
      ctx.font = '12px Segoe UI, Arial';
      ctx.fillText(max.toFixed(2), 6, pad.top + 4);
      ctx.fillText(min.toFixed(2), 6, pad.top + chartH);
      ctx.fillStyle = '#9e9e9e';
      ctx.fillText(candles[0].time || '', pad.left, height - 8);
      const lastTime = candles[candles.length - 1].time || '';
      const lastWidth = ctx.measureText(lastTime).width;
      ctx.fillText(lastTime, Math.max(pad.left, width - pad.right - lastWidth), height - 8);
    }

    window.addEventListener('resize', () => drawHistoryChart(lastHistoryCandles));

    function refreshActiveHistoryDetailSoon() {
      setTimeout(() => loadSymbolHistory(activeDetailSymbol || $('historySymbol').value), 1200);
    }

    async function syncHistoricalPrices() {
      await syncHistoricalPricesWithLookback(historyPayload().lookbackDays, 'Đồng bộ theo số ngày');
    }

    async function syncMaximumHistoricalPrices() {
      if (await shouldSkipApiSync('Đồng bộ giá lịch sử')) return;
      const all = $('historyScope').value === 'all';
      const target = all ? Array.from(selectedSymbols).sort().length + ' mã đã chọn' : (($('historySymbol').value || 'VN30F1M').trim().toUpperCase());
      const ok = window.confirm('Đồng bộ giá lịch sử cho ' + target + '. Tác vụ sẽ chạy ngầm, tự nghỉ giữa các request và tự chờ khi DNSE rate limit. Tiếp tục?');
      if (!ok) return;
      const payload = historyPayload();
      const symbols = all ? Array.from(selectedSymbols).sort() : [payload.symbol];
      printOutput('Đã đưa ' + symbols.length + ' mã vào hàng đợi đồng bộ lịch sử. Có thể tiếp tục dùng realtime trong lúc hệ thống chạy nền...');
      for (const symbol of symbols) {
        await requestJSON('/history/max-background', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ ...payload, symbol, marketType: inferMarketType(symbol) })
        });
      }
      printOutput('Đã tạo ' + symbols.length + ' tác vụ đồng bộ lịch sử. Kết quả sẽ tự cập nhật trong khung Kết quả.');
      refreshActiveHistoryDetailSoon();
      setTimeout(loadBackgroundHistoryJobs, 2000);
    }

    async function loadBackgroundHistoryJobs() {
      try {
        const data = await requestJSON('/history/max-background');
        const jobs = data.jobs || [];
        const running = jobs.some((job) => job.status === 'running');
        if (running) setTimeout(loadBackgroundHistoryJobs, 10000);
        else refreshActiveHistoryDetailSoon();
      } catch {}
    }

    async function syncHistoricalPricesWithLookback(lookbackDays, label) {
      if (await shouldSkipApiSync(label)) return;
      const payload = historyPayload();
      payload.lookbackDays = Math.max(1, Number(lookbackDays) || 365);
      const all = $('historyScope').value === 'all';
      printOutput('Đang chạy ' + label + ' cho ' + (all ? Array.from(selectedSymbols).sort().length + ' mã đã chọn' : payload.symbol) + ', lookbackDays=' + payload.lookbackDays + '...');
      await requestJSON(all ? '/history/full-all' : '/history/full', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(all ? { lookbackDays: payload.lookbackDays, symbols: Array.from(selectedSymbols).sort() } : payload)
      });
      refreshActiveHistoryDetailSoon();
    }

    async function syncTodayPrices() {
      if (await shouldSkipApiSync('Vá riêng hôm nay')) return;
      const payload = historyPayload();
      const all = $('historyScope').value === 'all';
      printOutput('Đang vá riêng dữ liệu hôm nay cho ' + (all ? Array.from(selectedSymbols).sort().length + ' mã đã chọn' : payload.symbol) + '...');
      await requestJSON(all ? '/history/today-all' : '/history/today', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(all ? { symbols: Array.from(selectedSymbols).sort() } : payload)
      });
      refreshActiveHistoryDetailSoon();
    }

    loadProfiles().then(loadSettings);
  </script>
` + layoutBottom
