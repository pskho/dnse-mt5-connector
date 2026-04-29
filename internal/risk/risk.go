package risk

import (
	"context"
	"fmt"
	"time"

	"dnse-mt5-connector/internal/config"
)

type duplicateStore interface {
	HasDuplicateOrder(ctx context.Context, accountNo, symbol, side string, quantity int, price float64, since time.Time) (bool, error)
}

type Engine struct {
	cfg   config.RiskConfig
	store duplicateStore
}

func NewEngine(cfg config.RiskConfig, store duplicateStore) *Engine {
	if cfg.MaxQuantity <= 0 {
		cfg.MaxQuantity = 10
	}
	if cfg.DuplicateWindowSeconds <= 0 {
		cfg.DuplicateWindowSeconds = 3
	}
	return &Engine{cfg: cfg, store: store}
}

func (e *Engine) Check(ctx context.Context, accountNo, symbol, side string, quantity int, price float64) error {
	if quantity > e.cfg.MaxQuantity {
		return fmt.Errorf("quantity %d exceeds max quantity %d", quantity, e.cfg.MaxQuantity)
	}

	since := time.Now().UTC().Add(-time.Duration(e.cfg.DuplicateWindowSeconds) * time.Second)
	duplicate, err := e.store.HasDuplicateOrder(ctx, accountNo, symbol, side, quantity, price, since)
	if err != nil {
		return fmt.Errorf("duplicate check failed: %w", err)
	}
	if duplicate {
		return fmt.Errorf("duplicate order rejected within %d seconds", e.cfg.DuplicateWindowSeconds)
	}
	return nil
}
