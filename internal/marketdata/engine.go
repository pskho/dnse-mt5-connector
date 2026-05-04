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

type symbolRuntimeState struct {
	mu                  sync.RWMutex
	lastWSTickAt        time.Time
	resolvedBoardID     string
	sampleRawRemaining  int
	sampleTickRemaining int
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
	states   map[string]*symbolRuntimeState
	status   *BridgeStatus
}

func NewEngine(cfg config.MarketDataConfig, apiKey, secret string, client MarketDataClient, history *HistoryService, appLog *logger.FileLogger) *Engine {
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

	for _, profile := range e.profiles {
		profile := profile
		if e.cfg.Mock {
			go e.runMock(ctx, profile)
			continue
		}
		go e.runWebSocketLoop(ctx, profile)
		if e.client != nil && profile.SupportsRESTFallback {
			go e.runRESTFallbackLoop(ctx, profile)
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
	e.logger.Info("marketdata_ws_connected", map[string]any{
		"url":      e.cfg.WebSocketURL,
		"symbol":   profile.Symbol,
		"channels": profile.Channels,
	})

	auth := e.authMessage()
	if err := ws.WriteText(auth); err != nil {
		return err
	}
	subscribe, _ := json.Marshal(map[string]any{
		"action":   "subscribe",
		"channels": e.subscribeChannels(profile),
	})
	if err := ws.WriteText(subscribe); err != nil {
		return err
	}
	e.logger.Info("marketdata_ws_subscribed", map[string]any{"symbol": profile.Symbol, "channels": profile.Channels})

	pingDone := make(chan struct{})
	defer close(pingDone)
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-pingDone:
				return
			case <-ticker.C:
				if err := ws.WritePing([]byte("keepalive")); err != nil {
					e.logger.Error("marketdata_ws_ping_failed", map[string]any{"symbol": profile.Symbol, "error": err.Error()})
					return
				}
			}
		}
	}()

	for ctx.Err() == nil {
		raw, err := ws.ReadText()
		if err != nil {
			return err
		}
		if e.shouldLogRawSample(profile.Symbol, raw) {
			e.logger.Info("marketdata_ws_market_sample", map[string]any{"symbol": profile.Symbol, "body": string(raw)})
		}
		if tick, ok := NormalizeTick(profile.Symbol, raw); ok {
			e.recordWSTick(profile.Symbol)
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
	boardID := e.boardID(ctx, profile)
	raw, err := e.client.FetchLatestTrade(ctx, profile.Symbol, boardID)
	if err != nil {
		e.logger.Error("marketdata_rest_fallback_error", map[string]any{
			"symbol":  profile.Symbol,
			"boardId": boardID,
			"error":   err.Error(),
		})
		return Tick{}, false
	}
	tick, ok := normalizeAny(strings.ToUpper(profile.Symbol), raw)
	if !ok {
		payload, _ := json.Marshal(raw)
		e.logger.Info("marketdata_rest_fallback_ignored", map[string]any{
			"symbol":  profile.Symbol,
			"boardId": boardID,
			"body":    string(payload),
		})
		return Tick{}, false
	}
	return tick, true
}

func (e *Engine) boardID(ctx context.Context, profile SymbolProfile) string {
	state := e.stateFor(profile.Symbol)
	state.mu.RLock()
	boardID := state.resolvedBoardID
	state.mu.RUnlock()
	if boardID != "" {
		return boardID
	}
	if profile.BoardID != "" {
		boardID = profile.BoardID
	}
	items, err := e.client.GetSecurityDefinition(ctx, profile.Symbol)
	if err != nil {
		e.logger.Error("marketdata_board_resolve_failed", map[string]any{"symbol": profile.Symbol, "error": err.Error()})
		return boardID
	}
	for _, item := range items {
		symbol := strings.ToUpper(strings.TrimSpace(firstStringFromMap(item, "symbol", "code", "ticker")))
		if symbol != "" && symbol != strings.ToUpper(profile.Symbol) {
			continue
		}
		found := strings.TrimSpace(firstStringFromMap(item, "boardId", "boardID"))
		if found != "" {
			boardID = found
			break
		}
	}
	if boardID != "" {
		state.mu.Lock()
		state.resolvedBoardID = boardID
		state.mu.Unlock()
		e.logger.Info("marketdata_board_resolved", map[string]any{"symbol": profile.Symbol, "boardId": boardID})
	}
	return boardID
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
	out := make([]map[string]any, 0, len(profile.Channels))
	for _, channel := range profile.Channels {
		channel = strings.TrimSpace(channel)
		if channel == "" {
			continue
		}
		out = append(out, map[string]any{
			"name":    channel,
			"symbols": []string{strings.ToUpper(profile.Symbol)},
		})
	}
	return out
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
