package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"dnse-mt5-connector/internal/config"
	"dnse-mt5-connector/internal/entrade"
	"dnse-mt5-connector/internal/logger"
	"dnse-mt5-connector/internal/marketdata"
	"dnse-mt5-connector/internal/service"
	"dnse-mt5-connector/internal/setup"
	"dnse-mt5-connector/internal/storage"
)

type HistorySyncer interface {
	Sync(ctx context.Context, firstTime, lastTime int64) (any, error)
	FullSync(ctx context.Context, lookbackDays int) (any, error)
}

type MarketDataStatusProvider interface {
	Status() marketdata.BridgeStatusSnapshot
}

type TelemetrySink interface {
	Track(ctx context.Context, name string, params map[string]any)
}

type Handler struct {
	orders    *service.OrderService
	positions *service.PositionService
	signals   *service.SignalService
	symbols   *service.SymbolCatalogService
	profiles  []marketdata.SymbolProfile
	market    MarketDataStatusProvider
	dnse      *DNSEClient
	history   HistorySyncer
	otp       OTPFetcher
	logger    *logger.FileLogger
	telemetry TelemetrySink
	statusMu  sync.Mutex
	lastAPIOK bool
	lastAPIAt time.Time
}

type errorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

type tradingTokenRequest struct {
	Passcode string `json:"passcode"`
	OTPType  string `json:"otpType"`
}

type killSwitchRequest struct {
	Enabled bool `json:"enabled"`
}

func NewHandler(orderService *service.OrderService, positionService *service.PositionService, signalService *service.SignalService, symbolService *service.SymbolCatalogService, profiles []marketdata.SymbolProfile, marketStatus MarketDataStatusProvider, dnseClient *DNSEClient, historyService HistorySyncer, otpFetcher OTPFetcher, appLog *logger.FileLogger, telemetrySink TelemetrySink) *Handler {
	return &Handler{orders: orderService, positions: positionService, signals: signalService, symbols: symbolService, profiles: profiles, market: marketStatus, dnse: dnseClient, history: historyService, otp: otpFetcher, logger: appLog, telemetry: telemetrySink}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", h.index)
	mux.HandleFunc("/ping", h.ping)
	mux.HandleFunc("/health", h.health)
	mux.HandleFunc("/status", h.status)
	mux.HandleFunc("/mode", h.mode)
	mux.HandleFunc("/signal", h.signal)
	mux.HandleFunc("/signals", h.pendingSignals)
	mux.HandleFunc("/confirm", h.confirm)
	mux.HandleFunc("/reject", h.reject)
	mux.HandleFunc("/order", h.order)
	mux.HandleFunc("/order/", h.orderByID) // Will handle both /order/:id and /order/client/:id manually
	mux.HandleFunc("/cancel", h.cancel)
	mux.HandleFunc("/close-deal", h.closeDeal)
	mux.HandleFunc("/history/sync", h.historySync)
	mux.HandleFunc("/history/full", h.historyFull)
	mux.HandleFunc("/history/backfill", h.historyBackfill)
	mux.HandleFunc("/history/today", h.historyToday)
	mux.HandleFunc("/history/full-all", h.historyFullAll)
	mux.HandleFunc("/history/backfill-all", h.historyBackfillAll)
	mux.HandleFunc("/history/today-all", h.historyTodayAll)
	mux.HandleFunc("/otp/latest", h.getLatestOTP)
	mux.HandleFunc("/accounts/orderable", h.orderableAccounts)
	mux.HandleFunc("/positions", h.positionsHandler)
	mux.HandleFunc("/position/", h.positionBySymbol)
	mux.HandleFunc("/kill-switch", h.killSwitch)
	mux.HandleFunc("/account", h.account)
	mux.HandleFunc("/self-test", h.selfTest)
	mux.HandleFunc("/loan-packages", h.loanPackages)
	mux.HandleFunc("/ppse", h.ppse)
	mux.HandleFunc("/symbols/derivatives", h.derivativeSymbols)
	mux.HandleFunc("/symbols/instruments", h.instrumentSymbols)
	mux.HandleFunc("/symbols/tickers", h.tickerSymbols)
	mux.HandleFunc("/symbols/mt5-layout", h.mt5Layout)
	mux.HandleFunc("/symbols/profiles", h.symbolProfiles)
	mux.HandleFunc("/registration/trading-token", h.registrationTradingToken)
	mux.HandleFunc("/registration/trading-token/refresh", h.refreshTradingToken)
	mux.HandleFunc("/registration/send-email-otp", h.sendEmailOTP)

	// UI Routes
	mux.HandleFunc("/setup", h.setupUI)
	mux.HandleFunc("/settings", h.settingsUI)
	mux.HandleFunc("/logs", h.logsUI)
	mux.HandleFunc("/api/settings", h.settingsAPI)
	mux.HandleFunc("/api/entrade/link", h.entradeLinkAPI)
	mux.HandleFunc("/api/logs/raw", h.logsRawAPI)
	mux.HandleFunc("/api/setup/install", h.setupInstallAPI)
	mux.HandleFunc("/support/export", h.supportExport)

	return h.recover(h.logRequests(mux))
}

func (h *Handler) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(indexHTML))
}

func (h *Handler) ping(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *Handler) order(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if h.signals != nil && h.signals.Mode() == service.ModeSemiAuto {
		writeJSON(w, http.StatusBadRequest, errorResponse{Success: false, Error: "direct order is disabled in semi-auto mode; use /signal and /confirm"})
		return
	}
	defer r.Body.Close()

	var req service.OrderRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		h.logger.Error("invalid_json", map[string]any{"error": err.Error(), "path": r.URL.Path})
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if h.signals != nil {
		h.signals.MarkMT5Activity()
	}

	if len(req.AccountNos) > 0 {
		responses, err := h.orders.PlaceOrders(r.Context(), req)
		status := http.StatusOK
		if err != nil {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, map[string]any{
			"success": err == nil,
			"orders":  responses,
			"error":   errorString(err),
		})
		return
	}
	resp, err := h.orders.PlaceOrder(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) signal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	defer r.Body.Close()

	var req service.SignalRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	resp, err := h.signals.Receive(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) pendingSignals(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"signals": h.signals.Pending()})
}

func (h *Handler) confirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	defer r.Body.Close()

	var req service.ConfirmRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	resp, err := h.signals.Confirm(r.Context(), req.SignalID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) reject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	defer r.Body.Close()

	var req service.RejectRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := h.signals.Reject(req.SignalID); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "signalId": req.SignalID})
}

func (h *Handler) mode(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"mode": h.signals.Mode()})
	case http.MethodPost:
		defer r.Body.Close()
		var req service.ModeRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		if err := h.signals.SetMode(req.Mode); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Success: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"mode": h.signals.Mode()})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if strings.Contains(r.Header.Get("Accept"), "text/html") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(systemStatusHTML))
		return
	}

	apiOK := h.apiOK(r.Context(), 3*time.Second)
	// Check market data and gmail by their respective structs (since we added them to handler or we just mock status for now)
	gmailOK := false
	if h.otp != nil {
		_, gmailOK = h.otp.GetLatestOTP()
		gmailOK = true // Simplified for now
	}

	marketStatus := marketdata.BridgeStatusSnapshot{}
	if h.market != nil {
		marketStatus = h.market.Status()
	}
	marketDataOK := marketStatus.PublisherStarted
	mt5MarketClientConnected := marketStatus.ActiveClients > 0

	writeJSON(w, http.StatusOK, map[string]any{
		"connected":                   true,
		"api_ok":                      apiOK,
		"token_valid":                 h.dnse.TokenValid(),
		"trading_token":               h.dnse.TradingTokenStatus(),
		"mt5_connected":               mt5MarketClientConnected,
		"mt5_signal_connected":        h.signals.MT5Connected(30 * time.Second),
		"market_data_ok":              marketDataOK,
		"market_data_active_clients":  marketStatus.ActiveClients,
		"market_data_last_connected":  marketStatus.LastClientConnectedAt,
		"market_data_last_disconnect": marketStatus.LastClientDisconnectedAt,
		"market_data_symbols":         marketStatus.Symbols,
		"gmail_ok":                    gmailOK,
		"system_enabled":              h.orders.IsEnabled(),
		"mode":                        h.signals.Mode(),
		"pendingSignals":              len(h.signals.Pending()),
	})
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !h.apiOK(r.Context(), 2*time.Second) {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "ERROR"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "OK"})
}

func (h *Handler) account(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if h.dnse != nil {
		accounts, err := h.dnse.GetAccounts(r.Context())
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"accounts": accounts})
		return
	}

	accounts, err := h.orders.Accounts(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"accounts": accounts})
}

func (h *Handler) orderableAccounts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	accounts, err := h.orders.OrderableAccounts(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"accounts": accounts})
}

func (h *Handler) selfTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	result := h.orders.SelfTest(r.Context())
	status := http.StatusOK
	if !result.Passed {
		status = http.StatusBadGateway
	}
	writeJSON(w, status, result)
}

func (h *Handler) orderByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/order/")

	var order service.OrderStatusResponse
	var err error

	if strings.HasPrefix(path, "client/") {
		clientOrderID := strings.TrimPrefix(path, "client/")
		if clientOrderID == "" || strings.Contains(clientOrderID, "/") {
			writeError(w, http.StatusBadRequest, "invalid client order id")
			return
		}
		order, err = h.orders.OrderByClient(r.Context(), clientOrderID)
	} else {
		id := path
		if id == "" || strings.Contains(id, "/") {
			writeError(w, http.StatusBadRequest, "invalid order id")
			return
		}
		order, err = h.orders.Order(r.Context(), id)
	}

	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "order not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "order lookup failed")
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	defer r.Body.Close()

	var req service.CancelRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	resp, err := h.orders.CancelOrder(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) closeDeal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	defer r.Body.Close()
	var req service.CloseDealRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	results, err := h.orders.CloseDeals(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "results": results})
}

func (h *Handler) positionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	positions, err := h.positions.GetAllPositions(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"positions": positions})
}

func (h *Handler) positionBySymbol(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	symbol := strings.TrimPrefix(r.URL.Path, "/position/")
	if symbol == "" || strings.Contains(symbol, "/") {
		writeError(w, http.StatusBadRequest, "invalid symbol")
		return
	}
	position, err := h.positions.GetCurrentPosition(r.Context(), symbol)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, position)
}

func (h *Handler) killSwitch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, map[string]any{"enabled": h.orders.IsEnabled()})
		return
	}
	defer r.Body.Close()

	var req killSwitchRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	h.orders.SetEnabled(r.Context(), req.Enabled)
	writeJSON(w, http.StatusOK, map[string]any{"enabled": h.orders.IsEnabled()})
}

func (h *Handler) loanPackages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if h.dnse == nil {
		writeError(w, http.StatusServiceUnavailable, "dnse client is not configured")
		return
	}

	accountNo := r.URL.Query().Get("accountNo")
	symbol := r.URL.Query().Get("symbol")
	marketType := r.URL.Query().Get("marketType")
	packages, err := h.dnse.GetLoanPackages(r.Context(), accountNo, symbol, marketType)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"loanPackages": packages})
}

func (h *Handler) ppse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if h.dnse == nil {
		writeError(w, http.StatusServiceUnavailable, "dnse client is not configured")
		return
	}

	accountNo := r.URL.Query().Get("accountNo")
	symbol := r.URL.Query().Get("symbol")
	marketType := r.URL.Query().Get("marketType")
	loanPackageID, err := parsePositiveInt(r.URL.Query().Get("loanPackageId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "loanPackageId must be a positive integer")
		return
	}
	price, err := parsePositiveFloat(r.URL.Query().Get("price"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "price must be a positive number")
		return
	}
	ppse, err := h.dnse.GetPPSE(r.Context(), accountNo, symbol, marketType, loanPackageID, price)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ppse)
}

func (h *Handler) derivativeSymbols(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if h.symbols == nil {
		writeError(w, http.StatusServiceUnavailable, "symbol catalog service is not configured")
		return
	}
	items, err := h.symbols.GetDerivativeSymbols(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"total": len(items),
		"data":  items,
	})
}

func (h *Handler) instrumentSymbols(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if h.symbols == nil {
		writeError(w, http.StatusServiceUnavailable, "symbol catalog service is not configured")
		return
	}

	exchanges := strings.Split(strings.TrimSpace(r.URL.Query().Get("exchange")), ",")
	if len(exchanges) == 1 && strings.TrimSpace(exchanges[0]) == "" {
		exchanges = []string{"HOSE", "HNX", "UPCOM"}
	}

	type instrumentFetcher interface {
		GetInstrumentSymbols(ctx context.Context, exchanges []string) ([]service.InstrumentSymbolInfo, error)
	}
	fetcher, ok := any(h.symbols).(instrumentFetcher)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "instrument catalog is not supported")
		return
	}

	items, err := fetcher.GetInstrumentSymbols(r.Context(), exchanges)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"total":     len(items),
		"exchanges": exchanges,
		"data":      items,
	})
}

func (h *Handler) tickerSymbols(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if h.symbols == nil {
		writeError(w, http.StatusServiceUnavailable, "symbol catalog service is not configured")
		return
	}

	type tickerFetcher interface {
		GetTickerMetadata(ctx context.Context, forceRefresh bool) ([]storage.TickerMetadataRecord, error)
	}
	fetcher, ok := any(h.symbols).(tickerFetcher)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "ticker catalog is not supported")
		return
	}

	forceRefresh := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("refresh")), "1") ||
		strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("refresh")), "true")
	items, err := fetcher.GetTickerMetadata(r.Context(), forceRefresh)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"total":   len(items),
		"refresh": forceRefresh,
		"data":    items,
	})
}

func (h *Handler) mt5Layout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if h.symbols == nil {
		writeError(w, http.StatusServiceUnavailable, "symbol catalog service is not configured")
		return
	}

	type layoutFetcher interface {
		GetMT5Layouts(ctx context.Context, symbols []string) ([]service.MT5SymbolLayout, error)
	}
	fetcher, ok := any(h.symbols).(layoutFetcher)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "mt5 layout is not supported")
		return
	}

	symbols := make([]string, 0, len(h.profiles))
	seen := make(map[string]struct{}, len(h.profiles))
	for _, profile := range h.profiles {
		symbol := strings.ToUpper(strings.TrimSpace(profile.Symbol))
		if symbol == "" {
			continue
		}
		if _, exists := seen[symbol]; exists {
			continue
		}
		seen[symbol] = struct{}{}
		symbols = append(symbols, symbol)
	}

	items, err := fetcher.GetMT5Layouts(r.Context(), symbols)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	if strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("format")), "json") {
		writeJSON(w, http.StatusOK, map[string]any{
			"total": len(items),
			"data":  items,
		})
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	var builder strings.Builder
	for _, item := range items {
		builder.WriteString(item.Symbol)
		builder.WriteByte('\t')
		builder.WriteString(item.GroupPath)
		builder.WriteByte('\t')
		builder.WriteString(strings.ReplaceAll(item.Description, "\t", " "))
		builder.WriteByte('\t')
		builder.WriteString(fmt.Sprintf("%d", item.Digits))
		builder.WriteByte('\t')
		builder.WriteString(item.Point)
		builder.WriteByte('\n')
	}
	_, _ = io.WriteString(w, builder.String())
}

func (h *Handler) symbolProfiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"total":    len(h.profiles),
		"profiles": h.profiles,
	})
}

func (h *Handler) registrationTradingToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if h.dnse == nil {
		writeError(w, http.StatusServiceUnavailable, "dnse client is not configured")
		return
	}
	defer r.Body.Close()

	var req tradingTokenRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	token, expiresAt, err := h.dnse.RegisterTradingTokenWithType(r.Context(), req.Passcode, req.OTPType)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"tradingToken": token,
		"expiresAt":    expiresAt.UTC().Format(time.RFC3339),
	})
}

func (h *Handler) refreshTradingToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if h.dnse == nil {
		writeError(w, http.StatusServiceUnavailable, "dnse client is not configured")
		return
	}
	if err := h.dnse.EnsureTradingToken(r.Context(), 8*time.Hour); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, h.dnse.TradingTokenStatus())
}

func (h *Handler) sendEmailOTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if h.dnse == nil {
		writeError(w, http.StatusServiceUnavailable, "dnse client is not configured")
		return
	}

	if err := h.dnse.SendEmailOTP(r.Context()); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "sent"})
}

func (h *Handler) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.logger.Info("incoming_request", map[string]any{
			"method":     r.Method,
			"path":       r.URL.Path,
			"remoteAddr": r.RemoteAddr,
		})
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				h.logger.Error("panic_recovered", map[string]any{"panic": rec, "path": r.URL.Path})
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) apiOK(ctx context.Context, timeout time.Duration) bool {
	if h.dnse == nil {
		return false
	}

	h.statusMu.Lock()
	if !h.lastAPIAt.IsZero() && time.Since(h.lastAPIAt) < 15*time.Second {
		cached := h.lastAPIOK
		h.statusMu.Unlock()
		return cached
	}
	h.statusMu.Unlock()

	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	_, err := h.dnse.GetAccounts(checkCtx)
	ok := err == nil

	h.statusMu.Lock()
	h.lastAPIOK = ok
	h.lastAPIAt = time.Now()
	h.statusMu.Unlock()
	return ok
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}

func parsePositiveInt(value string) (int, error) {
	var n int
	if _, err := fmt.Sscanf(strings.TrimSpace(value), "%d", &n); err != nil {
		return 0, err
	}
	if n <= 0 {
		return 0, errors.New("not positive")
	}
	return n, nil
}

func parsePositiveFloat(value string) (float64, error) {
	var n float64
	if _, err := fmt.Sscanf(strings.TrimSpace(value), "%f", &n); err != nil {
		return 0, err
	}
	if n <= 0 {
		return 0, errors.New("not positive")
	}
	return n, nil
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (h *Handler) historySync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		FirstTime  int64  `json:"firstTime"`
		LastTime   int64  `json:"lastTime"`
		Symbol     string `json:"symbol"`
		MarketType string `json:"marketType"`
		Resolution int    `json:"resolution"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if svc, ok := h.history.(interface {
		SyncWithOptions(ctx context.Context, opt marketdata.SyncOptions) (any, error)
	}); ok {
		result, err := svc.SyncWithOptions(r.Context(), marketdata.SyncOptions{
			FirstTime:  req.FirstTime,
			LastTime:   req.LastTime,
			Symbol:     req.Symbol,
			MarketType: req.MarketType,
			Resolution: req.Resolution,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, result)
		return
	}
	result, err := h.history.Sync(r.Context(), req.FirstTime, req.LastTime)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) historyFull(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		LookbackDays int    `json:"lookbackDays"`
		Symbol       string `json:"symbol"`
		MarketType   string `json:"marketType"`
		Resolution   int    `json:"resolution"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if svc, ok := h.history.(interface {
		SyncWithOptions(ctx context.Context, opt marketdata.SyncOptions) (any, error)
	}); ok {
		result, err := svc.SyncWithOptions(r.Context(), marketdata.SyncOptions{
			ForceFull:    true,
			LookbackDays: req.LookbackDays,
			Symbol:       req.Symbol,
			MarketType:   req.MarketType,
			Resolution:   req.Resolution,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, result)
		return
	}
	result, err := h.history.FullSync(r.Context(), req.LookbackDays)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) historyBackfill(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		LookbackDays int    `json:"lookbackDays"`
		Symbol       string `json:"symbol"`
		MarketType   string `json:"marketType"`
		Resolution   int    `json:"resolution"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if svc, ok := h.history.(interface {
		SyncWithOptions(ctx context.Context, opt marketdata.SyncOptions) (any, error)
	}); ok {
		result, err := svc.SyncWithOptions(r.Context(), marketdata.SyncOptions{
			ForceFull:    true,
			BeforeToday:  true,
			LookbackDays: req.LookbackDays,
			Symbol:       req.Symbol,
			MarketType:   req.MarketType,
			Resolution:   req.Resolution,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, result)
		return
	}
	writeError(w, http.StatusServiceUnavailable, "history backfill is not supported")
}

func (h *Handler) historyToday(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		Symbol     string `json:"symbol"`
		MarketType string `json:"marketType"`
		Resolution int    `json:"resolution"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if svc, ok := h.history.(interface {
		SyncWithOptions(ctx context.Context, opt marketdata.SyncOptions) (any, error)
	}); ok {
		result, err := svc.SyncWithOptions(r.Context(), marketdata.SyncOptions{
			TodayOnly:  true,
			Symbol:     req.Symbol,
			MarketType: req.MarketType,
			Resolution: req.Resolution,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, result)
		return
	}
	writeError(w, http.StatusServiceUnavailable, "today sync is not supported")
}

func (h *Handler) historyFullAll(w http.ResponseWriter, r *http.Request) {
	h.historySyncAll(w, r, "full")
}

func (h *Handler) historyBackfillAll(w http.ResponseWriter, r *http.Request) {
	h.historySyncAll(w, r, "backfill")
}

func (h *Handler) historyTodayAll(w http.ResponseWriter, r *http.Request) {
	h.historySyncAll(w, r, "today")
}

func (h *Handler) historySyncAll(w http.ResponseWriter, r *http.Request, mode string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		LookbackDays int `json:"lookbackDays"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	svc, ok := h.history.(interface {
		SyncWithOptions(ctx context.Context, opt marketdata.SyncOptions) (any, error)
	})
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "history multi-symbol sync is not supported")
		return
	}
	type historySnapshotCloner interface {
		CloneSnapshot(sourceSymbol, targetSymbol, marketType string, resolution int) bool
	}
	cloner, _ := h.history.(historySnapshotCloner)
	type item struct {
		Symbol     string `json:"symbol"`
		MarketType string `json:"marketType"`
		Resolution int    `json:"resolution"`
		Success    bool   `json:"success"`
		Message    string `json:"message"`
	}

	lookbackDays := req.LookbackDays
	if lookbackDays <= 0 {
		lookbackDays = 365
	}

	results := make([]item, 0, len(h.profiles))
	successCount := 0
	type syncJob struct {
		fetchSymbol string
		profile     marketdata.SymbolProfile
		members     []marketdata.SymbolProfile
	}
	jobsByCanonical := make(map[string]*syncJob)
	order := make([]string, 0, len(h.profiles))
	for _, profile := range h.profiles {
		canonical := strings.ToUpper(strings.TrimSpace(profile.Symbol))
		job := jobsByCanonical[canonical]
		if job == nil {
			job = &syncJob{
				fetchSymbol: canonical,
				profile: marketdata.SymbolProfile{
					Symbol:               canonical,
					AssetClass:           profile.AssetClass,
					MarketType:           profile.MarketType,
					Channels:             profile.Channels,
					Resolution:           profile.Resolution,
					BoardID:              profile.BoardID,
					SupportsRESTFallback: profile.SupportsRESTFallback,
				},
			}
			jobsByCanonical[canonical] = job
			order = append(order, canonical)
		}
		job.members = append(job.members, profile)
	}

	for _, canonical := range order {
		job := jobsByCanonical[canonical]
		opt := marketdata.SyncOptions{
			Symbol:     job.profile.Symbol,
			MarketType: job.profile.MarketType,
			Resolution: job.profile.Resolution,
		}
		switch mode {
		case "full":
			opt.ForceFull = true
			opt.LookbackDays = lookbackDays
		case "backfill":
			opt.ForceFull = true
			opt.BeforeToday = true
			opt.LookbackDays = lookbackDays
		case "today":
			opt.TodayOnly = true
		}

		res, err := svc.SyncWithOptions(r.Context(), opt)
		baseMessage := "completed"
		if payload, ok := res.(marketdata.SyncResult); ok && payload.Message != "" {
			baseMessage = payload.Message
		}
		for _, member := range job.members {
			entry := item{
				Symbol:     member.Symbol,
				MarketType: member.MarketType,
				Resolution: member.Resolution,
				Success:    err == nil,
			}
			if err != nil {
				entry.Message = err.Error()
			} else {
				if !strings.EqualFold(member.Symbol, job.fetchSymbol) && cloner != nil {
					cloner.CloneSnapshot(job.fetchSymbol, member.Symbol, member.MarketType, member.Resolution)
				}
				entry.Message = baseMessage
				if !strings.EqualFold(member.Symbol, job.fetchSymbol) {
					entry.Message = "Reused history from " + job.fetchSymbol
				}
				successCount++
			}
			results = append(results, entry)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":      successCount == len(results),
		"mode":         mode,
		"totalSymbols": len(results),
		"successCount": successCount,
		"results":      results,
	})
}

func (h *Handler) getLatestOTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if h.otp == nil {
		writeError(w, http.StatusServiceUnavailable, "otp service not enabled")
		return
	}
	code, valid := h.otp.GetLatestOTP()
	writeJSON(w, http.StatusOK, map[string]any{
		"otp":   code,
		"valid": valid,
	})
}

// --- UI Handlers ---

func (h *Handler) setupUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(setupHTML))
}

func (h *Handler) settingsUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(settingsHTML))
}

func (h *Handler) logsUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(logsHTML))
}

func (h *Handler) settingsAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		cfg, err := config.Load("config/config.yaml")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		// Mask secrets while keeping account rows visible in the WebUI.
		cfg.DNSE.APISecret = ""
		cfg.DNSE.SecretKey = ""
		cfg.DNSE.TradingToken = ""
		cfg.Entrade.Password = ""
		cfg.Entrade.TradingToken = ""
		for i := range cfg.Entrade.Accounts {
			cfg.Entrade.Accounts[i].Password = ""
			cfg.Entrade.Accounts[i].TradingToken = ""
		}
		cfg.Telemetry.APISecret = ""
		writeJSON(w, http.StatusOK, cfg)
		return
	}

	if r.Method == http.MethodPost {
		var req struct {
			APIKey                   string                        `json:"apiKey"`
			APISecret                string                        `json:"apiSecret"`
			AccountNo                string                        `json:"accountNo"`
			Mock                     bool                          `json:"mock"`
			Symbols                  []string                      `json:"symbols"`
			PrimarySymbol            string                        `json:"primarySymbol"`
			EntradeEnabled           bool                          `json:"entradeEnabled"`
			EntradeMock              bool                          `json:"entradeMock"`
			EntradeEnvironment       string                        `json:"entradeEnvironment"`
			EntradeDefaultAccountNos []string                      `json:"entradeDefaultAccountNos"`
			EntradeAccounts          []config.EntradeAccountConfig `json:"entradeAccounts"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json")
			return
		}

		cfg, err := config.Load("config/config.yaml")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		if req.APIKey != "" {
			cfg.DNSE.APIKey = req.APIKey
		}
		if req.APISecret != "" {
			cfg.DNSE.APISecret = req.APISecret
			cfg.DNSE.SecretKey = req.APISecret
		}
		if req.AccountNo != "" {
			cfg.DNSE.AccountNo = req.AccountNo
		}
		cfg.DNSE.Mock = req.Mock
		cfg.Entrade.Enabled = req.EntradeEnabled
		cfg.Entrade.Mock = req.EntradeMock
		if strings.TrimSpace(req.EntradeEnvironment) != "" {
			cfg.Entrade.Environment = strings.ToLower(strings.TrimSpace(req.EntradeEnvironment))
		}
		if req.EntradeDefaultAccountNos != nil {
			cfg.Entrade.DefaultAccountNos = normalizeAPIAccountNos(req.EntradeDefaultAccountNos)
		}
		if req.EntradeAccounts != nil {
			cfg.Entrade.Accounts = mergeEntradeAccountsForSave(req.EntradeAccounts, cfg.Entrade.Accounts)
		}
		if len(req.Symbols) > 0 {
			cfg.MarketData.Symbols = h.canonicalizeSymbols(r.Context(), req.Symbols)
		}
		if req.PrimarySymbol != "" {
			primary := req.PrimarySymbol
			canonicalPrimary := h.canonicalizeSymbols(r.Context(), []string{primary})
			if len(canonicalPrimary) > 0 {
				primary = canonicalPrimary[0]
			}
			cfg.MarketData.Symbol = primary
			cfg.History.Symbol = primary
			if profile, ok := marketdata.InferSymbolProfile(primary, cfg.MarketData.Channels); ok {
				cfg.History.MarketType = profile.MarketType
				cfg.History.Resolution = profile.Resolution
			}
		}

		if err := cfg.Save("config/config.yaml"); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save config")
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
		return
	}
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func (h *Handler) entradeLinkAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	defer r.Body.Close()

	var req struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		Environment string `json:"environment"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)
	req.Environment = strings.ToLower(strings.TrimSpace(req.Environment))
	if req.Environment == "" {
		req.Environment = "real"
	}
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	linkCfg := cfg.Entrade
	linkCfg.Enabled = true
	linkCfg.Mock = false
	linkCfg.Environment = req.Environment
	linkCfg.Username = req.Username
	linkCfg.Password = req.Password

	client := entrade.NewClient(linkCfg, h.logger)
	result, err := client.LinkAccount(r.Context(), req.Username, req.Password, req.Environment)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	accountID := strings.TrimSpace(result.InvestorAccountID)
	if accountID == "" {
		accountID = strings.TrimSpace(result.InvestorID)
	}
	cfg.Entrade.Enabled = true
	cfg.Entrade.Mock = false
	cfg.Entrade.Environment = result.Environment
	cfg.Entrade.Username = req.Username
	cfg.Entrade.Password = req.Password
	cfg.Entrade.InvestorID = result.InvestorID
	cfg.Entrade.AccountNo = accountID
	cfg.Entrade.DefaultAccountNos = []string{entrade.AccountReal}
	cfg.Entrade.Accounts = []config.EntradeAccountConfig{{
		ID:          entrade.AccountReal,
		Environment: result.Environment,
		Username:    req.Username,
		Password:    req.Password,
		InvestorID:  result.InvestorID,
		AccountNo:   accountID,
		Enabled:     true,
	}}
	if err := cfg.Save("config/config.yaml"); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save config")
		return
	}
	if h.telemetry != nil {
		h.telemetry.Track(r.Context(), "entrade_linked", map[string]any{
			"environment": result.Environment,
			"status":      result.Status,
			"packages":    len(result.LoanPackages),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"account": result,
	})
}

func (h *Handler) canonicalizeSymbols(ctx context.Context, symbols []string) []string {
	_ = ctx
	out := make([]string, 0, len(symbols))
	seen := make(map[string]struct{}, len(symbols))

	for _, symbol := range symbols {
		symbol = strings.ToUpper(strings.TrimSpace(symbol))
		if strings.HasPrefix(symbol, "VN100F") {
			symbol = "V100F" + strings.TrimPrefix(symbol, "VN100F")
		}
		if symbol == "" {
			continue
		}
		if _, ok := seen[symbol]; ok {
			continue
		}
		seen[symbol] = struct{}{}
		out = append(out, symbol)
	}
	return out
}

func normalizeAPIAccountNos(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.ToUpper(strings.TrimSpace(part))
			if part == "" {
				continue
			}
			if _, ok := seen[part]; ok {
				continue
			}
			seen[part] = struct{}{}
			out = append(out, part)
		}
	}
	return out
}

func mergeEntradeAccountsForSave(incoming, existing []config.EntradeAccountConfig) []config.EntradeAccountConfig {
	existingByID := map[string]config.EntradeAccountConfig{}
	for _, account := range existing {
		id := strings.ToUpper(strings.TrimSpace(account.ID))
		if id != "" {
			existingByID[id] = account
		}
	}
	out := make([]config.EntradeAccountConfig, 0, len(incoming))
	seen := map[string]struct{}{}
	for _, account := range incoming {
		account.ID = strings.ToUpper(strings.TrimSpace(account.ID))
		if account.ID == "" {
			continue
		}
		if _, ok := seen[account.ID]; ok {
			continue
		}
		seen[account.ID] = struct{}{}
		account.Environment = strings.ToLower(strings.TrimSpace(account.Environment))
		if account.Environment == "" {
			account.Environment = "paper"
		}
		account.Username = strings.TrimSpace(account.Username)
		account.Password = strings.TrimSpace(account.Password)
		account.InvestorID = strings.TrimSpace(account.InvestorID)
		account.AccountNo = strings.TrimSpace(account.AccountNo)
		account.TradingToken = strings.TrimSpace(account.TradingToken)
		if old, ok := existingByID[account.ID]; ok {
			if account.Password == "" {
				account.Password = old.Password
			}
			if account.TradingToken == "" {
				account.TradingToken = old.TradingToken
			}
		}
		out = append(out, account)
	}
	return out
}

func (h *Handler) logsRawAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if h.logger == nil {
		http.Error(w, "Logger is not available", http.StatusServiceUnavailable)
		return
	}

	data, err := h.logger.ReadTail(128 * 1024)
	if err != nil {
		http.Error(w, "Failed to read logs: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write(data)
}

func (h *Handler) setupInstallAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	folders, err := setup.DetectMT5Folders()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to detect MT5: "+err.Error())
		return
	}
	if len(folders) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"success": false, "message": "No MetaTrader 5 folders detected at standard paths."})
		return
	}

	// Just install into the first detected folder for MVP
	logs, err := setup.InstallFiles(folders[0].Path, h.logger)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"success": false, "message": "Failed to install: " + err.Error(), "logs": logs})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true, "message": "Successfully copied files.", "logs": logs})
}

func (h *Handler) supportExport(w http.ResponseWriter, r *http.Request) {
	data, err := setup.ExportSupportPackage("config/config.yaml", "logs/app.jsonl")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create support package: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\"support_package.zip\"")
	w.Write(data)
}
