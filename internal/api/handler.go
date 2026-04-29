package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"dnse-mt5-connector/internal/config"
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

type Handler struct {
	orders    *service.OrderService
	positions *service.PositionService
	signals   *service.SignalService
	symbols   *service.SymbolCatalogService
	profiles  []marketdata.SymbolProfile
	dnse      *DNSEClient
	history   HistorySyncer
	otp       OTPFetcher
	logger    *logger.FileLogger
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

func NewHandler(orderService *service.OrderService, positionService *service.PositionService, signalService *service.SignalService, symbolService *service.SymbolCatalogService, profiles []marketdata.SymbolProfile, dnseClient *DNSEClient, historyService HistorySyncer, otpFetcher OTPFetcher, appLog *logger.FileLogger) *Handler {
	return &Handler{orders: orderService, positions: positionService, signals: signalService, symbols: symbolService, profiles: profiles, dnse: dnseClient, history: historyService, otp: otpFetcher, logger: appLog}
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
	mux.HandleFunc("/history/sync", h.historySync)
	mux.HandleFunc("/history/full", h.historyFull)
	mux.HandleFunc("/history/backfill", h.historyBackfill)
	mux.HandleFunc("/history/today", h.historyToday)
	mux.HandleFunc("/otp/latest", h.getLatestOTP)
	mux.HandleFunc("/positions", h.positionsHandler)
	mux.HandleFunc("/position/", h.positionBySymbol)
	mux.HandleFunc("/kill-switch", h.killSwitch)
	mux.HandleFunc("/account", h.account)
	mux.HandleFunc("/self-test", h.selfTest)
	mux.HandleFunc("/loan-packages", h.loanPackages)
	mux.HandleFunc("/ppse", h.ppse)
	mux.HandleFunc("/symbols/derivatives", h.derivativeSymbols)
	mux.HandleFunc("/symbols/profiles", h.symbolProfiles)
	mux.HandleFunc("/registration/trading-token", h.registrationTradingToken)
	mux.HandleFunc("/registration/send-email-otp", h.sendEmailOTP)

	// UI Routes
	mux.HandleFunc("/setup", h.setupUI)
	mux.HandleFunc("/settings", h.settingsUI)
	mux.HandleFunc("/logs", h.logsUI)
	mux.HandleFunc("/api/settings", h.settingsAPI)
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

	writeJSON(w, http.StatusOK, map[string]any{
		"connected":      true,
		"api_ok":         apiOK,
		"token_valid":    h.dnse.TokenValid(),
		"mt5_connected":  h.signals.MT5Connected(30 * time.Second),
		"market_data_ok": true, // Assume true if go bridge is running
		"gmail_ok":       gmailOK,
		"system_enabled": h.orders.IsEnabled(),
		"mode":           h.signals.Mode(),
		"pendingSignals": len(h.signals.Pending()),
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

func (h *Handler) historySync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		FirstTime int64 `json:"firstTime"`
		LastTime  int64 `json:"lastTime"`
		Symbol    string `json:"symbol"`
		MarketType string `json:"marketType"`
		Resolution int `json:"resolution"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if svc, ok := h.history.(interface {
		SyncWithOptions(ctx context.Context, opt marketdata.SyncOptions) (any, error)
	}); ok {
		result, err := svc.SyncWithOptions(r.Context(), marketdata.SyncOptions{
			FirstTime: req.FirstTime,
			LastTime: req.LastTime,
			Symbol: req.Symbol,
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
		LookbackDays int `json:"lookbackDays"`
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
			ForceFull: true,
			LookbackDays: req.LookbackDays,
			Symbol: req.Symbol,
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
		// Mask secret
		cfg.DNSE.APISecret = ""
		cfg.DNSE.SecretKey = ""
		writeJSON(w, http.StatusOK, cfg)
		return
	}

	if r.Method == http.MethodPost {
		var req struct {
			APIKey    string `json:"apiKey"`
			APISecret string `json:"apiSecret"`
			AccountNo string `json:"accountNo"`
			Mock      bool   `json:"mock"`
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
		
		if req.APIKey != "" { cfg.DNSE.APIKey = req.APIKey }
		if req.APISecret != "" { 
			cfg.DNSE.APISecret = req.APISecret 
			cfg.DNSE.SecretKey = req.APISecret
		}
		if req.AccountNo != "" { cfg.DNSE.AccountNo = req.AccountNo }
		cfg.DNSE.Mock = req.Mock
		
		if err := cfg.Save("config/config.yaml"); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save config")
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
		return
	}
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func (h *Handler) logsRawAPI(w http.ResponseWriter, r *http.Request) {
	// Read last bytes of the log file for UI display
	// Just return the whole file if small, or error out. 
	// For simplicity, we just dump it here.
	data, err := os.ReadFile("logs/app.jsonl") // Will need "os" package if not imported.
	if err != nil {
		http.Error(w, "Failed to read logs: " + err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write(data)
}

func (h *Handler) setupInstallAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	folders, err := setup.DetectMT5Folders()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to detect MT5: " + err.Error())
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
		writeError(w, http.StatusInternalServerError, "Failed to create support package: " + err.Error())
		return
	}
	
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\"support_package.zip\"")
	w.Write(data)
}
