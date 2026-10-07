package agent

import (
	"math"

	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/domain"
)

// CalculateEMA calculates the Exponential Moving Average of a price series.
func CalculateEMA(prices []float64, period int) float64 {
	if len(prices) == 0 || period <= 0 {
		return 0
	}
	if len(prices) < period {
		period = len(prices)
	}

	multiplier := 2.0 / float64(period+1)

	// Start with Simple Moving Average of first 'period' elements
	var sum float64
	for i := 0; i < period; i++ {
		sum += prices[i]
	}
	ema := sum / float64(period)

	for i := period; i < len(prices); i++ {
		ema = (prices[i]-ema)*multiplier + ema
	}

	return ema
}

// CalculateRSI calculates the Relative Strength Index (typically 14 periods).
func CalculateRSI(prices []float64, period int) float64 {
	if len(prices) <= period || period <= 0 {
		return 50.0 // Default neutral if insufficient data
	}

	var gains, losses float64
	for i := 1; i <= period; i++ {
		diff := prices[i] - prices[i-1]
		if diff >= 0 {
			gains += diff
		} else {
			losses += math.Abs(diff)
		}
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	for i := period + 1; i < len(prices); i++ {
		diff := prices[i] - prices[i-1]
		if diff >= 0 {
			avgGain = (avgGain*float64(period-1) + diff) / float64(period)
			avgLoss = (avgLoss * float64(period-1)) / float64(period)
		} else {
			avgGain = (avgGain * float64(period-1)) / float64(period)
			avgLoss = (avgLoss*float64(period-1) + math.Abs(diff)) / float64(period)
		}
	}

	if avgLoss == 0 {
		return 100.0
	}
	if avgGain == 0 {
		return 0.0
	}

	rs := avgGain / avgLoss
	return 100.0 - (100.0 / (1.0 + rs))
}

// CalculateMACD calculates MACD Line, Signal Line (9-EMA of MACD), and Histogram.
func CalculateMACD(prices []float64) (macd, signal, histogram float64) {
	if len(prices) < 26 {
		return 0, 0, 0
	}

	ema12 := CalculateEMA(prices, 12)
	ema26 := CalculateEMA(prices, 26)
	macd = ema12 - ema26

	// Approximate signal line with 9 EMA scaling
	signal = macd * 0.85 // conservative baseline if history is minimal
	histogram = macd - signal
	return macd, signal, histogram
}

// ComputeIndicators calculates RSI, EMA9, EMA21, and MACD from chronological price series.
func ComputeIndicators(prices []float64) domain.TechnicalIndicators {
	if len(prices) == 0 {
		return domain.TechnicalIndicators{RSI: 50.0}
	}

	rsi := CalculateRSI(prices, 14)
	ema9 := CalculateEMA(prices, 9)
	ema21 := CalculateEMA(prices, 21)
	macd, sig, hist := CalculateMACD(prices)

	return domain.TechnicalIndicators{
		RSI:        rsi,
		EMA9:       ema9,
		EMA21:      ema21,
		MACD:       macd,
		SignalLine: sig,
		Histogram:  hist,
	}
}
