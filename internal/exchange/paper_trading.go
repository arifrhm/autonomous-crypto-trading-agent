package exchange

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/domain"
)

// OrderType represents execution model (MARKET or LIMIT).
type OrderType string

const (
	OrderTypeMarket OrderType = "MARKET"
	OrderTypeLimit  OrderType = "LIMIT"
)

// OrderRequest parameters for placing simulated trade orders.
type OrderRequest struct {
	Symbol   string           `json:"symbol"`
	Side     domain.Side      `json:"side"` // BUY or SELL
	Type     OrderType        `json:"type"`
	Price    float64          `json:"price"` // ignored for market order
	Quantity float64          `json:"quantity"`
	Reason   string           `json:"reason"`
}

// Balance tracks cash and current positions.
type AccountBalance struct {
	CashUSDT  float64 `json:"cash_usdt"`
	Positions map[string]domain.Position `json:"positions"`
}

// PaperEngine simulates a realistic crypto exchange with fee and slippage.
type PaperEngine struct {
	mu           sync.RWMutex
	cashUSDT     float64
	positions    map[string]domain.Position
	feeRate      float64 // e.g. 0.001 (0.1%)
	slippageRate float64 // e.g. 0.0005 (0.05%)
	tradeHistory []domain.ExecutedTrade
}

func NewPaperEngine(initialCash float64, feeRate, slippageRate float64) *PaperEngine {
	if initialCash <= 0 {
		initialCash = 10000.0
	}
	if feeRate <= 0 {
		feeRate = 0.001 // 0.1% Binance standard taker fee
	}
	if slippageRate <= 0 {
		slippageRate = 0.0005 // 0.05% slippage simulation
	}

	return &PaperEngine{
		cashUSDT:     initialCash,
		positions:    make(map[string]domain.Position),
		feeRate:      feeRate,
		slippageRate: slippageRate,
		tradeHistory: make([]domain.ExecutedTrade, 0),
	}
}

// ExecuteMarketOrder executes an instantaneous market order against the current market price with slippage.
func (e *PaperEngine) ExecuteMarketOrder(ctx context.Context, req OrderRequest, currentMarketPrice float64) (*domain.ExecutedTrade, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if req.Quantity <= 0 {
		return nil, fmt.Errorf("order quantity must be positive")
	}
	if currentMarketPrice <= 0 {
		return nil, fmt.Errorf("current market price must be positive")
	}

	// 1. Calculate realistic execution price with slippage
	// For BUY: price slips higher. For SELL: price slips lower.
	slippage := currentMarketPrice * e.slippageRate
	executedPrice := currentMarketPrice
	if req.Side == domain.SideBuy {
		executedPrice += slippage
	} else {
		executedPrice -= slippage
	}

	grossValue := executedPrice * req.Quantity
	fee := grossValue * e.feeRate

	pos := e.positions[req.Symbol]
	pos.Symbol = req.Symbol
	var realizedPnL float64

	if req.Side == domain.SideBuy {
		totalCost := grossValue + fee
		if totalCost > e.cashUSDT {
			return nil, fmt.Errorf("insufficient funds: required %.2f USDT, available %.2f USDT", totalCost, e.cashUSDT)
		}

		newQty := pos.Quantity + req.Quantity
		pos.AveragePrice = ((pos.Quantity * pos.AveragePrice) + grossValue) / newQty
		pos.Quantity = newQty
		e.cashUSDT -= totalCost

	} else if req.Side == domain.SideSell {
		if req.Quantity > pos.Quantity {
			return nil, fmt.Errorf("insufficient position: selling %.4f %s, holding %.4f", req.Quantity, req.Symbol, pos.Quantity)
		}

		netProceeds := grossValue - fee
		realizedPnL = (executedPrice - pos.AveragePrice) * req.Quantity - fee

		pos.Quantity -= req.Quantity
		e.cashUSDT += netProceeds
		if pos.Quantity <= 0 {
			pos.Quantity = 0
			pos.AveragePrice = 0
		}
	}

	pos.CurrentPrice = currentMarketPrice
	if pos.Quantity > 0 {
		pos.UnrealizedPnL = (currentMarketPrice - pos.AveragePrice) * pos.Quantity
	} else {
		pos.UnrealizedPnL = 0
	}
	pos.CashBalance = e.cashUSDT
	pos.UpdatedAt = time.Now().UTC()
	e.positions[req.Symbol] = pos

	tradeID, _ := uuid.NewV7()
	trade := domain.ExecutedTrade{
		ID:              tradeID.String(),
		Symbol:          req.Symbol,
		Side:            req.Side,
		Price:           executedPrice,
		Quantity:        req.Quantity,
		Fee:             fee,
		Slippage:        slippage,
		Reason:          req.Reason,
		AgentConfidence: 1.0,
		PnL:             realizedPnL,
		ExecutedAt:      time.Now().UTC(),
		CreatedAt:       time.Now().UTC(),
	}

	e.tradeHistory = append(e.tradeHistory, trade)
	return &trade, nil
}

// GetBalance returns snapshot of available cash and current positions.
func (e *PaperEngine) GetBalance() AccountBalance {
	e.mu.RLock()
	defer e.mu.RUnlock()

	posCopy := make(map[string]domain.Position, len(e.positions))
	for k, v := range e.positions {
		posCopy[k] = v
	}

	return AccountBalance{
		CashUSDT:  e.cashUSDT,
		Positions: posCopy,
	}
}

// GetTotalEquity calculates cash + total unrealized value of open positions.
func (e *PaperEngine) GetTotalEquity(marketPrices map[string]float64) float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()

	equity := e.cashUSDT
	for sym, pos := range e.positions {
		if pos.Quantity > 0 {
			price := marketPrices[sym]
			if price == 0 {
				price = pos.CurrentPrice
			}
			equity += pos.Quantity * price
		}
	}
	return equity
}

// GetTradeHistory returns all executed paper trades.
func (e *PaperEngine) GetTradeHistory() []domain.ExecutedTrade {
	e.mu.RLock()
	defer e.mu.RUnlock()

	res := make([]domain.ExecutedTrade, len(e.tradeHistory))
	copy(res, e.tradeHistory)
	return res
}

// Helper rounding
func round(val float64, precision int) float64 {
	ratio := math.Pow(10, float64(precision))
	return math.Round(val*ratio) / ratio
}
