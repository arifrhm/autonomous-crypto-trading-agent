package domain

import (
	"time"
)

// Observation captures the multi-modal state of the market and internal account at a specific point in time.
type Observation struct {
	Timestamp     time.Time            `json:"timestamp"`
	Symbol        string               `json:"symbol"`
	CurrentPrice  float64              `json:"current_price"`
	PriceHistory  []PriceTick          `json:"price_history"`
	Indicators    TechnicalIndicators  `json:"indicators"`
	Sentiment     SentimentState       `json:"sentiment"`
	Position      Position             `json:"position"`
	RecentTrades  []ExecutedTrade      `json:"recent_trades"`
}

// TechnicalIndicators encapsulates in-house computed momentum & trend indicators.
type TechnicalIndicators struct {
	RSI        float64 `json:"rsi"`        // Relative Strength Index (e.g. 14 period)
	EMA9       float64 `json:"ema_9"`      // 9-period Exponential Moving Average
	EMA21      float64 `json:"ema_21"`     // 21-period Exponential Moving Average
	MACD       float64 `json:"macd"`       // MACD line (EMA12 - EMA26)
	SignalLine float64 `json:"signal_line"`// Signal line (9-period EMA of MACD)
	Histogram  float64 `json:"histogram"`  // MACD - Signal
}

// SentimentState encapsulates news/market sentiment parsed by the LLM or CryptoPanic.
type SentimentState struct {
	Score        float64   `json:"score"`         // Normalized -1.0 (extremely bearish) to +1.0 (extremely bullish)
	Label        string    `json:"label"`         // "BEARISH", "NEUTRAL", "BULLISH"
	Headlines    []string  `json:"headlines"`     // Top recent headlines influencing the score
	LastUpdated  time.Time `json:"last_updated"`
}

// Position represents current simulated asset exposure and unrealized P&L.
type Position struct {
	Symbol        string    `json:"symbol"`
	Quantity      float64   `json:"quantity"`       // Base asset held (e.g. BTC)
	AveragePrice  float64   `json:"average_price"`  // Volume-weighted entry price
	CurrentPrice  float64   `json:"current_price"`  // Mark price for unrealized PnL
	UnrealizedPnL float64   `json:"unrealized_pnl"` // (CurrentPrice - AveragePrice) * Quantity
	CashBalance   float64   `json:"cash_balance"`   // Available quote balance (USDT)
	UpdatedAt     time.Time `json:"updated_at"`
}

// RiskAssessment evaluates risks before executing an order.
type RiskAssessment struct {
	StopLossPrice   float64 `json:"stop_loss_price"`
	TakeProfitPrice float64 `json:"take_profit_price"`
	MaxLossUSD      float64 `json:"max_loss_usd"`
	RiskRewardRatio float64 `json:"risk_reward_ratio"`
	WithinLimits    bool    `json:"within_limits"`
}

// Decision represents the structured trading action decided by the LLM brain after reasoning.
type Decision struct {
	ID             string          `json:"id"`
	Symbol         string          `json:"symbol"`
	Action         Action          `json:"action"`          // BUY, SELL, HOLD
	Quantity       float64         `json:"quantity"`        // Target quantity to trade
	Confidence     float64         `json:"confidence"`      // Confidence score between 0.0 and 1.0
	Reasoning      string          `json:"reasoning"`       // Explanation behind the decision
	RiskAssessment RiskAssessment  `json:"risk_assessment"` // Guardrail analysis
	SignalsUsed    map[string]any  `json:"signals_used"`    // Audit trail snapshot of input metrics
	CreatedAt      time.Time       `json:"created_at"`
}

// ActionResult captures the physical outcome of executing a Decision via the trading engine.
type ActionResult struct {
	DecisionID    string    `json:"decision_id"`
	TradeID       string    `json:"trade_id,omitempty"`
	Symbol        string    `json:"symbol"`
	Action        Action    `json:"action"`
	ExecutedPrice float64   `json:"executed_price"`
	ExecutedQty   float64   `json:"executed_qty"`
	Fee           float64   `json:"fee"`
	Slippage      float64   `json:"slippage"`
	RealizedPnL   float64   `json:"realized_pnl"`
	ExecutedAt    time.Time `json:"executed_at"`
	Success       bool      `json:"success"`
	ErrorMessage  string    `json:"error_message,omitempty"`
}

// ReflectionAnalysis captures periodic retrospective evaluation of agent performance.
type ReflectionAnalysis struct {
	Timestamp          time.Time `json:"timestamp"`
	TotalTrades        int       `json:"total_trades"`
	WinRate            float64   `json:"win_rate"`
	TotalPnL           float64   `json:"total_pnl"`
	Summary            string    `json:"summary"`
	StrategyAdjustments string   `json:"strategy_adjustments"`
}
