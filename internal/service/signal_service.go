package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"dnse-mt5-connector/internal/logger"
)

const (
	ModeManual   = "manual"
	ModeSemiAuto = "semi-auto"
	ModeAuto     = "auto"
)

type SignalRequest struct {
	AccountNo     string  `json:"accountNo,omitempty"`
	Symbol        string  `json:"symbol"`
	Side          string  `json:"side"`
	Quantity      int     `json:"quantity"`
	Price         float64 `json:"price,omitempty"`
	OrderType     string  `json:"orderType,omitempty"`
	MarketType    string  `json:"marketType,omitempty"`
	OrderCategory string  `json:"orderCategory,omitempty"`
}

type Signal struct {
	ID            string    `json:"id"`
	Timestamp     time.Time `json:"timestamp"`
	ExpiresAt     time.Time `json:"expiresAt"`
	AccountNo     string    `json:"accountNo,omitempty"`
	Symbol        string    `json:"symbol"`
	Side          string    `json:"side"`
	Quantity      int       `json:"quantity"`
	Price         float64   `json:"price"`
	OrderType     string    `json:"orderType"`
	MarketType    string    `json:"marketType"`
	OrderCategory string    `json:"orderCategory"`
}

type SignalResponse struct {
	SignalID  string    `json:"signalId"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type ConfirmRequest struct {
	SignalID string `json:"signalId"`
}

type RejectRequest struct {
	SignalID string `json:"signalId"`
}

type ModeRequest struct {
	Mode string `json:"mode"`
}

type SignalService struct {
	orders        *OrderService
	logger        *logger.FileLogger
	mu            sync.Mutex
	mode          string
	signals       map[string]Signal
	recentSignals map[string]time.Time
	ttl           time.Duration
	lastMT5Seen   time.Time
}

func NewSignalService(orderService *OrderService, appLog *logger.FileLogger) *SignalService {
	s := &SignalService{
		orders:        orderService,
		logger:        appLog,
		mode:          ModeManual,
		signals:       make(map[string]Signal),
		recentSignals: make(map[string]time.Time),
		ttl:           10 * time.Second,
	}
	go s.expireLoop()
	return s
}

func (s *SignalService) Receive(ctx context.Context, req SignalRequest) (SignalResponse, error) {
	normalized, err := normalizeSignal(req)
	if err != nil {
		s.log("error", "signal_rejected_validation", map[string]any{"error": err.Error(), "request": req})
		return SignalResponse{}, err
	}
	if s.Mode() == ModeAuto {
		return SignalResponse{}, errors.New("auto mode is disabled")
	}

	now := time.Now().UTC()
	signal := Signal{
		ID:            newSignalID(),
		Timestamp:     now,
		ExpiresAt:     now.Add(s.ttl),
		AccountNo:     normalized.AccountNo,
		Symbol:        normalized.Symbol,
		Side:          normalized.Side,
		Quantity:      normalized.Quantity,
		Price:         normalized.Price,
		OrderType:     normalized.OrderType,
		MarketType:    normalized.MarketType,
		OrderCategory: normalized.OrderCategory,
	}

	key := normalized.Symbol + "|" + normalized.Side

	s.mu.Lock()
	if lastTime, exists := s.recentSignals[key]; exists {
		if now.Sub(lastTime) < 3*time.Second {
			s.mu.Unlock()
			err := errors.New("duplicate signal rejected within 3 seconds")
			s.log("error", "signal_rejected_duplicate", map[string]any{"error": err.Error(), "request": req})
			return SignalResponse{}, err
		}
	}
	s.recentSignals[key] = now
	s.pruneExpiredLocked(now)
	s.signals[signal.ID] = signal
	s.lastMT5Seen = now
	s.mu.Unlock()

	s.log("info", "signal_received", map[string]any{"signal": signal, "mode": s.Mode()})
	return SignalResponse{SignalID: signal.ID, ExpiresAt: signal.ExpiresAt}, nil
}

func (s *SignalService) Confirm(ctx context.Context, signalID string) (OrderResponse, error) {
	signalID = strings.TrimSpace(signalID)
	if signalID == "" {
		return OrderResponse{}, errors.New("signalId is required")
	}

	s.mu.Lock()
	signal, ok := s.signals[signalID]
	if !ok {
		s.mu.Unlock()
		return OrderResponse{}, errors.New("signal not found or already handled")
	}
	if time.Now().UTC().After(signal.ExpiresAt) {
		delete(s.signals, signalID)
		s.mu.Unlock()
		s.log("info", "signal_expired", map[string]any{"signalId": signalID})
		return OrderResponse{}, errors.New("signal expired")
	}
	delete(s.signals, signalID)
	s.mu.Unlock()

	orderReq := OrderRequest{
		ClientOrderID: "signal-" + signal.ID,
		AccountNo:     signal.AccountNo,
		Symbol:        signal.Symbol,
		Side:          signal.Side,
		Quantity:      signal.Quantity,
		Price:         signal.Price,
		OrderType:     signal.OrderType,
		MarketType:    signal.MarketType,
		OrderCategory: signal.OrderCategory,
	}
	resp, err := s.orders.PlaceOrder(ctx, orderReq)
	if err != nil {
		s.log("error", "signal_confirm_failed", map[string]any{"signalId": signalID, "error": err.Error(), "signal": signal})
		return OrderResponse{}, err
	}
	s.log("info", "signal_confirmed", map[string]any{"signalId": signalID, "orderId": resp.OrderID})
	return resp, nil
}

func (s *SignalService) Reject(signalID string) error {
	signalID = strings.TrimSpace(signalID)
	if signalID == "" {
		return errors.New("signalId is required")
	}
	s.mu.Lock()
	signal, ok := s.signals[signalID]
	if ok {
		delete(s.signals, signalID)
	}
	s.mu.Unlock()
	if !ok {
		return errors.New("signal not found or already handled")
	}
	s.log("info", "signal_rejected", map[string]any{"signal": signal})
	return nil
}

func (s *SignalService) Pending() []Signal {
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneExpiredLocked(now)

	out := make([]Signal, 0, len(s.signals))
	for _, signal := range s.signals {
		out = append(out, signal)
	}
	return out
}

func (s *SignalService) SetMode(mode string) error {
	mode = strings.ToLower(strings.TrimSpace(mode))
	switch mode {
	case ModeManual, ModeSemiAuto:
	case ModeAuto:
		return errors.New("auto mode is disabled in this MVP")
	default:
		return errors.New("mode must be manual, semi-auto, or auto")
	}
	s.mu.Lock()
	s.mode = mode
	s.mu.Unlock()
	s.log("info", "trading_mode_updated", map[string]any{"mode": mode})
	return nil
}

func (s *SignalService) Mode() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mode
}

func (s *SignalService) MT5Connected(window time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.lastMT5Seen.IsZero() && time.Since(s.lastMT5Seen) <= window
}

func (s *SignalService) MarkMT5Activity() {
	s.mu.Lock()
	s.lastMT5Seen = time.Now().UTC()
	s.mu.Unlock()
}

func (s *SignalService) expireLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for now := range ticker.C {
		s.mu.Lock()
		s.pruneExpiredLocked(now.UTC())
		s.mu.Unlock()
	}
}

func (s *SignalService) pruneExpiredLocked(now time.Time) {
	for id, signal := range s.signals {
		if now.After(signal.ExpiresAt) {
			delete(s.signals, id)
			s.log("info", "signal_expired", map[string]any{"signalId": id, "signal": signal})
		}
	}
}

func normalizeSignal(req SignalRequest) (SignalRequest, error) {
	req.AccountNo = strings.TrimSpace(req.AccountNo)
	req.Symbol = strings.ToUpper(strings.TrimSpace(req.Symbol))
	req.Side = strings.ToUpper(strings.TrimSpace(req.Side))
	req.OrderType = strings.ToUpper(strings.TrimSpace(req.OrderType))
	req.MarketType = strings.ToUpper(strings.TrimSpace(req.MarketType))
	req.OrderCategory = strings.ToUpper(strings.TrimSpace(req.OrderCategory))
	if req.Symbol == "" {
		return req, errors.New("symbol is required")
	}
	if req.Side != "BUY" && req.Side != "SELL" {
		return req, errors.New("side must be BUY or SELL")
	}
	if req.Quantity <= 0 {
		return req, errors.New("quantity must be greater than zero")
	}
	if req.Price < 0 {
		return req, errors.New("price cannot be negative")
	}
	if req.OrderType == "" {
		req.OrderType = "MTL"
	}
	if req.MarketType == "" {
		req.MarketType = "DERIVATIVE"
	}
	if req.OrderCategory == "" {
		req.OrderCategory = "NORMAL"
	}
	return req, nil
}

func newSignalID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err == nil {
		return hex.EncodeToString(b[:])
	}
	return strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "")
}

func (s *SignalService) log(level, event string, details map[string]any) {
	if s.logger == nil {
		return
	}
	if level == "error" {
		s.logger.Error(event, details)
		return
	}
	s.logger.Info(event, details)
}
