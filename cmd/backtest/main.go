package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/backtest"
)

func main() {
	symbol := flag.String("symbol", "BTCUSDT", "Trading symbol pair")
	initialCash := flag.Float64("cash", 10000.0, "Initial starting cash in USDT")
	candlesCount := flag.Int("candles", 8760, "Number of 1-hour candles to simulate (8760 = 1 year)")
	outJSON := flag.String("out-json", "backtest_report.json", "JSON output report path")
	outCSV := flag.String("out-csv", "backtest_trades.csv", "CSV trades history path")
	flag.Parse()

	fmt.Printf("🚀 Starting Backtest for %s (%d candles, starting cash: $%.2f)...\n", *symbol, *candlesCount, *initialCash)

	// Generate synthetic test stream for demonstration
	candles := generateMarketCandles(*candlesCount)

	engine := backtest.NewEngine(*initialCash, 0.001, 0.0005)

	start := time.Now()
	report, trades, err := engine.Run(context.Background(), *symbol, candles)
	if err != nil {
		fmt.Printf("Error running backtest: %v\n", err)
		os.Exit(1)
	}

	elapsed := time.Since(start)

	fmt.Printf("\n====== 📊 BACKTEST RESULTS: %s ======\n", report.Symbol)
	fmt.Printf("Execution Speed  : %v for %d candles\n", elapsed, *candlesCount)
	fmt.Printf("Initial Equity   : $%.2f\n", report.InitialEquity)
	fmt.Printf("Final Equity     : $%.2f\n", report.FinalEquity)
	fmt.Printf("Total Return     : %.2f%%\n", report.TotalReturnPct)
	fmt.Printf("Total Trades     : %d (Wins: %d, Losses: %d)\n", report.TotalTrades, report.WinningTrades, report.LosingTrades)
	fmt.Printf("Win Rate         : %.2f%%\n", report.WinRatePct)
	fmt.Printf("Profit Factor    : %.2f\n", report.ProfitFactor)
	fmt.Printf("Max Drawdown     : %.2f%%\n", report.MaxDrawdownPct)
	fmt.Printf("Sharpe Ratio     : %.2f\n", report.SharpeRatio)
	fmt.Printf("======================================\n\n")

	_ = backtest.ExportReportJSON(report, *outJSON)
	_ = backtest.ExportTradesCSV(trades, *outCSV)

	fmt.Printf("✅ Reports exported to: %s & %s\n", *outJSON, *outCSV)
}

func generateMarketCandles(count int) []backtest.Candle {
	candles := make([]backtest.Candle, count)
	price := 60000.0
	now := time.Now().Add(-time.Duration(count) * time.Hour)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := 0; i < count; i++ {
		drift := (r.Float64() - 0.495) * 500.0
		price += drift
		if price < 1000 {
			price = 1000
		}
		candles[i] = backtest.Candle{
			Timestamp: now.Add(time.Duration(i) * time.Hour),
			Open:      price - 50,
			High:      price + 120,
			Low:       price - 120,
			Close:     price,
			Volume:    25.0,
		}
	}
	return candles
}
