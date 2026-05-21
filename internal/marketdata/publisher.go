package marketdata

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"sync"
	"time"

	"dnse-mt5-connector/internal/logger"
)

type Publisher struct {
	address string
	symbol  string
	store   *Store
	history *HistoryService
	logger  *logger.FileLogger
	status  *BridgeStatus
}

func NewPublisher(address, symbol string, store *Store, history *HistoryService, appLog *logger.FileLogger, status *BridgeStatus) *Publisher {
	return &Publisher{address: address, symbol: symbol, store: store, history: history, logger: appLog, status: status}
}

func (p *Publisher) Run(ctx context.Context) error {
	listener, err := net.Listen("tcp", p.address)
	if err != nil {
		return err
	}
	defer listener.Close()
	p.status.MarkPublisherStarted()
	p.logger.Info("marketdata_publisher_started", map[string]any{"address": p.address})

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			p.logger.Error("marketdata_publisher_accept_failed", map[string]any{"error": err.Error()})
			continue
		}
		go p.handle(ctx, conn)
	}
}

func (p *Publisher) handle(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Time{})
	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.SetNoDelay(true)
		_ = tcp.SetKeepAlive(true)
		_ = tcp.SetKeepAlivePeriod(30 * time.Second)
	}
	p.logger.Info("marketdata_client_connected", map[string]any{"remote": conn.RemoteAddr().String()})
	p.status.ClientConnected()
	defer func() {
		p.status.ClientDisconnected()
		p.logger.Info("marketdata_client_disconnected", map[string]any{"remote": conn.RemoteAddr().String()})
	}()

	subscribedSymbol := p.readSubscription(conn)
	if subscribedSymbol == "" {
		subscribedSymbol = p.symbol
	}

	ticks, unsubscribe := p.store.SubscribeSymbol(subscribedSymbol)
	defer unsubscribe()
	historyUpdates, unsubscribeHistory := p.history.SubscribeChanges()
	defer unsubscribeHistory()

	var writeMu sync.Mutex
	writeLine := func(line []byte) bool {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		_, err := conn.Write(line)
		return err == nil
	}

	if !p.writeHistorySnapshot(ctx, subscribedSymbol, writeLine) {
		return
	}

	if tick, ok := p.store.Latest(subscribedSymbol); ok && !writeLine(tick.JSONLine()) {
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case key, ok := <-historyUpdates:
			if !ok {
				return
			}
			if !strings.EqualFold(key.Symbol, subscribedSymbol) {
				continue
			}
			if !p.writeHistorySnapshot(ctx, subscribedSymbol, writeLine) {
				return
			}
		case tick, ok := <-ticks:
			if !ok || !writeLine(tick.JSONLine()) {
				return
			}
		}
	}
}

func (p *Publisher) readSubscription(conn net.Conn) string {
	_ = conn.SetReadDeadline(time.Now().Add(1500 * time.Millisecond))
	reader := bufio.NewReader(conn)
	line, err := reader.ReadBytes('\n')
	_ = conn.SetReadDeadline(time.Time{})
	if err != nil || len(line) == 0 {
		return ""
	}

	var payload struct {
		Type   string `json:"type"`
		Symbol string `json:"symbol"`
	}
	if err := json.Unmarshal(line, &payload); err != nil {
		return ""
	}
	if !strings.EqualFold(strings.TrimSpace(payload.Type), "subscribe") {
		return ""
	}
	symbol := strings.ToUpper(strings.TrimSpace(payload.Symbol))
	if symbol != "" {
		p.logger.Info("marketdata_client_subscribed", map[string]any{
			"remote": conn.RemoteAddr().String(),
			"symbol": symbol,
		})
	}
	return symbol
}

func (p *Publisher) writeHistorySnapshot(ctx context.Context, symbol string, writeLine func([]byte) bool) bool {
	if p.history != nil {
		p.history.EnsureSnapshotFromCache(ctx, symbol)
	}
	key, candles, ok := p.history.SnapshotForSymbol(symbol)
	if !ok {
		key = HistoryKey{Symbol: symbol}
		candles = nil
	}
	startLine, _ := json.Marshal(map[string]any{
		"type":   "history_start",
		"symbol": key.Symbol,
		"count":  len(candles),
	})
	if !writeLine(append(startLine, '\n')) {
		return false
	}
	for _, c := range candles {
		if !writeLine(c.JSONLine()) {
			return false
		}
	}
	endLine, _ := json.Marshal(map[string]any{
		"type":   "history_end",
		"symbol": key.Symbol,
		"count":  len(candles),
	})
	return writeLine(append(endLine, '\n'))
}
