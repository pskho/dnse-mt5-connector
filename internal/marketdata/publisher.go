package marketdata

import (
	"context"
	"encoding/json"
	"errors"
	"net"
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
}

func NewPublisher(address, symbol string, store *Store, history *HistoryService, appLog *logger.FileLogger) *Publisher {
	return &Publisher{address: address, symbol: symbol, store: store, history: history, logger: appLog}
}

func (p *Publisher) Run(ctx context.Context) error {
	listener, err := net.Listen("tcp", p.address)
	if err != nil {
		return err
	}
	defer listener.Close()
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
	defer p.logger.Info("marketdata_client_disconnected", map[string]any{"remote": conn.RemoteAddr().String()})

	ticks, unsubscribe := p.store.SubscribeSymbol(p.symbol)
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

	if !p.writeHistorySnapshot(writeLine) {
		return
	}

	if tick, ok := p.store.Latest(p.symbol); ok && !writeLine(tick.JSONLine()) {
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-historyUpdates:
			if !ok || !p.writeHistorySnapshot(writeLine) {
				return
			}
		case tick, ok := <-ticks:
			if !ok || !writeLine(tick.JSONLine()) {
				return
			}
		}
	}
}

func (p *Publisher) writeHistorySnapshot(writeLine func([]byte) bool) bool {
	key, candles := p.history.Snapshot()
	if p.symbol != "" && key.Symbol != "" && key.Symbol != p.symbol {
		return true
	}
	startLine, _ := json.Marshal(map[string]any{
		"type":  "history_start",
		"symbol": key.Symbol,
		"count": len(candles),
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
		"type":  "history_end",
		"symbol": key.Symbol,
		"count": len(candles),
	})
	return writeLine(append(endLine, '\n'))
}
