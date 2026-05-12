package marketdata

import (
	"encoding/json"
	"strings"
	"sync"
)

type Tick struct {
	Symbol      string  `json:"symbol"`
	Bid         float64 `json:"bid"`
	Ask         float64 `json:"ask"`
	Last        float64 `json:"last"`
	Volume      int64   `json:"volume"`
	TimestampMS int64   `json:"timestamp_ms"`
	Source      string  `json:"source,omitempty"`
}

type Store struct {
	mu                  sync.RWMutex
	latestBySymbol      map[string]Tick
	latest              Tick
	hasLatest           bool
	subscribers         map[chan Tick]struct{}
	subscribersBySymbol map[string]map[chan Tick]struct{}
}

func NewStore() *Store {
	return &Store{
		latestBySymbol:      make(map[string]Tick),
		subscribers:         make(map[chan Tick]struct{}),
		subscribersBySymbol: make(map[string]map[chan Tick]struct{}),
	}
}

func (s *Store) Update(tick Tick) {
	tick.Symbol = strings.ToUpper(strings.TrimSpace(tick.Symbol))
	if tick.Symbol == "" {
		return
	}
	s.mu.Lock()
	if previous, ok := s.latestBySymbol[tick.Symbol]; ok {
		if tick.Bid <= 0 {
			tick.Bid = previous.Bid
		}
		if tick.Ask <= 0 {
			tick.Ask = previous.Ask
		}
		if tick.Last <= 0 {
			tick.Last = previous.Last
		}
		if tick.Volume <= 0 {
			tick.Volume = previous.Volume
		}
		if tick.TimestampMS <= 0 {
			tick.TimestampMS = previous.TimestampMS
		}
	}
	if tick.Last <= 0 {
		s.mu.Unlock()
		return
	}
	if !isVNTradingTimestampMS(tick.TimestampMS) {
		s.mu.Unlock()
		return
	}
	if tick.Bid <= 0 {
		tick.Bid = tick.Last
	}
	if tick.Ask <= 0 {
		tick.Ask = tick.Last
	}
	s.latest = tick
	s.hasLatest = true
	s.latestBySymbol[tick.Symbol] = tick
	for ch := range s.subscribers {
		select {
		case ch <- tick:
		default:
		}
	}
	for ch := range s.subscribersBySymbol[tick.Symbol] {
		select {
		case ch <- tick:
		default:
		}
	}
	s.mu.Unlock()
}

func (s *Store) Latest(symbol string) (Tick, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return s.latest, s.hasLatest
	}
	tick, ok := s.latestBySymbol[symbol]
	return tick, ok
}

func (s *Store) LatestAny() (Tick, bool) {
	return s.Latest("")
}

func (s *Store) Subscribe() (<-chan Tick, func()) {
	ch := make(chan Tick, 32)
	s.mu.Lock()
	s.subscribers[ch] = struct{}{}
	if s.hasLatest {
		ch <- s.latest
	}
	s.mu.Unlock()
	return ch, func() {
		s.mu.Lock()
		if _, ok := s.subscribers[ch]; ok {
			delete(s.subscribers, ch)
			close(ch)
		}
		s.mu.Unlock()
	}
}

func (s *Store) SubscribeSymbol(symbol string) (<-chan Tick, func()) {
	ch := make(chan Tick, 32)
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	s.mu.Lock()
	if _, ok := s.subscribersBySymbol[symbol]; !ok {
		s.subscribersBySymbol[symbol] = make(map[chan Tick]struct{})
	}
	s.subscribersBySymbol[symbol][ch] = struct{}{}
	if tick, ok := s.latestBySymbol[symbol]; ok {
		ch <- tick
	}
	s.mu.Unlock()
	return ch, func() {
		s.mu.Lock()
		if subs, ok := s.subscribersBySymbol[symbol]; ok {
			if _, exists := subs[ch]; exists {
				delete(subs, ch)
				close(ch)
			}
			if len(subs) == 0 {
				delete(s.subscribersBySymbol, symbol)
			}
		}
		s.mu.Unlock()
	}
}

func (s *Store) Symbols() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.latestBySymbol))
	for symbol := range s.latestBySymbol {
		out = append(out, symbol)
	}
	return out
}

func (t Tick) JSONLine() []byte {
	raw, _ := json.Marshal(t)
	return append(raw, '\n')
}
