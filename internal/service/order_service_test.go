package service

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"dnse-mt5-connector/internal/dnsemodel"
	"dnse-mt5-connector/internal/logger"
	"dnse-mt5-connector/internal/storage"
)

func TestValidateOrderKeepsReferencePriceForMTL(t *testing.T) {
	svc := &OrderService{defaultAccountNo: "0001007412"}

	got, dnseSide, err := svc.validateOrder(OrderRequest{
		ClientOrderID: "test-mtl",
		Symbol:        "VN30F1M",
		Side:          "BUY",
		Quantity:      1,
		Price:         2018.8,
		OrderType:     "MTL",
		MarketType:    "DERIVATIVE",
	})
	if err != nil {
		t.Fatalf("validateOrder returned error: %v", err)
	}
	if dnseSide != "NB" {
		t.Fatalf("expected DNSE side NB, got %s", dnseSide)
	}
	if got.Price != 2018.8 {
		t.Fatalf("expected MTL reference price to be preserved, got %v", got.Price)
	}
}

func TestValidateOrderKeepsPriceForLO(t *testing.T) {
	svc := &OrderService{defaultAccountNo: "0001007412"}

	got, _, err := svc.validateOrder(OrderRequest{
		ClientOrderID: "test-lo",
		Symbol:        "VN30F1M",
		Side:          "SELL",
		Quantity:      1,
		Price:         2018.2,
		OrderType:     "LO",
		MarketType:    "DERIVATIVE",
	})
	if err != nil {
		t.Fatalf("validateOrder returned error: %v", err)
	}
	if got.Price != 2018.2 {
		t.Fatalf("expected LO price to be preserved, got %v", got.Price)
	}
}

func TestPlaceOrderFallsBackFromMTLToLOWhenDNSECannotDeterminePrice(t *testing.T) {
	ctx := context.Background()
	store, err := storage.NewSQLiteStore(filepath.Join(t.TempDir(), "orders.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	appLog, err := logger.NewFileLogger(filepath.Join(t.TempDir(), "app.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer appLog.Close()

	dnse := &fallbackDNSEClient{}
	positions := NewPositionService(dnse, nil, "0001007412")
	svc := NewOrderService(store, dnse, noopRiskEngine{}, appLog, "0001007412", positions, 10, nil)

	resp, err := svc.PlaceOrder(ctx, OrderRequest{
		ClientOrderID: "test-mtl-fallback",
		Symbol:        "VN30F1M",
		Side:          "BUY",
		Quantity:      1,
		Price:         2022.2,
		OrderType:     "MTL",
		MarketType:    "DERIVATIVE",
	})
	if err != nil {
		t.Fatalf("PlaceOrder returned error: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected successful fallback order")
	}
	if len(dnse.orderTypes) != 2 || dnse.orderTypes[0] != "MTL" || dnse.orderTypes[1] != "LO" {
		t.Fatalf("expected MTL then LO submissions, got %#v", dnse.orderTypes)
	}
	order, err := store.GetOrderByClientOrderID(ctx, "test-mtl-fallback")
	if err != nil {
		t.Fatal(err)
	}
	if order.OrderType != "LO" || order.Price != 2022.2 {
		t.Fatalf("expected persisted LO fallback at reference price, got type=%s price=%v", order.OrderType, order.Price)
	}
}

func TestPlaceOrderConvertsMT5ManualMTLToLOBeforeSubmitting(t *testing.T) {
	ctx := context.Background()
	store, err := storage.NewSQLiteStore(filepath.Join(t.TempDir(), "orders.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	appLog, err := logger.NewFileLogger(filepath.Join(t.TempDir(), "app.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer appLog.Close()

	dnse := &recordingDNSEClient{}
	positions := NewPositionService(dnse, nil, "0001007412")
	svc := NewOrderService(store, dnse, noopRiskEngine{}, appLog, "0001007412", positions, 10, nil)

	_, err = svc.PlaceOrder(ctx, OrderRequest{
		ClientOrderID: "test-mt5-mtl-as-lo",
		Source:        SourceMT5Manual,
		Symbol:        "VN30F1M",
		Side:          "BUY",
		Quantity:      1,
		Price:         2023,
		OrderType:     "MTL",
		MarketType:    "DERIVATIVE",
	})
	if err != nil {
		t.Fatalf("PlaceOrder returned error: %v", err)
	}
	if len(dnse.requests) != 1 {
		t.Fatalf("expected one DNSE submission, got %d", len(dnse.requests))
	}
	if got := dnse.requests[0]; got.OrderType != "LO" || got.Price != 2023 {
		t.Fatalf("expected LO submission at reference price, got type=%s price=%v", got.OrderType, got.Price)
	}
}

func TestPlaceOrderResolvesDerivativeAliasBeforeSubmitting(t *testing.T) {
	ctx := context.Background()
	store, err := storage.NewSQLiteStore(filepath.Join(t.TempDir(), "orders.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	appLog, err := logger.NewFileLogger(filepath.Join(t.TempDir(), "app.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer appLog.Close()

	dnse := &recordingDNSEClient{}
	positions := NewPositionService(dnse, nil, "0001007412")
	svc := NewOrderService(store, dnse, noopRiskEngine{}, appLog, "0001007412", positions, 10, nil)
	svc.SetOrderSymbolResolver(func(ctx context.Context, symbol, marketType string) (string, bool, error) {
		if symbol == "VN30F1M" && marketType == "DERIVATIVE" {
			return "41I1G6000", true, nil
		}
		return "", false, nil
	})

	_, err = svc.PlaceOrder(ctx, OrderRequest{
		ClientOrderID: "test-symbol-resolve",
		Source:        SourceMT5Manual,
		Symbol:        "VN30F1M",
		Side:          "SELL",
		Quantity:      1,
		Price:         2021.4,
		OrderType:     "MTL",
		MarketType:    "DERIVATIVE",
	})
	if err != nil {
		t.Fatalf("PlaceOrder returned error: %v", err)
	}
	if len(dnse.requests) != 1 {
		t.Fatalf("expected one DNSE submission, got %d", len(dnse.requests))
	}
	if got := dnse.requests[0]; got.Symbol != "41I1G6000" || got.OrderType != "LO" {
		t.Fatalf("expected resolved LO submission, got symbol=%s type=%s", got.Symbol, got.OrderType)
	}
}

type fallbackDNSEClient struct {
	orderTypes []string
}

func (f *fallbackDNSEClient) GetAccounts(ctx context.Context) ([]dnsemodel.Account, error) {
	return nil, nil
}

func (f *fallbackDNSEClient) GetLoanPackages(ctx context.Context, accountNo, symbol, marketType string) ([]dnsemodel.LoanPackage, error) {
	return []dnsemodel.LoanPackage{{ID: 1306, Type: marketType}}, nil
}

func (f *fallbackDNSEClient) PlaceOrder(ctx context.Context, req dnsemodel.PlaceOrderRequest) (dnsemodel.PlaceOrderResponse, error) {
	f.orderTypes = append(f.orderTypes, req.OrderType)
	if req.OrderType == "MTL" {
		return dnsemodel.PlaceOrderResponse{}, errors.New(`dnse api returned status 400: {"status":400,"code":"STOCK_PRICE_UNDEFINED","message":"Cannot determine stock price"}`)
	}
	return dnsemodel.PlaceOrderResponse{OrderID: "ORDER-LO", Status: "ACCEPTED"}, nil
}

func (f *fallbackDNSEClient) GetOrderStatus(ctx context.Context, accountNo, orderID, marketType, orderCategory string) (dnsemodel.OrderStatus, error) {
	return dnsemodel.OrderStatus{}, nil
}

func (f *fallbackDNSEClient) CancelOrder(ctx context.Context, accountNo, orderID, marketType, orderCategory string) (dnsemodel.CancelOrderResponse, error) {
	return dnsemodel.CancelOrderResponse{}, nil
}

func (f *fallbackDNSEClient) GetPositions(ctx context.Context, accountNo, marketType string) ([]dnsemodel.Position, error) {
	return nil, nil
}

type recordingDNSEClient struct {
	requests []dnsemodel.PlaceOrderRequest
}

func (r *recordingDNSEClient) GetAccounts(ctx context.Context) ([]dnsemodel.Account, error) {
	return nil, nil
}

func (r *recordingDNSEClient) GetLoanPackages(ctx context.Context, accountNo, symbol, marketType string) ([]dnsemodel.LoanPackage, error) {
	return []dnsemodel.LoanPackage{{ID: 1306, Type: marketType}}, nil
}

func (r *recordingDNSEClient) PlaceOrder(ctx context.Context, req dnsemodel.PlaceOrderRequest) (dnsemodel.PlaceOrderResponse, error) {
	r.requests = append(r.requests, req)
	return dnsemodel.PlaceOrderResponse{OrderID: "ORDER-1", Status: "ACCEPTED"}, nil
}

func (r *recordingDNSEClient) GetOrderStatus(ctx context.Context, accountNo, orderID, marketType, orderCategory string) (dnsemodel.OrderStatus, error) {
	return dnsemodel.OrderStatus{}, nil
}

func (r *recordingDNSEClient) CancelOrder(ctx context.Context, accountNo, orderID, marketType, orderCategory string) (dnsemodel.CancelOrderResponse, error) {
	return dnsemodel.CancelOrderResponse{}, nil
}

func (r *recordingDNSEClient) GetPositions(ctx context.Context, accountNo, marketType string) ([]dnsemodel.Position, error) {
	return nil, nil
}

type noopRiskEngine struct{}

func (noopRiskEngine) Check(ctx context.Context, accountNo, symbol, side string, quantity int, price float64) error {
	return nil
}
