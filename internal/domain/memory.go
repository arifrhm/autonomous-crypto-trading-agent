package domain

import (
	"encoding/json"
	"time"
)

// MarketMemory represents a vector-indexed episodic memory (News, Strategy Reflection, Regime Change).
type MarketMemory struct {
	ID         string          `json:"id"`
	Symbol     string          `json:"symbol"`
	Category   string          `json:"category"` // 'NEWS', 'TRADE_REFLECTION', 'REGIME_CHANGE'
	Content    string          `json:"content"`
	Metadata   json.RawMessage `json:"metadata"`
	Embedding  []float32       `json:"embedding,omitempty"`
	Similarity float64         `json:"similarity,omitempty"` // populated on similarity queries
	CreatedAt  time.Time       `json:"created_at"`
}
