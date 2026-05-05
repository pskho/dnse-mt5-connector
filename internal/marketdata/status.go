package marketdata

import (
	"sync"
	"time"
)

type BridgeStatusSnapshot struct {
	PublisherStarted         bool                       `json:"publisherStarted"`
	ActiveClients            int                        `json:"activeClients"`
	LastClientConnectedAt    time.Time                  `json:"lastClientConnectedAt,omitempty"`
	LastClientDisconnectedAt time.Time                  `json:"lastClientDisconnectedAt,omitempty"`
	Symbols                  []SymbolFeedStatusSnapshot `json:"symbols,omitempty"`
}

type SymbolFeedStatusSnapshot struct {
	Symbol             string    `json:"symbol"`
	Connected          bool      `json:"connected"`
	LastConnectedAt    time.Time `json:"lastConnectedAt,omitempty"`
	LastDisconnectedAt time.Time `json:"lastDisconnectedAt,omitempty"`
	LastMessageAt      time.Time `json:"lastMessageAt,omitempty"`
	LastTradeAt        time.Time `json:"lastTradeAt,omitempty"`
	LastQuoteAt        time.Time `json:"lastQuoteAt,omitempty"`
	LastOHLCAt         time.Time `json:"lastOhlcAt,omitempty"`
	LastSource         string    `json:"lastSource,omitempty"`
	ReconnectCount     int       `json:"reconnectCount"`
	LastError          string    `json:"lastError,omitempty"`
	StaleSeconds       int64     `json:"staleSeconds,omitempty"`
}

type BridgeStatus struct {
	mu                       sync.RWMutex
	publisherStarted         bool
	activeClients            int
	lastClientConnectedAt    time.Time
	lastClientDisconnectedAt time.Time
	symbols                  map[string]*symbolFeedStatus
}

type symbolFeedStatus struct {
	connected          bool
	lastConnectedAt    time.Time
	lastDisconnectedAt time.Time
	lastMessageAt      time.Time
	lastTradeAt        time.Time
	lastQuoteAt        time.Time
	lastOHLCAt         time.Time
	lastSource         string
	reconnectCount     int
	lastError          string
}

func NewBridgeStatus() *BridgeStatus {
	return &BridgeStatus{symbols: make(map[string]*symbolFeedStatus)}
}

func (s *BridgeStatus) MarkPublisherStarted() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.publisherStarted = true
	s.mu.Unlock()
}

func (s *BridgeStatus) ClientConnected() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.activeClients++
	s.lastClientConnectedAt = time.Now()
	s.mu.Unlock()
}

func (s *BridgeStatus) ClientDisconnected() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.activeClients > 0 {
		s.activeClients--
	}
	s.lastClientDisconnectedAt = time.Now()
	s.mu.Unlock()
}

func (s *BridgeStatus) MarkWSConnected(symbol string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	st := s.symbolStatusLocked(symbol)
	st.connected = true
	st.lastConnectedAt = time.Now()
	st.lastError = ""
	s.mu.Unlock()
}

func (s *BridgeStatus) MarkWSDisconnected(symbol string, err error) {
	if s == nil {
		return
	}
	s.mu.Lock()
	st := s.symbolStatusLocked(symbol)
	st.connected = false
	st.lastDisconnectedAt = time.Now()
	st.reconnectCount++
	if err != nil {
		st.lastError = err.Error()
	}
	s.mu.Unlock()
}

func (s *BridgeStatus) MarkMarketMessage(symbol, source string) {
	if s == nil {
		return
	}
	now := time.Now()
	s.mu.Lock()
	st := s.symbolStatusLocked(symbol)
	st.lastMessageAt = now
	st.lastSource = source
	switch source {
	case "trade", "trade_extra":
		st.lastTradeAt = now
	case "quote":
		st.lastQuoteAt = now
	case "ohlc", "market_index":
		st.lastOHLCAt = now
	}
	s.mu.Unlock()
}

func (s *BridgeStatus) symbolStatusLocked(symbol string) *symbolFeedStatus {
	if s.symbols == nil {
		s.symbols = make(map[string]*symbolFeedStatus)
	}
	st := s.symbols[symbol]
	if st == nil {
		st = &symbolFeedStatus{}
		s.symbols[symbol] = st
	}
	return st
}

func (s *BridgeStatus) Snapshot() BridgeStatusSnapshot {
	if s == nil {
		return BridgeStatusSnapshot{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	symbols := make([]SymbolFeedStatusSnapshot, 0, len(s.symbols))
	now := time.Now()
	for symbol, st := range s.symbols {
		staleSeconds := int64(0)
		if !st.lastMessageAt.IsZero() {
			staleSeconds = int64(now.Sub(st.lastMessageAt).Seconds())
		}
		symbols = append(symbols, SymbolFeedStatusSnapshot{
			Symbol:             symbol,
			Connected:          st.connected,
			LastConnectedAt:    st.lastConnectedAt,
			LastDisconnectedAt: st.lastDisconnectedAt,
			LastMessageAt:      st.lastMessageAt,
			LastTradeAt:        st.lastTradeAt,
			LastQuoteAt:        st.lastQuoteAt,
			LastOHLCAt:         st.lastOHLCAt,
			LastSource:         st.lastSource,
			ReconnectCount:     st.reconnectCount,
			LastError:          st.lastError,
			StaleSeconds:       staleSeconds,
		})
	}
	return BridgeStatusSnapshot{
		PublisherStarted:         s.publisherStarted,
		ActiveClients:            s.activeClients,
		LastClientConnectedAt:    s.lastClientConnectedAt,
		LastClientDisconnectedAt: s.lastClientDisconnectedAt,
		Symbols:                  symbols,
	}
}
