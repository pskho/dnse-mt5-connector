package marketdata

import (
	"sync"
	"time"
)

type BridgeStatusSnapshot struct {
	PublisherStarted         bool      `json:"publisherStarted"`
	ActiveClients            int       `json:"activeClients"`
	LastClientConnectedAt    time.Time `json:"lastClientConnectedAt,omitempty"`
	LastClientDisconnectedAt time.Time `json:"lastClientDisconnectedAt,omitempty"`
}

type BridgeStatus struct {
	mu                       sync.RWMutex
	publisherStarted         bool
	activeClients            int
	lastClientConnectedAt    time.Time
	lastClientDisconnectedAt time.Time
}

func NewBridgeStatus() *BridgeStatus {
	return &BridgeStatus{}
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

func (s *BridgeStatus) Snapshot() BridgeStatusSnapshot {
	if s == nil {
		return BridgeStatusSnapshot{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return BridgeStatusSnapshot{
		PublisherStarted:         s.publisherStarted,
		ActiveClients:            s.activeClients,
		LastClientConnectedAt:    s.lastClientConnectedAt,
		LastClientDisconnectedAt: s.lastClientDisconnectedAt,
	}
}
