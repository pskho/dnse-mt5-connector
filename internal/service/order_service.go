package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"dnse-mt5-connector/internal/dnsemodel"
	"dnse-mt5-connector/internal/logger"
	"dnse-mt5-connector/internal/storage"
)

type Store interface {
	InsertOrder(ctx context.Context, order storage.Order) error
	GetOrder(ctx context.Context, id string) (storage.Order, error)
	GetOrderByClientOrderID(ctx context.Context, clientOrderID string) (storage.Order, error)
	UpdateOrderStatus(ctx context.Context, id, status, rawResponse string, updatedAt time.Time) error
	InsertLog(ctx context.Context, level, event, details string) error
}

type DNSEClient interface {
	GetAccounts(ctx context.Context) ([]dnsemodel.Account, error)
	GetLoanPackages(ctx context.Context, accountNo, symbol, marketType string) ([]dnsemodel.LoanPackage, error)
	PlaceOrder(ctx context.Context, req dnsemodel.PlaceOrderRequest) (dnsemodel.PlaceOrderResponse, error)
	GetOrderStatus(ctx context.Context, accountNo, orderID, marketType, orderCategory string) (dnsemodel.OrderStatus, error)
	CancelOrder(ctx context.Context, accountNo, orderID, marketType, orderCategory string) (dnsemodel.CancelOrderResponse, error)
}

type DealCloser interface {
	CloseDealsBySymbol(ctx context.Context, accountNo, symbol, orderType string) ([]dnsemodel.CloseDealResponse, error)
}

type securityDefinitionClient interface {
	GetSecurityDefinition(ctx context.Context, symbol string) ([]map[string]any, error)
}

type RiskEngine interface {
	Check(ctx context.Context, accountNo, symbol, side string, quantity int, price float64) error
}

type OrderService struct {
	store             Store
	dnse              DNSEClient
	risk              RiskEngine
	logger            *logger.FileLogger
	defaultAccountNo  string
	defaultAccountNos []string
	positions         *PositionService
	maxOpenPosition   int
	systemEnabled     bool
	idempotency       map[string]idempotencyEntry
	routes            *TradingRouteManager
	priceResolver     func(symbol, side string) (float64, bool)
	mu                sync.Mutex
}

type idempotencyEntry struct {
	createdAt  time.Time
	processing bool
}

func NewOrderService(store Store, dnseClient DNSEClient, riskEngine RiskEngine, appLog *logger.FileLogger, defaultAccountNo string, positions *PositionService, maxOpenPosition int, routes *TradingRouteManager) *OrderService {
	if maxOpenPosition <= 0 {
		maxOpenPosition = 10
	}
	defaultAccounts := normalizeAccountList(strings.Split(defaultAccountNo, ","))
	if len(defaultAccounts) == 0 && strings.TrimSpace(defaultAccountNo) != "" {
		defaultAccounts = []string{strings.TrimSpace(defaultAccountNo)}
	}
	primaryAccount := ""
	if len(defaultAccounts) > 0 {
		primaryAccount = defaultAccounts[0]
	}
	return &OrderService{
		store:             store,
		dnse:              dnseClient,
		risk:              riskEngine,
		logger:            appLog,
		defaultAccountNo:  primaryAccount,
		defaultAccountNos: defaultAccounts,
		positions:         positions,
		maxOpenPosition:   maxOpenPosition,
		systemEnabled:     true,
		idempotency:       make(map[string]idempotencyEntry),
		routes:            routes,
	}
}

func (s *OrderService) SetPriceResolver(resolve func(symbol, side string) (float64, bool)) {
	s.mu.Lock()
	s.priceResolver = resolve
	s.mu.Unlock()
}

func (s *OrderService) PlaceOrder(ctx context.Context, req OrderRequest) (OrderResponse, error) {
	s.log(ctx, "info", "order_received", map[string]any{"request": req})

	if !s.IsEnabled() {
		err := errors.New("system disabled by kill switch")
		s.log(ctx, "error", "risk_rejected_kill_switch", map[string]any{"error": err.Error(), "request": req})
		return OrderResponse{}, err
	}

	normalized, dnseSide, err := s.validateOrder(req)
	if err != nil {
		s.log(ctx, "error", "validation_failed", map[string]any{"error": err.Error(), "request": req})
		return OrderResponse{}, err
	}
	s.log(ctx, "info", "validation_passed", map[string]any{"symbol": normalized.Symbol, "side": normalized.Side})

	s.fillPriceFromRealtime(ctx, &normalized)

	idempotencyKey := orderIdempotencyKey(normalized)
	if !s.reserveOrder(idempotencyKey, 3*time.Second) {
		err := errors.New("duplicate order rejected within 3 seconds")
		s.log(ctx, "error", "risk_rejected_idempotency", map[string]any{
			"error":   err.Error(),
			"key":     idempotencyKey,
			"request": normalized,
		})
		return OrderResponse{}, err
	}
	defer s.finishOrder(idempotencyKey)

	if err := s.risk.Check(ctx, normalized.AccountNo, normalized.Symbol, normalized.Side, normalized.Quantity, normalized.Price); err != nil {
		s.log(ctx, "error", "risk_rejected", map[string]any{"error": err.Error(), "request": normalized})
		return OrderResponse{}, err
	}

	if err := s.checkMarketOpen(time.Now()); err != nil {
		s.log(ctx, "error", "risk_rejected_market_closed", map[string]any{"error": err.Error(), "request": normalized})
		return OrderResponse{}, err
	}

	if err := s.checkPriceBand(ctx, normalized); err != nil {
		s.log(ctx, "error", "risk_rejected_price_band", map[string]any{"error": err.Error(), "request": normalized})
		return OrderResponse{}, err
	}

	if err := s.checkPositionLimit(ctx, normalized); err != nil {
		s.log(ctx, "error", "risk_rejected_position_limit", map[string]any{"error": err.Error(), "request": normalized})
		return OrderResponse{}, err
	}

	if err := s.ensureLoanPackage(ctx, &normalized); err != nil {
		s.log(ctx, "error", "loan_package_resolve_failed", map[string]any{"error": err.Error(), "request": normalized})
		return OrderResponse{}, err
	}

	rawRequest, _ := json.Marshal(normalized)
	apiResp, err := s.dnse.PlaceOrder(ctx, dnsemodel.PlaceOrderRequest{
		ClientOrderID: normalized.ClientOrderID,
		AccountNo:     normalized.AccountNo,
		Symbol:        normalized.Symbol,
		Side:          dnseSide,
		Quantity:      normalized.Quantity,
		Price:         normalized.Price,
		OrderType:     normalized.OrderType,
		LoanPackageID: normalized.LoanPackageID,
		MarketType:    normalized.MarketType,
		OrderCategory: normalized.OrderCategory,
	})
	if err != nil {
		s.log(ctx, "error", "order_submit_failed", map[string]any{"error": err.Error(), "request": normalized})
		return OrderResponse{}, err
	}

	now := time.Now().UTC()
	order := storage.Order{
		ID:            apiResp.OrderID,
		ClientOrderID: normalized.ClientOrderID,
		AccountNo:     normalized.AccountNo,
		Symbol:        normalized.Symbol,
		Side:          normalized.Side,
		DNSESide:      dnseSide,
		Quantity:      normalized.Quantity,
		Price:         normalized.Price,
		OrderType:     normalized.OrderType,
		Status:        apiResp.Status,
		RawRequest:    string(rawRequest),
		RawResponse:   apiResp.RawResponse,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.store.InsertOrder(ctx, order); err != nil {
		s.log(ctx, "error", "order_persist_failed", map[string]any{"error": err.Error(), "orderId": apiResp.OrderID})
		return OrderResponse{
			Success: true,
			OrderID: apiResp.OrderID,
			Status:  apiResp.Status,
			Message: "order accepted by DNSE but local persistence failed; check logs immediately",
		}, nil
	}

	s.log(ctx, "info", "order_accepted", map[string]any{"orderId": apiResp.OrderID, "status": apiResp.Status})
	return OrderResponse{Success: true, OrderID: apiResp.OrderID, Status: apiResp.Status, AccountNo: normalized.AccountNo}, nil
}

func (s *OrderService) PlaceOrders(ctx context.Context, req OrderRequest) ([]OrderResponse, error) {
	if req.Source == "" {
		req.Source = SourceOrderAPI
	}
	if s.routes != nil {
		s.routes.ApplyDefaults(&req)
	}
	accounts := normalizeAccountList(req.AccountNos)
	if len(accounts) == 0 && strings.TrimSpace(req.AccountNo) != "" {
		accounts = []string{strings.TrimSpace(req.AccountNo)}
	}
	accounts = s.resolveAccountAliases(accounts)
	if len(accounts) == 0 && s.routes != nil {
		_, routedAccounts, routeErr := s.routes.GroupAccounts(req.Source, req.RouteGroupID, req.Side, req.Symbol, req.Quantity)
		if routeErr != nil {
			return nil, routeErr
		}
		accounts = routedAccounts
		accounts = s.resolveAccountAliases(accounts)
	}
	if len(accounts) == 0 {
		accounts = append(accounts, s.defaultAccountNos...)
		accounts = s.resolveAccountAliases(accounts)
	}
	if len(accounts) == 0 {
		resp, err := s.PlaceOrder(ctx, req)
		if err != nil {
			return nil, err
		}
		return []OrderResponse{resp}, nil
	}
	out := make([]OrderResponse, 0, len(accounts))
	var firstErr error
	for _, accountNo := range accounts {
		itemReq := req
		itemReq.AccountNo = accountNo
		itemReq.AccountNos = nil
		if len(accounts) > 1 && strings.TrimSpace(itemReq.ClientOrderID) != "" {
			itemReq.ClientOrderID = itemReq.ClientOrderID + "-" + accountNo
		}
		resp, err := s.PlaceOrder(ctx, itemReq)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			out = append(out, OrderResponse{Success: false, AccountNo: accountNo, Message: err.Error()})
			continue
		}
		out = append(out, resp)
	}
	if firstErr != nil {
		allFailed := true
		for _, resp := range out {
			if resp.Success {
				allFailed = false
				break
			}
		}
		if allFailed {
			return out, firstErr
		}
	}
	return out, nil
}

func (s *OrderService) CloseDeals(ctx context.Context, req CloseDealRequest) ([]dnsemodel.CloseDealResponse, error) {
	closer, ok := s.dnse.(DealCloser)
	if !ok {
		return nil, errors.New("close deal is not supported by the selected trading provider")
	}
	symbol := strings.ToUpper(strings.TrimSpace(req.Symbol))
	if symbol == "" {
		symbol = "VN30F1M"
	}
	orderType := strings.ToUpper(strings.TrimSpace(req.OrderType))
	if orderType == "" {
		orderType = "MTL"
	}
	accounts := normalizeAccountList(req.AccountNos)
	if len(accounts) == 0 && strings.TrimSpace(req.AccountNo) != "" {
		accounts = []string{strings.TrimSpace(req.AccountNo)}
	}
	accounts = s.resolveAccountAliases(accounts)
	if len(accounts) == 0 && s.routes != nil {
		source := req.Source
		if source == "" {
			source = SourceSignalAPI
		}
		_, routedAccounts, routeErr := s.routes.GroupAccounts(source, req.RouteGroupID, "SELL", symbol, 1)
		if routeErr != nil {
			return nil, routeErr
		}
		accounts = routedAccounts
		accounts = s.resolveAccountAliases(accounts)
	}
	if len(accounts) == 0 {
		accounts = append(accounts, s.defaultAccountNos...)
		accounts = s.resolveAccountAliases(accounts)
	}
	if len(accounts) == 0 {
		return nil, errors.New("accountNo or accountNos is required for close deal")
	}
	out := make([]dnsemodel.CloseDealResponse, 0, len(accounts))
	var firstErr error
	for _, accountNo := range accounts {
		items, err := closer.CloseDealsBySymbol(ctx, accountNo, symbol, orderType)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			out = append(out, dnsemodel.CloseDealResponse{Success: false, AccountNo: accountNo, Status: "ERROR", RawResponse: err.Error()})
			continue
		}
		if len(items) == 0 {
			out = append(out, dnsemodel.CloseDealResponse{Success: true, AccountNo: accountNo, Status: "NO_OPEN_DEAL"})
			continue
		}
		for _, item := range items {
			item.AccountNo = accountNo
			out = append(out, item)
		}
	}
	if firstErr != nil {
		allFailed := true
		for _, item := range out {
			if item.Success {
				allFailed = false
				break
			}
		}
		if allFailed {
			return out, firstErr
		}
	}
	return out, nil
}

func (s *OrderService) Accounts(ctx context.Context) ([]Account, error) {
	accounts, err := s.dnse.GetAccounts(ctx)
	if err != nil {
		s.log(ctx, "error", "accounts_failed", map[string]any{"error": err.Error()})
		return nil, err
	}
	out := make([]Account, 0, len(accounts))
	for _, account := range accounts {
		out = append(out, Account{
			AccountNo: account.AccountNo,
			Type:      account.DerivativeAccountStatus,
		})
	}
	return out, nil
}

func (s *OrderService) OrderableAccounts(ctx context.Context) ([]Account, error) {
	accounts, err := s.Accounts(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Account, 0, len(accounts))
	for _, account := range accounts {
		status := strings.ToUpper(strings.TrimSpace(account.Type))
		if status == "" || status == "ACTIVE" || strings.Contains(status, "ACTIVE") {
			out = append(out, account)
		}
	}
	return out, nil
}

func (s *OrderService) Order(ctx context.Context, id string) (OrderStatusResponse, error) {
	order, err := s.store.GetOrder(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return OrderStatusResponse{}, err
		}
		s.log(ctx, "error", "order_lookup_failed", map[string]any{"error": err.Error(), "orderId": id})
		return OrderStatusResponse{}, err
	}
	marketType, orderCategory := orderLookupParams(order)
	status, err := s.dnse.GetOrderStatus(ctx, order.AccountNo, id, marketType, orderCategory)
	if err != nil {
		s.log(ctx, "error", "order_status_failed", map[string]any{"error": err.Error(), "orderId": id})
		return OrderStatusResponse{OrderID: order.ID, Status: order.Status, Stale: true, Warning: err.Error()}, nil
	}
	if err := s.store.UpdateOrderStatus(ctx, order.ID, status.Status, status.RawResponse, time.Now().UTC()); err != nil {
		s.log(ctx, "error", "order_status_persist_failed", map[string]any{"error": err.Error(), "orderId": id})
	}
	s.log(ctx, "info", "order_status_updated", map[string]any{"orderId": id, "status": status.Status, "filledQuantity": status.FilledQuantity, "remainingQuantity": status.RemainingQuantity})
	return OrderStatusResponse{
		OrderID:           id,
		Status:            status.Status,
		FilledQuantity:    status.FilledQuantity,
		RemainingQuantity: status.RemainingQuantity,
	}, nil
}

func (s *OrderService) OrderByClient(ctx context.Context, clientOrderID string) (OrderStatusResponse, error) {
	order, err := s.store.GetOrderByClientOrderID(ctx, clientOrderID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return OrderStatusResponse{}, err
		}
		s.log(ctx, "error", "order_client_lookup_failed", map[string]any{"error": err.Error(), "clientOrderId": clientOrderID})
		return OrderStatusResponse{}, err
	}
	return s.Order(ctx, order.ID)
}

func (s *OrderService) CancelOrder(ctx context.Context, req CancelRequest) (CancelResponse, error) {
	req.OrderID = strings.TrimSpace(req.OrderID)
	req.AccountNo = strings.TrimSpace(req.AccountNo)
	req.MarketType = strings.ToUpper(strings.TrimSpace(req.MarketType))
	req.OrderCategory = strings.ToUpper(strings.TrimSpace(req.OrderCategory))
	if req.OrderID == "" {
		return CancelResponse{}, errors.New("orderId is required")
	}
	if req.OrderCategory == "" {
		req.OrderCategory = "NORMAL"
	}

	order, err := s.store.GetOrder(ctx, req.OrderID)
	if err == nil {
		if req.AccountNo == "" {
			req.AccountNo = order.AccountNo
		}
		if req.MarketType == "" || req.OrderCategory == "" {
			marketType, orderCategory := orderLookupParams(order)
			if req.MarketType == "" {
				req.MarketType = marketType
			}
			if req.OrderCategory == "" {
				req.OrderCategory = orderCategory
			}
		}
	} else if !errors.Is(err, storage.ErrNotFound) {
		return CancelResponse{}, err
	}
	if req.AccountNo == "" {
		req.AccountNo = s.defaultAccountNo
	}
	if req.MarketType == "" {
		req.MarketType = "DERIVATIVE"
	}
	if req.AccountNo == "" {
		return CancelResponse{}, errors.New("accountNo is required")
	}

	s.log(ctx, "info", "cancel_request", map[string]any{"request": req})
	resp, err := s.dnse.CancelOrder(ctx, req.AccountNo, req.OrderID, req.MarketType, req.OrderCategory)
	if err != nil {
		s.log(ctx, "error", "cancel_failed", map[string]any{"error": err.Error(), "request": req})
		return CancelResponse{}, err
	}
	if err := s.store.UpdateOrderStatus(ctx, req.OrderID, resp.Status, resp.RawResponse, time.Now().UTC()); err != nil && !errors.Is(err, storage.ErrNotFound) {
		s.log(ctx, "error", "cancel_status_persist_failed", map[string]any{"error": err.Error(), "orderId": req.OrderID})
	}
	s.log(ctx, "info", "cancel_submitted", map[string]any{"orderId": req.OrderID, "status": resp.Status})
	return CancelResponse{
		Success:           true,
		OrderID:           resp.OrderID,
		Status:            resp.Status,
		FilledQuantity:    resp.FilledQuantity,
		RemainingQuantity: resp.RemainingQuantity,
	}, nil
}

func (s *OrderService) SetEnabled(ctx context.Context, enabled bool) {
	s.mu.Lock()
	s.systemEnabled = enabled
	s.mu.Unlock()
	s.log(ctx, "error", "kill_switch_updated", map[string]any{"enabled": enabled})
}

func (s *OrderService) IsEnabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.systemEnabled
}

func (s *OrderService) validateOrder(req OrderRequest) (OrderRequest, string, error) {
	req.ClientOrderID = strings.TrimSpace(req.ClientOrderID)
	req.AccountNo = strings.TrimSpace(req.AccountNo)
	req.Source = normalizeSource(req.Source)
	req.RouteGroupID = normalizeGroupID(req.RouteGroupID)
	req.Symbol = strings.ToUpper(strings.TrimSpace(req.Symbol))
	req.Side = strings.ToUpper(strings.TrimSpace(req.Side))
	req.OrderType = strings.ToUpper(strings.TrimSpace(req.OrderType))
	req.MarketType = strings.ToUpper(strings.TrimSpace(req.MarketType))
	req.OrderCategory = strings.ToUpper(strings.TrimSpace(req.OrderCategory))
	if s.routes != nil {
		s.routes.ApplyDefaults(&req)
	}
	if req.OrderType == "" {
		req.OrderType = "LO"
	}
	if req.OrderCategory == "" {
		req.OrderCategory = "NORMAL"
	}

	if req.ClientOrderID == "" {
		req.ClientOrderID = fmt.Sprintf("mt5-%d", time.Now().UnixNano())
	}
	if req.AccountNo == "" {
		req.AccountNo = s.defaultAccountNo
	}
	req.AccountNo = s.resolveAccountAlias(req.AccountNo)
	if req.AccountNo == "" {
		req.AccountNo = s.defaultAccountNo
	}
	if req.AccountNo == "" {
		return req, "", errors.New("accountNo is required")
	}
	if req.Symbol == "" {
		return req, "", errors.New("symbol is required")
	}
	if req.MarketType == "" {
		req.MarketType = "DERIVATIVE"
	}
	if req.MarketType != "STOCK" && req.MarketType != "DERIVATIVE" {
		return req, "", errors.New("marketType must be STOCK or DERIVATIVE")
	}
	if req.Quantity <= 0 {
		return req, "", errors.New("quantity must be greater than zero")
	}
	if req.Price < 0 {
		return req, "", errors.New("price cannot be negative")
	}
	if req.OrderType == "LO" && req.Price <= 0 {
		return req, "", errors.New("price must be greater than zero for LO orders")
	}
	switch req.Side {
	case "BUY":
		return req, "NB", nil
	case "SELL":
		return req, "NS", nil
	default:
		return req, "", errors.New("side must be BUY or SELL")
	}
}

func (s *OrderService) ensureLoanPackage(ctx context.Context, req *OrderRequest) error {
	if req.LoanPackageID != nil {
		return nil
	}
	packages, err := s.dnse.GetLoanPackages(ctx, req.AccountNo, req.Symbol, req.MarketType)
	if err != nil {
		return fmt.Errorf("get loan packages: %w", err)
	}
	if len(packages) == 0 {
		return errors.New("no loan packages returned by DNSE for this order")
	}
	selected := selectLoanPackage(req.MarketType, packages)
	if selected.ID <= 0 {
		return errors.New("DNSE returned loan packages without a valid id")
	}
	req.LoanPackageID = &selected.ID
	s.log(ctx, "info", "loan_package_selected", map[string]any{
		"accountNo":     req.AccountNo,
		"symbol":        req.Symbol,
		"marketType":    req.MarketType,
		"loanPackageId": selected.ID,
		"name":          selected.Name,
		"type":          selected.Type,
	})
	return nil
}

func (s *OrderService) resolveAccountAliases(accounts []string) []string {
	out := make([]string, 0, len(accounts))
	seen := make(map[string]struct{}, len(accounts))
	add := func(account string) {
		if account == "" {
			return
		}
		key := strings.ToUpper(account)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, account)
	}
	for _, account := range accounts {
		resolved := s.resolveAccountAlias(account)
		if resolved == "" {
			continue
		}
		add(resolved)
	}
	return out
}

func (s *OrderService) resolveAccountAlias(accountNo string) string {
	accountNo = strings.TrimSpace(accountNo)
	if accountNo == "" {
		return ""
	}
	upper := strings.ToUpper(accountNo)
	defaultUpper := strings.ToUpper(strings.TrimSpace(s.defaultAccountNo))
	if isVirtualEntradeAccount(accountNo) && !strings.HasPrefix(defaultUpper, "ENTRADE_") {
		s.log(context.Background(), "info", "order_account_alias_ignored", map[string]any{
			"accountNo":        upper,
			"defaultAccountNo": s.defaultAccountNo,
			"reason":           "virtual entrade alias received while DNSE is the active trading provider",
		})
		return ""
	}
	return accountNo
}

func isVirtualEntradeAccount(accountNo string) bool {
	upper := strings.ToUpper(strings.TrimSpace(accountNo))
	return upper == "ENTRADE_DEMO" || upper == "ENTRADE_REAL"
}

func (s *OrderService) fillPriceFromRealtime(ctx context.Context, req *OrderRequest) {
	if req == nil || req.Price > 0 {
		return
	}
	s.mu.Lock()
	resolve := s.priceResolver
	s.mu.Unlock()
	if resolve == nil {
		return
	}
	price, ok := resolve(req.Symbol, req.Side)
	if !ok || price <= 0 {
		return
	}
	req.Price = price
	s.log(ctx, "info", "order_price_filled_from_realtime", map[string]any{
		"symbol": req.Symbol,
		"side":   req.Side,
		"price":  price,
		"source": req.Source,
	})
}

func (s *OrderService) checkPositionLimit(ctx context.Context, req OrderRequest) error {
	if s.positions == nil {
		return errors.New("position service is not configured")
	}
	position, err := s.positions.GetCurrentPositionForAccount(ctx, req.AccountNo, req.Symbol)
	if err != nil {
		return fmt.Errorf("cannot fetch current position: %w", err)
	}
	exposure := signedExposureAfter(position, req.Side, req.Quantity)
	if exposure > s.maxOpenPosition {
		return fmt.Errorf("order would increase %s exposure to %d, above max open position %d", req.Symbol, exposure, s.maxOpenPosition)
	}
	s.log(ctx, "info", "position_risk_passed", map[string]any{
		"symbol":          req.Symbol,
		"position":        marshalPosition(position),
		"newExposure":     exposure,
		"maxOpenPosition": s.maxOpenPosition,
	})
	return nil
}

func (s *OrderService) checkPriceBand(ctx context.Context, req OrderRequest) error {
	if req.OrderType != "LO" || req.Price <= 0 {
		return nil
	}
	client, ok := s.dnse.(securityDefinitionClient)
	if !ok {
		return nil
	}
	items, err := client.GetSecurityDefinition(ctx, req.Symbol)
	if err != nil || len(items) == 0 {
		if err != nil {
			s.log(ctx, "error", "price_band_lookup_failed", map[string]any{"symbol": req.Symbol, "error": err.Error()})
		}
		return nil
	}
	for _, item := range items {
		boardID := strings.ToUpper(strings.TrimSpace(firstMapString(item, "boardId", "boardID")))
		if boardID != "" && boardID != "G1" {
			continue
		}
		floorPrice := firstMapFloat(item, "floorPrice")
		ceilingPrice := firstMapFloat(item, "ceilingPrice")
		if floorPrice > 0 && req.Price < floorPrice {
			return fmt.Errorf("price %.2f is below floor %.2f for %s", req.Price, floorPrice, req.Symbol)
		}
		if ceilingPrice > 0 && req.Price > ceilingPrice {
			return fmt.Errorf("price %.2f is above ceiling %.2f for %s", req.Price, ceilingPrice, req.Symbol)
		}
		return nil
	}
	return nil
}

func (s *OrderService) checkMarketOpen(now time.Time) error {
	local, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		local = time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)
	}
	t := now.In(local)
	if t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
		return errors.New("market is closed on weekends")
	}
	minute := t.Hour()*60 + t.Minute()
	morningOpen := 8*60 + 45
	morningClose := 11*60 + 30
	afternoonOpen := 13 * 60
	afternoonClose := 14*60 + 45
	if (minute >= morningOpen && minute <= morningClose) || (minute >= afternoonOpen && minute <= afternoonClose) {
		return nil
	}
	return errors.New("market is closed outside 08:45-11:30 and 13:00-14:45 Asia/Ho_Chi_Minh")
}

func selectLoanPackage(marketType string, packages []dnsemodel.LoanPackage) dnsemodel.LoanPackage {
	if marketType == "STOCK" {
		for _, pkg := range packages {
			if pkg.ID > 0 && strings.EqualFold(pkg.Type, "N") {
				return pkg
			}
		}
		for _, pkg := range packages {
			if pkg.ID > 0 && pkg.InitialRate >= 1 {
				return pkg
			}
		}
	}
	for _, pkg := range packages {
		if pkg.ID > 0 {
			return pkg
		}
	}
	return dnsemodel.LoanPackage{}
}

func firstMapString(item map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			if text, ok := value.(string); ok {
				return strings.TrimSpace(text)
			}
		}
	}
	return ""
}

func firstMapFloat(item map[string]any, keys ...string) float64 {
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

func inferMarketType(symbol string) string {
	if strings.HasPrefix(symbol, "VN30F") {
		return "DERIVATIVE"
	}
	return "STOCK"
}

func orderLookupParams(order storage.Order) (string, string) {
	var req OrderRequest
	if err := json.Unmarshal([]byte(order.RawRequest), &req); err == nil {
		marketType := strings.ToUpper(strings.TrimSpace(req.MarketType))
		if marketType == "" {
			marketType = inferMarketType(order.Symbol)
		}
		orderCategory := strings.ToUpper(strings.TrimSpace(req.OrderCategory))
		if orderCategory == "" {
			orderCategory = "NORMAL"
		}
		return marketType, orderCategory
	}
	return inferMarketType(order.Symbol), "NORMAL"
}

func (s *OrderService) log(ctx context.Context, level, event string, details map[string]any) {
	raw, _ := json.Marshal(details)
	if level == "error" {
		s.logger.Error(event, details)
	} else {
		s.logger.Info(event, details)
	}
	_ = s.store.InsertLog(ctx, level, event, string(raw))
}

func orderIdempotencyKey(req OrderRequest) string {
	raw := fmt.Sprintf("%s|%s|%s|%d", req.AccountNo, req.Symbol, req.Side, req.Quantity)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func normalizeAccountList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func (s *OrderService) reserveOrder(key string, window time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	for existingKey, entry := range s.idempotency {
		if !entry.processing && now.Sub(entry.createdAt) > 30*time.Second {
			delete(s.idempotency, existingKey)
		}
	}

	if entry, exists := s.idempotency[key]; exists {
		if entry.processing || now.Sub(entry.createdAt) < window {
			return false
		}
	}
	s.idempotency[key] = idempotencyEntry{createdAt: now, processing: true}
	return true
}

func (s *OrderService) finishOrder(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.idempotency[key]
	if !exists {
		return
	}
	entry.processing = false
	s.idempotency[key] = entry
}
