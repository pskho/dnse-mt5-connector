package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"dnse-mt5-connector/internal/logger"
	"dnse-mt5-connector/internal/storage"
)

const defaultDerivativeCatalogURL = "https://services-staging.entrade.com.vn/papertrade-entrade-api/derivatives"

type SymbolCatalogService struct {
	http        *http.Client
	logger      *logger.FileLogger
	url         string
	instruments InstrumentCatalogClient
	tickers     TickerCatalogClient
	store       TickerMetadataStore
	mu          sync.Mutex
	cache       map[string][]InstrumentSymbolInfo
	derivatives []DerivativeSymbolInfo
	tickerCache []storage.TickerMetadataRecord
}

type InstrumentCatalogClient interface {
	GetInstruments(ctx context.Context, exchange string) ([]InstrumentSymbolInfo, error)
}

type TickerCatalogClient interface {
	GetTickers(ctx context.Context, symbol string) ([]TickerMetadataInfo, error)
}

type TickerMetadataStore interface {
	UpsertTickerMetadata(ctx context.Context, records []storage.TickerMetadataRecord) error
	LoadTickerMetadata(ctx context.Context) ([]storage.TickerMetadataRecord, error)
	GetTickerMetadataBySymbol(ctx context.Context, symbol string) (storage.TickerMetadataRecord, error)
}

type TickerMetadataInfo struct {
	Symbol      string
	FeedSymbol  string
	Exchange    string
	Type        string
	BoardID     string
	Name        string
	Description string
	RawJSON     string
}

type DerivativeSymbolInfo struct {
	Symbol         string  `json:"symbol"`
	Type           string  `json:"type"`
	MarketPrice    float64 `json:"marketPrice"`
	CeilingPrice   float64 `json:"ceilingPrice"`
	FloorPrice     float64 `json:"floorPrice"`
	BasicPrice     float64 `json:"basicPrice"`
	ModifiedDate   string  `json:"modifiedDate"`
	ExpirationDate string  `json:"expirationDate"`
	ID             string  `json:"id"`
}

type derivativeCatalogResponse struct {
	Total int                    `json:"total"`
	Data  []DerivativeSymbolInfo `json:"data"`
}

type InstrumentSymbolInfo struct {
	Symbol   string `json:"symbol"`
	Exchange string `json:"exchange"`
	Type     string `json:"type,omitempty"`
}

type MT5SymbolLayout struct {
	Symbol      string `json:"symbol"`
	GroupPath   string `json:"groupPath"`
	Description string `json:"description"`
	Exchange    string `json:"exchange"`
	Type        string `json:"type"`
	Digits      int    `json:"digits"`
	Point       string `json:"point"`
}

func NewSymbolCatalogService(appLog *logger.FileLogger, catalogURL string, instruments InstrumentCatalogClient, tickers TickerCatalogClient, store TickerMetadataStore) *SymbolCatalogService {
	catalogURL = strings.TrimSpace(catalogURL)
	if catalogURL == "" {
		catalogURL = defaultDerivativeCatalogURL
	}
	return &SymbolCatalogService{
		http:        &http.Client{Timeout: 10 * time.Second},
		logger:      appLog,
		url:         catalogURL,
		instruments: instruments,
		tickers:     tickers,
		store:       store,
		cache:       make(map[string][]InstrumentSymbolInfo),
	}
}

func (s *SymbolCatalogService) GetDerivativeSymbols(ctx context.Context) ([]DerivativeSymbolInfo, error) {
	s.mu.Lock()
	if len(s.derivatives) > 0 {
		cached := append([]DerivativeSymbolInfo(nil), s.derivatives...)
		s.mu.Unlock()
		return cached, nil
	}
	s.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errors.New("failed to fetch derivative catalog")
	}
	var payload derivativeCatalogResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	payload.Data = mergeKnownDerivativeTargets(payload.Data)
	s.mu.Lock()
	s.derivatives = append([]DerivativeSymbolInfo(nil), payload.Data...)
	s.mu.Unlock()
	s.logger.Info("derivative_catalog_loaded", map[string]any{
		"url":   s.url,
		"total": len(payload.Data),
	})
	return payload.Data, nil
}

func (s *SymbolCatalogService) ResolveMarketFeedSymbol(ctx context.Context, displaySymbol string) (string, bool, error) {
	displaySymbol = strings.ToUpper(strings.TrimSpace(displaySymbol))
	if displaySymbol == "" {
		return "", false, nil
	}

	if record, err := s.GetTickerMetadataBySymbol(ctx, displaySymbol); err == nil {
		if feed := strings.ToUpper(strings.TrimSpace(record.FeedSymbol)); feed != "" {
			return feed, true, nil
		}
	} else if !errors.Is(err, storage.ErrNotFound) {
		return "", false, err
	}

	items, err := s.GetDerivativeSymbols(ctx)
	if err != nil {
		return "", false, err
	}
	for _, item := range items {
		alias := strings.ToUpper(strings.TrimSpace(item.Type))
		code := strings.ToUpper(strings.TrimSpace(item.Symbol))
		if alias == displaySymbol && code != "" {
			return code, true, nil
		}
	}
	return "", false, nil
}

func (s *SymbolCatalogService) ResolveTickerBoard(ctx context.Context, symbol string) (string, string, bool, error) {
	record, err := s.GetTickerMetadataBySymbol(ctx, symbol)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return "", "", false, nil
		}
		return "", "", false, err
	}
	return strings.ToUpper(strings.TrimSpace(record.FeedSymbol)), strings.ToUpper(strings.TrimSpace(record.BoardID)), true, nil
}

func mergeKnownDerivativeTargets(items []DerivativeSymbolInfo) []DerivativeSymbolInfo {
	known := []DerivativeSymbolInfo{
		{Type: "VN30F1M"},
		{Type: "VN30F2M"},
		{Type: "VN30F1Q"},
		{Type: "VN30F2Q"},
		{Type: "V100F1M"},
		{Type: "V100F2M"},
		{Type: "V100F1Q"},
		{Type: "V100F2Q"},
	}

	byType := make(map[string]DerivativeSymbolInfo, len(items)+len(known))
	for _, item := range items {
		key := strings.ToUpper(strings.TrimSpace(item.Type))
		if key == "" {
			key = strings.ToUpper(strings.TrimSpace(item.Symbol))
			item.Type = key
		}
		byType[key] = item
	}

	for _, item := range known {
		key := strings.ToUpper(strings.TrimSpace(item.Type))
		if _, ok := byType[key]; ok {
			continue
		}
		item.Symbol = key
		item.ID = key
		byType[key] = item
	}

	out := make([]DerivativeSymbolInfo, 0, len(byType))
	for _, item := range byType {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToUpper(out[i].Type) < strings.ToUpper(out[j].Type)
	})
	return out
}

func (s *SymbolCatalogService) GetInstrumentSymbols(ctx context.Context, exchanges []string) ([]InstrumentSymbolInfo, error) {
	normalized := normalizeExchanges(exchanges)
	if len(normalized) == 0 {
		normalized = []string{"HOSE", "HNX", "UPCOM", "INDEX", "DERIVATIVE"}
	}

	all, err := s.GetTickerMetadata(ctx, false)
	if err != nil {
		return nil, err
	}

	results := make([]InstrumentSymbolInfo, 0, len(all))
	exchangeSet := make(map[string]struct{}, len(normalized))
	for _, exchange := range normalized {
		exchangeSet[exchange] = struct{}{}
	}
	for _, item := range all {
		exchange := strings.ToUpper(strings.TrimSpace(item.Exchange))
		if _, ok := exchangeSet[exchange]; !ok {
			continue
		}
		results = append(results, InstrumentSymbolInfo{
			Symbol:   item.Symbol,
			Exchange: exchange,
			Type:     item.Type,
		})
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Exchange == results[j].Exchange {
			return results[i].Symbol < results[j].Symbol
		}
		return results[i].Exchange < results[j].Exchange
	})

	s.logger.Info("instrument_catalog_loaded", map[string]any{
		"exchanges": normalized,
		"total":     len(results),
	})
	return results, nil
}

func (s *SymbolCatalogService) GetTickerMetadata(ctx context.Context, forceRefresh bool) ([]storage.TickerMetadataRecord, error) {
	s.mu.Lock()
	if !forceRefresh && len(s.tickerCache) > 0 {
		cached := append([]storage.TickerMetadataRecord(nil), s.tickerCache...)
		s.mu.Unlock()
		return cached, nil
	}
	s.mu.Unlock()

	if !forceRefresh && s.store != nil {
		cached, err := s.store.LoadTickerMetadata(ctx)
		if err == nil && len(cached) > 0 {
			s.mu.Lock()
			s.tickerCache = append([]storage.TickerMetadataRecord(nil), cached...)
			s.mu.Unlock()
			return cached, nil
		}
	}

	if s.tickers == nil {
		return nil, errors.New("ticker catalog client is not configured")
	}
	items, err := s.tickers.GetTickers(ctx, "")
	if err != nil {
		return nil, err
	}
	records := make([]storage.TickerMetadataRecord, 0, len(items))
	for _, item := range items {
		symbol := strings.ToUpper(strings.TrimSpace(item.Symbol))
		if symbol == "" {
			continue
		}
		records = append(records, storage.TickerMetadataRecord{
			Symbol:      symbol,
			FeedSymbol:  strings.ToUpper(strings.TrimSpace(defaultString(item.FeedSymbol, symbol))),
			Exchange:    strings.ToUpper(strings.TrimSpace(item.Exchange)),
			Type:        strings.ToUpper(strings.TrimSpace(item.Type)),
			BoardID:     strings.ToUpper(strings.TrimSpace(item.BoardID)),
			Name:        strings.TrimSpace(item.Name),
			Description: strings.TrimSpace(item.Description),
			RawJSON:     item.RawJSON,
		})
	}
	records = mergeDefaultTickerMetadata(records)
	if s.store != nil {
		if err := s.store.UpsertTickerMetadata(ctx, records); err != nil {
			return nil, err
		}
	}
	s.mu.Lock()
	s.tickerCache = append([]storage.TickerMetadataRecord(nil), records...)
	s.mu.Unlock()
	return records, nil
}

func (s *SymbolCatalogService) GetTickerMetadataBySymbol(ctx context.Context, symbol string) (storage.TickerMetadataRecord, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return storage.TickerMetadataRecord{}, errors.New("symbol is required")
	}
	if s.store != nil {
		record, err := s.store.GetTickerMetadataBySymbol(ctx, symbol)
		if err == nil {
			return record, nil
		}
		if !errors.Is(err, storage.ErrNotFound) {
			return storage.TickerMetadataRecord{}, err
		}
	}
	items, err := s.GetTickerMetadata(ctx, false)
	if err != nil {
		return storage.TickerMetadataRecord{}, err
	}
	for _, item := range items {
		if strings.EqualFold(item.Symbol, symbol) || strings.EqualFold(item.FeedSymbol, symbol) {
			return item, nil
		}
	}
	return storage.TickerMetadataRecord{}, storage.ErrNotFound
}

func normalizeExchanges(exchanges []string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(exchanges))
	for _, exchange := range exchanges {
		exchange = strings.ToUpper(strings.TrimSpace(exchange))
		if exchange == "" {
			continue
		}
		if _, ok := seen[exchange]; ok {
			continue
		}
		seen[exchange] = struct{}{}
		out = append(out, exchange)
	}
	return out
}

func defaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return fallback
}

func mergeDefaultTickerMetadata(items []storage.TickerMetadataRecord) []storage.TickerMetadataRecord {
	seen := make(map[string]storage.TickerMetadataRecord, len(items)+16)
	for _, item := range items {
		seen[item.Symbol] = item
	}
	defaults := []storage.TickerMetadataRecord{
		{Symbol: "VNINDEX", FeedSymbol: "VNINDEX", Exchange: "INDEX", Type: "INDEX", Name: "VNINDEX"},
		{Symbol: "VN30", FeedSymbol: "VN30", Exchange: "INDEX", Type: "INDEX", Name: "VN30"},
		{Symbol: "HNX", FeedSymbol: "HNX", Exchange: "INDEX", Type: "INDEX", Name: "HNX"},
		{Symbol: "HNX30", FeedSymbol: "HNX30", Exchange: "INDEX", Type: "INDEX", Name: "HNX30"},
		{Symbol: "VN100", FeedSymbol: "VN100", Exchange: "INDEX", Type: "INDEX", Name: "VN100"},
		{Symbol: "UPCOM", FeedSymbol: "UPCOM", Exchange: "INDEX", Type: "INDEX", Name: "UPCOM"},
		{Symbol: "VNXALLSHARE", FeedSymbol: "VNXALLSHARE", Exchange: "INDEX", Type: "INDEX", Name: "VNXALLSHARE"},
	}
	for _, item := range defaults {
		if _, ok := seen[item.Symbol]; !ok {
			seen[item.Symbol] = item
		}
	}
	out := make([]storage.TickerMetadataRecord, 0, len(seen))
	for _, item := range seen {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Symbol < out[j].Symbol })
	return out
}

func (s *SymbolCatalogService) GetMT5Layouts(ctx context.Context, symbols []string) ([]MT5SymbolLayout, error) {
	items, err := s.GetTickerMetadata(ctx, false)
	if err != nil {
		return nil, err
	}

	bySymbol := make(map[string]storage.TickerMetadataRecord, len(items))
	byFeed := make(map[string]storage.TickerMetadataRecord, len(items))
	for _, item := range items {
		if symbol := strings.ToUpper(strings.TrimSpace(item.Symbol)); symbol != "" {
			bySymbol[symbol] = item
		}
		if feed := strings.ToUpper(strings.TrimSpace(item.FeedSymbol)); feed != "" {
			byFeed[feed] = item
		}
	}

	seen := make(map[string]struct{}, len(symbols))
	layouts := make([]MT5SymbolLayout, 0, len(symbols))
	for _, symbol := range symbols {
		symbol = strings.ToUpper(strings.TrimSpace(symbol))
		if symbol == "" {
			continue
		}
		if _, ok := seen[symbol]; ok {
			continue
		}
		seen[symbol] = struct{}{}

		record, ok := bySymbol[symbol]
		if !ok {
			if feedRecord, feedOK := byFeed[symbol]; feedOK {
				record = feedRecord
				record.Symbol = symbol
				ok = true
			}
		}
		if !ok {
			record = storage.TickerMetadataRecord{
				Symbol:      symbol,
				FeedSymbol:  symbol,
				Exchange:    classifyLayoutExchange(symbol),
				Type:        classifyLayoutType(symbol),
				Name:        symbol,
				Description: symbol,
			}
		}
		layouts = append(layouts, buildMT5Layout(record))
	}

	sort.Slice(layouts, func(i, j int) bool {
		if layouts[i].GroupPath == layouts[j].GroupPath {
			return layouts[i].Symbol < layouts[j].Symbol
		}
		return layouts[i].GroupPath < layouts[j].GroupPath
	})
	return layouts, nil
}

func buildMT5Layout(record storage.TickerMetadataRecord) MT5SymbolLayout {
	meta := parseTickerRawJSON(record.RawJSON)
	exchange := strings.ToUpper(strings.TrimSpace(defaultString(record.Exchange, meta["exchange"])))
	assetType := strings.ToUpper(strings.TrimSpace(defaultString(record.Type, meta["type"])))
	name := firstNonEmpty(
		record.Name,
		meta["shortName"],
		meta["companyNameVie"],
		meta["companyName"],
		meta["name"],
		record.Symbol,
	)
	sector := firstNonEmpty(meta["sectorIndex"], meta["sector"], meta["industry"], "Khác")
	groupPath := buildMT5GroupPath(record.Symbol, exchange, assetType, meta, sector)
	description := buildMT5Description(record.Symbol, name, exchange, sector, meta)

	return MT5SymbolLayout{
		Symbol:      strings.ToUpper(strings.TrimSpace(record.Symbol)),
		GroupPath:   groupPath,
		Description: description,
		Exchange:    exchange,
		Type:        assetType,
		Digits:      mt5Digits(record.Symbol, assetType),
		Point:       mt5Point(record.Symbol, assetType),
	}
}

func mt5Digits(symbol, assetType string) int {
	switch classifyLayoutTypeWithFallback(symbol, assetType) {
	case "DERIVATIVE":
		return 1
	case "INDEX":
		return 2
	default:
		return 2
	}
}

func mt5Point(symbol, assetType string) string {
	switch classifyLayoutTypeWithFallback(symbol, assetType) {
	case "DERIVATIVE":
		return "0.1"
	case "INDEX":
		return "0.01"
	default:
		return "0.01"
	}
}

func buildMT5GroupPath(symbol, exchange, assetType string, meta map[string]string, sector string) string {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	switch classifyLayoutTypeWithFallback(symbol, assetType) {
	case "INDEX":
		return "DNSE\\Chỉ số\\" + sanitizeMT5Group(firstNonEmpty(meta["exchange"], exchange, "Tổng hợp"))
	case "DERIVATIVE":
		family := "Khác"
		switch {
		case strings.HasPrefix(symbol, "VN30F"):
			family = "VN30"
		case strings.HasPrefix(symbol, "V100F"):
			family = "VN100"
		}
		return "DNSE\\Phái sinh\\" + sanitizeMT5Group(family)
	default:
		exchangeGroup := sanitizeMT5Group(firstNonEmpty(exchange, "Khác"))
		sectorGroup := sanitizeMT5Group(sector)
		return "DNSE\\Cổ phiếu\\" + exchangeGroup + "\\" + sectorGroup
	}
}

func buildMT5Description(symbol, name, exchange, sector string, meta map[string]string) string {
	parts := []string{symbol}
	if strings.TrimSpace(name) != "" && !strings.EqualFold(strings.TrimSpace(name), symbol) {
		parts = append(parts, name)
	}
	if exchange != "" {
		parts = append(parts, exchange)
	}
	if sector != "" && !strings.EqualFold(sector, "Khác") {
		parts = append(parts, sector)
	}
	if short := strings.TrimSpace(meta["shortName"]); short != "" && !strings.EqualFold(short, name) {
		parts = append(parts, short)
	}
	return strings.Join(parts, " | ")
}

func parseTickerRawJSON(raw string) map[string]string {
	out := map[string]string{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return out
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return out
	}
	for _, key := range []string{
		"exchange", "floor", "market", "type", "groupType", "sector", "sectorIndex",
		"industry", "subIndustry", "shortName", "companyName", "companyNameVie",
		"name", "symbolType",
	} {
		if value, ok := payload[key]; ok {
			if text := strings.TrimSpace(toString(value)); text != "" {
				out[key] = text
			}
		}
	}
	if out["exchange"] == "" {
		out["exchange"] = out["floor"]
	}
	if out["type"] == "" {
		out["type"] = out["groupType"]
	}
	return out
}

func toString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%.0f", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func sanitizeMT5Group(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "Khác"
	}
	replacer := strings.NewReplacer("\\", "-", "/", "-", ":", "-", "*", "-", "?", "", "\"", "", "<", "", ">", "", "|", "-", "\t", " ")
	value = replacer.Replace(value)
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		return "Khác"
	}
	return value
}

func classifyLayoutType(symbol string) string {
	return classifyLayoutTypeWithFallback(symbol, "")
}

func classifyLayoutTypeWithFallback(symbol, rawType string) string {
	rawType = strings.ToUpper(strings.TrimSpace(rawType))
	if rawType == "INDEX" || rawType == "DERIVATIVE" || rawType == "STOCK" {
		return rawType
	}
	switch {
	case strings.HasPrefix(symbol, "VN30F"), strings.HasPrefix(symbol, "V100F"), looksLikeDerivativeFeedCode(symbol):
		return "DERIVATIVE"
	case strings.HasPrefix(symbol, "VN"), symbol == "HNX", symbol == "HNX30", symbol == "UPCOM", symbol == "VNXALLSHARE":
		return "INDEX"
	default:
		return "STOCK"
	}
}

func classifyLayoutExchange(symbol string) string {
	switch classifyLayoutType(symbol) {
	case "INDEX":
		return "INDEX"
	case "DERIVATIVE":
		return "DERIVATIVE"
	default:
		return "STOCK"
	}
}

func looksLikeDerivativeFeedCode(symbol string) bool {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if len(symbol) < 6 {
		return false
	}
	if !(symbol[0] >= '0' && symbol[0] <= '9' && symbol[1] >= '0' && symbol[1] <= '9') {
		return false
	}
	if symbol[2] != 'I' || !(symbol[3] >= '0' && symbol[3] <= '9') {
		return false
	}
	return strings.Contains(symbol, "G") || strings.Contains(symbol, "C")
}
