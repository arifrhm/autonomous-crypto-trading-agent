package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/domain"
)

// MemoryRepository defines methods for storing and performing vector similarity search.
type MemoryRepository interface {
	Insert(ctx context.Context, memory domain.MarketMemory) error
	SearchSimilar(ctx context.Context, symbol string, category string, queryEmbedding []float32, limit int) ([]domain.MarketMemory, error)
}

type pgxMemoryRepo struct {
	pool *pgxpool.Pool
}

// NewMemoryRepository creates a new pgvector-backed MemoryRepository.
func NewMemoryRepository(pool *pgxpool.Pool) MemoryRepository {
	return &pgxMemoryRepo{pool: pool}
}

// formatVector converts []float32 to PostgreSQL vector literal string: '[0.1,0.2,...]'
func formatVector(v []float32) string {
	var sb strings.Builder
	sb.WriteByte('[')
	for i, val := range v {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(fmt.Sprintf("%f", val))
	}
	sb.WriteByte(']')
	return sb.String()
}

func (r *pgxMemoryRepo) Insert(ctx context.Context, memory domain.MarketMemory) error {
	query := `
		INSERT INTO market_memories (id, symbol, category, content, metadata, embedding)
		VALUES (COALESCE(NULLIF($1, '')::uuid, gen_random_uuid()), $2, $3, $4, $5, $6::vector)
	`
	metadataJSON := memory.Metadata
	if len(metadataJSON) == 0 {
		metadataJSON = []byte("{}")
	}

	vecStr := formatVector(memory.Embedding)
	_, err := r.pool.Exec(ctx, query,
		memory.ID,
		memory.Symbol,
		memory.Category,
		memory.Content,
		metadataJSON,
		vecStr,
	)
	if err != nil {
		return fmt.Errorf("insert market memory with embedding: %w", err)
	}
	return nil
}

// SearchSimilar executes an Approximate Nearest Neighbor (ANN) search via Cosine distance (<=>)
func (r *pgxMemoryRepo) SearchSimilar(ctx context.Context, symbol string, category string, queryEmbedding []float32, limit int) ([]domain.MarketMemory, error) {
	if len(queryEmbedding) == 0 {
		return nil, fmt.Errorf("query embedding vector cannot be empty")
	}

	query := `
		SELECT id, symbol, category, content, metadata, 
		       1 - (embedding <=> $1::vector) AS similarity, created_at
		FROM market_memories
		WHERE ($2 = '' OR symbol = $2)
		  AND ($3 = '' OR category = $3)
		ORDER BY embedding <=> $1::vector ASC
		LIMIT $4
	`
	vecStr := formatVector(queryEmbedding)

	rows, err := r.pool.Query(ctx, query, vecStr, symbol, category, limit)
	if err != nil {
		return nil, fmt.Errorf("search similar memories: %w", err)
	}
	defer rows.Close()

	var results []domain.MarketMemory
	for rows.Next() {
		var m domain.MarketMemory
		if err := rows.Scan(
			&m.ID, &m.Symbol, &m.Category, &m.Content,
			&m.Metadata, &m.Similarity, &m.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan similar memory: %w", err)
		}
		results = append(results, m)
	}
	return results, rows.Err()
}
