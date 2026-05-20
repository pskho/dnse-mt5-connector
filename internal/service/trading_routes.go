package service

import (
	"context"
	"errors"
	"sort"
	"strings"

	"dnse-mt5-connector/internal/config"
	"dnse-mt5-connector/internal/dnsemodel"
)

const (
	SourceDashboard  = "dashboard"
	SourceMT5Manual  = "mt5_manual"
	SourceSuperTrend = "supertrend"
	SourceSignalAPI  = "signal_api"
	SourceOrderAPI   = "order_api"
)

type TradingRouteManager struct {
	cfg config.TradingConfig
}

func NewTradingRouteManager(cfg config.TradingConfig) *TradingRouteManager {
	return &TradingRouteManager{cfg: cfg}
}

func (m *TradingRouteManager) GroupAccounts(source, groupID, side, symbol string, requestedQuantity int) (string, []string, error) {
	if m == nil {
		return "", nil, nil
	}
	groupID = normalizeGroupID(groupID)
	if groupID == "" {
		groupID = m.routeForSource(source)
	}
	group, ok := m.groupByID(groupID)
	if !ok {
		return groupID, nil, errors.New("execution group not found: " + groupID)
	}
	if !group.Enabled {
		return group.ID, nil, errors.New("execution group is disabled: " + group.Name)
	}
	side = strings.ToUpper(strings.TrimSpace(side))
	if side == "BUY" && !group.AllowBuy {
		return group.ID, nil, errors.New("execution group does not allow BUY: " + group.Name)
	}
	if side == "SELL" && !group.AllowSell {
		return group.ID, nil, errors.New("execution group does not allow SELL: " + group.Name)
	}
	if group.MaxQuantity > 0 && requestedQuantity > group.MaxQuantity {
		return group.ID, nil, errors.New("quantity exceeds execution group limit")
	}
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if len(group.Symbols) > 0 && symbol != "" {
		allowed := false
		for _, item := range group.Symbols {
			if strings.EqualFold(item, symbol) {
				allowed = true
				break
			}
		}
		if !allowed {
			return group.ID, nil, errors.New("symbol is not allowed for execution group: " + group.Name)
		}
	}
	accounts := normalizeAccountList(group.AccountNos)
	if len(accounts) == 0 {
		return group.ID, nil, errors.New("execution group has no accounts: " + group.Name)
	}
	return group.ID, accounts, nil
}

func (m *TradingRouteManager) ApplyDefaults(req *OrderRequest) string {
	if m == nil || req == nil {
		return ""
	}
	groupID := normalizeGroupID(req.RouteGroupID)
	if groupID == "" {
		groupID = m.routeForSource(req.Source)
	}
	group, ok := m.groupByID(groupID)
	if !ok {
		return groupID
	}
	if req.OrderType == "" && group.OrderType != "" {
		req.OrderType = group.OrderType
	}
	if req.Quantity <= 0 && group.DefaultQuantity > 0 {
		req.Quantity = group.DefaultQuantity
	}
	if req.MarketType == "" && group.MarketType != "" {
		req.MarketType = group.MarketType
	}
	if req.OrderCategory == "" && group.OrderCategory != "" {
		req.OrderCategory = group.OrderCategory
	}
	req.RouteGroupID = groupID
	return groupID
}

func (m *TradingRouteManager) routeForSource(source string) string {
	source = normalizeSource(source)
	switch source {
	case SourceDashboard:
		return normalizeGroupID(m.cfg.Routes.Dashboard)
	case SourceMT5Manual:
		return normalizeGroupID(m.cfg.Routes.MT5Manual)
	case SourceSuperTrend:
		return normalizeGroupID(m.cfg.Routes.SuperTrend)
	case SourceSignalAPI:
		return normalizeGroupID(m.cfg.Routes.SignalAPI)
	default:
		return normalizeGroupID(m.cfg.Routes.OrderAPI)
	}
}

func (m *TradingRouteManager) groupByID(groupID string) (config.ExecutionGroupConfig, bool) {
	groupID = normalizeGroupID(groupID)
	for _, group := range m.cfg.Groups {
		if normalizeGroupID(group.ID) == groupID {
			return group, true
		}
	}
	return config.ExecutionGroupConfig{}, false
}

func (m *TradingRouteManager) ConcreteEntradeAccounts() []string {
	if m == nil {
		return nil
	}
	seen := map[string]struct{}{}
	out := []string{}
	for _, group := range m.cfg.Groups {
		for _, account := range group.AccountNos {
			account = strings.ToUpper(strings.TrimSpace(account))
			if account == "" || account == "ENTRADE_DEMO" || account == "ENTRADE_REAL" || !strings.HasPrefix(account, "ENTRADE_") {
				continue
			}
			if _, ok := seen[account]; ok {
				continue
			}
			seen[account] = struct{}{}
			out = append(out, account)
		}
	}
	sort.Strings(out)
	return out
}

func normalizeSource(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "_")
	switch value {
	case "mt5", "manual_mt5", "mt5_manual":
		return SourceMT5Manual
	case "supertrend", "super_trend", "mt5_supertrend":
		return SourceSuperTrend
	case "dashboard", "web":
		return SourceDashboard
	case "signal", "signal_api", "api_signal":
		return SourceSignalAPI
	case "order", "order_api", "api_order":
		return SourceOrderAPI
	default:
		return value
	}
}

func normalizeGroupID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "_", "-")
	value = strings.ReplaceAll(value, " ", "-")
	return value
}

type RoutedTradingClient struct {
	dnse    DNSEClient
	entrade DNSEClient
}

func NewRoutedTradingClient(dnse DNSEClient, entrade DNSEClient) *RoutedTradingClient {
	return &RoutedTradingClient{dnse: dnse, entrade: entrade}
}

func (c *RoutedTradingClient) GetAccounts(ctx context.Context) ([]dnsemodel.Account, error) {
	var out []dnsemodel.Account
	var firstErr error
	if c.dnse != nil {
		accounts, err := c.dnse.GetAccounts(ctx)
		if err != nil {
			firstErr = err
		} else {
			out = append(out, accounts...)
		}
	}
	if c.entrade != nil {
		accounts, err := c.entrade.GetAccounts(ctx)
		if err != nil && firstErr == nil {
			firstErr = err
		} else {
			out = append(out, accounts...)
		}
	}
	out = dedupeAccounts(out)
	if len(out) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return out, nil
}

func (c *RoutedTradingClient) GetLoanPackages(ctx context.Context, accountNo, symbol, marketType string) ([]dnsemodel.LoanPackage, error) {
	client := c.clientForAccount(accountNo)
	if client == nil {
		return nil, errors.New("no trading provider is configured for account")
	}
	return client.GetLoanPackages(ctx, accountNo, symbol, marketType)
}

func (c *RoutedTradingClient) PlaceOrder(ctx context.Context, req dnsemodel.PlaceOrderRequest) (dnsemodel.PlaceOrderResponse, error) {
	client := c.clientForAccount(req.AccountNo)
	if client == nil {
		return dnsemodel.PlaceOrderResponse{}, errors.New("no trading provider is configured for account")
	}
	return client.PlaceOrder(ctx, req)
}

func (c *RoutedTradingClient) GetOrderStatus(ctx context.Context, accountNo, orderID, marketType, orderCategory string) (dnsemodel.OrderStatus, error) {
	client := c.clientForAccount(accountNo)
	if client == nil {
		return dnsemodel.OrderStatus{}, errors.New("no trading provider is configured for account")
	}
	return client.GetOrderStatus(ctx, accountNo, orderID, marketType, orderCategory)
}

func (c *RoutedTradingClient) CancelOrder(ctx context.Context, accountNo, orderID, marketType, orderCategory string) (dnsemodel.CancelOrderResponse, error) {
	client := c.clientForAccount(accountNo)
	if client == nil {
		return dnsemodel.CancelOrderResponse{}, errors.New("no trading provider is configured for account")
	}
	return client.CancelOrder(ctx, accountNo, orderID, marketType, orderCategory)
}

func (c *RoutedTradingClient) GetPositions(ctx context.Context, accountNo, marketType string) ([]dnsemodel.Position, error) {
	base := c.clientForAccount(accountNo)
	if base == nil {
		return nil, errors.New("no trading provider is configured for account")
	}
	client, ok := base.(interface {
		GetPositions(ctx context.Context, accountNo, marketType string) ([]dnsemodel.Position, error)
	})
	if !ok {
		return nil, errors.New("position lookup is not supported for account")
	}
	return client.GetPositions(ctx, accountNo, marketType)
}

func (c *RoutedTradingClient) CloseDealsBySymbol(ctx context.Context, accountNo, symbol, orderType string) ([]dnsemodel.CloseDealResponse, error) {
	base := c.clientForAccount(accountNo)
	if base == nil {
		return nil, errors.New("no trading provider is configured for account")
	}
	client, ok := base.(DealCloser)
	if !ok {
		return nil, errors.New("close deal is not supported for account")
	}
	return client.CloseDealsBySymbol(ctx, accountNo, symbol, orderType)
}

func (c *RoutedTradingClient) clientForAccount(accountNo string) DNSEClient {
	if isEntradeAccount(accountNo) && c.entrade != nil {
		return c.entrade
	}
	if c.dnse != nil {
		return c.dnse
	}
	return c.entrade
}

func isEntradeAccount(accountNo string) bool {
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(accountNo)), "ENTRADE")
}

func dedupeAccounts(accounts []dnsemodel.Account) []dnsemodel.Account {
	seen := map[string]struct{}{}
	out := make([]dnsemodel.Account, 0, len(accounts))
	for _, account := range accounts {
		key := strings.ToUpper(strings.TrimSpace(account.AccountNo))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, account)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AccountNo < out[j].AccountNo })
	return out
}
