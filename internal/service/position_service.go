package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"dnse-mt5-connector/internal/dnsemodel"
	"dnse-mt5-connector/internal/logger"
)

type PositionClient interface {
	GetPositions(ctx context.Context, accountNo, marketType string) ([]dnsemodel.Position, error)
}

type PositionService struct {
	dnse             PositionClient
	logger           *logger.FileLogger
	defaultAccountNo string
}

func NewPositionService(dnse PositionClient, appLog *logger.FileLogger, defaultAccountNo string) *PositionService {
	return &PositionService{
		dnse:             dnse,
		logger:           appLog,
		defaultAccountNo: strings.TrimSpace(defaultAccountNo),
	}
}

func (s *PositionService) GetCurrentPosition(ctx context.Context, symbol string) (Position, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return Position{}, errors.New("symbol is required")
	}
	positions, err := s.GetAllPositions(ctx)
	if err != nil {
		return Position{}, err
	}
	for _, position := range positions {
		if position.Symbol == symbol {
			return position, nil
		}
	}
	return Position{Symbol: symbol, Direction: "FLAT"}, nil
}

func (s *PositionService) GetAllPositions(ctx context.Context) ([]Position, error) {
	if s.defaultAccountNo == "" {
		return nil, errors.New("accountNo is required for position lookup")
	}
	rawPositions, err := s.dnse.GetPositions(ctx, s.defaultAccountNo, "DERIVATIVE")
	if err != nil {
		s.log("error", "position_fetch_failed", map[string]any{"error": err.Error()})
		return nil, err
	}

	bySymbol := make(map[string]Position)
	for _, raw := range rawPositions {
		symbol := strings.ToUpper(strings.TrimSpace(raw.Symbol))
		if symbol == "" {
			continue
		}
		position := bySymbol[symbol]
		position.Symbol = symbol

		side := strings.ToUpper(strings.TrimSpace(raw.Side))
		quantity := raw.Quantity
		switch side {
		case "LONG", "NB", "BUY":
			position.LongQuantity += quantity
		case "SHORT", "NS", "SELL":
			position.ShortQuantity += quantity
		default:
			if quantity >= 0 {
				position.LongQuantity += quantity
			} else {
				position.ShortQuantity += -quantity
			}
		}
		position.NetQuantity = position.LongQuantity - position.ShortQuantity
		position.Direction = direction(position.NetQuantity)
		bySymbol[symbol] = position
	}

	out := make([]Position, 0, len(bySymbol))
	for _, position := range bySymbol {
		out = append(out, position)
	}
	s.log("info", "positions_loaded", map[string]any{"count": len(out)})
	return out, nil
}

func direction(net int) string {
	switch {
	case net > 0:
		return "LONG"
	case net < 0:
		return "SHORT"
	default:
		return "FLAT"
	}
}

func (s *PositionService) log(level, event string, details map[string]any) {
	if s.logger == nil {
		return
	}
	if level == "error" {
		s.logger.Error(event, details)
		return
	}
	s.logger.Info(event, details)
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func signedExposureAfter(position Position, side string, quantity int) int {
	net := position.NetQuantity
	if strings.EqualFold(side, "BUY") {
		net += quantity
	} else {
		net -= quantity
	}
	return absInt(net)
}

func marshalPosition(position Position) string {
	raw, _ := json.Marshal(position)
	return string(raw)
}
