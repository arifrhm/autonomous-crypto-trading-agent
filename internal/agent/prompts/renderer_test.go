package prompts_test

import (
	"strings"
	"testing"

	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/agent"
	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/agent/prompts"
	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/domain"
)

func TestPromptRenderer_RenderTradingPrompts(t *testing.T) {
	renderer, err := prompts.NewPromptRenderer()
	if err != nil {
		t.Fatalf("failed to initialize prompt renderer: %v", err)
	}

	sysPrompt, err := renderer.RenderTradingSystemPrompt()
	if err != nil {
		t.Fatalf("failed to render trading system prompt: %v", err)
	}
	if !strings.Contains(sysPrompt, "quantitative crypto trading agent") {
		t.Errorf("system prompt missing key instructions: %s", sysPrompt)
	}

	data := prompts.TradingPromptData{
		Observation: &agent.Observation{
			Symbol:       "BTCUSDT",
			CurrentPrice: 65420.50,
			Indicators: agent.TechnicalIndicators{
				RSI:        42.5,
				EMA9:       65300.0,
				EMA21:      65150.0,
				MACD:       12.5,
				SignalLine: 10.0,
				Histogram:  2.5,
			},
			Sentiment: agent.SentimentState{
				Score:     0.65,
				Label:     "BULLISH",
				Headlines: []string{"Major ETF institutional inflow hits $500M"},
			},
			Position: agent.Position{
				Quantity:      0.5,
				AveragePrice:  64000.0,
				UnrealizedPnL: 710.25,
				CashBalance:   5000.0,
			},
		},
		SimilarMemories: []domain.MarketMemory{
			{
				Category:   "NEWS",
				Content:    "Last month ETF inflow triggered +5% impulse move",
				Similarity: 0.88,
			},
		},
		MaxPositionUSD: 1000.0,
		MinConfidence:  0.75,
	}

	userPrompt, err := renderer.RenderTradingUserPrompt(data)
	if err != nil {
		t.Fatalf("failed to render trading user prompt: %v", err)
	}

	if !strings.Contains(userPrompt, "BTCUSDT") || !strings.Contains(userPrompt, "65420.50") {
		t.Errorf("user prompt missing market price data: %s", userPrompt)
	}
	if !strings.Contains(userPrompt, "Historical Similar Precedents") {
		t.Errorf("user prompt missing RAG vector memories: %s", userPrompt)
	}
}

func TestPromptRenderer_RenderSentimentAndReflection(t *testing.T) {
	renderer, err := prompts.NewPromptRenderer()
	if err != nil {
		t.Fatalf("failed to initialize prompt renderer: %v", err)
	}

	sentPrompt, err := renderer.RenderSentimentUserPrompt(prompts.SentimentPromptData{
		Symbol:    "ETHUSDT",
		Headlines: []string{"Vitalik proposes gas limit reduction"},
	})
	if err != nil {
		t.Fatalf("failed to render sentiment prompt: %v", err)
	}
	if !strings.Contains(sentPrompt, "ETHUSDT") {
		t.Errorf("sentiment prompt missing symbol: %s", sentPrompt)
	}

	refPrompt, err := renderer.RenderReflectionUserPrompt(prompts.ReflectionPromptData{
		ActionResult: &agent.ActionResult{
			DecisionID:    "dec-123",
			Symbol:        "BTCUSDT",
			Action:        domain.ActionBuy,
			ExecutedPrice: 65000.0,
			RealizedPnL:   150.0,
			Success:       true,
		},
		WinRate:  68.5,
		TotalPnL: 1250.0,
	})
	if err != nil {
		t.Fatalf("failed to render reflection prompt: %v", err)
	}
	if !strings.Contains(refPrompt, "68.5%") || !strings.Contains(refPrompt, "1250.00") {
		t.Errorf("reflection prompt missing metric data: %s", refPrompt)
	}
}
