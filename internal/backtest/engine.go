package backtest

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/agent"
	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/domain"
	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/exchange"
)

// Candle represents an OHLCV historical candlestick.
type Candle struct {
	Timestamp time.Time `json:"timestamp"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    float64   `json:"volume"`
}

// BacktestReport summarizes quantitative performance metrics.
type BacktestReport struct {
	Symbol        string        `json:"symbol"`
	InitialEquity float64       `json:"initial_equity"`
	FinalEquity   float64       `json:"final_equity"`
	TotalReturnPct float64      `json:"total_return_pct"`
	TotalTrades   int           `json:"total_trades"`
	WinningTrades int           `json:"winning_trades"`
	LosingTrades  int           `json:"losing_trades"`
	WinRatePct    float64       `json:"win_rate_pct"`
	ProfitFactor  float64       `json:"profit_factor"`
	MaxDrawdownPct float64      `json:"max_drawdown_pct"`
	SharpeRatio   float64       `json:"sharpe_ratio"`
	Duration      time.Duration `json:"backtest_duration"`
}

// Engine replays historical candlestick data and evaluates strategy performance.
type Engine struct {
	initialCash  float64
	feeRate      float64
	slippageRate float64
}

func NewEngine(initialCash float64, feeRate, slippageRate float64) *Engine {
	if initialCash <= 0 {
		initialCash = 10000.0
	}
	return &Engine{
		initialCash:  initialCash,
		feeRate:      feeRate,
		slippageRate: slippageRate,
	}
}

// Run executes the backtest over the provided historical candles using rule-based/agentic logic.
func (e *Engine) Run(ctx context.Context, symbol string, candles []Candle) (*BacktestReport, []domain.ExecutedTrade, error) {
	startTime := time.Now()
	paper := exchange.NewPaperEngine(e.initialCash, e.feeRate, e.slippageRate)

	if len(candles) < 30 {
		return nil, nil, fmt.Errorf("insufficient candle data (need >= 30, got %d)", len(candles))
	}

	prices := make([]float64, 0, len(candles))
	peakEquity := e.initialCash
	maxDrawdownPct := 0.0
	var returns []float64

	prevEquity := e.initialCash

	// Replay loop
	for i, c := range candles {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		default:
		}

		prices = append(prices, c.Close)
		if i < 26 {
			continue // Warmup indicators
		}

		indicators := agent.ComputeIndicators(prices)
		bal := paper.GetBalance()
		pos := bal.Positions[symbol]

		// Quantitative Signal Strategy:
		// BUY: RSI < 45 (pullback) and cash available
		// SELL: RSI > 60 or 5% gain and holding position
		if indicators.RSI < 45 && bal.CashUSDT > 1000 {
			qty := 1000.0 / c.Close // $1,000 fixed allocation
			_, _ = paper.ExecuteMarketOrder(ctx, exchange.OrderRequest{
				Symbol:   symbol,
				Side:     domain.SideBuy,
				Type:     exchange.OrderTypeMarket,
				Quantity: qty,
				Reason:   fmt.Sprintf("RSI Oversold Dip (%.1f)", indicators.RSI),
			}, c.Close)

		} else if indicators.RSI > 60 && pos.Quantity > 0 {
			_, _ = paper.ExecuteMarketOrder(ctx, exchange.OrderRequest{
				Symbol:   symbol,
				Side:     domain.SideSell,
				Type:     exchange.OrderTypeMarket,
				Quantity: pos.Quantity,
				Reason:   fmt.Sprintf("RSI Exit (%.1f)", indicators.RSI),
			}, c.Close)
		}

		// Track Equity and Drawdown
		currentEquity := paper.GetTotalEquity(map[string]float64{symbol: c.Close})
		if currentEquity > peakEquity {
			peakEquity = currentEquity
		}
		dd := (peakEquity - currentEquity) / peakEquity * 100.0
		if dd > maxDrawdownPct {
			maxDrawdownPct = dd
		}

		// Daily return estimate
		dailyRet := (currentEquity - prevEquity) / prevEquity
		returns = append(returns, dailyRet)
		prevEquity = currentEquity
	}

	lastPrice := candles[len(candles)-1].Close
	finalEquity := paper.GetTotalEquity(map[string]float64{symbol: lastPrice})
	trades := paper.GetTradeHistory()

	report := calculateReport(symbol, e.initialCash, finalEquity, trades, maxDrawdownPct, returns, time.Since(startTime))
	return report, trades, nil
}

func calculateReport(
	symbol string,
	initialEquity, finalEquity float64,
	trades []domain.ExecutedTrade,
	maxDD float64,
	returns []float64,
	duration time.Duration,
) *BacktestReport {
	wins := 0
	losses := 0
	grossProfit := 0.0
	grossLoss := 0.0

	for _, t := range trades {
		if t.Side == domain.SideSell {
			if t.PnL > 0 {
				wins++
				grossProfit += t.PnL
			} else if t.PnL < 0 {
				losses++
				grossLoss += math.Abs(t.PnL)
			}
		}
	}

	winRate := 0.0
	closedTrades := wins + losses
	if closedTrades > 0 {
		winRate = (float64(wins) / float64(closedTrades)) * 100.0
	}

	profitFactor := 1.0
	if grossLoss > 0 {
		profitFactor = grossProfit / grossLoss
	} else if grossProfit > 0 {
		profitFactor = 99.0
	}

	sharpe := calculateSharpeRatio(returns)

	return &BacktestReport{
		Symbol:         symbol,
		InitialEquity:  initialEquity,
		FinalEquity:    finalEquity,
		TotalReturnPct: ((finalEquity - initialEquity) / initialEquity) * 100.0,
		TotalTrades:    len(trades),
		WinningTrades:  wins,
		LosingTrades:   losses,
		WinRatePct:     winRate,
		ProfitFactor:   profitFactor,
		MaxDrawdownPct: maxDD,
		SharpeRatio:    sharpe,
		Duration:       duration,
	}
}

func calculateSharpeRatio(returns []float64) float64 {
	if len(returns) < 2 {
		return 0.0
	}
	var sum float64
	for _, r := range returns {
		sum += r
	}
	mean := sum / float64(len(returns))

	var varianceSum float64
	for _, r := range returns {
		diff := r - mean
		varianceSum += diff * diff
	}
	stdDev := math.Sqrt(varianceSum / float64(len(returns)-1))
	if stdDev == 0 {
		return 0.0
	}

	// Annualized Sharpe (assuming 365 trading days for crypto)
	return (mean / stdDev) * math.Sqrt(365)
}

// ExportReportJSON writes report metrics to JSON file.
func ExportReportJSON(report *BacktestReport, filePath string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644)
}

// ExportTradesCSV writes executed trade history to CSV.
func ExportTradesCSV(trades []domain.ExecutedTrade, filePath string) error {
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	_ = w.Write([]string{"trade_id", "symbol", "side", "price", "quantity", "fee", "pnl", "executed_at", "reason"})
	for _, t := range trades {
		_ = w.Write([]string{
			t.ID,
			t.Symbol,
			string(t.Side),
			fmt.Sprintf("%.2f", t.Price),
			fmt.Sprintf("%.4f", t.Quantity),
			fmt.Sprintf("%.4f", t.Fee),
			fmt.Sprintf("%.2f", t.PnL),
			t.ExecutedAt.Format(time.RFC3339),
			t.Reason,
		})
	}
	return nil
}
