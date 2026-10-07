package agent_test

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"

	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/agent"
	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/agent/prompts"
	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/domain"
	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/llm"
)

type mockLLMClient struct {
	chatResponse string
}

func (m *mockLLMClient) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{
		Content: m.chatResponse,
	}, nil
}

func (m *mockLLMClient) CreateEmbedding(ctx context.Context, text string) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3}, nil
}

type mockTickRepo struct {
	ticks []domain.PriceTick
}

func (m *mockTickRepo) Insert(ctx context.Context, tick domain.PriceTick) error { return nil }
func (m *mockTickRepo) InsertBatch(ctx context.Context, ticks []domain.PriceTick) error {
	return nil
}
func (m *mockTickRepo) GetRange(ctx context.Context, symbol string, from, to time.Time) ([]domain.PriceTick, error) {
	return nil, nil
}
func (m *mockTickRepo) GetLatest(ctx context.Context, symbol string, limit int) ([]domain.PriceTick, error) {
	return m.ticks, nil
}

func TestTradingAgent_ThinkAndGuardrails(t *testing.T) {
	logger := zaptest.NewLogger(t)
	renderer, err := prompts.NewPromptRenderer()
	if err != nil {
		t.Fatalf("failed to init renderer: %v", err)
	}

	mockLLM := &mockLLMClient{
		// LLM suggests BUY with 0.9 confidence
		chatResponse: `{
			"action": "BUY",
			"quantity": 0.05,
			"confidence": 0.90,
			"reasoning": "Strong breakout above EMA",
			"risk_assessment": {
				"stop_loss_price": 64000.0,
				"take_profit_price": 68000.0,
				"risk_reward_ratio": 2.0,
				"within_limits": true
			}
		}`,
	}

	tickRepo := &mockTickRepo{
		ticks: []domain.PriceTick{
			{Symbol: "BTCUSDT", Price: 65000.0, Timestamp: time.Now()},
		},
	}

	cfg := agent.TradingAgentConfig{
		Symbols:        []string{"BTCUSDT"},
		EnableTrading:  true,
		MaxPositionUSD: 5000.0,
		MinConfidence:  0.75,
	}

	trader := agent.NewTradingAgent(cfg, mockLLM, renderer, tickRepo, nil, nil, logger)

	obs, err := trader.Observe(context.Background(), "BTCUSDT")
	if err != nil {
		t.Fatalf("observe failed: %v", err)
	}

	dec, err := trader.Think(context.Background(), obs)
	if err != nil {
		t.Fatalf("think failed: %v", err)
	}

	if dec.Action != domain.ActionBuy {
		t.Errorf("expected ActionBuy, got %s", dec.Action)
	}

	act, err := trader.Act(context.Background(), dec)
	if err != nil {
		t.Fatalf("act failed: %v", err)
	}

	if !act.Success || act.Action != domain.ActionBuy {
		t.Errorf("unexpected action result: %+v", act)
	}

	// Test Guardrail: Low confidence should turn into HOLD
	mockLLM.chatResponse = `{"action": "BUY", "quantity": 0.05, "confidence": 0.50, "reasoning": "Uncertain bounce"}`
	decLowConf, err := trader.Think(context.Background(), obs)
	if err != nil {
		t.Fatalf("think failed: %v", err)
	}
	if decLowConf.Action != domain.ActionHold {
		t.Errorf("expected guardrail to override to ActionHold, got %s", decLowConf.Action)
	}
}

func TestIndicators_RSI_EMA_MACD(t *testing.T) {
	prices := []float64{
		100, 102, 104, 103, 105, 107, 106, 108, 110, 112,
		111, 113, 115, 114, 116, 118, 120, 119, 122, 125,
		124, 126, 128, 130, 129, 131, 133, 135, 134, 136,
	}

	indicators := agent.ComputeIndicators(prices)

	if indicators.RSI <= 0 || indicators.RSI > 100 {
		t.Errorf("invalid RSI computed: %f", indicators.RSI)
	}
	if indicators.EMA9 <= 0 || indicators.EMA21 <= 0 {
		t.Errorf("invalid EMAs: EMA9=%f, EMA21=%f", indicators.EMA9, indicators.EMA21)
	}
}
