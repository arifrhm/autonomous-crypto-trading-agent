package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/domain"
)

type pgxTradeRepo struct {
	pool *pgxpool.Pool
}

// NewTradeRepository creates a new TradeRepository backed by pgxpool.
func NewTradeRepository(pool *pgxpool.Pool) TradeRepository {
	return &pgxTradeRepo{pool: pool}
}

func (r *pgxTradeRepo) Insert(ctx context.Context, trade domain.ExecutedTrade) error {
	query := `
		INSERT INTO trades (id, symbol, side, price, quantity, fee, slippage, reason, agent_confidence, pnl, executed_at)
		VALUES (COALESCE(NULLIF($1, '')::uuid, gen_random_uuid()), $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := r.pool.Exec(ctx, query,
		trade.ID,
		trade.Symbol,
		trade.Side,
		trade.Price,
		trade.Quantity,
		trade.Fee,
		trade.Slippage,
		trade.Reason,
		trade.AgentConfidence,
		trade.PnL,
		trade.ExecutedAt,
	)
	if err != nil {
		return fmt.Errorf("insert trade: %w", err)
	}
	return nil
}

func (r *pgxTradeRepo) GetRecent(ctx context.Context, symbol string, limit int) ([]domain.ExecutedTrade, error) {
	query := `
		SELECT id, symbol, side, price, quantity, fee, slippage, reason, agent_confidence, pnl, executed_at, created_at
		FROM trades
		WHERE symbol = $1
		ORDER BY executed_at DESC
		LIMIT $2
	`
	rows, err := r.pool.Query(ctx, query, symbol, limit)
	if err != nil {
		return nil, fmt.Errorf("query recent trades: %w", err)
	}
	defer rows.Close()

	var results []domain.ExecutedTrade
	for rows.Next() {
		var t domain.ExecutedTrade
		if err := rows.Scan(
			&t.ID, &t.Symbol, &t.Side, &t.Price, &t.Quantity,
			&t.Fee, &t.Slippage, &t.Reason, &t.AgentConfidence,
			&t.PnL, &t.ExecutedAt, &t.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan executed trade: %w", err)
		}
		results = append(results, t)
	}
	return results, rows.Err()
}

func (r *pgxTradeRepo) GetByDateRange(ctx context.Context, symbol string, from, to time.Time) ([]domain.ExecutedTrade, error) {
	query := `
		SELECT id, symbol, side, price, quantity, fee, slippage, reason, agent_confidence, pnl, executed_at, created_at
		FROM trades
		WHERE symbol = $1 AND executed_at >= $2 AND executed_at <= $3
		ORDER BY executed_at ASC
	`
	rows, err := r.pool.Query(ctx, query, symbol, from, to)
	if err != nil {
		return nil, fmt.Errorf("query trades by date range: %w", err)
	}
	defer rows.Close()

	var results []domain.ExecutedTrade
	for rows.Next() {
		var t domain.ExecutedTrade
		if err := rows.Scan(
			&t.ID, &t.Symbol, &t.Side, &t.Price, &t.Quantity,
			&t.Fee, &t.Slippage, &t.Reason, &t.AgentConfidence,
			&t.PnL, &t.ExecutedAt, &t.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan executed trade in range: %w", err)
		}
		results = append(results, t)
	}
	return results, rows.Err()
}
