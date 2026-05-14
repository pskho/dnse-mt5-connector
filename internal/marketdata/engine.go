package marketdata

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"dnse-mt5-connector/internal/config"
	"dnse-mt5-connector/internal/logger"
)

type MarketDataClient interface {
	GetSecurityDefinition(ctx context.Context, symbol string) ([]map[string]any, error)
	FetchLatestTrade(ctx context.Context, symbol, boardID string) (any, error)
}

type FeedSymbolResolver interface {
	ResolveMarketFeedSymbol(ctx context.Context, displaySymbol string) (string, bool, error)
	ResolveTickerBoard(ctx context.Context, symbol string) (string, string, bool, error)
}

type symbolRuntimeState struct {
	mu                  sync.RWMutex
	lastWSTickAt        time.Time
	resolvedBoardID     string
	resolvedFeedSymbol  string
	nextRESTAttemptAt   time.Time
	sampleRawRemaining  int
	sampleTickRemaining int
}

type restThrottleState struct {
	mu            sync.Mutex
	nextAllowedAt time.Time
}

type Engine struct {
	cfg      config.MarketDataConfig
	apiKey   string
	secret   string
	client   MarketDataClient
	store    *Store
	history  *HistoryService
	logger   *logger.FileLogger
	profiles []SymbolProfile
	statesMu sync.Mutex
	states   map[string]*symbolRuntimeState
	status   *BridgeStatus
	restGate *restThrottleState
	resolver FeedSymbolResolver
}

func NewEngine(cfg config.MarketDataConfig, apiKey, secret string, client MarketDataClient, resolver FeedSymbolResolver, history *HistoryService, appLog *logger.FileLogger) *Engine {
	profiles := BuildProfiles(cfg)
	states := make(map[string]*symbolRuntimeState, len(profiles))
	for _, profile := range profiles {
		states[profile.Symbol] = &symbolRuntimeState{
			sampleRawRemaining:  10,
			sampleTickRemaining: 10,
		}
	}
	return &Engine{
		cfg:      cfg,
		apiKey:   apiKey,
		secret:   secret,
		client:   client,
		store:    NewStore(),
		history:  history,
		logger:   appLog,
		profiles: profiles,
		states:   states,
		status:   NewBridgeStatus(),
		restGate: &restThrottleState{},
		resolver: resolver,
	}
}

func (e *Engine) Store() *Store {
	return e.store
}

func (e *Engine) Profiles() []SymbolProfile {
	out := make([]SymbolProfile, len(e.profiles))
	copy(out, e.profiles)
	return out
}

func (e *Engine) Status() BridgeStatusSnapshot {
	if e == nil || e.status == nil {
		return BridgeStatusSnapshot{}
	}
	return e.status.Snapshot()
}

func (e *Engine) LatestTick(symbol string) (Tick, bool) {
	if e == nil || e.store == nil {
		return Tick{}, false
	}
	return e.store.Latest(symbol)
}

func (e *Engine) Start(ctx context.Context) {
	if !e.cfg.Enabled {
		e.logger.Info("marketdata_disabled", nil)
		return
	}

	publisher := NewPublisher(e.cfg.BridgeAddress, e.cfg.Symbol, e.store, e.history, e.logger, e.status)
	go func() {
		if err := publisher.Run(ctx); err != nil {
			e.logger.Error("marketdata_publisher_failed", map[string]any{"error": err.Error()})
		}
	}()

	if len(e.profiles) == 0 {
		e.logger.Error("marketdata_profiles_empty", map[string]any{"symbols": e.cfg.Symbols})
		return
	}

	primarySymbol := strings.ToUpper(strings.TrimSpace(e.cfg.Symbol))
	for _, profile := range e.profiles {
		profile := profile
		if e.cfg.Mock {
			go e.runMock(ctx, profile)
			continue
		}
		go e.runWebSocketLoop(ctx, profile)
		if e.client != nil && profile.SupportsRESTFallback && strings.EqualFold(profile.Symbol, primarySymbol) {
			go e.runRESTFallbackLoop(ctx, profile)
		} else if e.client != nil && profile.SupportsRESTFallback {
			e.logger.Info("marketdata_rest_fallback_skipped", map[string]any{
				"symbol": profile.Symbol,
				"reason": "ws_first_non_primary",
			})
		}
	}
}

func (e *Engine) runWebSocketLoop(ctx context.Context, profile SymbolProfile) {
	delay := time.Duration(e.cfg.ReconnectSeconds) * time.Second
	for ctx.Err() == nil {
		err := e.runWebSocket(ctx, profile)
		if err != nil && ctx.Err() == nil {
			e.logger.Error("marketdata_ws_disconnected", map[string]any{
				"symbol":             profile.Symbol,
				"error":              err.Error(),
				"reconnectInSeconds": int(delay.Seconds()),
			})
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}
}

func (e *Engine) runWebSocket(ctx context.Context, profile SymbolProfile) error {
	if strings.TrimSpace(e.apiKey) == "" || strings.TrimSpace(e.secret) == "" {
		return errors.New("market data websocket requires dnse api_key and api_secret")
	}

	ws, err := dialWebSocket(e.cfg.WebSocketURL, 10*time.Second)
	if err != nil {
		return err
	}
	defer ws.Close()
	if e.status != nil {
		e.status.MarkWSConnected(profile.Symbol)
	}
	e.logger.Info("marketdata_ws_connected", map[string]any{
		"url":      e.cfg.WebSocketURL,
		"symbol":   profile.Symbol,
		"channels": profile.Channels,
	})

	welcome, err := ws.ReadText()
	if err != nil {
		return fmt.Errorf("read websocket welcome: %w", err)
	}
	e.logger.Info("marketdata_ws_welcome", map[string]any{"symbol": profile.Symbol, "body": string(welcome)})

	auth := e.authMessage()
	if err := ws.WriteText(auth); err != nil {
		return err
	}
	authResp, err := ws.ReadText()
	if err != nil {
		return fmt.Errorf("read websocket auth response: %w", err)
	}
	if action := wsAction(authResp); action != "auth_success" {
		return fmt.Errorf("websocket auth failed: %s", strings.TrimSpace(string(authResp)))
	}
	e.logger.Info("marketdata_ws_authenticated", map[string]any{"symbol": profile.Symbol})

	channels := e.subscribeChannels(profile)
	subscribe, _ := json.Marshal(map[string]any{
		"action":   "subscribe",
		"channels": channels,
	})
	if err := ws.WriteText(subscribe); err != nil {
		return err
	}
	e.logger.Info("marketdata_ws_subscribed", map[string]any{"symbol": profile.Symbol, "channels": channels})

	pingDone := make(chan struct{})
	defer close(pingDone)
	go func() {
		ticker := time.NewTicker(25 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-pingDone:
				return
			case <-ticker.C:
				if err := ws.WriteText([]byte(`{"action":"ping"}`)); err != nil {
					e.logger.Error("marketdata_ws_ping_failed", map[string]any{"symbol": profile.Symbol, "error": err.Error()})
					return
				}
			}
		}
	}()

	sessionDeadline := time.Now().Add(7*time.Hour + 45*time.Minute)
	for ctx.Err() == nil {
		if time.Now().After(sessionDeadline) {
			return errors.New("websocket proactive reconnect before 8h session limit")
		}
		raw, err := ws.ReadText()
		if err != nil {
			if e.status != nil {
				e.status.MarkWSDisconnected(profile.Symbol, err)
			}
			return err
		}
		if len(raw) == 0 {
			return errors.New("websocket empty market data frame")
		}
		if e.shouldLogRawSample(profile.Symbol, raw) {
			e.logger.Info("marketdata_ws_market_sample", map[string]any{"symbol": profile.Symbol, "body": string(raw)})
		}
		switch action := wsAction(raw); action {
		case "ping":
			_ = ws.WriteText([]byte(`{"action":"pong"}`))
			continue
		case "pong", "subscribed", "welcome", "auth_success":
			continue
		case "error", "auth_error":
			return fmt.Errorf("websocket server error: %s", strings.TrimSpace(string(raw)))
		}
		if tick, ok := NormalizeTick("", raw); ok {
			tick.Symbol = strings.ToUpper(profile.Symbol)
			e.recordWSTick(profile.Symbol)
			if e.status != nil {
				e.status.MarkMarketMessage(profile.Symbol, tick.Source)
			}
			e.store.Update(tick)
			if e.consumeTickSample(profile.Symbol) {
				e.logger.Info("marketdata_ws_tick_sample", map[string]any{
					"symbol":      tick.Symbol,
					"bid":         tick.Bid,
					"ask":         tick.Ask,
					"last":        tick.Last,
					"volume":      tick.Volume,
					"timestampMS": tick.TimestampMS,
				})
			}
			continue
		}
		e.logger.Info("marketdata_ws_message_ignored", map[string]any{"symbol": profile.Symbol, "body": string(raw)})
	}
	return ctx.Err()
}

func (e *Engine) runRESTFallbackLoop(ctx context.Context, profile SymbolProfile) {
	if e.client == nil {
		return
	}
	pollTicker := time.NewTicker(400 * time.Millisecond)
	defer pollTicker.Stop()

	e.logger.Info("marketdata_rest_fallback_started", map[string]any{
		"symbol":         profile.Symbol,
		"pollIntervalMs": 400,
		"idleTriggerMs":  2500,
	})

	var lastSent Tick
	var hasLastSent bool
	for {
		select {
		case <-ctx.Done():
			return
		case <-pollTicker.C:
			if e.wsRecentlyActive(profile.Symbol, 2500*time.Millisecond) {
				continue
			}
			fallbackTick, ok := e.fetchFallbackTick(ctx, profile)
			if !ok {
				continue
			}
			if hasLastSent && sameTick(lastSent, fallbackTick) {
				continue
			}
			lastSent = fallbackTick
			hasLastSent = true
			if e.status != nil {
				e.status.MarkMarketMessage(profile.Symbol, fallbackTick.Source)
			}
			e.store.Update(fallbackTick)
			if e.consumeTickSample(profile.Symbol) {
				e.logger.Info("marketdata_rest_tick_sample", map[string]any{
					"symbol":      fallbackTick.Symbol,
					"bid":         fallbackTick.Bid,
					"ask":         fallbackTick.Ask,
					"last":        fallbackTick.Last,
					"volume":      fallbackTick.Volume,
					"timestampMS": fallbackTick.TimestampMS,
				})
			}
		}
	}
}

func (e *Engine) fetchFallbackTick(ctx context.Context, profile SymbolProfile) (Tick, bool) {
	state := e.stateFor(profile.Symbol)
	state.mu.RLock()
	nextAttempt := state.nextRESTAttemptAt
	state.mu.RUnlock()
	if !nextAttempt.IsZero() && time.Now().Before(nextAttempt) {
		return Tick{}, false
	}

	e.waitRESTTurn(ctx)

	feedSymbol, boardID := e.resolveFeedSymbol(ctx, profile)
	raw, err := e.client.FetchLatestTrade(ctx, feedSymbol, boardID)
	if err != nil {
		delay := 5 * time.Second
		if strings.Contains(strings.ToLower(err.Error()), "429") || strings.Contains(strings.ToLower(err.Error()), "rate limit") {
			delay = 15 * time.Second
		}
		state.mu.Lock()
		state.nextRESTAttemptAt = time.Now().Add(delay)
		state.mu.Unlock()
		e.logger.Error("marketdata_rest_fallback_error", map[string]any{
			"symbol":     profile.Symbol,
			"feedSymbol": feedSymbol,
			"boardId":    boardID,
			"error":      err.Error(),
		})
		return Tick{}, false
	}
	state.mu.Lock()
	state.nextRESTAttemptAt = time.Now().Add(3 * time.Second)
	state.mu.Unlock()
	tick, ok := normalizeAny("", raw)
	if !ok {
		payload, _ := json.Marshal(raw)
		e.logger.Info("marketdata_rest_fallback_ignored", map[string]any{
			"symbol":     profile.Symbol,
			"feedSymbol": feedSymbol,
			"boardId":    boardID,
			"body":       string(payload),
		})
		return Tick{}, false
	}
	tick.Symbol = strings.ToUpper(profile.Symbol)
	return tick, true
}

func (e *Engine) resolveFeedSymbol(ctx context.Context, profile SymbolProfile) (string, string) {
	state := e.stateFor(profile.Symbol)
	state.mu.RLock()
	boardID := state.resolvedBoardID
	feedSymbol := state.resolvedFeedSymbol
	state.mu.RUnlock()
	if boardID != "" && feedSymbol != "" {
		return feedSymbol, boardID
	}
	if feedSymbol == "" {
		feedSymbol = strings.ToUpper(strings.TrimSpace(profile.Symbol))
	}
	if profile.BoardID != "" {
		boardID = profile.BoardID
	}
	if e.resolver != nil {
		if resolvedFeed, resolvedBoard, ok, err := e.resolver.ResolveTickerBoard(ctx, profile.Symbol); err != nil {
			e.logger.Error("marketdata_ticker_metadata_resolve_failed", map[string]any{"symbol": profile.Symbol, "error": err.Error()})
		} else if ok {
			if strings.TrimSpace(resolvedFeed) != "" {
				feedSymbol = strings.ToUpper(strings.TrimSpace(resolvedFeed))
			}
			if strings.TrimSpace(resolvedBoard) != "" {
				boardID = strings.ToUpper(strings.TrimSpace(resolvedBoard))
			}
		}
	}
	if e.resolver != nil && isDerivativeSymbol(profile.Symbol) {
		if resolved, ok, err := e.resolver.ResolveMarketFeedSymbol(ctx, profile.Symbol); err != nil {
			e.logger.Error("marketdata_feed_symbol_resolve_failed", map[string]any{"symbol": profile.Symbol, "error": err.Error()})
		} else if ok && strings.TrimSpace(resolved) != "" {
			feedSymbol = strings.ToUpper(strings.TrimSpace(resolved))
		}
	}
	items, err := e.client.GetSecurityDefinition(ctx, profile.Symbol)
	if err != nil {
		e.logger.Error("marketdata_board_resolve_failed", map[string]any{"symbol": profile.Symbol, "error": err.Error()})
		return feedSymbol, boardID
	}
	bestBoard := boardID
	if found := preferredBoardID(items); found != "" {
		bestBoard = found
	}
	for _, item := range items {
		foundSymbol := strings.ToUpper(strings.TrimSpace(firstStringFromMap(item, "symbol", "code", "ticker")))
		if foundSymbol != "" && !isDerivativeSymbol(profile.Symbol) {
			feedSymbol = foundSymbol
		}
		found := strings.TrimSpace(firstStringFromMap(item, "boardId", "boardID"))
		if found != "" && found == bestBoard {
			boardID = found
			break
		}
	}
	if boardID == "" {
		boardID = bestBoard
	}
	state.mu.Lock()
	if boardID != "" {
		state.resolvedBoardID = boardID
	}
	if feedSymbol != "" {
		state.resolvedFeedSymbol = feedSymbol
	}
	state.mu.Unlock()
	e.logger.Info("marketdata_board_resolved", map[string]any{"symbol": profile.Symbol, "feedSymbol": feedSymbol, "boardId": boardID})
	return feedSymbol, boardID
}

func preferredBoardID(items []map[string]any) string {
	preferred := []string{"G1", "G4", "G7", "T1", "T3", "T4", "T6"}
	available := make(map[string]struct{}, len(items))
	for _, item := range items {
		boardID := strings.TrimSpace(firstStringFromMap(item, "boardId", "boardID"))
		if boardID != "" {
			available[boardID] = struct{}{}
		}
	}
	for _, boardID := range preferred {
		if _, ok := available[boardID]; ok {
			return boardID
		}
	}
	for boardID := range available {
		return boardID
	}
	return ""
}

func (e *Engine) wsRecentlyActive(symbol string, maxIdle time.Duration) bool {
	state := e.stateFor(symbol)
	state.mu.RLock()
	last := state.lastWSTickAt
	state.mu.RUnlock()
	return !last.IsZero() && time.Since(last) <= maxIdle
}

func (e *Engine) recordWSTick(symbol string) {
	state := e.stateFor(symbol)
	state.mu.Lock()
	state.lastWSTickAt = time.Now()
	state.mu.Unlock()
}

func sameTick(a, b Tick) bool {
	return a.Symbol == b.Symbol &&
		a.TimestampMS == b.TimestampMS &&
		a.Last == b.Last &&
		a.Bid == b.Bid &&
		a.Ask == b.Ask &&
		a.Volume == b.Volume
}

func firstStringFromMap(item map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			switch v := value.(type) {
			case string:
				return strings.TrimSpace(v)
			}
		}
	}
	return ""
}

func (e *Engine) shouldLogRawSample(symbol string, raw []byte) bool {
	state := e.stateFor(symbol)
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.sampleRawRemaining <= 0 {
		return false
	}
	text := string(raw)
	for _, skip := range []string{`"action":"welcome"`, `"action":"auth_success"`, `"action":"subscribed"`} {
		if strings.Contains(text, skip) {
			return false
		}
	}
	state.sampleRawRemaining--
	return true
}

func (e *Engine) consumeTickSample(symbol string) bool {
	state := e.stateFor(symbol)
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.sampleTickRemaining <= 0 {
		return false
	}
	state.sampleTickRemaining--
	return true
}

func (e *Engine) authMessage() []byte {
	timestamp := time.Now().Unix()
	nonce := nonce()
	message := fmt.Sprintf("%s:%d:%s", e.apiKey, timestamp, nonce)
	mac := hmac.New(sha256.New, []byte(e.secret))
	mac.Write([]byte(message))
	signature := hex.EncodeToString(mac.Sum(nil))
	raw, _ := json.Marshal(map[string]any{
		"action":    "auth",
		"api_key":   e.apiKey,
		"signature": signature,
		"timestamp": timestamp,
		"nonce":     nonce,
	})
	return raw
}

func (e *Engine) subscribeChannels(profile SymbolProfile) []map[string]any {
	feedSymbol, _ := e.resolveFeedSymbol(context.Background(), profile)
	out := make([]map[string]any, 0, len(profile.Channels))
	for _, channel := range profile.Channels {
		channel = normalizeConfiguredChannel(channel, profile.BoardID, profile.Resolution)
		if channel == "" {
			continue
		}
		entry := map[string]any{"name": channel}
		if !strings.HasPrefix(strings.ToLower(channel), "market_index.") {
			entry["symbols"] = []string{strings.ToUpper(feedSymbol)}
		} else {
			entry["symbols"] = []string{}
		}
		out = append(out, entry)
	}
	return out
}

func normalizeConfiguredChannel(channel, boardID string, resolution int) string {
	boardID = strings.ToUpper(strings.TrimSpace(boardID))
	if boardID == "" {
		boardID = "G1"
	}
	if resolution <= 0 {
		resolution = 1
	}
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case "trades.json", "trade.json", "tick.json", "trades":
		return "tick." + boardID + ".json"
	case "trade_extra.json", "trades_extra.json", "tick_extra.json":
		return "tick_extra." + boardID + ".json"
	case "quotes.json", "quote.json", "top_price.json", "quotes":
		return "top_price." + boardID + ".json"
	case "ohlc.json", "ohlc":
		return "ohlc." + strconv.Itoa(resolution) + ".json"
	case "ohlc_closed.json", "ohlc_closed":
		return "ohlc_closed." + strconv.Itoa(resolution) + ".json"
	default:
		return strings.TrimSpace(channel)
	}
}

func wsAction(raw []byte) string {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(firstStringFromMap(payload, "action", "a")))
}

func (e *Engine) waitRESTTurn(ctx context.Context) {
	if e.restGate == nil {
		return
	}
	for {
		e.restGate.mu.Lock()
		now := time.Now()
		if now.After(e.restGate.nextAllowedAt) || now.Equal(e.restGate.nextAllowedAt) {
			e.restGate.nextAllowedAt = now.Add(750 * time.Millisecond)
			e.restGate.mu.Unlock()
			return
		}
		wait := e.restGate.nextAllowedAt.Sub(now)
		e.restGate.mu.Unlock()
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

func (e *Engine) runMock(ctx context.Context, profile SymbolProfile) {
	e.logger.Info("marketdata_mock_started", map[string]any{"symbol": profile.Symbol})
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	base := 1200.0
	var volume int64 = 1
	for {
		select {
		case <-ctx.Done():
			return
		case t := <-ticker.C:
			wave := math.Sin(float64(t.UnixMilli()) / 2500)
			last := base + wave*2
			e.store.Update(Tick{
				Symbol:      strings.ToUpper(profile.Symbol),
				Bid:         last - 0.1,
				Ask:         last + 0.1,
				Last:        last,
				Volume:      volume,
				TimestampMS: t.UnixMilli(),
			})
			volume++
		}
	}
}

func (e *Engine) stateFor(symbol string) *symbolRuntimeState {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	e.statesMu.Lock()
	defer e.statesMu.Unlock()
	if state, ok := e.states[symbol]; ok {
		return state
	}
	state := &symbolRuntimeState{
		sampleRawRemaining:  10,
		sampleTickRemaining: 10,
	}
	e.states[symbol] = state
	return state
}

func nonce() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
