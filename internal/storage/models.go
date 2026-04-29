package storage

import "time"

type Order struct {
	ID            string
	ClientOrderID string
	AccountNo     string
	Symbol        string
	Side          string
	DNSESide      string
	Quantity      int
	Price         float64
	OrderType     string
	Status        string
	RawRequest    string
	RawResponse   string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type LogRecord struct {
	ID        int64
	Level     string
	Event     string
	Details   string
	CreatedAt time.Time
}

type TradingToken struct {
	Token     string
	ExpiresAt time.Time
	UpdatedAt time.Time
}

type HistoryCandleRecord struct {
	Symbol     string
	MarketType string
	Resolution int
	TimeMS     int64
	Open       float64
	High       float64
	Low        float64
	Close      float64
	TickVolume int64
	UpdatedAt  time.Time
}
