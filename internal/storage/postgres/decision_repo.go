package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/domain"
)

type pgxDecisionRepo struct {
	pool *pgxpool.Pool
}

// NewDecisionRepository creates a new DecisionRepository backed by pgxpool.
func NewDecisionRepository(pool *pgxpool.Pool) DecisionRepository {
	return &pgxDecisionRepo{pool: pool}
}

func (r *pgxDecisionRepo) Insert(ctx context.Context, decision domain.AgentDecision) error {
	query := `
		INSERT INTO agent_decisions (id, symbol, action, confidence, reasoning, signals_used)
		VALUES (COALESCE(NULLIF($1, '')::uuid, gen_random_uuid()), $2, $3, $4, $5, $6)
	`
	signalsJSON := decision.SignalsUsed
	if len(signalsJSON) == 0 {
		signalsJSON = []byte("{}")
	}

	_, err := r.pool.Exec(ctx, query,
		decision.ID,
		decision.Symbol,
		decision.Action,
		decision.Confidence,
		decision.Reasoning,
		signalsJSON,
	)
	if err != nil {
		return fmt.Errorf("insert agent decision: %w", err)
	}
	return nil
}

func (r *pgxDecisionRepo) GetLatest(ctx context.Context, symbol string, limit int) ([]domain.AgentDecision, error) {
	query := `
		SELECT id, symbol, action, confidence, reasoning, signals_used, created_at
		FROM agent_decisions
		WHERE symbol = $1
		ORDER BY created_at DESC
		LIMIT $2
	`
	rows, err := r.pool.Query(ctx, query, symbol, limit)
	if err != nil {
		return nil, fmt.Errorf("query latest agent decisions: %w", err)
	}
	defer rows.Close()

	var results []domain.AgentDecision
	for rows.Next() {
		var d domain.AgentDecision
		if err := rows.Scan(
			&d.ID, &d.Symbol, &d.Action, &d.Confidence,
			&d.Reasoning, &d.SignalsUsed, &d.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan agent decision: %w", err)
		}
		results = append(results, d)
	}
	return results, rows.Err()
}
