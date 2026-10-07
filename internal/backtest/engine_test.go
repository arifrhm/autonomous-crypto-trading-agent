package backtest_test

import (
	"context"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/backtest"
)

func generateSyntheticCandles(count int) []backtest.Candle {
	candles := make([]backtest.Candle, count)
	price := 50000.0
	now := time.Now().Add(-time.Duration(count) * time.Hour)

	r := rand.New(rand.NewSource(42)) // deterministic seed

	for i := 0; i < count; i++ {
		// Sine wave + random noise to guarantee periodic RSI oversold/overbought swings
		wave := math.Sin(float64(i)/10.0) * 800.0
		change := (r.Float64() - 0.5) * 500.0
		price += wave*0.2 + change
		if price < 1000 {
			price = 1000
		}

		candles[i] = backtest.Candle{
			Timestamp: now.Add(time.Duration(i) * time.Hour),
			Open:      price - 50,
			High:      price + 100,
			Low:       price - 100,
			Close:     price,
			Volume:    15.5,
		}
	}
	return candles
}

func TestBacktestEngine_Run(t *testing.T) {
	engine := backtest.NewEngine(10000.0, 0.001, 0.0005)
	candles := generateSyntheticCandles(500)

	report, trades, err := engine.Run(context.Background(), "BTCUSDT", candles)
	if err != nil {
		t.Fatalf("backtest failed: %v", err)
	}

	if report.InitialEquity != 10000.0 {
		t.Errorf("expected initial equity 10000, got %f", report.InitialEquity)
	}
	if report.TotalTrades == 0 {
		t.Errorf("expected trades to be executed, got 0")
	}
	if len(trades) != report.TotalTrades {
		t.Errorf("trades count mismatch: %d vs %d", len(trades), report.TotalTrades)
	}
}

// Benchmark 1 year of hourly candles (8,760 candles) in < 60s (target is typically < 100ms in Go)
func BenchmarkBacktestEngine_OneYear(b *testing.B) {
	engine := backtest.NewEngine(10000.0, 0.001, 0.0005)
	candles := generateSyntheticCandles(8760) // 1 full year 1H data

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := engine.Run(context.Background(), "BTCUSDT", candles)
		if err != nil {
			b.Fatalf("benchmark failed: %v", err)
		}
	}
}
