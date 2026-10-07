package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/domain"
)

// PriceTickRepository defines the storage interface for tick data.
type PriceTickRepository interface {
	Insert(ctx context.Context, tick domain.PriceTick) error
	InsertBatch(ctx context.Context, ticks []domain.PriceTick) error
	GetRange(ctx context.Context, symbol string, from, to time.Time) ([]domain.PriceTick, error)
	GetLatest(ctx context.Context, symbol string, limit int) ([]domain.PriceTick, error)
}

// TradeRepository defines the storage interface for executed trades.
type TradeRepository interface {
	Insert(ctx context.Context, trade domain.ExecutedTrade) error
	GetRecent(ctx context.Context, symbol string, limit int) ([]domain.ExecutedTrade, error)
	GetByDateRange(ctx context.Context, symbol string, from, to time.Time) ([]domain.ExecutedTrade, error)
}

// DecisionRepository defines the audit trail interface for agent decisions.
type DecisionRepository interface {
	Insert(ctx context.Context, decision domain.AgentDecision) error
	GetLatest(ctx context.Context, symbol string, limit int) ([]domain.AgentDecision, error)
}

type pgxPriceTickRepo struct {
	pool *pgxpool.Pool
}

// NewPriceTickRepository creates a new PriceTickRepository backed by pgxpool.
func NewPriceTickRepository(pool *pgxpool.Pool) PriceTickRepository {
	return &pgxPriceTickRepo{pool: pool}
}

func (r *pgxPriceTickRepo) Insert(ctx context.Context, tick domain.PriceTick) error {
	query := `
		INSERT INTO price_ticks (symbol, price, volume, trade_id, is_buyer_maker, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.pool.Exec(ctx, query,
		tick.Symbol,
		tick.Price,
		tick.Volume,
		tick.TradeID,
		tick.IsBuyerMaker,
		tick.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("insert price tick: %w", err)
	}
	return nil
}

// InsertBatch uses PostgreSQL COPY protocol (CopyFrom) for maximum throughput.
func (r *pgxPriceTickRepo) InsertBatch(ctx context.Context, ticks []domain.PriceTick) error {
	if len(ticks) == 0 {
		return nil
	}

	rows := make([][]any, len(ticks))
	for i, t := range ticks {
		rows[i] = []any{t.Symbol, t.Price, t.Volume, t.TradeID, t.IsBuyerMaker, t.Timestamp}
	}

	copyCount, err := r.pool.CopyFrom(
		ctx,
		[]string{"price_ticks"},
		[]string{"symbol", "price", "volume", "trade_id", "is_buyer_maker", "timestamp"},
		&sliceCopySource{rows: rows},
	)
	if err != nil {
		return fmt.Errorf("batch insert price ticks via CopyFrom: %w", err)
	}
	if int(copyCount) != len(ticks) {
		return fmt.Errorf("expected to copy %d rows, but copied %d", len(ticks), copyCount)
	}
	return nil
}

func (r *pgxPriceTickRepo) GetRange(ctx context.Context, symbol string, from, to time.Time) ([]domain.PriceTick, error) {
	query := `
		SELECT id, symbol, price, volume, trade_id, is_buyer_maker, timestamp, created_at
		FROM price_ticks
		WHERE symbol = $1 AND timestamp >= $2 AND timestamp <= $3
		ORDER BY timestamp ASC
	`
	rows, err := r.pool.Query(ctx, query, symbol, from, to)
	if err != nil {
		return nil, fmt.Errorf("query price ticks range: %w", err)
	}
	defer rows.Close()

	var result []domain.PriceTick
	for rows.Next() {
		var t domain.PriceTick
		if err := rows.Scan(&t.ID, &t.Symbol, &t.Price, &t.Volume, &t.TradeID, &t.IsBuyerMaker, &t.Timestamp, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan price tick: %w", err)
		}
		result = append(result, t)
	}
	return result, rows.Err()
}

func (r *pgxPriceTickRepo) GetLatest(ctx context.Context, symbol string, limit int) ([]domain.PriceTick, error) {
	query := `
		SELECT id, symbol, price, volume, trade_id, is_buyer_maker, timestamp, created_at
		FROM price_ticks
		WHERE symbol = $1
		ORDER BY timestamp DESC
		LIMIT $2
	`
	rows, err := r.pool.Query(ctx, query, symbol, limit)
	if err != nil {
		return nil, fmt.Errorf("query latest price ticks: %w", err)
	}
	defer rows.Close()

	var result []domain.PriceTick
	for rows.Next() {
		var t domain.PriceTick
		if err := rows.Scan(&t.ID, &t.Symbol, &t.Price, &t.Volume, &t.TradeID, &t.IsBuyerMaker, &t.Timestamp, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan latest price tick: %w", err)
		}
		result = append(result, t)
	}
	return result, rows.Err()
}

// sliceCopySource implements pgx.CopyFromSource interface
type sliceCopySource struct {
	rows [][]any
	idx  int
}

func (s *sliceCopySource) Next() bool {
	s.idx++
	return s.idx <= len(s.rows)
}

func (s *sliceCopySource) Values() ([]any, error) {
	return s.rows[s.idx-1], nil
}

func (s *sliceCopySource) Err() error {
	return nil
}
