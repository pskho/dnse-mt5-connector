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
	states     map[string]*accountState
}

type accountState struct {
	token      string
	expiresAt  time.Time
	investorID string
}

type authResponse struct {
	Token string `json:"token"`
}

type LinkAccountResult struct {
	Username          string                  `json:"username"`
	Environment       string                  `json:"environment"`
	InvestorID        string                  `json:"investorId"`
	InvestorAccountID string                  `json:"investorAccountId"`
	Status            string                  `json:"status"`
	TotalCash         float64                 `json:"totalCash,omitempty"`
	AvailableCash     float64                 `json:"availableCash,omitempty"`
	LoanPackages      []dnsemodel.LoanPackage `json:"loanPackages,omitempty"`
	TokenExpiresAt    time.Time               `json:"tokenExpiresAt,omitempty"`
}

const (
	AccountDemo     = "ENTRADE_DEMO"
	AccountReal     = "ENTRADE_REAL"
	defaultStateKey = "__DEFAULT__"
)

func NewClient(cfg config.EntradeConfig, appLog *logger.FileLogger) *Client {
	return &Client{
		cfg: cfg,
		http: &http.Client{
			Timeout: 15 * time.Second,
		},
		logger:     appLog,
		investorID: strings.TrimSpace(cfg.InvestorID),
		states:     make(map[string]*accountState),
	}
}

func (c *Client) configuredAccountIDs() []string {
	out := make([]string, 0, len(c.cfg.Accounts)+len(c.cfg.DefaultAccountNos)+1)
	seen := map[string]struct{}{}
	add := func(value string) {
		value = strings.ToUpper(strings.TrimSpace(value))
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	for _, account := range c.cfg.Accounts {
		if !account.Enabled {
			continue
		}
		add(account.ID)
	}
	if len(out) == 0 {
		for _, accountNo := range c.cfg.DefaultAccountNos {
			add(accountNo)
		}
	}
	if len(out) == 0 {
		add(c.cfg.AccountNo)
	}
	if len(out) == 0 && strings.TrimSpace(c.cfg.Username) != "" {
		add(AccountDemo)
	}
	return out
}

func (c *Client) accountProfile(accountNo string) config.EntradeAccountConfig {
	key := strings.ToUpper(strings.TrimSpace(accountNo))
	if key == "" {
		if len(c.cfg.DefaultAccountNos) > 0 {
			key = strings.ToUpper(strings.TrimSpace(c.cfg.DefaultAccountNos[0]))
		}
		if key == "" && len(c.cfg.Accounts) > 0 {
			key = strings.ToUpper(strings.TrimSpace(c.cfg.Accounts[0].ID))
		}
	}
	for _, account := range c.cfg.Accounts {
		if !account.Enabled {
			continue
		}
		if strings.EqualFold(account.ID, key) {
			return fillAccountProfile(account, c.cfg)
		}
	}
	if key == "" {
		key = defaultStateKey
	}
	return fillAccountProfile(config.EntradeAccountConfig{
		ID:          key,
		Environment: accountEnvironment(key, c.cfg.Environment),
	}, c.cfg)
}

func fillAccountProfile(account config.EntradeAccountConfig, cfg config.EntradeConfig) config.EntradeAccountConfig {
	account.ID = strings.ToUpper(strings.TrimSpace(account.ID))
	account.Environment = strings.ToLower(strings.TrimSpace(account.Environment))
	if account.Environment == "" {
		account.Environment = accountEnvironment(account.ID, cfg.Environment)
	}
	if account.Username == "" {
		account.Username = cfg.Username
	}
	if account.Password == "" {
		account.Password = cfg.Password
	}
	if account.InvestorID == "" {
		account.InvestorID = cfg.InvestorID
	}
	if account.AccountNo == "" && !knownVirtualAccount(account.ID) {
		account.AccountNo = cfg.AccountNo
	}
	if account.TradingToken == "" {
		account.TradingToken = cfg.TradingToken
	}
	return account
}

func profileStateKey(profile config.EntradeAccountConfig, fallback string) string {
	key := strings.ToUpper(strings.TrimSpace(profile.ID))
	if key == "" {
		key = strings.ToUpper(strings.TrimSpace(fallback))
	}
	if key == "" {
		return defaultStateKey
	}
	return key
}

func (c *Client) GetAccounts(ctx context.Context) ([]dnsemodel.Account, error) {
	if c.cfg.Mock {
		accounts := c.configuredAccountIDs()
		if len(accounts) == 0 {
			accounts = []string{AccountDemo, AccountReal}
		}
		out := make([]dnsemodel.Account, 0, len(accounts))
		for _, accountNo := range accounts {
			out = append(out, dnsemodel.Account{AccountNo: accountNo, DerivativeAccountStatus: "ACTIVE"})
		}
		return out, nil
	}
	accounts := make([]dnsemodel.Account, 0, 2)
	for _, accountNo := range c.configuredAccountIDs() {
		profile := c.accountProfile(accountNo)
		investorID, err := c.ensureInvestorID(ctx, accountNo)
		if err != nil {
			if c.logger != nil {
				c.logger.Error("entrade_account_login_failed", map[string]any{"accountNo": accountNo, "error": err.Error()})
			}
			continue
		}
		var raw map[string]any
		if err := c.doForAccount(ctx, accountNo, http.MethodGet, fmt.Sprintf("/investors/%s/investor_account", url.PathEscape(investorID)), nil, &raw); err != nil {
			if c.logger != nil {
				c.logger.Error("entrade_account_lookup_failed", map[string]any{"accountNo": accountNo, "error": err.Error()})
			}
			continue
		}
		status := firstString(raw, "status")
		if status == "" {
			status = "UNKNOWN"
		}
		accounts = append(accounts, dnsemodel.Account{
			AccountNo:               defaultString(profile.ID, accountNo),
			DerivativeAccountStatus: status,
		})
	}
	if len(accounts) == 0 {
		return nil, errors.New("entrade returned no orderable demo/real accounts")
	}
	return accounts, nil
}

func (c *Client) LinkAccount(ctx context.Context, username, password, environment string) (LinkAccountResult, error) {
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	environment = strings.ToLower(strings.TrimSpace(environment))
	if environment == "" || environment == "prod" {
		environment = "real"
	}
	if environment != "real" && environment != "paper" {
		environment = "real"
	}
	if username == "" || password == "" {
		return LinkAccountResult{}, errors.New("username and password are required")
	}

	token, expiresAt, investorID, err := c.authenticate(ctx, username, password)
	if err != nil {
		return LinkAccountResult{}, err
	}
	if investorID == "" {
		return LinkAccountResult{}, errors.New("entrade investor_id is missing and cannot be decoded from token")
	}

	baseURL := c.cfg.BaseURL
	if environment != "real" {
		baseURL = c.cfg.PaperBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")

	var accountRaw map[string]any
	if err := c.doWithToken(ctx, token, http.MethodGet, baseURL+fmt.Sprintf("/investors/%s/investor_account", url.PathEscape(investorID)), nil, &accountRaw); err != nil {
		return LinkAccountResult{}, fmt.Errorf("load entrade master account: %w", err)
	}

	var packagesRaw map[string]any
	var loanPackages []dnsemodel.LoanPackage
	if err := c.doWithToken(ctx, token, http.MethodGet, baseURL+fmt.Sprintf("/investors/%s/derivative_margin_portfolios", url.PathEscape(investorID)), nil, &packagesRaw); err == nil {
		loanPackages = simplifyLoanPackages(packagesRaw, "VN30F1M")
	}

	accountID := firstString(accountRaw, "investorAccountId", "id", "investorId")
	return LinkAccountResult{
		Username:          username,
		Environment:       environment,
		InvestorID:        investorID,
		InvestorAccountID: accountID,
		Status:            defaultString(firstString(accountRaw, "status"), "UNKNOWN"),
		TotalCash:         firstFloat(accountRaw, "totalCash"),
		AvailableCash:     firstFloat(accountRaw, "availableCash"),
		LoanPackages:      loanPackages,
		TokenExpiresAt:    expiresAt,
	}, nil
}

func (c *Client) GetLoanPackages(ctx context.Context, accountNo, symbol, marketType string) ([]dnsemodel.LoanPackage, error) {
	if c.cfg.Mock {
		return []dnsemodel.LoanPackage{{ID: 34, Name: "Mock Entrade derivative margin", Type: "DERIVATIVE", InitialRate: 0.05}}, nil
	}
	investorID, err := c.ensureInvestorID(ctx, accountNo)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := c.doForAccount(ctx, accountNo, http.MethodGet, fmt.Sprintf("/investors/%s/derivative_margin_portfolios", url.PathEscape(investorID)), nil, &raw); err != nil {
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
	investorID, err := c.ensureInvestorID(ctx, req.AccountNo)
	if err != nil {
		return dnsemodel.PlaceOrderResponse{}, err
	}
	symbol, err := c.resolveDerivativeSymbol(ctx, req.AccountNo, req.Symbol)
	if err != nil {
		return dnsemodel.PlaceOrderResponse{}, err
	}
	portfolioID := 0
	if req.LoanPackageID != nil {
		portfolioID = *req.LoanPackageID
	}
	if portfolioID <= 0 {
		profile := c.accountProfile(req.AccountNo)
		portfolioID = profile.LoanPackageID
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
	if err := c.doForAccount(ctx, req.AccountNo, http.MethodPost, "/derivative/orders", payload, &raw); err != nil {
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
	if err := c.doForAccount(ctx, accountNo, http.MethodGet, "/derivative/orders/"+url.PathEscape(orderID), nil, &raw); err != nil {
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
	if err := c.doForAccount(ctx, accountNo, http.MethodDelete, "/derivative/orders/"+url.PathEscape(orderID), nil, &raw); err != nil {
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
	investorAccountID, err := c.resolveInvestorAccountID(ctx, accountNo)
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/derivative/deals?investorAccountId=%s&_start=0&_end=100&_sort=modifiedDate&_order=DESC", url.QueryEscape(investorAccountID))
	var raw map[string]any
	if err := c.doForAccount(ctx, accountNo, http.MethodGet, path, nil, &raw); err != nil {
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

func (c *Client) CloseDealsBySymbol(ctx context.Context, accountNo, symbol, orderType string) ([]dnsemodel.CloseDealResponse, error) {
	if c.cfg.Mock {
		return []dnsemodel.CloseDealResponse{{Success: true, DealID: "MOCK-DEAL", Status: "CLOSE_SUBMITTED", AccountNo: accountNo}}, nil
	}
	symbol = strings.ToUpper(strings.TrimSpace(defaultString(symbol, "VN30F1M")))
	if symbol != "VN30F1M" {
		return nil, errors.New("entrade provider supports closing VN30F1M only")
	}
	orderType = strings.ToUpper(strings.TrimSpace(orderType))
	if orderType == "" {
		orderType = "MTL"
	}
	deals, err := c.fetchDeals(ctx, accountNo)
	if err != nil {
		return nil, err
	}
	out := make([]dnsemodel.CloseDealResponse, 0, len(deals))
	for _, deal := range deals {
		dealID := firstString(deal, "id")
		if dealID == "" || isClosedDeal(deal) {
			continue
		}
		payload := map[string]any{
			"orderType":   orderType,
			"triggeredBy": "mt5-bot-close-deal",
		}
		var raw map[string]any
		if err := c.doForAccount(ctx, accountNo, http.MethodPost, "/derivative/deals/"+url.PathEscape(dealID)+"/_close_deal", payload, &raw); err != nil {
			out = append(out, dnsemodel.CloseDealResponse{Success: false, DealID: dealID, AccountNo: accountNo, Status: "ERROR", RawResponse: err.Error()})
			continue
		}
		rawBody, _ := json.Marshal(raw)
		out = append(out, dnsemodel.CloseDealResponse{
			Success:     true,
			DealID:      dealID,
			Status:      defaultString(firstString(raw, "status", "state"), "CLOSE_SUBMITTED"),
			AccountNo:   accountNo,
			RawResponse: string(rawBody),
		})
	}
	return out, nil
}

func (c *Client) fetchDeals(ctx context.Context, accountNo string) ([]map[string]any, error) {
	investorAccountID, err := c.resolveInvestorAccountID(ctx, accountNo)
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/derivative/deals?investorAccountId=%s&_start=0&_end=100&_sort=modifiedDate&_order=DESC", url.QueryEscape(investorAccountID))
	var raw map[string]any
	if err := c.doForAccount(ctx, accountNo, http.MethodGet, path, nil, &raw); err != nil {
		return nil, err
	}
	items, _ := raw["data"].([]any)
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if firstInt(m, "openQuantity", "remainingTradeableQuantity") <= 0 || isClosedDeal(m) {
			continue
		}
		out = append(out, m)
	}
	return out, nil
}

func (c *Client) ResolveDerivativeSymbol(ctx context.Context, displaySymbol string) (string, error) {
	return c.resolveDerivativeSymbol(ctx, "", displaySymbol)
}

func (c *Client) resolveDerivativeSymbol(ctx context.Context, accountNo, displaySymbol string) (string, error) {
	displaySymbol = strings.ToUpper(strings.TrimSpace(displaySymbol))
	if displaySymbol == "" || displaySymbol == "VN30F1M" {
		displaySymbol = "VN30F1M"
	}
	var raw map[string]any
	if err := c.doForAccount(ctx, accountNo, http.MethodGet, "/derivatives", nil, &raw); err != nil {
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

func (c *Client) ensureToken(ctx context.Context, accountNo string) error {
	profile := c.accountProfile(accountNo)
	key := profileStateKey(profile, accountNo)
	c.mu.RLock()
	state := c.states[key]
	token := ""
	expiresAt := time.Time{}
	if state != nil {
		token = state.token
		expiresAt = state.expiresAt
	}
	c.mu.RUnlock()
	if token != "" && time.Now().UTC().Before(expiresAt.Add(-5*time.Minute)) {
		return nil
	}
	return c.login(ctx, accountNo)
}

func (c *Client) ensureInvestorID(ctx context.Context, accountNo string) (string, error) {
	profile := c.accountProfile(accountNo)
	if c.cfg.Mock {
		return defaultString(profile.InvestorID, defaultString(c.cfg.InvestorID, "1000000036")), nil
	}
	if strings.TrimSpace(profile.InvestorID) != "" {
		return strings.TrimSpace(profile.InvestorID), nil
	}
	key := profileStateKey(profile, accountNo)
	c.mu.RLock()
	if state := c.states[key]; state != nil && strings.TrimSpace(state.investorID) != "" {
		investorID := strings.TrimSpace(state.investorID)
		c.mu.RUnlock()
		return investorID, nil
	}
	c.mu.RUnlock()
	if err := c.ensureToken(ctx, accountNo); err != nil {
		return "", err
	}
	c.mu.RLock()
	state := c.states[key]
	token := ""
	if state != nil {
		token = state.token
	}
	c.mu.RUnlock()
	investorID := investorIDFromJWT(token)
	if investorID == "" {
		return "", errors.New("entrade investor_id is missing and cannot be decoded from token")
	}
	c.mu.Lock()
	state = c.states[key]
	if state == nil {
		state = &accountState{}
		c.states[key] = state
	}
	state.investorID = investorID
	c.mu.Unlock()
	return investorID, nil
}

func (c *Client) login(ctx context.Context, accountNo string) error {
	profile := c.accountProfile(accountNo)
	if strings.TrimSpace(profile.Username) == "" || strings.TrimSpace(profile.Password) == "" {
		return fmt.Errorf("entrade username/password are required for account %s", profileStateKey(profile, accountNo))
	}
	auth, expiresAt, investorID, err := c.authenticate(ctx, profile.Username, profile.Password)
	if err != nil {
		return err
	}
	key := profileStateKey(profile, accountNo)
	c.mu.Lock()
	state := c.states[key]
	if state == nil {
		state = &accountState{}
		c.states[key] = state
	}
	state.token = auth
	state.expiresAt = expiresAt
	if state.investorID == "" {
		state.investorID = defaultString(profile.InvestorID, investorID)
	}
	c.token = auth
	c.expiresAt = expiresAt
	if c.investorID == "" {
		c.investorID = investorID
	}
	c.mu.Unlock()
	if c.logger != nil {
		c.logger.Info("entrade_login_success", map[string]any{"accountNo": key, "expiresAt": expiresAt.UTC().Format(time.RFC3339), "investorId": investorID})
	}
	return nil
}

func (c *Client) authenticate(ctx context.Context, username, password string) (string, time.Time, string, error) {
	payload := map[string]string{"username": username, "password": password}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.AuthURL, bytes.NewReader(body))
	if err != nil {
		return "", time.Time{}, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", time.Time{}, "", err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", time.Time{}, "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", time.Time{}, "", fmt.Errorf("entrade auth returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var auth authResponse
	if err := json.Unmarshal(respBody, &auth); err != nil {
		return "", time.Time{}, "", err
	}
	if auth.Token == "" {
		return "", time.Time{}, "", errors.New("entrade auth response missing token")
	}
	expiresAt := jwtExpiresAt(auth.Token)
	if expiresAt.IsZero() {
		expiresAt = time.Now().UTC().Add(8 * time.Hour)
	}
	investorID := investorIDFromJWT(auth.Token)
	return auth.Token, expiresAt, investorID, nil
}

func (c *Client) do(ctx context.Context, method, path string, payload any, out any) error {
	return c.doForAccount(ctx, "", method, path, payload, out)
}

func (c *Client) doForAccount(ctx context.Context, accountNo, method, path string, payload any, out any) error {
	return c.doInternal(ctx, accountNo, method, path, payload, out, true)
}

func (c *Client) doWithToken(ctx context.Context, token, method, fullURL string, payload any, out any) error {
	var body []byte
	var err error
	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, fullURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", "Bearer "+token)
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

func (c *Client) doInternal(ctx context.Context, accountNo, method, path string, payload any, out any, allowRetry bool) error {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if err := c.ensureToken(ctx, accountNo); err != nil {
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
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.baseURLForAccount(accountNo), "/")+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.mu.RLock()
	profile := c.accountProfile(accountNo)
	key := profileStateKey(profile, accountNo)
	token := ""
	if state := c.states[key]; state != nil {
		token = state.token
	}
	c.mu.RUnlock()
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if tradingToken := strings.TrimSpace(defaultString(profile.TradingToken, c.cfg.TradingToken)); tradingToken != "" {
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
		if err := c.login(ctx, accountNo); err == nil {
			return c.doInternal(ctx, accountNo, method, path, payload, out, false)
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
	return c.baseURLForAccount("")
}

func (c *Client) baseURLForAccount(accountNo string) string {
	profile := c.accountProfile(accountNo)
	env := accountEnvironment(defaultString(profile.ID, accountNo), defaultString(profile.Environment, c.cfg.Environment))
	if env == "real" {
		return c.cfg.BaseURL
	}
	return c.cfg.PaperBaseURL
}

func accountEnvironment(accountNo, fallback string) string {
	accountNo = strings.ToUpper(strings.TrimSpace(accountNo))
	switch accountNo {
	case AccountReal:
		return "real"
	case AccountDemo:
		return "paper"
	}
	fallback = strings.ToLower(strings.TrimSpace(fallback))
	if fallback == "real" || fallback == "prod" {
		return "real"
	}
	return "paper"
}

func knownVirtualAccount(accountNo string) bool {
	accountNo = strings.ToUpper(strings.TrimSpace(accountNo))
	return accountNo == "" || accountNo == AccountDemo || accountNo == AccountReal || accountNo == defaultStateKey
}

func (c *Client) resolveInvestorAccountID(ctx context.Context, accountNo string) (string, error) {
	accountNo = strings.TrimSpace(accountNo)
	profile := c.accountProfile(accountNo)
	if strings.TrimSpace(profile.AccountNo) != "" {
		return strings.TrimSpace(profile.AccountNo), nil
	}
	if accountNo != "" && !knownVirtualAccount(accountNo) {
		return accountNo, nil
	}
	investorID, err := c.ensureInvestorID(ctx, accountNo)
	if err != nil {
		return "", err
	}
	var raw map[string]any
	if err := c.doForAccount(ctx, accountNo, http.MethodGet, fmt.Sprintf("/investors/%s/investor_account", url.PathEscape(investorID)), nil, &raw); err != nil {
		return "", err
	}
	id := firstString(raw, "investorAccountId", "id", "investorId")
	if id == "" {
		id = investorID
	}
	return id, nil
}

func isClosedDeal(item map[string]any) bool {
	return strings.EqualFold(firstString(item, "closed"), "true") ||
		strings.EqualFold(firstString(item, "state"), "CLOSED") ||
		strings.EqualFold(firstString(item, "status"), "CLOSED")
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
