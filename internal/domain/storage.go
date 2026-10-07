package domain

import (
	"encoding/json"
	"time"
)

// PriceTick represents recorded market tick in the database.
type PriceTick struct {
	ID           int64     `json:"id"`
	Symbol       string    `json:"symbol"`
	Price        float64   `json:"price"`
	Volume       float64   `json:"volume"`
	TradeID      int64     `json:"trade_id"`
	IsBuyerMaker bool      `json:"is_buyer_maker"`
	Timestamp    time.Time `json:"timestamp"`
	CreatedAt    time.Time `json:"created_at"`
}

// Side represents order action side (BUY / SELL).
type Side string

const (
	SideBuy  Side = "BUY"
	SideSell Side = "SELL"
)

// ExecutedTrade represents paper/live trade recorded.
type ExecutedTrade struct {
	ID              string    `json:"id"`
	Symbol          string    `json:"symbol"`
	Side            Side      `json:"side"`
	Price           float64   `json:"price"`
	Quantity        float64   `json:"quantity"`
	Fee             float64   `json:"fee"`
	Slippage        float64   `json:"slippage"`
	Reason          string    `json:"reason"`
	AgentConfidence float64   `json:"agent_confidence"`
	PnL             float64   `json:"pnl"`
	ExecutedAt      time.Time `json:"executed_at"`
	CreatedAt       time.Time `json:"created_at"`
}

// Action represents agent decision action (BUY, SELL, HOLD).
type Action string

const (
	ActionBuy  Action = "BUY"
	ActionSell Action = "SELL"
	ActionHold Action = "HOLD"
)

// AgentDecision represents audit log of decisions made by the agent.
type AgentDecision struct {
	ID          string          `json:"id"`
	Symbol      string          `json:"symbol"`
	Action      Action          `json:"action"`
	Confidence  float64         `json:"confidence"`
	Reasoning   string          `json:"reasoning"`
	SignalsUsed json.RawMessage `json:"signals_used"`
	CreatedAt   time.Time       `json:"created_at"`
}
