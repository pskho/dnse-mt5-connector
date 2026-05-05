package entrade

import (
	"bytes"
	"context"
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
)

type Client struct {
	cfg        config.EntradeConfig
	http       *http.Client
	logger     *logger.FileLogger
	mu         sync.RWMutex
	token      string
	expiresAt  time.Time
	investorID string
}

type authResponse struct {
	Token string `json:"token"`
}

func NewClient(cfg config.EntradeConfig, appLog *logger.FileLogger) *Client {
	return &Client{
		cfg: cfg,
		http: &http.Client{
			Timeout: 15 * time.Second,
		},
		logger:     appLog,
		investorID: strings.TrimSpace(cfg.InvestorID),
	}
}

func (c *Client) GetAccounts(ctx context.Context) ([]dnsemodel.Account, error) {
	if c.cfg.Mock {
		return []dnsemodel.Account{{AccountNo: defaultString(c.cfg.AccountNo, "1000000036"), DerivativeAccountStatus: "ACTIVE"}}, nil
	}
	investorID, err := c.ensureInvestorID(ctx)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := c.do(ctx, http.MethodGet, fmt.Sprintf("/investors/%s/investor_account", url.PathEscape(investorID)), nil, &raw); err != nil {
		return nil, err
	}
	accountNo := firstString(raw, "investorAccountId", "id", "investorId")
	if accountNo == "" {
		accountNo = investorID
	}
	status := firstString(raw, "status")
	if status == "" {
		status = "UNKNOWN"
	}
	return []dnsemodel.Account{{AccountNo: accountNo, DerivativeAccountStatus: status}}, nil
}

func (c *Client) GetLoanPackages(ctx context.Context, accountNo, symbol, marketType string) ([]dnsemodel.LoanPackage, error) {
	if c.cfg.Mock {
		return []dnsemodel.LoanPackage{{ID: 34, Name: "Mock Entrade derivative margin", Type: "DERIVATIVE", InitialRate: 0.05}}, nil
	}
	investorID, err := c.ensureInvestorID(ctx)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := c.do(ctx, http.MethodGet, fmt.Sprintf("/investors/%s/derivative_margin_portfolios", url.PathEscape(investorID)), nil, &raw); err != nil {
		return nil, err
	}
	return simplifyLoanPackages(raw, symbol), nil
}

func (c *Client) PlaceOrder(ctx context.Context, req dnsemodel.PlaceOrderRequest) (dnsemodel.PlaceOrderResponse, error) {
	if c.cfg.Mock {
		return dnsemodel.PlaceOrderResponse{
			OrderID:     fmt.Sprintf("ENTRADE-MOCK-%d", time.Now().UnixNano()),
			Status:      "PendingNew",
			RawResponse: `{"mock":true}`,
		}, nil
	}
	if !strings.EqualFold(strings.TrimSpace(req.Symbol), "VN30F1M") {
		return dnsemodel.PlaceOrderResponse{}, errors.New("entrade provider supports VN30F1M only")
	}
	investorID, err := c.ensureInvestorID(ctx)
	if err != nil {
		return dnsemodel.PlaceOrderResponse{}, err
	}
	symbol, err := c.ResolveDerivativeSymbol(ctx, req.Symbol)
	if err != nil {
		return dnsemodel.PlaceOrderResponse{}, err
	}
	portfolioID := 0
	if req.LoanPackageID != nil {
		portfolioID = *req.LoanPackageID
	}
	if portfolioID <= 0 {
		packages, err := c.GetLoanPackages(ctx, req.AccountNo, req.Symbol, "DERIVATIVE")
		if err != nil {
			return dnsemodel.PlaceOrderResponse{}, err
		}
		for _, pkg := range packages {
			if pkg.ID > 0 {
				portfolioID = pkg.ID
				break
			}
		}
	}
	if portfolioID <= 0 {
		return dnsemodel.PlaceOrderResponse{}, errors.New("entrade bankMarginPortfolioId is required")
	}

	payload := map[string]any{
		"bankMarginPortfolioId": portfolioID,
		"investorId":            parseInt(investorID),
		"symbol":                symbol,
		"price":                 req.Price,
		"orderType":             defaultString(req.OrderType, "LO"),
		"side":                  req.Side,
		"quantity":              req.Quantity,
	}
	var raw map[string]any
	if err := c.do(ctx, http.MethodPost, "/derivative/orders", payload, &raw); err != nil {
		return dnsemodel.PlaceOrderResponse{}, err
	}
	rawBody, _ := json.Marshal(raw)
	orderID := firstString(raw, "id")
	if orderID == "" {
		return dnsemodel.PlaceOrderResponse{}, errors.New("entrade order response missing id")
	}
	return dnsemodel.PlaceOrderResponse{
		OrderID:     orderID,
		Status:      defaultString(firstString(raw, "orderStatus", "status"), "PendingNew"),
		RawResponse: string(rawBody),
	}, nil
}

func (c *Client) GetOrderStatus(ctx context.Context, accountNo, orderID, marketType, orderCategory string) (dnsemodel.OrderStatus, error) {
	if c.cfg.Mock {
		return dnsemodel.OrderStatus{OrderID: orderID, Status: "New", RemainingQuantity: 1}, nil
	}
	var raw map[string]any
	if err := c.do(ctx, http.MethodGet, "/derivative/orders/"+url.PathEscape(orderID), nil, &raw); err != nil {
		return dnsemodel.OrderStatus{}, err
	}
	rawBody, _ := json.Marshal(raw)
	quantity := firstInt(raw, "quantity")
	filled := firstInt(raw, "fillQuantity", "filledQuantity")
	remaining := firstInt(raw, "leavesQuantity", "remainingQuantity")
	if remaining == 0 && quantity > filled {
		remaining = quantity - filled
	}
	return dnsemodel.OrderStatus{
		OrderID:           orderID,
		Status:            defaultString(firstString(raw, "orderStatus", "status"), "UNKNOWN"),
		FilledQuantity:    filled,
		RemainingQuantity: remaining,
		RawResponse:       string(rawBody),
	}, nil
}

func (c *Client) CancelOrder(ctx context.Context, accountNo, orderID, marketType, orderCategory string) (dnsemodel.CancelOrderResponse, error) {
	if c.cfg.Mock {
		return dnsemodel.CancelOrderResponse{Success: true, OrderID: orderID, Status: "Canceled"}, nil
	}
	var raw map[string]any
	if err := c.do(ctx, http.MethodDelete, "/derivative/orders/"+url.PathEscape(orderID), nil, &raw); err != nil {
		return dnsemodel.CancelOrderResponse{}, err
	}
	rawBody, _ := json.Marshal(raw)
	return dnsemodel.CancelOrderResponse{
		Success:           true,
		OrderID:           defaultString(firstString(raw, "id"), orderID),
		Status:            defaultString(firstString(raw, "orderStatus", "status"), "UNKNOWN"),
		FilledQuantity:    firstInt(raw, "fillQuantity", "filledQuantity"),
		RemainingQuantity: firstInt(raw, "leavesQuantity", "remainingQuantity"),
		RawResponse:       string(rawBody),
	}, nil
}

func (c *Client) GetPositions(ctx context.Context, accountNo, marketType string) ([]dnsemodel.Position, error) {
	if c.cfg.Mock {
		return []dnsemodel.Position{}, nil
	}
	accountNo = strings.TrimSpace(accountNo)
	if accountNo == "" {
		accountNo = strings.TrimSpace(c.cfg.AccountNo)
	}
	if accountNo == "" {
		investorID, err := c.ensureInvestorID(ctx)
		if err != nil {
			return nil, err
		}
		accountNo = investorID
	}
	path := fmt.Sprintf("/derivative/deals?investorAccountId=%s&_start=0&_end=100&_sort=modifiedDate&_order=DESC", url.QueryEscape(accountNo))
	var raw map[string]any
	if err := c.do(ctx, http.MethodGet, path, nil, &raw); err != nil {
		return nil, err
	}
	items, _ := raw["data"].([]any)
	out := make([]dnsemodel.Position, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if strings.EqualFold(firstString(m, "closed"), "true") || strings.EqualFold(firstString(m, "state"), "CLOSED") {
			continue
		}
		qty := firstInt(m, "openQuantity", "remainingTradeableQuantity")
		if qty <= 0 {
			continue
		}
		side := firstString(m, "side")
		if side == "NB" {
			side = "LONG"
		} else if side == "NS" {
			side = "SHORT"
		}
		out = append(out, dnsemodel.Position{
			ID:       firstString(m, "id"),
			Symbol:   "VN30F1M",
			Side:     side,
			Quantity: qty,
		})
	}
	return out, nil
}

func (c *Client) ResolveDerivativeSymbol(ctx context.Context, displaySymbol string) (string, error) {
	displaySymbol = strings.ToUpper(strings.TrimSpace(displaySymbol))
	if displaySymbol == "" || displaySymbol == "VN30F1M" {
		displaySymbol = "VN30F1M"
	}
	var raw map[string]any
	if err := c.do(ctx, http.MethodGet, "/derivatives", nil, &raw); err != nil {
		return "", err
	}
	items, _ := raw["data"].([]any)
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if strings.EqualFold(firstString(m, "type"), displaySymbol) {
			return firstString(m, "symbol", "id"), nil
		}
		if strings.EqualFold(firstString(m, "symbol"), displaySymbol) {
			return firstString(m, "symbol", "id"), nil
		}
	}
	if displaySymbol == "VN30F1M" {
		return "", errors.New("entrade derivative catalog did not return VN30F1M")
	}
	return displaySymbol, nil
}

func (c *Client) ensureToken(ctx context.Context) error {
	c.mu.RLock()
	token := c.token
	expiresAt := c.expiresAt
	c.mu.RUnlock()
	if token != "" && time.Now().UTC().Before(expiresAt.Add(-5*time.Minute)) {
		return nil
	}
	return c.login(ctx)
}

func (c *Client) ensureInvestorID(ctx context.Context) (string, error) {
	if c.cfg.Mock {
		return defaultString(c.cfg.InvestorID, "1000000036"), nil
	}
	if strings.TrimSpace(c.investorID) != "" {
		return strings.TrimSpace(c.investorID), nil
	}
	if err := c.ensureToken(ctx); err != nil {
		return "", err
	}
	c.mu.RLock()
	token := c.token
	c.mu.RUnlock()
	investorID := investorIDFromJWT(token)
	if investorID == "" {
		return "", errors.New("entrade investor_id is missing and cannot be decoded from token")
	}
	c.mu.Lock()
	c.investorID = investorID
	c.mu.Unlock()
	return investorID, nil
}

func (c *Client) login(ctx context.Context) error {
	if strings.TrimSpace(c.cfg.Username) == "" || strings.TrimSpace(c.cfg.Password) == "" {
		return errors.New("entrade username/password are required")
	}
	payload := map[string]string{"username": c.cfg.Username, "password": c.cfg.Password}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.AuthURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("entrade auth returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var auth authResponse
	if err := json.Unmarshal(respBody, &auth); err != nil {
		return err
	}
	if auth.Token == "" {
		return errors.New("entrade auth response missing token")
	}
	expiresAt := jwtExpiresAt(auth.Token)
	if expiresAt.IsZero() {
		expiresAt = time.Now().UTC().Add(8 * time.Hour)
	}
	investorID := investorIDFromJWT(auth.Token)
	c.mu.Lock()
	c.token = auth.Token
	c.expiresAt = expiresAt
	if c.investorID == "" {
		c.investorID = investorID
	}
	c.mu.Unlock()
	if c.logger != nil {
		c.logger.Info("entrade_login_success", map[string]any{"expiresAt": expiresAt.UTC().Format(time.RFC3339), "investorId": investorID})
	}
	return nil
}

func (c *Client) do(ctx context.Context, method, path string, payload any, out any) error {
	return c.doInternal(ctx, method, path, payload, out, true)
}

func (c *Client) doInternal(ctx context.Context, method, path string, payload any, out any, allowRetry bool) error {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if !strings.Contains(path, "/derivatives") && !strings.Contains(path, "/investor_account") {
		if err := c.ensureToken(ctx); err != nil {
			return err
		}
	} else if err := c.ensureToken(ctx); err != nil {
		return err
	}
	var body []byte
	var err error
	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.baseURL(), "/")+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.mu.RLock()
	token := c.token
	c.mu.RUnlock()
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if tradingToken := strings.TrimSpace(c.cfg.TradingToken); tradingToken != "" {
		req.Header.Set("Trading-Token", tradingToken)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if allowRetry && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) {
		if err := c.login(ctx); err == nil {
			return c.doInternal(ctx, method, path, payload, out, false)
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("entrade api returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) baseURL() string {
	if strings.EqualFold(strings.TrimSpace(c.cfg.Environment), "real") || strings.EqualFold(strings.TrimSpace(c.cfg.Environment), "prod") {
		return c.cfg.BaseURL
	}
	return c.cfg.PaperBaseURL
}

func simplifyLoanPackages(raw map[string]any, symbol string) []dnsemodel.LoanPackage {
	items, _ := raw["data"].([]any)
	out := make([]dnsemodel.LoanPackage, 0, len(items))
	symbol = strings.ToUpper(strings.TrimSpace(defaultString(symbol, "VN30F1M")))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		pkg := dnsemodel.LoanPackage{
			ID:           firstInt(m, "id"),
			Name:         firstString(m, "name"),
			InterestRate: firstFloat(m, "interestRate"),
			Type:         "DERIVATIVE",
		}
		if portfolio, ok := m["portfolio"].([]any); ok {
			for _, p := range portfolio {
				pm, ok := p.(map[string]any)
				if !ok {
					continue
				}
				if strings.EqualFold(firstString(pm, "symbol"), symbol) || strings.EqualFold(symbol, "VN30F1M") {
					pkg.InitialRate = firstFloat(pm, "initialRate")
					pkg.MaintenanceRate = firstFloat(pm, "maintenanceRate")
					pkg.LiquidRate = firstFloat(pm, "liquidateRate", "liquidRate")
					break
				}
			}
		}
		if pkg.ID > 0 {
			out = append(out, pkg)
		}
	}
	return out
}

func jwtExpiresAt(token string) time.Time {
	payload := jwtPayload(token)
	if payload == nil {
		return time.Time{}
	}
	exp := firstInt64(payload, "exp")
	if exp <= 0 {
		return time.Time{}
	}
	return time.Unix(exp, 0).UTC()
}

func investorIDFromJWT(token string) string {
	payload := jwtPayload(token)
	if payload == nil {
		return ""
	}
	if id := firstString(payload, "investorId", "investorIdStr"); id != "" {
		return id
	}
	return firstString(payload, "sub")
}

func jwtPayload(token string) map[string]any {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	return payload
}

func firstString(item map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			switch v := value.(type) {
			case string:
				return strings.TrimSpace(v)
			case float64:
				return fmt.Sprintf("%.0f", v)
			case int:
				return strconv.Itoa(v)
			case bool:
				return strconv.FormatBool(v)
			}
		}
	}
	return ""
}

func firstInt(item map[string]any, keys ...string) int {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			switch v := value.(type) {
			case float64:
				return int(v)
			case int:
				return v
			case string:
				n, _ := strconv.Atoi(strings.TrimSpace(v))
				return n
			}
		}
	}
	return 0
}

func firstInt64(item map[string]any, keys ...string) int64 {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			switch v := value.(type) {
			case float64:
				return int64(v)
			case int64:
				return v
			case int:
				return int64(v)
			case string:
				n, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
				return n
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
			case string:
				n, _ := strconv.ParseFloat(strings.TrimSpace(v), 64)
				return n
			}
		}
	}
	return 0
}

func parseInt(value string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(value))
	return n
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}
