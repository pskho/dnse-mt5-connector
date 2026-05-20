package marketdata

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"dnse-mt5-connector/internal/config"
	"dnse-mt5-connector/internal/logger"
	"dnse-mt5-connector/internal/storage"
)

type HistoryCandle struct {
	IsHistory  int     `json:"is_history"`
	Time       int64   `json:"time"`
	Open       float64 `json:"open"`
	High       float64 `json:"high"`
	Low        float64 `json:"low"`
	Close      float64 `json:"close"`
	TickVolume int64   `json:"tick_volume"`
}

func (c HistoryCandle) JSONLine() []byte {
	raw, _ := json.Marshal(c)
	return append(raw, '\n')
}

type HistorySyncLogger interface {
	LogHistorySync(ctx context.Context, firstTime, lastTime int64, status string, candlesSynced int) error
}

type HistoryCache interface {
	UpsertHistoryCandles(ctx context.Context, records []storage.HistoryCandleRecord) error
	LoadHistoryCandles(ctx context.Context, symbol, marketType string, resolution int, fromMS, toMS int64) ([]storage.HistoryCandleRecord, error)
	GetHistoryCoverage(ctx context.Context, symbol, marketType string, resolution int) (minMS, maxMS int64, count int, err error)
}

type TickerMetadataLookup interface {
	GetTickerMetadataBySymbol(ctx context.Context, symbol string) (storage.TickerMetadataRecord, error)
}

type SyncRequest struct {
	FirstTime int64 `json:"firstTime"`
	LastTime  int64 `json:"lastTime"`
}

type SyncResult struct {
	Success       bool   `json:"success"`
	Message       string `json:"message"`
	CandlesSynced int    `json:"candlesSynced"`
}

type SyncOptions struct {
	FirstTime    int64
	LastTime     int64
	ForceFull    bool
	BeforeToday  bool
	TodayOnly    bool
	LookbackDays int
	Symbol       string
	MarketType   string
	Resolution   int
}

type HistoryKey struct {
	Symbol     string
	MarketType string
	Resolution int
}

type HistoryService struct {
	cfg       config.HistoryConfig
	client    HistoryClient
	logger    *logger.FileLogger
	store     HistorySyncLogger
	cache     HistoryCache
	tickers   TickerMetadataLookup
	mu        sync.RWMutex
	snapshots map[string]historySnapshot
	subs      map[chan HistoryKey]struct{}
	reqGate   *historyRequestGate
}

type historyRequestGate struct {
	mu            sync.Mutex
	nextAllowedAt time.Time
}

type historySnapshot struct {
	key     HistoryKey
	candles []HistoryCandle
}

type HistoryClient interface {
	FetchOHLC(ctx context.Context, symbol, marketType string, resolution int, from, to int64) (map[string]any, error)
}

func NewHistoryService(cfg config.HistoryConfig, client HistoryClient, store HistorySyncLogger, logger *logger.FileLogger) *HistoryService {
	var cache HistoryCache
	if c, ok := store.(HistoryCache); ok {
		cache = c
	}
	var tickers TickerMetadataLookup
	if t, ok := store.(TickerMetadataLookup); ok {
		tickers = t
	}
	return &HistoryService{
		cfg:       cfg,
		client:    client,
		store:     store,
		logger:    logger,
		cache:     cache,
		tickers:   tickers,
		snapshots: make(map[string]historySnapshot),
		subs:      make(map[chan HistoryKey]struct{}),
		reqGate:   &historyRequestGate{},
	}
}

func (s *HistoryService) Fetch(ctx context.Context) error {
	// Startup fetch is now handled by the MQL5 EA triggering Sync
	return nil
}

func (s *HistoryService) NeedsBootstrap(ctx context.Context, symbol, marketType string, resolution int) bool {
	if s.cache == nil {
		return true
	}
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		symbol = strings.ToUpper(strings.TrimSpace(s.cfg.Symbol))
	}
	marketType = strings.ToUpper(strings.TrimSpace(marketType))
	if marketType == "" {
		marketType = strings.ToUpper(strings.TrimSpace(s.cfg.MarketType))
	}
	if resolution <= 0 {
		resolution = s.cfg.Resolution
	}
	_, maxMS, count, err := s.cache.GetHistoryCoverage(ctx, symbol, marketType, resolution)
	if err != nil {
		s.logger.Error("history_bootstrap_coverage_failed", map[string]any{
			"symbol": symbol, "marketType": marketType, "resolution": resolution, "error": err.Error(),
		})
		return true
	}
	if count == 0 {
		return true
	}
	requiredFromMS := latestRequiredBootstrapDayStartMS()
	if requiredFromMS > 0 && maxMS < requiredFromMS {
		s.logger.Info("history_bootstrap_stale_cache", map[string]any{
			"symbol": symbol, "marketType": marketType, "resolution": resolution,
			"maxMS": maxMS, "requiredFromMS": requiredFromMS,
		})
		return true
	}
	return false
}

func (s *HistoryService) Sync(ctx context.Context, firstTime, lastTime int64) (any, error) {
	return s.SyncWithOptions(ctx, SyncOptions{
		FirstTime: firstTime,
		LastTime:  lastTime,
	})
}

func (s *HistoryService) FullSync(ctx context.Context, lookbackDays int) (any, error) {
	return s.SyncWithOptions(ctx, SyncOptions{
		ForceFull:    true,
		LookbackDays: lookbackDays,
	})
}

func (s *HistoryService) BackfillBeforeToday(ctx context.Context, lookbackDays int) (any, error) {
	return s.SyncWithOptions(ctx, SyncOptions{
		ForceFull:    true,
		BeforeToday:  true,
		LookbackDays: lookbackDays,
	})
}

func (s *HistoryService) SyncToday(ctx context.Context) (any, error) {
	return s.SyncWithOptions(ctx, SyncOptions{
		TodayOnly: true,
	})
}

func (s *HistoryService) SyncWithOptions(ctx context.Context, opt SyncOptions) (any, error) {
	if !s.cfg.Enabled {
		return SyncResult{Success: false, Message: "History sync disabled"}, nil
	}

	symbol := strings.ToUpper(strings.TrimSpace(opt.Symbol))
	if symbol == "" {
		symbol = strings.ToUpper(strings.TrimSpace(s.cfg.Symbol))
	}
	marketType := strings.ToUpper(strings.TrimSpace(opt.MarketType))
	if marketType == "" {
		marketType = strings.ToUpper(strings.TrimSpace(s.cfg.MarketType))
	}
	resolution := opt.Resolution
	if resolution <= 0 {
		resolution = s.cfg.Resolution
	}

	var from, to int64
	now := time.Now().Unix()
	endOfYesterday := endOfPreviousDayUnix()
	startOfToday := startOfTodayUnix()

	firstTime := opt.FirstTime
	lastTime := opt.LastTime
	lookbackDays := opt.LookbackDays
	if lookbackDays <= 0 {
		lookbackDays = s.cfg.InitialLookbackDays
	}

	if opt.ForceFull {
		from = time.Now().Add(-time.Duration(lookbackDays) * 24 * time.Hour).Unix()
		to = now
		statusLabel := "history_sync_full_rebuild"
		if opt.BeforeToday {
			to = endOfYesterday
			statusLabel = "history_sync_backfill_before_today"
		}
		s.logger.Info(statusLabel, map[string]any{"days": lookbackDays, "symbol": symbol, "marketType": marketType, "resolution": resolution, "to": to})
	} else if opt.TodayOnly {
		from = startOfToday
		to = now
		s.logger.Info("history_sync_today", map[string]any{"symbol": symbol, "marketType": marketType, "resolution": resolution, "from": from, "to": to})
	} else if firstTime == 0 && lastTime == 0 {
		// No data, full rebuild up to initial lookback
		from = time.Now().Add(-time.Duration(lookbackDays) * 24 * time.Hour).Unix()
		to = now
		s.logger.Info("history_sync_initial", map[string]any{"days": lookbackDays, "symbol": symbol, "marketType": marketType, "resolution": resolution})
	} else {
		if !s.cfg.IncrementalSync {
			return SyncResult{Success: false, Message: "Incremental sync disabled"}, nil
		}
		from = lastTime / 1000 // Convert ms to s
		to = now

		gapDays := (to - from) / (24 * 3600)
		if gapDays > int64(s.cfg.InitialLookbackDays) && !s.cfg.FullRebuild {
			s.logger.Error("history_sync_gap_too_large", map[string]any{"gapDays": gapDays})
			s.store.LogHistorySync(ctx, firstTime, lastTime, "skipped_gap_too_large", 0)
			return SyncResult{Success: false, Message: "Gap too large, skipped"}, nil
		}
		s.logger.Info("history_sync_incremental", map[string]any{"from": from, "to": to, "symbol": symbol, "marketType": marketType, "resolution": resolution})
	}

	if opt.BeforeToday && to > endOfYesterday {
		to = endOfYesterday
	}
	if to <= from {
		return SyncResult{Success: true, Message: "No historical range before today to sync", CandlesSynced: 0}, nil
	}

	fromMS := from * 1000
	toMS := to * 1000
	sessionPolicy := s.sessionPolicy(ctx, symbol, marketType)
	if s.cache != nil && !opt.ForceFull {
		requiredFromMS, requiredToMS, hasRequiredCandles := requiredTradingCoverageRange(fromMS, toMS, resolution, sessionPolicy)
		minMS, maxMS, count, err := s.cache.GetHistoryCoverage(ctx, symbol, marketType, resolution)
		if err == nil && count > 0 && (!hasRequiredCandles || (minMS <= requiredFromMS && maxMS >= requiredToMS)) {
			cached, loadErr := s.cache.LoadHistoryCandles(ctx, symbol, marketType, resolution, fromMS, toMS)
			if loadErr == nil && cacheRecordsCompleteForRange(cached, resolution, fromMS, toMS, sessionPolicy) {
				stagedCandles := recordsToCandles(cached)
				stagedCandles = fillTradingSessionGapsForRangeIfNeeded(symbol, marketType, resolution, fromMS, toMS, stagedCandles, sessionPolicy)
				key := HistoryKey{
					Symbol:     symbol,
					MarketType: marketType,
					Resolution: resolution,
				}
				s.replaceStagedCandles(key, stagedCandles)
				s.cloneTickerAliasSnapshots(ctx, key)
				status := "completed_cached"
				message := "History loaded from local cache"
				if opt.BeforeToday {
					status = "completed_cached_backfill_before_today"
					message = "Historical backfill through yesterday loaded from local cache"
				} else if opt.TodayOnly {
					status = "completed_cached_today"
					message = "Today history loaded from local cache"
				}
				s.store.LogHistorySync(ctx, firstTime, lastTime, status, len(stagedCandles))
				s.logger.Info("history_cache_hit", map[string]any{
					"symbol": symbol, "marketType": marketType, "resolution": resolution,
					"fromMS": fromMS, "toMS": toMS, "candles": len(stagedCandles),
				})
				return SyncResult{Success: true, Message: message, CandlesSynced: len(stagedCandles)}, nil
			} else if loadErr != nil {
				s.logger.Error("history_cache_load_failed", map[string]any{
					"symbol": symbol, "marketType": marketType, "resolution": resolution, "error": loadErr.Error(),
				})
			} else {
				s.logger.Info("history_cache_gap_detected", map[string]any{
					"symbol": symbol, "marketType": marketType, "resolution": resolution,
					"fromMS": fromMS, "toMS": toMS, "cached": len(cached),
				})
			}
		}
	}

	totalCandles := 0
	batchDays := s.cfg.MaxBatchDays
	if opt.ForceFull && lookbackDays > batchDays {
		batchDays = lookbackDays
	}
	if batchDays <= 0 {
		batchDays = 30
	}
	batchSeconds := int64(batchDays * 24 * 3600)
	currentFrom := from

	// Replace the staged candle snapshot atomically for the next bridge client.
	stagedCandles := make([]HistoryCandle, 0)
	seen := make(map[int64]bool)

	for currentFrom < to {
		currentTo := currentFrom + batchSeconds
		if currentTo > to {
			currentTo = to
		}

		batchAdded := 0
		pageFrom := currentFrom
		for page := 0; page < 100 && pageFrom <= currentTo; page++ {
			raw, err := s.fetchOHLCWithRetry(ctx, symbol, marketType, resolution, pageFrom, currentTo)
			if err != nil {
				s.store.LogHistorySync(ctx, firstTime, lastTime, "failed", totalCandles)
				return SyncResult{Success: false, Message: err.Error()}, err
			}

			tList, _ := raw["t"].([]any)
			oList, _ := raw["o"].([]any)
			hList, _ := raw["h"].([]any)
			lList, _ := raw["l"].([]any)
			cList, _ := raw["c"].([]any)
			vList, _ := raw["v"].([]any)

			count := len(tList)
			for i := 0; i < count; i++ {
				t := parseInt64(tList, i)
				if t <= 0 {
					continue
				}
				if t < 100000000000 {
					t *= 1000
				}
				if seen[t] {
					continue
				}
				seen[t] = true

				stagedCandles = append(stagedCandles, HistoryCandle{
					IsHistory:  1,
					Time:       t,
					Open:       parseFloat64(oList, i),
					High:       parseFloat64(hList, i),
					Low:        parseFloat64(lList, i),
					Close:      parseFloat64(cList, i),
					TickVolume: parseInt64(vList, i),
				})
				batchAdded++
				totalCandles++
			}

			nextTime := parseFlexibleInt64(raw["nextTime"])
			if nextTime <= 0 || nextTime > currentTo || nextTime <= pageFrom {
				break
			}
			pageFrom = nextTime
		}

		s.logger.Info("history_sync_batch_completed", map[string]any{"currentFrom": currentFrom, "currentTo": currentTo, "added": batchAdded})
		currentFrom = currentTo + 1
	}

	sort.Slice(stagedCandles, func(i, j int) bool {
		return stagedCandles[i].Time < stagedCandles[j].Time
	})
	stagedCandles = fillTradingSessionGapsForRangeIfNeeded(symbol, marketType, resolution, fromMS, toMS, stagedCandles, sessionPolicy)

	if s.cache != nil && len(stagedCandles) > 0 {
		if err := s.cache.UpsertHistoryCandles(ctx, candlesToRecords(symbol, marketType, resolution, stagedCandles)); err != nil {
			s.logger.Error("history_cache_save_failed", map[string]any{
				"symbol": symbol, "marketType": marketType, "resolution": resolution, "error": err.Error(),
			})
		} else {
			s.logger.Info("history_cache_saved", map[string]any{
				"symbol": symbol, "marketType": marketType, "resolution": resolution, "candles": len(stagedCandles),
			})
		}
	}

	key := HistoryKey{
		Symbol:     symbol,
		MarketType: marketType,
		Resolution: resolution,
	}
	s.replaceStagedCandles(key, stagedCandles)
	s.cloneTickerAliasSnapshots(ctx, key)

	status := "completed_incremental"
	message := "Sync completed"
	if opt.ForceFull || (firstTime == 0 && lastTime == 0) {
		status = "completed_full"
		message = "Full history sync completed"
		if opt.BeforeToday {
			status = "completed_backfill_before_today"
			message = "Historical backfill through yesterday completed"
		}
	} else if opt.TodayOnly {
		status = "completed_today"
		message = "Today sync completed"
	}

	s.store.LogHistorySync(ctx, firstTime, lastTime, status, totalCandles)
	s.logger.Info("history_sync_completed", map[string]any{"totalAdded": totalCandles, "status": status})

	return SyncResult{Success: true, Message: message, CandlesSynced: totalCandles}, nil
}

func (s *HistoryService) replaceStagedCandles(key HistoryKey, stagedCandles []HistoryCandle) {
	s.mu.Lock()
	s.snapshots[historyKeyString(key)] = historySnapshot{
		key:     key,
		candles: stagedCandles,
	}
	for ch := range s.subs {
		select {
		case ch <- key:
		default:
		}
	}
	s.mu.Unlock()
}

func (s *HistoryService) cloneTickerAliasSnapshots(ctx context.Context, key HistoryKey) {
	if s == nil || s.tickers == nil {
		return
	}
	record, err := s.tickers.GetTickerMetadataBySymbol(ctx, key.Symbol)
	if err != nil {
		return
	}
	for _, alias := range []string{record.Symbol, record.FeedSymbol} {
		alias = strings.ToUpper(strings.TrimSpace(alias))
		if alias == "" || strings.EqualFold(alias, key.Symbol) {
			continue
		}
		s.CloneSnapshot(key.Symbol, alias, key.MarketType, key.Resolution)
	}
}

func (s *HistoryService) Candles() []HistoryCandle {
	_, out := s.Snapshot()
	return out
}

func (s *HistoryService) Snapshot() (HistoryKey, []HistoryCandle) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	primarySymbol := strings.ToUpper(strings.TrimSpace(s.cfg.Symbol))
	if primarySymbol != "" {
		for _, snapshot := range s.snapshots {
			if strings.EqualFold(snapshot.key.Symbol, primarySymbol) {
				out := make([]HistoryCandle, len(snapshot.candles))
				copy(out, snapshot.candles)
				return snapshot.key, out
			}
		}
	}
	for _, snapshot := range s.snapshots {
		out := make([]HistoryCandle, len(snapshot.candles))
		copy(out, snapshot.candles)
		return snapshot.key, out
	}
	return HistoryKey{}, nil
}

func (s *HistoryService) SnapshotForSymbol(symbol string) (HistoryKey, []HistoryCandle, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	for _, snapshot := range s.snapshots {
		if strings.EqualFold(snapshot.key.Symbol, symbol) {
			out := make([]HistoryCandle, len(snapshot.candles))
			copy(out, snapshot.candles)
			return snapshot.key, out, true
		}
	}
	return HistoryKey{}, nil, false
}

func (s *HistoryService) CloneSnapshot(sourceSymbol, targetSymbol, marketType string, resolution int) bool {
	sourceSymbol = strings.ToUpper(strings.TrimSpace(sourceSymbol))
	targetSymbol = strings.ToUpper(strings.TrimSpace(targetSymbol))
	marketType = strings.ToUpper(strings.TrimSpace(marketType))
	if sourceSymbol == "" || targetSymbol == "" {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, snapshot := range s.snapshots {
		if !strings.EqualFold(snapshot.key.Symbol, sourceSymbol) {
			continue
		}
		key := HistoryKey{Symbol: targetSymbol, MarketType: marketType, Resolution: resolution}
		copied := make([]HistoryCandle, len(snapshot.candles))
		copy(copied, snapshot.candles)
		s.snapshots[historyKeyString(key)] = historySnapshot{
			key:     key,
			candles: copied,
		}
		for ch := range s.subs {
			select {
			case ch <- key:
			default:
			}
		}
		return true
	}
	return false
}

func (s *HistoryService) ClearCandles() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots = make(map[string]historySnapshot)
}

func (s *HistoryService) SubscribeChanges() (<-chan HistoryKey, func()) {
	ch := make(chan HistoryKey, 8)
	s.mu.Lock()
	s.subs[ch] = struct{}{}
	s.mu.Unlock()
	return ch, func() {
		s.mu.Lock()
		if _, ok := s.subs[ch]; ok {
			delete(s.subs, ch)
			close(ch)
		}
		s.mu.Unlock()
	}
}

func historyKeyString(key HistoryKey) string {
	return strings.ToUpper(strings.TrimSpace(key.Symbol)) + "|" +
		strings.ToUpper(strings.TrimSpace(key.MarketType)) + "|" +
		strconv.Itoa(key.Resolution)
}

func (s *HistoryService) fetchOHLCWithRetry(ctx context.Context, symbol, marketType string, resolution int, from, to int64) (map[string]any, error) {
	backoffs := []time.Duration{0, 5 * time.Second, 10 * time.Second, 20 * time.Second}
	var lastErr error
	for attempt, wait := range backoffs {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}
		}
		s.waitRequestTurn(ctx)
		raw, err := s.client.FetchOHLC(ctx, symbol, marketType, resolution, from, to)
		if err == nil {
			return raw, nil
		}
		lastErr = err
		if !isRateLimitError(err) {
			return nil, err
		}
		s.logger.Error("history_fetch_rate_limited", map[string]any{
			"symbol":     symbol,
			"marketType": marketType,
			"resolution": resolution,
			"from":       from,
			"to":         to,
			"attempt":    attempt + 1,
			"error":      err.Error(),
		})
	}
	return nil, lastErr
}

func (s *HistoryService) waitRequestTurn(ctx context.Context) {
	if s.reqGate == nil {
		return
	}
	for {
		s.reqGate.mu.Lock()
		now := time.Now()
		if now.After(s.reqGate.nextAllowedAt) || now.Equal(s.reqGate.nextAllowedAt) {
			s.reqGate.nextAllowedAt = now.Add(1200 * time.Millisecond)
			s.reqGate.mu.Unlock()
			return
		}
		wait := s.reqGate.nextAllowedAt.Sub(now)
		s.reqGate.mu.Unlock()
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "429") || strings.Contains(text, "rate limit")
}

func parseInt64(arr []any, i int) int64 {
	if i >= len(arr) {
		return 0
	}
	switch v := arr[i].(type) {
	case float64:
		return int64(v)
	case int:
		return int64(v)
	}
	return 0
}

func parseFlexibleInt64(value any) int64 {
	switch v := value.(type) {
	case float64:
		return int64(v)
	case int:
		return int64(v)
	case int64:
		return v
	case json.Number:
		n, _ := v.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		return n
	default:
		return 0
	}
}

func parseFloat64(arr []any, i int) float64 {
	if i >= len(arr) {
		return 0
	}
	switch v := arr[i].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	}
	return 0
}

var vnLocation = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		return time.FixedZone("ICT", 7*3600)
	}
	return loc
}()

type tradingSessionPolicy struct {
	MorningOpen       int
	OpeningAuctionEnd int
	MorningClose      int
	AfternoonOpen     int
	ClosingAuction    int
	AfternoonClose    int
}

func (s *HistoryService) sessionPolicy(ctx context.Context, symbol, marketType string) tradingSessionPolicy {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	marketType = strings.ToUpper(strings.TrimSpace(marketType))
	if marketType == "DERIVATIVE" || isDerivativeSymbol(symbol) {
		return derivativeKRXPolicy()
	}
	if isHOSEIndexSymbol(symbol) {
		return hoseKRXPolicy()
	}
	if isHNXOrUPCOMSymbol(symbol) {
		return hnxUPCOMKRXPolicy()
	}
	if marketType != "STOCK" || s == nil || s.tickers == nil || symbol == "" {
		return defaultKRXPolicy()
	}
	record, err := s.tickers.GetTickerMetadataBySymbol(ctx, symbol)
	if err != nil {
		return defaultKRXPolicy()
	}
	exchange := strings.ToUpper(strings.TrimSpace(record.Exchange))
	if exchange == "HOSE" || exchange == "HSX" {
		return hoseKRXPolicy()
	}
	if exchange == "HNX" || exchange == "UPCOM" {
		return hnxUPCOMKRXPolicy()
	}
	return defaultKRXPolicy()
}

func isHNXOrUPCOMSymbol(symbol string) bool {
	switch strings.ToUpper(strings.TrimSpace(symbol)) {
	case "HNX", "HNX30", "UPCOM":
		return true
	default:
		return false
	}
}

func realtimePolicyForSymbol(symbol string) tradingSessionPolicy {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if isDerivativeSymbol(symbol) {
		return derivativeKRXPolicy()
	}
	if isHOSEIndexSymbol(symbol) {
		return hoseKRXPolicy()
	}
	if isHNXOrUPCOMSymbol(symbol) {
		return hnxUPCOMKRXPolicy()
	}
	return tradingSessionPolicy{
		MorningOpen:    8*60 + 45,
		MorningClose:   11*60 + 30,
		AfternoonOpen:  13 * 60,
		AfternoonClose: 15 * 60,
	}
}

func isHOSEIndexSymbol(symbol string) bool {
	switch strings.ToUpper(strings.TrimSpace(symbol)) {
	case "VNINDEX", "VN30", "VN100", "VNXALLSHARE", "VNDIVIDEND", "VN50GROWTH", "VNMITECH":
		return true
	default:
		return false
	}
}

func derivativeKRXPolicy() tradingSessionPolicy {
	return tradingSessionPolicy{
		MorningOpen:       8*60 + 45,
		OpeningAuctionEnd: 9 * 60,
		MorningClose:      11*60 + 30,
		AfternoonOpen:     13 * 60,
		ClosingAuction:    14*60 + 30,
		AfternoonClose:    14*60 + 45,
	}
}

func hoseKRXPolicy() tradingSessionPolicy {
	return tradingSessionPolicy{
		MorningOpen:       9 * 60,
		OpeningAuctionEnd: 9*60 + 15,
		MorningClose:      11*60 + 30,
		AfternoonOpen:     13 * 60,
		ClosingAuction:    14*60 + 30,
		AfternoonClose:    14*60 + 45,
	}
}

func hnxUPCOMKRXPolicy() tradingSessionPolicy {
	return tradingSessionPolicy{
		MorningOpen:    9 * 60,
		MorningClose:   11*60 + 30,
		AfternoonOpen:  13 * 60,
		AfternoonClose: 15 * 60,
	}
}

func defaultKRXPolicy() tradingSessionPolicy {
	return hoseKRXPolicy()
}

func normalizeTradingSessionPolicy(policy tradingSessionPolicy) tradingSessionPolicy {
	if policy.MorningOpen <= 0 {
		policy.MorningOpen = 9 * 60
	}
	if policy.MorningClose <= 0 {
		policy.MorningClose = 11*60 + 30
	}
	if policy.AfternoonOpen <= 0 {
		policy.AfternoonOpen = 13 * 60
	}
	if policy.AfternoonClose <= 0 {
		policy.AfternoonClose = 14*60 + 45
	}
	if policy.OpeningAuctionEnd <= policy.MorningOpen {
		policy.OpeningAuctionEnd = 0
	}
	if policy.ClosingAuction <= 0 || policy.ClosingAuction > policy.AfternoonClose {
		policy.ClosingAuction = 0
	}
	return policy
}

func fillTradingSessionGaps(candles []HistoryCandle, resolution int) []HistoryCandle {
	return fillTradingSessionGapsWithPolicy(candles, resolution, defaultKRXPolicy())
}

func fillTradingSessionGapsWithPolicy(candles []HistoryCandle, resolution int, policy tradingSessionPolicy) []HistoryCandle {
	if len(candles) < 2 {
		return candles
	}
	if resolution <= 0 {
		resolution = 1
	}
	stepMS := int64(resolution) * 60_000
	if stepMS <= 0 {
		stepMS = 60_000
	}

	out := make([]HistoryCandle, 0, len(candles)+1024)
	out = append(out, candles[0])

	for i := 1; i < len(candles); i++ {
		prev := out[len(out)-1]
		curr := candles[i]

		for ts := prev.Time + stepMS; ts < curr.Time; ts += stepMS {
			if !sameTradingSessionWithPolicy(prev.Time, ts, policy) || !sameTradingSessionWithPolicy(ts, curr.Time, policy) {
				break
			}
			price := prev.Close
			if price <= 0 {
				price = prev.Open
			}
			out = append(out, HistoryCandle{
				IsHistory:  1,
				Time:       ts,
				Open:       price,
				High:       price,
				Low:        price,
				Close:      price,
				TickVolume: 0,
			})
			prev = out[len(out)-1]
		}

		out = append(out, curr)
	}

	return out
}

func fillDerivativeSessionGaps(candles []HistoryCandle) []HistoryCandle {
	return fillTradingSessionGapsWithPolicy(candles, 1, derivativeKRXPolicy())
}

func fillTradingSessionGapsIfNeeded(symbol, marketType string, resolution int, candles []HistoryCandle) []HistoryCandle {
	_ = marketType
	if resolution == 1 && strings.TrimSpace(symbol) != "" {
		return fillTradingSessionGaps(candles, resolution)
	}
	return candles
}

func fillTradingSessionGapsForRangeIfNeeded(symbol, marketType string, resolution int, fromMS, toMS int64, candles []HistoryCandle, policy tradingSessionPolicy) []HistoryCandle {
	_ = marketType
	if resolution == 1 && strings.TrimSpace(symbol) != "" {
		return fillTradingSessionGapsForRange(candles, resolution, fromMS, toMS, policy)
	}
	return candles
}

func fillDerivativeSessionGapsIfNeeded(symbol, marketType string, candles []HistoryCandle) []HistoryCandle {
	return fillTradingSessionGapsIfNeeded(symbol, marketType, 1, candles)
}

func fillTradingSessionGapsForRange(candles []HistoryCandle, resolution int, fromMS, toMS int64, policy tradingSessionPolicy) []HistoryCandle {
	if len(candles) == 0 {
		return candles
	}
	candles = normalizeKRXCandles(candles, policy)
	if len(candles) == 0 {
		return candles
	}
	if resolution <= 0 {
		resolution = 1
	}
	stepMS := int64(resolution) * 60_000
	if stepMS <= 0 {
		stepMS = 60_000
	}

	actual := make(map[int64]HistoryCandle, len(candles))
	days := make(map[string]struct{})
	for _, candle := range candles {
		actual[candle.Time] = candle
		t := time.UnixMilli(candle.Time).In(vnLocation)
		days[t.Format("2006-01-02")] = struct{}{}
	}

	out := make([]HistoryCandle, 0, len(candles)+1024)
	seen := make(map[int64]struct{}, len(candles)+1024)
	for _, candle := range candles {
		if !isVNTradingTimestampMSWithPolicy(candle.Time, policy) {
			out = append(out, candle)
			seen[candle.Time] = struct{}{}
		}
	}

	rangeStart := alignUpToStepMS(fromMS, stepMS)
	rangeEnd := lastCompletedCandleStartMS(toMS, stepMS)
	if rangeEnd < rangeStart {
		return candles
	}

	var lastCandle HistoryCandle
	hasLast := false
	for ts := rangeStart; ts <= rangeEnd; ts += stepMS {
		if !isVNTradingTimestampMSWithPolicy(ts, policy) {
			continue
		}
		dayKey := time.UnixMilli(ts).In(vnLocation).Format("2006-01-02")
		if _, ok := days[dayKey]; !ok {
			continue
		}
		if candle, ok := actual[ts]; ok {
			if _, exists := seen[ts]; !exists {
				out = append(out, candle)
				seen[ts] = struct{}{}
			}
			lastCandle = candle
			hasLast = true
			continue
		}
		if !hasLast || !sameTradingDay(lastCandle.Time, ts) {
			first, ok := firstActualOnDayAtOrAfter(actual, dayKey, ts)
			if !ok {
				continue
			}
			lastCandle = first
			hasLast = true
		}
		price := lastCandle.Close
		if price <= 0 {
			price = lastCandle.Open
		}
		if price <= 0 {
			continue
		}
		if _, exists := seen[ts]; exists {
			continue
		}
		out = append(out, HistoryCandle{
			IsHistory:  1,
			Time:       ts,
			Open:       price,
			High:       price,
			Low:        price,
			Close:      price,
			TickVolume: 0,
		})
		seen[ts] = struct{}{}
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Time < out[j].Time
	})
	return out
}

func normalizeKRXCandles(candles []HistoryCandle, policy tradingSessionPolicy) []HistoryCandle {
	if len(candles) == 0 {
		return candles
	}
	sorted := make([]HistoryCandle, len(candles))
	copy(sorted, candles)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Time < sorted[j].Time
	})

	byTime := make(map[int64]HistoryCandle, len(sorted))
	order := make([]int64, 0, len(sorted))
	for _, candle := range sorted {
		bucketMS, ok := krxCandleStartMS(candle.Time, policy)
		if !ok {
			continue
		}
		existing, exists := byTime[bucketMS]
		if !exists {
			candle.Time = bucketMS
			candle.IsHistory = 1
			byTime[bucketMS] = candle
			order = append(order, bucketMS)
			continue
		}
		if candle.High > existing.High || existing.High <= 0 {
			existing.High = candle.High
		}
		if candle.Low < existing.Low || existing.Low <= 0 {
			existing.Low = candle.Low
		}
		if candle.Close > 0 {
			existing.Close = candle.Close
		}
		existing.TickVolume += candle.TickVolume
		byTime[bucketMS] = existing
	}

	out := make([]HistoryCandle, 0, len(order))
	for _, ts := range order {
		out = append(out, byTime[ts])
	}
	return out
}

func sameTradingDay(aMS, bMS int64) bool {
	a := time.UnixMilli(aMS).In(vnLocation)
	b := time.UnixMilli(bMS).In(vnLocation)
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}

func firstActualOnDayAtOrAfter(actual map[int64]HistoryCandle, dayKey string, fromMS int64) (HistoryCandle, bool) {
	var out HistoryCandle
	found := false
	for ts, candle := range actual {
		if ts < fromMS {
			continue
		}
		if time.UnixMilli(ts).In(vnLocation).Format("2006-01-02") != dayKey {
			continue
		}
		if !found || ts < out.Time {
			out = candle
			found = true
		}
	}
	return out, found
}

func cacheRecordsCompleteForRange(records []storage.HistoryCandleRecord, resolution int, fromMS, toMS int64, policy tradingSessionPolicy) bool {
	if resolution <= 0 {
		resolution = 1
	}
	stepMS := int64(resolution) * 60_000
	if stepMS <= 0 {
		stepMS = 60_000
	}
	checkFrom, checkTo, hasExpected := requiredTradingCoverageRangeWithStep(fromMS, toMS, stepMS, policy)
	if !hasExpected {
		return true
	}
	if len(records) == 0 {
		return false
	}

	seen := make(map[int64]struct{}, len(records))
	for _, record := range records {
		seen[record.TimeMS] = struct{}{}
	}
	for ts := checkFrom; ts <= checkTo; ts += stepMS {
		if !isVNTradingTimestampMSWithPolicy(ts, policy) {
			continue
		}
		if _, ok := seen[ts]; !ok {
			return false
		}
	}
	return true
}

func requiredTradingCoverageRange(fromMS, toMS int64, resolution int, policy tradingSessionPolicy) (int64, int64, bool) {
	if resolution <= 0 {
		resolution = 1
	}
	stepMS := int64(resolution) * 60_000
	if stepMS <= 0 {
		stepMS = 60_000
	}
	return requiredTradingCoverageRangeWithStep(fromMS, toMS, stepMS, policy)
}

func requiredTradingCoverageRangeWithStep(fromMS, toMS, stepMS int64, policy tradingSessionPolicy) (int64, int64, bool) {
	checkFrom := alignUpToStepMS(fromMS, stepMS)
	checkTo := lastCompletedCandleStartMS(toMS, stepMS)
	if checkTo < checkFrom {
		return 0, 0, false
	}
	first := int64(0)
	last := int64(0)
	for ts := checkFrom; ts <= checkTo; ts += stepMS {
		if !isVNTradingTimestampMSWithPolicy(ts, policy) {
			continue
		}
		if first == 0 {
			first = ts
		}
		last = ts
	}
	return first, last, first != 0
}

func alignUpToStepMS(value, stepMS int64) int64 {
	if stepMS <= 0 {
		return value
	}
	remainder := value % stepMS
	if remainder == 0 {
		return value
	}
	return value + stepMS - remainder
}

func lastCompletedCandleStartMS(toMS, stepMS int64) int64 {
	if stepMS <= 0 {
		return toMS
	}
	aligned := toMS - (toMS % stepMS)
	if aligned == toMS {
		return aligned
	}
	return aligned - stepMS
}

func candlesToRecords(symbol, marketType string, resolution int, candles []HistoryCandle) []storage.HistoryCandleRecord {
	out := make([]storage.HistoryCandleRecord, 0, len(candles))
	now := time.Now().UTC()
	for _, candle := range candles {
		out = append(out, storage.HistoryCandleRecord{
			Symbol:     symbol,
			MarketType: marketType,
			Resolution: resolution,
			TimeMS:     candle.Time,
			Open:       candle.Open,
			High:       candle.High,
			Low:        candle.Low,
			Close:      candle.Close,
			TickVolume: candle.TickVolume,
			UpdatedAt:  now,
		})
	}
	return out
}

func recordsToCandles(records []storage.HistoryCandleRecord) []HistoryCandle {
	out := make([]HistoryCandle, 0, len(records))
	for _, record := range records {
		out = append(out, HistoryCandle{
			IsHistory:  1,
			Time:       record.TimeMS,
			Open:       record.Open,
			High:       record.High,
			Low:        record.Low,
			Close:      record.Close,
			TickVolume: record.TickVolume,
		})
	}
	return out
}

func sameTradingSession(aMS, bMS int64) bool {
	return sameTradingSessionWithPolicy(aMS, bMS, tradingSessionPolicy{})
}

func sameTradingSessionWithPolicy(aMS, bMS int64, policy tradingSessionPolicy) bool {
	a := time.UnixMilli(aMS).In(vnLocation)
	b := time.UnixMilli(bMS).In(vnLocation)

	if a.Year() != b.Year() || a.Month() != b.Month() || a.Day() != b.Day() {
		return false
	}
	if !isWeekday(a) || !isWeekday(b) {
		return false
	}
	return sessionIDWithPolicy(a, policy) != 0 && sessionIDWithPolicy(a, policy) == sessionIDWithPolicy(b, policy)
}

func isWeekday(t time.Time) bool {
	return t.Weekday() >= time.Monday && t.Weekday() <= time.Friday
}

func sessionID(t time.Time) int {
	return sessionIDWithPolicy(t, tradingSessionPolicy{})
}

func sessionIDWithPolicy(t time.Time, policy tradingSessionPolicy) int {
	policy = normalizeTradingSessionPolicy(policy)
	minuteOfDay := t.Hour()*60 + t.Minute()
	switch {
	case minuteOfDay >= policy.MorningOpen && minuteOfDay <= policy.MorningClose:
		return 1
	case minuteOfDay >= policy.AfternoonOpen && minuteOfDay <= policy.AfternoonClose:
		return 2
	default:
		return 0
	}
}

func isKRXCandleMinute(t time.Time, policy tradingSessionPolicy) bool {
	start, ok := krxCandleStart(t, policy)
	return ok && start.Year() == t.Year() && start.Month() == t.Month() && start.Day() == t.Day() &&
		start.Hour() == t.Hour() && start.Minute() == t.Minute()
}

func krxCandleStartMS(timestampMS int64, policy tradingSessionPolicy) (int64, bool) {
	if timestampMS <= 0 {
		return 0, false
	}
	t := time.UnixMilli(timestampMS).In(vnLocation)
	start, ok := krxCandleStart(t, policy)
	if !ok {
		return 0, false
	}
	return start.UnixMilli(), true
}

func krxCandleStart(t time.Time, policy tradingSessionPolicy) (time.Time, bool) {
	if !isWeekday(t) {
		return time.Time{}, false
	}
	policy = normalizeTradingSessionPolicy(policy)
	minuteOfDay := t.Hour()*60 + t.Minute()
	if minuteOfDay < policy.MorningOpen || minuteOfDay > policy.AfternoonClose {
		return time.Time{}, false
	}
	if minuteOfDay > policy.MorningClose && minuteOfDay < policy.AfternoonOpen {
		return time.Time{}, false
	}
	if policy.OpeningAuctionEnd > 0 && minuteOfDay >= policy.MorningOpen && minuteOfDay < policy.OpeningAuctionEnd {
		return time.Date(t.Year(), t.Month(), t.Day(), policy.MorningOpen/60, policy.MorningOpen%60, 0, 0, vnLocation), true
	}
	if policy.ClosingAuction > 0 && minuteOfDay >= policy.ClosingAuction && minuteOfDay <= policy.AfternoonClose {
		return time.Date(t.Year(), t.Month(), t.Day(), policy.ClosingAuction/60, policy.ClosingAuction%60, 0, 0, vnLocation), true
	}
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, vnLocation), true
}

func isVNTradingTimestampMS(timestampMS int64) bool {
	return isVNTradingTimestampMSWithPolicy(timestampMS, tradingSessionPolicy{
		MorningOpen:    8*60 + 45,
		MorningClose:   11*60 + 30,
		AfternoonOpen:  13 * 60,
		AfternoonClose: 15 * 60,
	})
}

func isVNTradingTimestampMSWithPolicy(timestampMS int64, policy tradingSessionPolicy) bool {
	if timestampMS <= 0 {
		return false
	}
	t := time.UnixMilli(timestampMS).In(vnLocation)
	if !isWeekday(t) {
		return false
	}
	return isKRXCandleMinute(t, policy)
}

func normalizeRealtimeTickTimestampForSymbolMS(symbol string, timestampMS int64) (int64, bool) {
	policy := realtimePolicyForSymbol(symbol)
	startMS, ok := krxCandleStartMS(timestampMS, policy)
	if !ok {
		return 0, false
	}
	t := time.UnixMilli(timestampMS).In(vnLocation)
	start := time.UnixMilli(startMS).In(vnLocation)
	if t.Hour() == start.Hour() && t.Minute() == start.Minute() {
		return timestampMS, true
	}
	return startMS, true
}

func endOfPreviousDayUnix() int64 {
	nowVN := time.Now().In(vnLocation)
	startOfToday := time.Date(nowVN.Year(), nowVN.Month(), nowVN.Day(), 0, 0, 0, 0, vnLocation)
	return startOfToday.Add(-time.Second).Unix()
}

func startOfTodayUnix() int64 {
	nowVN := time.Now().In(vnLocation)
	return time.Date(nowVN.Year(), nowVN.Month(), nowVN.Day(), 0, 0, 0, 0, vnLocation).Unix()
}

func latestRequiredBootstrapDayStartMS() int64 {
	day := time.Now().In(vnLocation)
	for i := 0; i < 7; i++ {
		day = day.AddDate(0, 0, -1)
		if isWeekday(day) {
			start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, vnLocation)
			return start.UnixMilli()
		}
	}
	return 0
}
