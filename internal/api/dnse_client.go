package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"dnse-mt5-connector/internal/config"
	"dnse-mt5-connector/internal/dnsemodel"
	"dnse-mt5-connector/internal/logger"
	"dnse-mt5-connector/internal/storage"
)

type tokenStore interface {
	SaveTradingToken(ctx context.Context, token string, expiresAt time.Time) error
	GetTradingToken(ctx context.Context) (storage.TradingToken, error)
}

type OTPFetcher interface {
	GetLatestOTP() (string, bool)
}

type DNSEClient struct {
	cfg       config.DNSEConfig
	http      *http.Client
	logger    *logger.FileLogger
	store     tokenStore
	otp       OTPFetcher
	mu        sync.RWMutex
	token     string
	expiresAt time.Time
	passcode  string
	otpType   string
}

type TradingTokenStatus struct {
	Valid            bool      `json:"valid"`
	ExpiresAt        time.Time `json:"expiresAt,omitempty"`
	RemainingSeconds int64     `json:"remainingSeconds"`
}

type tradingTokenResponse struct {
	TradingToken string `json:"tradingToken"`
	Token        string `json:"token"`
	ExpiresIn    int    `json:"expiresIn"`
}

type InstrumentInfo struct {
	Symbol   string `json:"symbol"`
	Exchange string `json:"exchange"`
	Type     string `json:"type,omitempty"`
}

type TickerInfo struct {
	Symbol      string `json:"symbol"`
	FeedSymbol  string `json:"feedSymbol"`
	Exchange    string `json:"exchange"`
	Type        string `json:"type,omitempty"`
	BoardID     string `json:"boardId,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	RawJSON     string `json:"rawJson,omitempty"`
}

func NewDNSEClient(cfg config.DNSEConfig, appLog *logger.FileLogger, store tokenStore) *DNSEClient {
	tokenTTL := time.Time{}
	if cfg.TradingToken != "" {
		tokenTTL = time.Now().Add(24 * time.Hour)
	}
	return &DNSEClient{
		cfg: cfg,
		http: &http.Client{
			Timeout: 15 * time.Second,
		},
		logger:    appLog,
		store:     store,
		token:     cfg.TradingToken,
		expiresAt: tokenTTL,
	}
}

func (c *DNSEClient) SetOTPFetcher(otp OTPFetcher) {
	c.otp = otp
}

func (c *DNSEClient) LoadPersistedToken(ctx context.Context) {
	if c.store == nil {
		return
	}
	token, err := c.store.GetTradingToken(ctx)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			c.logger.Error("dnse_token_load_failed", map[string]any{"error": err.Error()})
		}
		return
	}
	if token.Token == "" || time.Now().UTC().After(token.ExpiresAt.Add(-30*time.Second)) {
		c.logger.Info("dnse_token_ignored", map[string]any{"reason": "missing_or_expired", "expiresAt": token.ExpiresAt.UTC().Format(time.RFC3339)})
		return
	}
	c.setToken(token.Token, token.ExpiresAt)
	c.logger.Info("dnse_token_loaded", map[string]any{"expiresAt": token.ExpiresAt.UTC().Format(time.RFC3339)})
}

func (c *DNSEClient) TokenValid() bool {
	if c.cfg.Mock {
		return true
	}
	token, expiresAt := c.tokenState()
	return token != "" && time.Now().UTC().Before(expiresAt.Add(-30*time.Second))
}

func (c *DNSEClient) TradingTokenStatus() TradingTokenStatus {
	if c.cfg.Mock {
		return TradingTokenStatus{Valid: true, ExpiresAt: time.Now().UTC().Add(8 * time.Hour), RemainingSeconds: int64((8 * time.Hour).Seconds())}
	}
	token, expiresAt := c.tokenState()
	remaining := int64(0)
	if !expiresAt.IsZero() {
		remaining = int64(time.Until(expiresAt).Seconds())
		if remaining < 0 {
			remaining = 0
		}
	}
	return TradingTokenStatus{
		Valid:            token != "" && time.Now().UTC().Before(expiresAt.Add(-30*time.Second)),
		ExpiresAt:        expiresAt,
		RemainingSeconds: remaining,
	}
}

func (c *DNSEClient) EnsureTradingToken(ctx context.Context, minValidity time.Duration) error {
	if c.cfg.Mock {
		return nil
	}
	if minValidity <= 0 {
		minValidity = 30 * time.Second
	}
	token, expiresAt := c.tokenState()
	if token != "" && time.Now().UTC().Before(expiresAt.Add(-minValidity)) {
		return nil
	}
	return c.refreshToken(ctx)
}

func (c *DNSEClient) SendEmailOTP(ctx context.Context) error {
	if c.cfg.Mock {
		c.logger.Info("dnse_send_email_otp", map[string]any{"mock": true})
		return nil
	}
	return c.do(ctx, http.MethodPost, "/registration/send-email-otp", nil, nil, false, true)
}

func (c *DNSEClient) RegisterTradingToken(ctx context.Context, passcode string) (string, time.Time, error) {
	return c.RegisterTradingTokenWithType(ctx, passcode, "email_otp")
}

func (c *DNSEClient) RegisterTradingTokenWithType(ctx context.Context, passcode, otpType string) (string, time.Time, error) {
	passcode = strings.TrimSpace(passcode)
	if passcode == "" {
		return "", time.Time{}, errors.New("passcode is required")
	}
	otpType = strings.TrimSpace(otpType)
	if otpType == "" {
		otpType = "email_otp"
	}

	c.mu.Lock()
	c.passcode = passcode
	c.otpType = otpType
	c.mu.Unlock()

	if c.cfg.Mock {
		token := fmt.Sprintf("mock-token-%d", time.Now().Unix())
		expiresAt := time.Now().Add(30 * time.Minute)
		c.setToken(token, expiresAt)
		c.persistToken(context.Background(), token, expiresAt)
		c.logger.Info("dnse_token_registered", map[string]any{"mock": true, "expiresAt": expiresAt.UTC().Format(time.RFC3339)})
		return token, expiresAt, nil
	}

	token, expiresAt, err := c.fetchTradingToken(ctx, passcode, otpType)
	if err != nil {
		return "", time.Time{}, err
	}
	c.setToken(token, expiresAt)
	c.persistToken(ctx, token, expiresAt)
	return token, expiresAt, nil
}

func (c *DNSEClient) GetAccounts(ctx context.Context) ([]dnsemodel.Account, error) {
	if c.cfg.Mock {
		c.logger.Info("dnse_api_request", map[string]any{"method": http.MethodGet, "path": "/accounts", "mock": true})
		accounts := []dnsemodel.Account{{
			AccountNo:               defaultString(c.cfg.AccountNo, "MOCK001"),
			DerivativeAccountStatus: "ACTIVE",
		}}
		c.logger.Info("dnse_api_response", map[string]any{"path": "/accounts", "statusCode": 200, "mock": true})
		return accounts, nil
	}
	if err := c.validateCredentials(); err != nil {
		return nil, err
	}

	var raw any
	if err := c.do(ctx, http.MethodGet, "/accounts", nil, &raw, false, true); err != nil {
		return nil, err
	}
	return simplifyAccounts(raw, c.cfg.AccountNo), nil
}

func (c *DNSEClient) PlaceOrder(ctx context.Context, req dnsemodel.PlaceOrderRequest) (dnsemodel.PlaceOrderResponse, error) {
	if c.cfg.Mock {
		c.logger.Info("dnse_api_request", map[string]any{"method": http.MethodPost, "path": "/orders", "mock": true})
		resp := dnsemodel.PlaceOrderResponse{
			OrderID: fmt.Sprintf("MOCK-%d", time.Now().UnixNano()),
			Status:  "ACCEPTED",
		}
		c.logger.Info("dnse_api_response", map[string]any{"path": "/orders", "statusCode": 200, "mock": true, "orderId": resp.OrderID})
		return resp, nil
	}

	var raw map[string]any
	marketType := defaultString(req.MarketType, "STOCK")
	orderCategory := defaultString(req.OrderCategory, "NORMAL")
	path := fmt.Sprintf("/accounts/orders?marketType=%s&orderCategory=%s", url.QueryEscape(marketType), url.QueryEscape(orderCategory))
	if err := c.do(ctx, http.MethodPost, path, req, &raw, true, true); err != nil {
		return dnsemodel.PlaceOrderResponse{}, err
	}
	rawBody, _ := json.Marshal(raw)
	orderID := firstString(raw, "orderId", "id")
	if orderID == "" {
		return dnsemodel.PlaceOrderResponse{}, errors.New("dnse order response missing id")
	}
	return dnsemodel.PlaceOrderResponse{
		OrderID:     orderID,
		Status:      defaultString(firstString(raw, "status", "orderStatus"), "ACCEPTED"),
		RawResponse: string(rawBody),
	}, nil
}

func (c *DNSEClient) GetOrderStatus(ctx context.Context, accountNo, orderID, marketType, orderCategory string) (dnsemodel.OrderStatus, error) {
	if strings.TrimSpace(orderID) == "" {
		return dnsemodel.OrderStatus{}, errors.New("order id is required")
	}
	if c.cfg.Mock {
		c.logger.Info("dnse_api_request", map[string]any{"method": http.MethodGet, "path": "/orders/" + orderID, "mock": true})
		status := dnsemodel.OrderStatus{OrderID: orderID, Status: "PENDING", FilledQuantity: 0, RemainingQuantity: 1}
		c.logger.Info("dnse_api_response", map[string]any{"path": "/orders/" + orderID, "statusCode": 200, "mock": true})
		return status, nil
	}
	if strings.TrimSpace(accountNo) == "" {
		return dnsemodel.OrderStatus{}, errors.New("accountNo is required for order lookup")
	}
	marketType = strings.ToUpper(strings.TrimSpace(defaultString(marketType, "STOCK")))
	orderCategory = strings.ToUpper(strings.TrimSpace(defaultString(orderCategory, "NORMAL")))

	var raw map[string]any
	path := fmt.Sprintf("/accounts/%s/orders/%s?marketType=%s&orderCategory=%s",
		url.PathEscape(accountNo),
		url.PathEscape(orderID),
		url.QueryEscape(marketType),
		url.QueryEscape(orderCategory),
	)
	if err := c.do(ctx, http.MethodGet, path, nil, &raw, false, true); err != nil {
		return dnsemodel.OrderStatus{}, err
	}
	rawBody, _ := json.Marshal(raw)
	return dnsemodel.OrderStatus{
		OrderID:           orderID,
		Status:            defaultString(firstString(raw, "status", "orderStatus"), "UNKNOWN"),
		FilledQuantity:    firstInt(raw, "filledQuantity", "fillQuantity"),
		RemainingQuantity: firstInt(raw, "remainingQuantity", "leaveQuantity"),
		RawResponse:       string(rawBody),
	}, nil
}

func (c *DNSEClient) CancelOrder(ctx context.Context, accountNo, orderID, marketType, orderCategory string) (dnsemodel.CancelOrderResponse, error) {
	accountNo = strings.TrimSpace(accountNo)
	orderID = strings.TrimSpace(orderID)
	marketType = strings.ToUpper(strings.TrimSpace(defaultString(marketType, "DERIVATIVE")))
	orderCategory = strings.ToUpper(strings.TrimSpace(defaultString(orderCategory, "NORMAL")))
	if accountNo == "" {
		return dnsemodel.CancelOrderResponse{}, errors.New("accountNo is required")
	}
	if orderID == "" {
		return dnsemodel.CancelOrderResponse{}, errors.New("orderId is required")
	}
	if c.cfg.Mock {
		c.logger.Info("dnse_api_request", map[string]any{"method": http.MethodDelete, "path": "/orders/" + orderID, "mock": true})
		return dnsemodel.CancelOrderResponse{Success: true, OrderID: orderID, Status: "CANCELLED"}, nil
	}

	var raw map[string]any
	path := fmt.Sprintf("/accounts/%s/orders/%s?marketType=%s&orderCategory=%s",
		url.PathEscape(accountNo),
		url.PathEscape(orderID),
		url.QueryEscape(marketType),
		url.QueryEscape(orderCategory),
	)
	if err := c.do(ctx, http.MethodDelete, path, nil, &raw, true, true); err != nil {
		return dnsemodel.CancelOrderResponse{}, err
	}
	rawBody, _ := json.Marshal(raw)
	return dnsemodel.CancelOrderResponse{
		Success:           true,
		OrderID:           defaultString(firstString(raw, "orderId", "id"), orderID),
		Status:            defaultString(firstString(raw, "status", "orderStatus"), "CANCELLED"),
		FilledQuantity:    firstInt(raw, "filledQuantity", "fillQuantity"),
		RemainingQuantity: firstInt(raw, "remainingQuantity", "leaveQuantity"),
		RawResponse:       string(rawBody),
	}, nil
}

func (c *DNSEClient) GetLoanPackages(ctx context.Context, accountNo, symbol, marketType string) ([]dnsemodel.LoanPackage, error) {
	accountNo = strings.TrimSpace(accountNo)
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	marketType = strings.ToUpper(strings.TrimSpace(defaultString(marketType, "STOCK")))
	if accountNo == "" {
		return nil, errors.New("accountNo is required")
	}
	if symbol == "" {
		return nil, errors.New("symbol is required")
	}
	if c.cfg.Mock {
		return []dnsemodel.LoanPackage{{ID: 1, Name: "Mock package", Type: marketType}}, nil
	}

	path := fmt.Sprintf("/accounts/%s/loan-packages?marketType=%s&symbol=%s",
		url.PathEscape(accountNo),
		url.QueryEscape(marketType),
		url.QueryEscape(symbol),
	)
	var raw any
	if err := c.do(ctx, http.MethodGet, path, nil, &raw, false, true); err != nil {
		return nil, err
	}
	return simplifyLoanPackages(raw), nil
}

func (c *DNSEClient) FetchOHLC(ctx context.Context, symbol, marketType string, resolution int, from, to int64) (map[string]any, error) {
	if c.cfg.Mock {
		var tList, oList, hList, lList, cList, vList []any
		// Generate 1 candle per hour for the requested range (to avoid huge payloads in mock)
		for t := from; t <= to; t += 3600 {
			tList = append(tList, float64(t))
			oList = append(oList, 1200.0)
			hList = append(hList, 1200.0)
			lList = append(lList, 1200.0)
			cList = append(cList, 1200.0)
			vList = append(vList, float64(1))
		}
		return map[string]any{
			"t": tList,
			"o": oList,
			"h": hList,
			"l": lList,
			"c": cList,
			"v": vList,
		}, nil
	}
	path := fmt.Sprintf("/price/ohlc?symbol=%s&type=%s&resolution=%d&from=%d&to=%d",
		url.QueryEscape(symbol),
		url.QueryEscape(marketType),
		resolution,
		from,
		to,
	)
	var raw map[string]any
	if err := c.do(ctx, http.MethodGet, path, nil, &raw, false, true); err != nil {
		return nil, err
	}
	return raw, nil
}

func (c *DNSEClient) GetSecurityDefinition(ctx context.Context, symbol string) ([]map[string]any, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return nil, errors.New("symbol is required")
	}
	if c.cfg.Mock {
		return []map[string]any{{
			"symbol":  symbol,
			"boardId": "G1",
		}}, nil
	}

	path := fmt.Sprintf("/price/%s/secdef", url.PathEscape(symbol))
	var raw any
	if err := c.do(ctx, http.MethodGet, path, nil, &raw, false, true); err != nil {
		return nil, err
	}
	switch v := raw.(type) {
	case []any:
		return mapsFromSlice(v), nil
	case map[string]any:
		for _, key := range []string{"data", "items", "securities", "results"} {
			if arr, ok := v[key].([]any); ok {
				return mapsFromSlice(arr), nil
			}
		}
		return []map[string]any{v}, nil
	default:
		return nil, nil
	}
}

func (c *DNSEClient) GetInstruments(ctx context.Context, exchange string) ([]InstrumentInfo, error) {
	exchange = strings.ToUpper(strings.TrimSpace(exchange))
	if exchange == "" {
		return nil, errors.New("exchange is required")
	}
	if c.cfg.Mock {
		return []InstrumentInfo{
			{Symbol: "HPG", Exchange: exchange},
			{Symbol: "FPT", Exchange: exchange},
			{Symbol: "SSI", Exchange: exchange},
		}, nil
	}

	var lastErr error
	builders := []func(page, pageSize int) string{
		func(page, pageSize int) string {
			return fmt.Sprintf("/instruments?exchange=%s&page=%d&pageSize=%d", url.QueryEscape(exchange), page, pageSize)
		},
		func(page, pageSize int) string {
			return fmt.Sprintf("/instruments?floor=%s&page=%d&pageSize=%d", url.QueryEscape(exchange), page, pageSize)
		},
		func(page, pageSize int) string {
			return fmt.Sprintf("/instruments?market=%s&page=%d&pageSize=%d", url.QueryEscape(exchange), page, pageSize)
		},
	}

	for _, buildPath := range builders {
		items, ok, err := c.fetchPagedInstruments(ctx, exchange, buildPath)
		if err != nil {
			lastErr = err
			continue
		}
		if ok && len(items) > 0 {
			return mergeInstrumentDefaults(exchange, items), nil
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return mergeInstrumentDefaults(exchange, nil), nil
}

func (c *DNSEClient) GetTickers(ctx context.Context, symbol string) ([]TickerInfo, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if c.cfg.Mock {
		return []TickerInfo{
			{Symbol: "VNINDEX", FeedSymbol: "VNINDEX", Exchange: "INDEX", Type: "INDEX", Name: "VNINDEX", Description: "Chỉ số của toàn bộ cổ phiếu đang niêm yết trên HSX"},
			{Symbol: "VN30", FeedSymbol: "VN30", Exchange: "INDEX", Type: "INDEX", Name: "VN30", Description: "Chỉ số của 30 cổ phiếu có vốn hoá và thanh khoản lớn nhất trên HSX"},
			{Symbol: "HPG", FeedSymbol: "HPG", Exchange: "HOSE", Type: "STOCK", BoardID: "G1", Name: "HPG"},
			{Symbol: "VN30F1M", FeedSymbol: "41I1G5000", Exchange: "DERIVATIVE", Type: "DERIVATIVE", BoardID: "G1", Name: "VN30F1M"},
		}, nil
	}

	baseURL := "https://api.dnse.com.vn/market-api/tickers?symbol=" + url.QueryEscape(symbol)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("dnse ticker api returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var raw any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	return simplifyTickers(raw), nil
}

func (c *DNSEClient) fetchPagedInstruments(ctx context.Context, exchange string, buildPath func(page, pageSize int) string) ([]InstrumentInfo, bool, error) {
	const pageSize = 500

	seen := make(map[string]InstrumentInfo)
	var previousPageSignature string

	for page := 1; page <= 20; page++ {
		path := buildPath(page, pageSize)
		var raw any
		if err := c.do(ctx, http.MethodGet, path, nil, &raw, false, true); err != nil {
			if page == 1 {
				return nil, false, err
			}
			break
		}

		items := simplifyInstruments(raw, exchange)
		if len(items) == 0 {
			break
		}

		signature := instrumentPageSignature(items)
		if signature != "" && signature == previousPageSignature {
			break
		}
		previousPageSignature = signature

		for _, item := range items {
			seen[item.Symbol] = item
		}

		total, currentPage, currentPageSize, hasNext := extractInstrumentPagination(raw)
		if total > 0 && len(seen) >= total {
			break
		}
		if hasNext {
			continue
		}
		if currentPage > 0 && currentPageSize > 0 && currentPage >= 1 && len(items) < currentPageSize {
			break
		}
		if len(items) < pageSize {
			break
		}
	}

	out := make([]InstrumentInfo, 0, len(seen))
	for _, item := range seen {
		out = append(out, item)
	}
	return out, len(out) > 0, nil
}

func (c *DNSEClient) FetchLatestTrade(ctx context.Context, symbol, boardID string) (any, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	boardID = strings.TrimSpace(boardID)
	if symbol == "" {
		return nil, errors.New("symbol is required")
	}
	if c.cfg.Mock {
		return map[string]any{
			"trades": []map[string]any{{
				"symbol":        symbol,
				"lastPrice":     1200.0,
				"matchPrice":    1200.0,
				"matchVolume":   1,
				"matchTime":     time.Now().Unix(),
				"boardId":       defaultString(boardID, "G1"),
				"lastQuantity":  1,
				"tradeQuantity": 1,
			}},
		}, nil
	}

	path := fmt.Sprintf("/price/%s/trades/latest", url.PathEscape(symbol))
	if boardID != "" {
		path += "?boardId=" + url.QueryEscape(boardID)
	}
	var raw any
	if err := c.do(ctx, http.MethodGet, path, nil, &raw, false, true); err != nil {
		return nil, err
	}
	return raw, nil
}

func (c *DNSEClient) GetPPSE(ctx context.Context, accountNo, symbol, marketType string, loanPackageID int, price float64) (dnsemodel.PPSE, error) {
	accountNo = strings.TrimSpace(accountNo)
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	marketType = strings.ToUpper(strings.TrimSpace(defaultString(marketType, "STOCK")))
	if accountNo == "" {
		return dnsemodel.PPSE{}, errors.New("accountNo is required")
	}
	if symbol == "" {
		return dnsemodel.PPSE{}, errors.New("symbol is required")
	}
	if loanPackageID <= 0 {
		return dnsemodel.PPSE{}, errors.New("loanPackageId must be greater than zero")
	}
	if price <= 0 {
		return dnsemodel.PPSE{}, errors.New("price must be greater than zero")
	}
	if c.cfg.Mock {
		return dnsemodel.PPSE{Price: price, QmaxBuy: 1000, QmaxSell: 1000}, nil
	}

	path := fmt.Sprintf("/accounts/%s/ppse?marketType=%s&symbol=%s&loanPackageId=%d&price=%s",
		url.PathEscape(accountNo),
		url.QueryEscape(marketType),
		url.QueryEscape(symbol),
		loanPackageID,
		url.QueryEscape(fmt.Sprintf("%.0f", price)),
	)
	var raw map[string]any
	if err := c.do(ctx, http.MethodGet, path, nil, &raw, false, true); err != nil {
		return dnsemodel.PPSE{}, err
	}
	return dnsemodel.PPSE{
		Price:    firstFloat(raw, "price"),
		QmaxBuy:  firstInt(raw, "qmaxBuy", "qmax_buy"),
		QmaxSell: firstInt(raw, "qmaxSell", "qmax_sell"),
	}, nil
}

func (c *DNSEClient) GetPositions(ctx context.Context, accountNo, marketType string) ([]dnsemodel.Position, error) {
	accountNo = strings.TrimSpace(accountNo)
	marketType = strings.ToUpper(strings.TrimSpace(defaultString(marketType, "DERIVATIVE")))
	if accountNo == "" {
		return nil, errors.New("accountNo is required")
	}
	if c.cfg.Mock {
		return []dnsemodel.Position{}, nil
	}

	path := fmt.Sprintf("/accounts/%s/positions?marketType=%s&pageSize=100",
		url.PathEscape(accountNo),
		url.QueryEscape(marketType),
	)
	var raw any
	if err := c.do(ctx, http.MethodGet, path, nil, &raw, false, true); err != nil {
		return nil, err
	}
	return simplifyPositions(raw), nil
}

func (c *DNSEClient) do(ctx context.Context, method, path string, payload any, out any, requireTradingToken bool, allowRetry bool) error {
	if err := c.validateCredentials(); err != nil {
		return err
	}
	if requireTradingToken {
		if err := c.ensureToken(ctx); err != nil {
			return err
		}
	}

	status, body, err := c.send(ctx, method, path, payload, requireTradingToken)
	if err != nil {
		return err
	}

	if requireTradingToken && (status == http.StatusUnauthorized || status == http.StatusForbidden) && allowRetry {
		c.logger.Error("dnse_invalid_token", map[string]any{"path": path, "statusCode": status})
		if err := c.refreshToken(ctx); err != nil {
			return fmt.Errorf("refresh trading token failed: %w", err)
		}
		return c.do(ctx, method, path, payload, out, requireTradingToken, false)
	}

	if status < 200 || status >= 300 {
		return fmt.Errorf("dnse api returned status %d: %s", status, string(body))
	}
	if out == nil || len(body) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode dnse response: %w", err)
	}
	return nil
}

func (c *DNSEClient) send(ctx context.Context, method, path string, payload any, includeTradingToken bool) (int, []byte, error) {
	body, err := encodeJSON(payload)
	if err != nil {
		return 0, nil, err
	}

	date := formatDNSEDate(time.Now().UTC())
	url := strings.TrimRight(c.cfg.BaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "dnse-go-connector")
	// CRITICAL: Use raw map assignment, NOT Header.Set() for auth headers.
	// Go's Header.Set() canonicalizes names ("date" → "Date", "x-signature" → "X-Signature").
	// DNSE server checks lowercase header names, so canonicalization breaks auth.
	// See docs/DNSE_API_AUTH.md for full explanation.
	req.Header["x-api-key"] = []string{c.cfg.APIKey}
	req.Header["date"] = []string{date}
	signatureHeader := GenerateSignature(c.cfg.APIKey, c.cfg.APISecret, method, requestPathOnly(path), date)
	req.Header["x-signature"] = []string{signatureHeader}
	if token := c.currentToken(); includeTradingToken && token != "" {
		req.Header.Set("trading-token", token)
	}

	logDetails := map[string]any{
		"method":              method,
		"path":                path,
		"signingPath":         requestPathOnly(path),
		"date":                date,
		"apiKeyLength":        len(c.cfg.APIKey),
		"signatureHeaderSize": len(signatureHeader),
		"tradingTokenSent":    includeTradingToken && c.currentToken() != "",
	}
	if payload != nil {
		logDetails["payload"] = string(body)
	}
	c.logger.Info("dnse_api_request", logDetails)
	resp, err := c.http.Do(req)
	if err != nil {
		c.logger.Error("dnse_api_error", map[string]any{"path": path, "error": err.Error()})
		return 0, nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	c.logger.Info("dnse_api_response", map[string]any{"path": path, "statusCode": resp.StatusCode, "body": string(respBody)})
	return resp.StatusCode, respBody, nil
}

func (c *DNSEClient) fetchTradingToken(ctx context.Context, passcode, otpType string) (string, time.Time, error) {
	if err := c.validateCredentials(); err != nil {
		return "", time.Time{}, err
	}
	payload := map[string]string{
		"otpType":  otpType,
		"passcode": passcode,
	}
	body, err := encodeJSON(payload)
	if err != nil {
		return "", time.Time{}, err
	}

	path := "/registration/trading-token"
	date := formatDNSEDate(time.Now().UTC())
	url := strings.TrimRight(c.cfg.BaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", time.Time{}, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "dnse-go-connector")
	// CRITICAL: See comment in send() — raw map required for auth headers
	req.Header["x-api-key"] = []string{c.cfg.APIKey}
	req.Header["date"] = []string{date}
	signatureHeader := GenerateSignature(c.cfg.APIKey, c.cfg.APISecret, http.MethodPost, path, date)
	req.Header["x-signature"] = []string{signatureHeader}

	c.logger.Info("dnse_token_request", map[string]any{
		"path":                path,
		"signingPath":         path,
		"date":                date,
		"apiKeyLength":        len(c.cfg.APIKey),
		"signatureHeaderSize": len(signatureHeader),
	})
	resp, err := c.http.Do(req)
	if err != nil {
		c.logger.Error("dnse_token_error", map[string]any{"error": err.Error()})
		return "", time.Time{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", time.Time{}, err
	}
	c.logger.Info("dnse_token_response", map[string]any{"statusCode": resp.StatusCode, "body": maskTokenResponse(respBody)})

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", time.Time{}, fmt.Errorf("dnse token api returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var tokenResp tradingTokenResponse
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return "", time.Time{}, fmt.Errorf("decode trading token response: %w", err)
	}
	token := defaultString(tokenResp.TradingToken, tokenResp.Token)
	if token == "" {
		return "", time.Time{}, errors.New("trading token missing in dnse response")
	}
	expiresIn := tokenResp.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 8 * 60 * 60
	}
	return token, time.Now().Add(time.Duration(expiresIn) * time.Second), nil
}

func (c *DNSEClient) ensureToken(ctx context.Context) error {
	if c.cfg.TradingToken == "" && c.cfg.Mock {
		return nil
	}
	if token, expiresAt := c.tokenState(); token != "" && time.Now().Before(expiresAt.Add(-30*time.Second)) {
		return nil
	}
	return c.refreshToken(ctx)
}

func (c *DNSEClient) refreshToken(ctx context.Context) error {
	c.mu.RLock()
	passcode := c.passcode
	otpType := c.otpType
	c.mu.RUnlock()

	if otpType == "" || otpType == "email_otp" {
		otpType = "email_otp"
		if c.otp != nil {
			c.logger.Info("dnse_trigger_email_otp", nil)
			if err := c.SendEmailOTP(ctx); err != nil {
				return fmt.Errorf("failed to send email otp: %w", err)
			}
			c.logger.Info("dnse_waiting_for_otp", nil)
			var otp string
			for i := 0; i < 10; i++ {
				time.Sleep(1 * time.Second)
				code, valid := c.otp.GetLatestOTP()
				if valid {
					otp = code
					break
				}
			}
			if otp != "" {
				passcode = otp
			} else if passcode == "" {
				return errors.New("timeout waiting for email OTP and no existing passcode")
			}
		}
	}

	if passcode == "" {
		if c.currentToken() != "" {
			return nil
		}
		return errors.New("trading token is missing; call POST /registration/trading-token first")
	}

	token, expiresAt, err := c.fetchTradingToken(ctx, passcode, otpType)
	if err != nil {
		return err
	}
	c.setToken(token, expiresAt)
	c.persistToken(ctx, token, expiresAt)
	return nil
}

func (c *DNSEClient) currentToken() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.token
}

func (c *DNSEClient) tokenState() (string, time.Time) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.token, c.expiresAt
}

func (c *DNSEClient) setToken(token string, expiresAt time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.token = token
	c.expiresAt = expiresAt
}

func (c *DNSEClient) persistToken(ctx context.Context, token string, expiresAt time.Time) {
	if c.store == nil || token == "" {
		return
	}
	if err := c.store.SaveTradingToken(ctx, token, expiresAt); err != nil {
		c.logger.Error("dnse_token_persist_failed", map[string]any{"error": err.Error()})
		return
	}
	c.logger.Info("dnse_token_persisted", map[string]any{"expiresAt": expiresAt.UTC().Format(time.RFC3339)})
}

func (c *DNSEClient) validateCredentials() error {
	if c.cfg.Mock {
		return nil
	}
	if strings.TrimSpace(c.cfg.APIKey) == "" {
		return errors.New("dnse api_key is missing in config/config.yaml")
	}
	if strings.TrimSpace(c.cfg.APISecret) == "" {
		return errors.New("dnse api_secret is missing in config/config.yaml")
	}
	return nil
}

// GenerateSignature creates an HTTP Signature for DNSE OpenAPI.
//
// IMPORTANT - 3 rules that MUST NOT be changed (verified working via live API test):
//  1. Signing string line order: (request-target) → date → nonce
//  2. base64 output MUST be URL-encoded (url.QueryEscape), matching Python parse.quote(safe="")
//  3. `headers` field in output is always "(request-target) date" even though nonce is in signing string
//
// Reference: https://github.com/dnse-tech/openapi-sdk/blob/main/python/dnse/common.py
// Reference: docs/DNSE_API_AUTH.md
func GenerateSignature(apiKey, secret, method, path, date string) string {
	nonce := generateNonce()
	// signing string per DNSE Python SDK: (request-target) + date + nonce
	signingString := fmt.Sprintf("(request-target): %s %s\ndate: %s\nnonce: %s", strings.ToLower(method), path, date, nonce)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingString))
	raw := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	// URL-encode per Python's parse.quote(encoded, safe="")
	// This converts "/" → "%2F", "+" → "%2B", "=" → "%3D" in the base64 string
	signature := url.QueryEscape(raw)
	return fmt.Sprintf(
		`Signature keyId="%s",algorithm="hmac-sha256",headers="(request-target) date",signature="%s",nonce="%s"`,
		apiKey,
		signature,
		nonce,
	)
}

func encodeJSON(payload any) ([]byte, error) {
	if payload == nil {
		return nil, nil
	}
	return json.Marshal(payload)
}

func simplifyAccounts(raw any, preferredAccountNo string) []dnsemodel.Account {
	items := flattenAccountItems(raw)
	accounts := make([]dnsemodel.Account, 0, len(items))
	for _, item := range items {
		accountNo := firstString(item, "accountNo", "account_no", "id")
		if accountNo == "" {
			continue
		}
		if preferredAccountNo != "" && accountNo != preferredAccountNo {
			continue
		}
		status := firstString(item, "derivativeAccountStatus", "derivative_account_status", "status")
		if status == "" {
			status = nestedString(item, "derivative", "status")
		}
		if status == "" {
			status = "UNKNOWN"
		}
		accounts = append(accounts, dnsemodel.Account{
			AccountNo:               accountNo,
			DerivativeAccountStatus: status,
		})
	}
	return accounts
}

func simplifyLoanPackages(raw any) []dnsemodel.LoanPackage {
	items := flattenLoanPackageItems(raw)
	packages := make([]dnsemodel.LoanPackage, 0, len(items))
	for _, item := range items {
		id := firstInt(item, "id")
		if id == 0 {
			continue
		}
		packages = append(packages, dnsemodel.LoanPackage{
			ID:              id,
			Name:            firstString(item, "name"),
			InterestRate:    firstFloat(item, "interestRate", "interest_rate"),
			InitialRate:     firstFloat(item, "initialRate", "initial_rate"),
			MaintenanceRate: firstFloat(item, "maintenanceRate", "maintenance_rate"),
			LiquidRate:      firstFloat(item, "liquidRate", "liquid_rate"),
			Type:            firstString(item, "type"),
		})
	}
	return packages
}

func simplifyInstruments(raw any, exchange string) []InstrumentInfo {
	items := make([]InstrumentInfo, 0, 1024)
	appendMap := func(item map[string]any) {
		symbol := strings.ToUpper(strings.TrimSpace(firstString(item, "symbol", "secSymbol", "ticker", "code")))
		if symbol == "" {
			return
		}
		instrumentExchange := strings.ToUpper(strings.TrimSpace(firstString(item, "exchange", "floor", "market")))
		if instrumentExchange == "" {
			instrumentExchange = exchange
		}
		items = append(items, InstrumentInfo{
			Symbol:   symbol,
			Exchange: instrumentExchange,
			Type:     strings.TrimSpace(firstString(item, "type", "instrumentType")),
		})
	}

	var walk func(value any)
	walk = func(value any) {
		switch v := value.(type) {
		case []any:
			for _, item := range v {
				walk(item)
			}
		case map[string]any:
			if symbol := firstString(v, "symbol", "secSymbol", "ticker", "code"); strings.TrimSpace(symbol) != "" {
				appendMap(v)
				return
			}
			for _, key := range []string{"data", "items", "results", "list", "rows", "instruments"} {
				if nested, ok := v[key]; ok {
					walk(nested)
				}
			}
		}
	}
	walk(raw)

	seen := make(map[string]InstrumentInfo, len(items))
	for _, item := range items {
		seen[item.Symbol] = item
	}
	out := make([]InstrumentInfo, 0, len(seen))
	for _, item := range seen {
		out = append(out, item)
	}
	return out
}

func extractInstrumentPagination(raw any) (total, page, pageSize int, hasNext bool) {
	root, ok := raw.(map[string]any)
	if !ok {
		return 0, 0, 0, false
	}

	total = firstInt(root, "total", "totalCount", "totalElements", "count")
	page = firstInt(root, "page", "pageNumber", "currentPage")
	pageSize = firstInt(root, "pageSize", "size", "perPage", "limit")
	hasNext = firstBool(root, "hasNext", "hasMore")

	if !hasNext {
		totalPages := firstInt(root, "totalPages", "pageCount")
		if totalPages > 0 && page > 0 && page < totalPages {
			hasNext = true
		}
	}

	for _, key := range []string{"pagination", "meta"} {
		nested, ok := root[key].(map[string]any)
		if !ok {
			continue
		}
		if total == 0 {
			total = firstInt(nested, "total", "totalCount", "totalElements", "count")
		}
		if page == 0 {
			page = firstInt(nested, "page", "pageNumber", "currentPage")
		}
		if pageSize == 0 {
			pageSize = firstInt(nested, "pageSize", "size", "perPage", "limit")
		}
		if !hasNext {
			hasNext = firstBool(nested, "hasNext", "hasMore")
			totalPages := firstInt(nested, "totalPages", "pageCount")
			if !hasNext && totalPages > 0 && page > 0 && page < totalPages {
				hasNext = true
			}
		}
	}

	return total, page, pageSize, hasNext
}

func instrumentPageSignature(items []InstrumentInfo) string {
	if len(items) == 0 {
		return ""
	}
	first := items[0].Symbol
	last := items[len(items)-1].Symbol
	return first + "|" + last + "|" + strconv.Itoa(len(items))
}

func mergeInstrumentDefaults(exchange string, items []InstrumentInfo) []InstrumentInfo {
	seen := make(map[string]InstrumentInfo, len(items)+8)
	for _, item := range items {
		seen[item.Symbol] = item
	}

	defaults := []InstrumentInfo{
		{Symbol: "VNINDEX", Exchange: "INDEX", Type: "INDEX"},
		{Symbol: "VN30", Exchange: "INDEX", Type: "INDEX"},
		{Symbol: "HNX", Exchange: "INDEX", Type: "INDEX"},
		{Symbol: "HNX30", Exchange: "INDEX", Type: "INDEX"},
		{Symbol: "VN100", Exchange: "INDEX", Type: "INDEX"},
		{Symbol: "UPCOM", Exchange: "INDEX", Type: "INDEX"},
		{Symbol: "VNXALLSHARE", Exchange: "INDEX", Type: "INDEX"},
	}
	if strings.EqualFold(exchange, "DERIVATIVE") || strings.EqualFold(exchange, "PS") {
		defaults = append(defaults,
			InstrumentInfo{Symbol: "VN30F1M", Exchange: "DERIVATIVE", Type: "DERIVATIVE"},
			InstrumentInfo{Symbol: "VN30F2M", Exchange: "DERIVATIVE", Type: "DERIVATIVE"},
			InstrumentInfo{Symbol: "VN30F1Q", Exchange: "DERIVATIVE", Type: "DERIVATIVE"},
			InstrumentInfo{Symbol: "VN30F2Q", Exchange: "DERIVATIVE", Type: "DERIVATIVE"},
			InstrumentInfo{Symbol: "V100F1M", Exchange: "DERIVATIVE", Type: "DERIVATIVE"},
			InstrumentInfo{Symbol: "V100F2M", Exchange: "DERIVATIVE", Type: "DERIVATIVE"},
			InstrumentInfo{Symbol: "V100F1Q", Exchange: "DERIVATIVE", Type: "DERIVATIVE"},
			InstrumentInfo{Symbol: "V100F2Q", Exchange: "DERIVATIVE", Type: "DERIVATIVE"},
		)
	}

	for _, item := range defaults {
		if _, ok := seen[item.Symbol]; ok {
			continue
		}
		seen[item.Symbol] = item
	}

	out := make([]InstrumentInfo, 0, len(seen))
	for _, item := range seen {
		out = append(out, item)
	}
	return out
}

func simplifyTickers(raw any) []TickerInfo {
	items := make([]TickerInfo, 0, 2048)

	appendMap := func(item map[string]any) {
		rawJSON, _ := json.Marshal(item)
		symbol := strings.ToUpper(strings.TrimSpace(firstString(item, "symbolType", "displaySymbol", "ticker", "symbol", "code")))
		if symbol == "" {
			return
		}
		feedSymbol := strings.ToUpper(strings.TrimSpace(firstString(item, "symbol", "code", "secSymbol", "ticker")))
		if feedSymbol == "" {
			feedSymbol = symbol
		}
		exchange := strings.ToUpper(strings.TrimSpace(firstString(item, "exchange", "floor", "market", "marketId")))
		if exchange == "" {
			exchange = classifyTickerExchange(symbol, firstString(item, "type", "instrumentType", "securityType", "assetClass"))
		}
		items = append(items, TickerInfo{
			Symbol:      symbol,
			FeedSymbol:  feedSymbol,
			Exchange:    exchange,
			Type:        normalizeTickerType(symbol, firstString(item, "type", "instrumentType", "securityType", "assetClass")),
			BoardID:     strings.TrimSpace(firstString(item, "boardId", "boardID")),
			Name:        strings.TrimSpace(firstString(item, "name", "fullName", "viName", "title")),
			Description: strings.TrimSpace(firstString(item, "description", "desc", "remark", "enName")),
			RawJSON:     string(rawJSON),
		})
	}

	var walk func(value any)
	walk = func(value any) {
		switch v := value.(type) {
		case []any:
			for _, item := range v {
				walk(item)
			}
		case map[string]any:
			if len(v) == 0 {
				return
			}
			if hasTickerIdentity(v) {
				appendMap(v)
				return
			}
			for _, key := range []string{"data", "items", "results", "list", "rows", "tickers"} {
				if nested, ok := v[key]; ok {
					walk(nested)
				}
			}
		}
	}
	walk(raw)

	seen := make(map[string]TickerInfo, len(items))
	for _, item := range items {
		seen[item.Symbol] = item
	}
	out := make([]TickerInfo, 0, len(seen))
	for _, item := range seen {
		out = append(out, item)
	}
	return out
}

func hasTickerIdentity(item map[string]any) bool {
	return firstString(item, "symbolType", "displaySymbol", "ticker", "symbol", "code") != ""
}

func normalizeTickerType(symbol, rawType string) string {
	rawType = strings.ToUpper(strings.TrimSpace(rawType))
	switch {
	case rawType != "":
		return rawType
	case strings.HasPrefix(symbol, "VN30F"), strings.HasPrefix(symbol, "V100F"):
		return "DERIVATIVE"
	case strings.HasPrefix(symbol, "VN"), symbol == "HNX", symbol == "HNX30", symbol == "UPCOM", symbol == "VNXALLSHARE":
		return "INDEX"
	default:
		return "STOCK"
	}
}

func classifyTickerExchange(symbol, rawType string) string {
	switch normalizeTickerType(symbol, rawType) {
	case "INDEX":
		return "INDEX"
	case "DERIVATIVE":
		return "DERIVATIVE"
	default:
		return "STOCK"
	}
}

func firstBool(item map[string]any, keys ...string) bool {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			switch v := value.(type) {
			case bool:
				return v
			case string:
				parsed, _ := strconv.ParseBool(strings.TrimSpace(v))
				return parsed
			case float64:
				return v != 0
			case int:
				return v != 0
			}
		}
	}
	return false
}

func simplifyPositions(raw any) []dnsemodel.Position {
	items := flattenPositionItems(raw)
	positions := make([]dnsemodel.Position, 0, len(items))
	for _, item := range items {
		symbol := strings.ToUpper(firstString(item, "symbol", "secSymbol"))
		if symbol == "" {
			continue
		}
		side := firstString(item, "side", "positionSide", "positionType")
		longQty := firstInt(item, "longQuantity", "longQty")
		shortQty := firstInt(item, "shortQuantity", "shortQty")
		if side == "" && (longQty > 0 || shortQty > 0) {
			if longQty > 0 {
				positions = append(positions, dnsemodel.Position{
					ID:       firstString(item, "id", "positionId"),
					Symbol:   symbol,
					Side:     "LONG",
					Quantity: longQty,
				})
			}
			if shortQty > 0 {
				positions = append(positions, dnsemodel.Position{
					ID:       firstString(item, "id", "positionId"),
					Symbol:   symbol,
					Side:     "SHORT",
					Quantity: shortQty,
				})
			}
			continue
		}

		quantity := firstInt(item, "quantity", "openQuantity", "netQuantity", "volume")
		if side == "" {
			if quantity < 0 {
				side = "SHORT"
				quantity = -quantity
			} else {
				side = "LONG"
			}
		}
		positions = append(positions, dnsemodel.Position{
			ID:       firstString(item, "id", "positionId"),
			Symbol:   symbol,
			Side:     side,
			Quantity: quantity,
		})
	}
	return positions
}

func flattenLoanPackageItems(raw any) []map[string]any {
	switch v := raw.(type) {
	case []any:
		return mapsFromSlice(v)
	case map[string]any:
		for _, key := range []string{"loanPackages", "loan_packages", "data", "items"} {
			if arr, ok := v[key].([]any); ok {
				return mapsFromSlice(arr)
			}
		}
		return []map[string]any{v}
	default:
		return nil
	}
}

func flattenPositionItems(raw any) []map[string]any {
	switch v := raw.(type) {
	case []any:
		return mapsFromSlice(v)
	case map[string]any:
		for _, key := range []string{"positions", "data", "items"} {
			if arr, ok := v[key].([]any); ok {
				return mapsFromSlice(arr)
			}
		}
		return []map[string]any{v}
	default:
		return nil
	}
}

func flattenAccountItems(raw any) []map[string]any {
	switch v := raw.(type) {
	case []any:
		return mapsFromSlice(v)
	case map[string]any:
		for _, key := range []string{"accounts", "data", "items"} {
			if arr, ok := v[key].([]any); ok {
				return mapsFromSlice(arr)
			}
		}
		return []map[string]any{v}
	default:
		return nil
	}
}

func mapsFromSlice(items []any) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func firstString(item map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			switch v := value.(type) {
			case string:
				if trimmed := strings.TrimSpace(v); trimmed != "" {
					return trimmed
				}
			case float64:
				return fmt.Sprintf("%.0f", v)
			}
		}
	}
	return ""
}

func firstInt(item map[string]any, keys ...string) int {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			switch v := value.(type) {
			case int:
				return v
			case int64:
				return int(v)
			case float64:
				return int(v)
			case string:
				var n int
				if _, err := fmt.Sscanf(strings.TrimSpace(v), "%d", &n); err == nil {
					return n
				}
			}
		}
	}
	return 0
}

func firstFloat(item map[string]any, keys ...string) float64 {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			switch v := value.(type) {
			case float64:
				return v
			case int:
				return float64(v)
			case int64:
				return float64(v)
			case string:
				var n float64
				if _, err := fmt.Sscanf(strings.TrimSpace(v), "%f", &n); err == nil {
					return n
				}
			}
		}
	}
	return 0
}

func nestedString(item map[string]any, objectKey, valueKey string) string {
	raw, ok := item[objectKey]
	if !ok {
		return ""
	}
	nested, ok := raw.(map[string]any)
	if !ok {
		return ""
	}
	return firstString(nested, valueKey)
}

func defaultString(primary, fallback string) string {
	if strings.TrimSpace(primary) != "" {
		return primary
	}
	return fallback
}

func buildSigningString(method, path, date string) string {
	return fmt.Sprintf("(request-target): %s %s\ndate: %s", strings.ToLower(method), path, date)
}

func formatDNSEDate(t time.Time) string {
	return t.UTC().Format("Mon, 02 Jan 2006 15:04:05 +0000")
}

func requestPathOnly(path string) string {
	if idx := strings.Index(path, "?"); idx >= 0 {
		return path[:idx]
	}
	return path
}

func generateNonce() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return strings.ReplaceAll(fmt.Sprintf("%d", time.Now().UnixNano()), "-", "")
	}
	return fmt.Sprintf("%x", b[:])
}

func maskTokenResponse(body []byte) string {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return "<unparseable token response>"
	}
	for _, key := range []string{"tradingToken", "token"} {
		if value, ok := payload[key].(string); ok && value != "" {
			payload[key] = maskSecret(value)
		}
	}
	masked, err := json.Marshal(payload)
	if err != nil {
		return "<masked token response>"
	}
	return string(masked)
}

func maskSecret(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 8 {
		return "***"
	}
	return value[:4] + "..." + value[len(value)-4:]
}
