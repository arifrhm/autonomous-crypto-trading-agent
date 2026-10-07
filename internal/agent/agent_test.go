package agent_test

import (
	"context"
	"testing"
	"time"

	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/agent"
	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/domain"
)

// MockAgent implements agent.Agent for isolated testing of pipeline orchestrators.
type MockAgent struct {
	ObservedSymbol string
	DecisionMade   *agent.Decision
	ActionResult   *agent.ActionResult
}

func (m *MockAgent) Observe(ctx context.Context, symbol string) (*agent.Observation, error) {
	m.ObservedSymbol = symbol
	return &agent.Observation{
		Timestamp:    time.Now(),
		Symbol:       symbol,
		CurrentPrice: 65000.0,
		Indicators: agent.TechnicalIndicators{
			RSI: 45.0,
		},
		Position: agent.Position{
			Symbol:      symbol,
			CashBalance: 10000.0,
		},
	}, nil
}

func (m *MockAgent) Think(ctx context.Context, obs *agent.Observation) (*agent.Decision, error) {
	decision := &agent.Decision{
		ID:         "mock-decision-1",
		Symbol:     obs.Symbol,
		Action:     domain.ActionBuy,
		Quantity:   0.1,
		Confidence: 0.85,
		Reasoning:  "RSI is neutral-bullish with strong orderbook support",
	}
	m.DecisionMade = decision
	return decision, nil
}

func (m *MockAgent) Act(ctx context.Context, decision *agent.Decision) (*agent.ActionResult, error) {
	res := &agent.ActionResult{
		DecisionID:    decision.ID,
		Symbol:        decision.Symbol,
		Action:        decision.Action,
		ExecutedPrice: 65000.0,
		ExecutedQty:   decision.Quantity,
		Success:       true,
		ExecutedAt:    time.Now(),
	}
	m.ActionResult = res
	return res, nil
}

func (m *MockAgent) Reflect(ctx context.Context, result *agent.ActionResult) error {
	return nil
}

func (m *MockAgent) Run(ctx context.Context) error {
	return nil
}

// Compile-time check that MockAgent satisfies agent.Agent interface
var _ agent.Agent = (*MockAgent)(nil)

func TestMockAgentFlow(t *testing.T) {
	ctx := context.Background()
	mock := &MockAgent{}

	obs, err := mock.Observe(ctx, "BTCUSDT")
	if err != nil {
		t.Fatalf("unexpected observe error: %v", err)
	}
	if obs.Symbol != "BTCUSDT" || obs.CurrentPrice != 65000.0 {
		t.Errorf("observe data mismatch: %+v", obs)
	}

	dec, err := mock.Think(ctx, obs)
	if err != nil {
		t.Fatalf("unexpected think error: %v", err)
	}
	if dec.Action != domain.ActionBuy || dec.Confidence < 0.8 {
		t.Errorf("think decision unexpected: %+v", dec)
	}

	act, err := mock.Act(ctx, dec)
	if err != nil {
		t.Fatalf("unexpected act error: %v", err)
	}
	if !act.Success || act.ExecutedQty != 0.1 {
		t.Errorf("action execution unexpected: %+v", act)
	}

	if err := mock.Reflect(ctx, act); err != nil {
		t.Fatalf("unexpected reflect error: %v", err)
	}
}
