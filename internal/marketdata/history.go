package marketdata

import (
	"context"
	"encoding/json"
	"sort"
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
	cfg     config.HistoryConfig
	client  HistoryClient
	logger  *logger.FileLogger
	store   HistorySyncLogger
	cache   HistoryCache
	mu      sync.RWMutex
	candles []HistoryCandle
	current HistoryKey
	subs    map[chan struct{}]struct{}
}

type HistoryClient interface {
	FetchOHLC(ctx context.Context, symbol, marketType string, resolution int, from, to int64) (map[string]any, error)
}

func NewHistoryService(cfg config.HistoryConfig, client HistoryClient, store HistorySyncLogger, logger *logger.FileLogger) *HistoryService {
	var cache HistoryCache
	if c, ok := store.(HistoryCache); ok {
		cache = c
	}
	return &HistoryService{
		cfg:    cfg,
		client: client,
		store:  store,
		logger: logger,
		cache:  cache,
		subs:   make(map[chan struct{}]struct{}),
	}
}

func (s *HistoryService) Fetch(ctx context.Context) error {
	// Startup fetch is now handled by the MQL5 EA triggering Sync
	return nil
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
	if s.cache != nil {
		minMS, maxMS, count, err := s.cache.GetHistoryCoverage(ctx, symbol, marketType, resolution)
		if err == nil && count > 0 && minMS <= fromMS && maxMS >= toMS {
			cached, loadErr := s.cache.LoadHistoryCandles(ctx, symbol, marketType, resolution, fromMS, toMS)
			if loadErr == nil && len(cached) > 0 {
				stagedCandles := recordsToCandles(cached)
				stagedCandles = fillDerivativeSessionGapsIfNeeded(symbol, marketType, stagedCandles)
				s.replaceStagedCandles(HistoryKey{
					Symbol:     symbol,
					MarketType: marketType,
					Resolution: resolution,
				}, stagedCandles)
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
			}
		}
	}

	totalCandles := 0
	batchSeconds := int64(s.cfg.MaxBatchDays * 24 * 3600)
	currentFrom := from

	// Replace the staged candle snapshot atomically for the next bridge client.
	stagedCandles := make([]HistoryCandle, 0)
	seen := make(map[int64]bool)

	for currentFrom < to {
		currentTo := currentFrom + batchSeconds
		if currentTo > to {
			currentTo = to
		}

		raw, err := s.client.FetchOHLC(ctx, symbol, marketType, resolution, currentFrom, currentTo)
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
		batchAdded := 0
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

		s.logger.Info("history_sync_batch_completed", map[string]any{"currentFrom": currentFrom, "currentTo": currentTo, "added": batchAdded})
		currentFrom = currentTo + 1
	}

	sort.Slice(stagedCandles, func(i, j int) bool {
		return stagedCandles[i].Time < stagedCandles[j].Time
	})
	stagedCandles = fillDerivativeSessionGapsIfNeeded(symbol, marketType, stagedCandles)

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

	s.replaceStagedCandles(HistoryKey{
		Symbol:     symbol,
		MarketType: marketType,
		Resolution: resolution,
	}, stagedCandles)

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
	s.current = key
	s.candles = stagedCandles
	for ch := range s.subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
	s.mu.Unlock()
}

func (s *HistoryService) Candles() []HistoryCandle {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]HistoryCandle, len(s.candles))
	copy(out, s.candles)
	return out
}

func (s *HistoryService) Snapshot() (HistoryKey, []HistoryCandle) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]HistoryCandle, len(s.candles))
	copy(out, s.candles)
	return s.current, out
}

func (s *HistoryService) ClearCandles() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.current = HistoryKey{}
	s.candles = make([]HistoryCandle, 0)
}

func (s *HistoryService) SubscribeChanges() (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
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

func fillDerivativeSessionGaps(candles []HistoryCandle) []HistoryCandle {
	if len(candles) < 2 {
		return candles
	}

	out := make([]HistoryCandle, 0, len(candles)+1024)
	out = append(out, candles[0])

	for i := 1; i < len(candles); i++ {
		prev := out[len(out)-1]
		curr := candles[i]

		for ts := prev.Time + 60_000; ts < curr.Time; ts += 60_000 {
			if !sameTradingSession(prev.Time, ts) || !sameTradingSession(ts, curr.Time) {
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

func fillDerivativeSessionGapsIfNeeded(symbol, marketType string, candles []HistoryCandle) []HistoryCandle {
	if strings.EqualFold(strings.TrimSpace(marketType), "DERIVATIVE") || strings.HasPrefix(strings.ToUpper(strings.TrimSpace(symbol)), "VN30F") {
		return fillDerivativeSessionGaps(candles)
	}
	return candles
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
	a := time.UnixMilli(aMS).In(vnLocation)
	b := time.UnixMilli(bMS).In(vnLocation)

	if a.Year() != b.Year() || a.Month() != b.Month() || a.Day() != b.Day() {
		return false
	}
	if !isWeekday(a) || !isWeekday(b) {
		return false
	}
	return sessionID(a) != 0 && sessionID(a) == sessionID(b)
}

func isWeekday(t time.Time) bool {
	return t.Weekday() >= time.Monday && t.Weekday() <= time.Friday
}

func sessionID(t time.Time) int {
	minuteOfDay := t.Hour()*60 + t.Minute()
	switch {
	case minuteOfDay >= 8*60+45 && minuteOfDay <= 11*60+29:
		return 1
	case minuteOfDay >= 13*60 && minuteOfDay <= 14*60+44:
		return 2
	default:
		return 0
	}
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
