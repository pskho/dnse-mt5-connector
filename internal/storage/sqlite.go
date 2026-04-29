package storage

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(time.Hour)
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Migrate(ctx context.Context) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS orders (
			id TEXT PRIMARY KEY,
			client_order_id TEXT NOT NULL,
			account_no TEXT NOT NULL,
			symbol TEXT NOT NULL,
			side TEXT NOT NULL,
			dnse_side TEXT NOT NULL,
			quantity INTEGER NOT NULL,
			price REAL NOT NULL,
			order_type TEXT NOT NULL,
			status TEXT NOT NULL,
			raw_request TEXT NOT NULL,
			raw_response TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_orders_dedupe
			ON orders(account_no, symbol, side, quantity, price, created_at);`,
		`CREATE TABLE IF NOT EXISTS logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			level TEXT NOT NULL,
			event TEXT NOT NULL,
			details TEXT NOT NULL,
			created_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS trading_tokens (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			token TEXT NOT NULL,
			expires_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS history_syncs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			first_time INTEGER NOT NULL,
			last_time INTEGER NOT NULL,
			status TEXT NOT NULL,
			candles_synced INTEGER NOT NULL,
			created_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS history_candles (
			symbol TEXT NOT NULL,
			market_type TEXT NOT NULL,
			resolution INTEGER NOT NULL,
			time_ms INTEGER NOT NULL,
			open REAL NOT NULL,
			high REAL NOT NULL,
			low REAL NOT NULL,
			close REAL NOT NULL,
			tick_volume INTEGER NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY(symbol, market_type, resolution, time_ms)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_history_candles_lookup
			ON history_candles(symbol, market_type, resolution, time_ms);`,
	}
	for _, q := range queries {
		if _, err := s.db.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) InsertOrder(ctx context.Context, order Order) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO orders
		(id, client_order_id, account_no, symbol, side, dnse_side, quantity, price, order_type, status, raw_request, raw_response, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		order.ID, order.ClientOrderID, order.AccountNo, order.Symbol, order.Side, order.DNSESide,
		order.Quantity, order.Price, order.OrderType, order.Status, order.RawRequest, order.RawResponse,
		formatTime(order.CreatedAt), formatTime(order.UpdatedAt),
	)
	return err
}

func (s *SQLiteStore) GetOrder(ctx context.Context, id string) (Order, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, client_order_id, account_no, symbol, side, dnse_side,
		quantity, price, order_type, status, raw_request, raw_response, created_at, updated_at
		FROM orders WHERE id = ?`, id)

	var order Order
	var createdAt, updatedAt string
	err := row.Scan(&order.ID, &order.ClientOrderID, &order.AccountNo, &order.Symbol, &order.Side,
		&order.DNSESide, &order.Quantity, &order.Price, &order.OrderType, &order.Status,
		&order.RawRequest, &order.RawResponse, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Order{}, ErrNotFound
	}
	if err != nil {
		return Order{}, err
	}
	order.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	order.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return order, nil
}

func (s *SQLiteStore) GetOrderByClientOrderID(ctx context.Context, clientOrderID string) (Order, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, client_order_id, account_no, symbol, side, dnse_side,
		quantity, price, order_type, status, raw_request, raw_response, created_at, updated_at
		FROM orders WHERE client_order_id = ?`, clientOrderID)

	var order Order
	var createdAt, updatedAt string
	err := row.Scan(&order.ID, &order.ClientOrderID, &order.AccountNo, &order.Symbol, &order.Side,
		&order.DNSESide, &order.Quantity, &order.Price, &order.OrderType, &order.Status,
		&order.RawRequest, &order.RawResponse, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Order{}, ErrNotFound
	}
	if err != nil {
		return Order{}, err
	}
	order.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	order.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return order, nil
}

func (s *SQLiteStore) UpdateOrderStatus(ctx context.Context, id, status, rawResponse string, updatedAt time.Time) error {
	result, err := s.db.ExecContext(ctx, `UPDATE orders
		SET status = ?, raw_response = CASE WHEN ? = '' THEN raw_response ELSE ? END, updated_at = ?
		WHERE id = ?`,
		status, rawResponse, rawResponse, formatTime(updatedAt), id,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) HasDuplicateOrder(ctx context.Context, accountNo, symbol, side string, quantity int, price float64, since time.Time) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM orders
		WHERE account_no = ? AND symbol = ? AND side = ? AND quantity = ? AND price = ? AND created_at >= ?`,
		accountNo, symbol, side, quantity, price, formatTime(since),
	).Scan(&count)
	return count > 0, err
}

func (s *SQLiteStore) InsertLog(ctx context.Context, level, event, details string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO logs(level, event, details, created_at) VALUES (?, ?, ?, ?)`,
		level, event, details, formatTime(time.Now().UTC()))
	return err
}

func (s *SQLiteStore) SaveTradingToken(ctx context.Context, token string, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO trading_tokens(id, token, expires_at, updated_at)
		VALUES (1, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET token = excluded.token, expires_at = excluded.expires_at, updated_at = excluded.updated_at`,
		token, formatTime(expiresAt), formatTime(time.Now().UTC()))
	return err
}

func (s *SQLiteStore) GetTradingToken(ctx context.Context) (TradingToken, error) {
	row := s.db.QueryRowContext(ctx, `SELECT token, expires_at, updated_at FROM trading_tokens WHERE id = 1`)
	var token TradingToken
	var expiresAt, updatedAt string
	err := row.Scan(&token.Token, &expiresAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return TradingToken{}, ErrNotFound
	}
	if err != nil {
		return TradingToken{}, err
	}
	token.ExpiresAt, _ = time.Parse(time.RFC3339Nano, expiresAt)
	token.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return token, nil
}

func (s *SQLiteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteStore) LogHistorySync(ctx context.Context, firstTime, lastTime int64, status string, candlesSynced int) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO history_syncs(first_time, last_time, status, candles_synced, created_at) VALUES (?, ?, ?, ?, ?)`,
		firstTime, lastTime, status, candlesSynced, formatTime(time.Now().UTC()))
	return err
}

func (s *SQLiteStore) UpsertHistoryCandles(ctx context.Context, records []HistoryCandleRecord) error {
	if len(records) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO history_candles
		(symbol, market_type, resolution, time_ms, open, high, low, close, tick_volume, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(symbol, market_type, resolution, time_ms) DO UPDATE SET
			open = excluded.open,
			high = excluded.high,
			low = excluded.low,
			close = excluded.close,
			tick_volume = excluded.tick_volume,
			updated_at = excluded.updated_at`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()
	now := formatTime(time.Now().UTC())
	for _, record := range records {
		if _, err := stmt.ExecContext(ctx,
			record.Symbol,
			record.MarketType,
			record.Resolution,
			record.TimeMS,
			record.Open,
			record.High,
			record.Low,
			record.Close,
			record.TickVolume,
			defaultStringTime(record.UpdatedAt, now),
		); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) LoadHistoryCandles(ctx context.Context, symbol, marketType string, resolution int, fromMS, toMS int64) ([]HistoryCandleRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT symbol, market_type, resolution, time_ms, open, high, low, close, tick_volume, updated_at
		FROM history_candles
		WHERE symbol = ? AND market_type = ? AND resolution = ? AND time_ms BETWEEN ? AND ?
		ORDER BY time_ms ASC`,
		symbol, marketType, resolution, fromMS, toMS)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []HistoryCandleRecord
	for rows.Next() {
		var record HistoryCandleRecord
		var updatedAt string
		if err := rows.Scan(&record.Symbol, &record.MarketType, &record.Resolution, &record.TimeMS, &record.Open, &record.High, &record.Low, &record.Close, &record.TickVolume, &updatedAt); err != nil {
			return nil, err
		}
		record.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
		out = append(out, record)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) GetHistoryCoverage(ctx context.Context, symbol, marketType string, resolution int) (minMS, maxMS int64, count int, err error) {
	row := s.db.QueryRowContext(ctx, `SELECT COALESCE(MIN(time_ms), 0), COALESCE(MAX(time_ms), 0), COUNT(1)
		FROM history_candles
		WHERE symbol = ? AND market_type = ? AND resolution = ?`,
		symbol, marketType, resolution)
	err = row.Scan(&minMS, &maxMS, &count)
	return
}

var ErrNotFound = errors.New("not found")

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func defaultStringTime(t time.Time, fallback string) string {
	if t.IsZero() {
		return fallback
	}
	return formatTime(t)
}
