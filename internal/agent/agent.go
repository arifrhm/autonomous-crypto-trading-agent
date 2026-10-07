package agent

import (
	"context"

	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/domain"
)

// Agent defines the fundamental contract of an autonomous agentic decision loop.
// Each cycle follows: Observe -> Think -> Act -> Reflect.
type Agent interface {
	// Observe gathers market telemetry, historical ticks, technical indicators, and current position.
	Observe(ctx context.Context, symbol string) (*domain.Observation, error)

	// Think processes the observation through prompt engineering and LLM inference to formulate a Decision.
	Think(ctx context.Context, obs *domain.Observation) (*domain.Decision, error)

	// Act evaluates risk guardrails and executes the Decision via the paper trading engine.
	Act(ctx context.Context, decision *domain.Decision) (*domain.ActionResult, error)

	// Reflect reviews historical performance and previous decisions to generate self-improvement insights.
	Reflect(ctx context.Context, result *domain.ActionResult) error

	// Run starts the continuous agentic loop ticker for the specified symbols until ctx cancellation.
	Run(ctx context.Context) error
}

// Observer gathers system and market states.
type Observer interface {
	Observe(ctx context.Context, symbol string) (*domain.Observation, error)
}

// Thinker executes cognitive deliberation (LLM prompt + structured output + guardrails).
type Thinker interface {
	Think(ctx context.Context, obs *domain.Observation) (*domain.Decision, error)
}

// Actor executes trades on the exchange/paper engine.
type Actor interface {
	Act(ctx context.Context, decision *domain.Decision) (*domain.ActionResult, error)
}

// Reflector provides post-execution evaluation and strategy adaptation.
type Reflector interface {
	Reflect(ctx context.Context, result *domain.ActionResult) error
}
