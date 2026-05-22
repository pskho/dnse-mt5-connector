package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"dnse-mt5-connector/internal/logger"
	"dnse-mt5-connector/internal/storage"
)

func TestResolveMarketFeedSymbolPrefersFreshDerivativeCatalogForContinuousAlias(t *testing.T) {
	ctx := context.Background()
	store, err := storage.NewSQLiteStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertTickerMetadata(ctx, []storage.TickerMetadataRecord{{
		Symbol:     "VN30F1M",
		FeedSymbol: "41I1G5000",
		Exchange:   "DERIVATIVE",
		Type:       "DERIVATIVE",
		BoardID:    "G1",
		Name:       "VN30F1M",
	}}); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"total":1,"data":[{"type":"VN30F1M","symbol":"41I1G6000"}]}`))
	}))
	defer server.Close()

	appLog, err := logger.NewFileLogger(filepath.Join(t.TempDir(), "app.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer appLog.Close()

	svc := NewSymbolCatalogService(appLog, server.URL, nil, nil, store)
	got, ok, err := svc.ResolveMarketFeedSymbol(ctx, "VN30F1M")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected VN30F1M to resolve")
	}
	if got != "41I1G6000" {
		t.Fatalf("expected fresh feed symbol 41I1G6000, got %s", got)
	}
}

func TestResolveMarketFeedSymbolUsesFreshTickerWhenDerivativeCatalogIsPlaceholder(t *testing.T) {
	ctx := context.Background()
	store, err := storage.NewSQLiteStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertTickerMetadata(ctx, []storage.TickerMetadataRecord{{
		Symbol:     "VN30F1M",
		FeedSymbol: "41I1G5000",
		Exchange:   "DERIVATIVE",
		Type:       "DERIVATIVE",
		BoardID:    "G1",
		Name:       "VN30F1M",
	}}); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"total":1,"data":[{"type":"VN30F1M","symbol":"VN30F1M"}]}`))
	}))
	defer server.Close()

	appLog, err := logger.NewFileLogger(filepath.Join(t.TempDir(), "app.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer appLog.Close()

	svc := NewSymbolCatalogService(appLog, server.URL, nil, fakeTickerCatalogClient{
		items: []TickerMetadataInfo{{
			Symbol:     "VN30F1M",
			FeedSymbol: "41I1G6000",
			Exchange:   "DERIVATIVE",
			Type:       "DERIVATIVE",
			BoardID:    "G1",
			Name:       "VN30F1M",
		}},
	}, store)
	got, ok, err := svc.ResolveMarketFeedSymbol(ctx, "VN30F1M")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected VN30F1M to resolve")
	}
	if got != "41I1G6000" {
		t.Fatalf("expected fresh ticker feed symbol 41I1G6000, got %s", got)
	}
}

type fakeTickerCatalogClient struct {
	items []TickerMetadataInfo
}

func (f fakeTickerCatalogClient) GetTickers(ctx context.Context, symbol string) ([]TickerMetadataInfo, error) {
	return append([]TickerMetadataInfo(nil), f.items...), nil
}
