package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/agent/prompts"
	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/domain"
	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/llm"
	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/storage/postgres"
)

type TradingAgentConfig struct {
	Symbols             []string
	LoopInterval        time.Duration
	EnableTrading       bool
	MaxPositionUSD      float64
	MinConfidence       float64
	ReflectionFrequency int
}

type TradingAgent struct {
	cfg          TradingAgentConfig
	logger       *zap.Logger
	llmClient    llm.Client
	renderer     prompts.PromptRenderer
	tickRepo     postgres.PriceTickRepository
	decisionRepo postgres.DecisionRepository
	memoryRepo   postgres.MemoryRepository

	mu          sync.RWMutex
	positions   map[string]domain.Position
	cycleCount  int
	tradeCount  int
	totalPnL    float64
	recentWins  int
}

func NewTradingAgent(
	cfg TradingAgentConfig,
	llmClient llm.Client,
	renderer prompts.PromptRenderer,
	tickRepo postgres.PriceTickRepository,
	decisionRepo postgres.DecisionRepository,
	memoryRepo postgres.MemoryRepository,
	logger *zap.Logger,
) *TradingAgent {
	if cfg.LoopInterval <= 0 {
		cfg.LoopInterval = 30 * time.Second
	}
	if cfg.MaxPositionUSD <= 0 {
		cfg.MaxPositionUSD = 1000.0
	}
	if cfg.MinConfidence <= 0 {
		cfg.MinConfidence = 0.70
	}
	if cfg.ReflectionFrequency <= 0 {
		cfg.ReflectionFrequency = 10
	}

	positions := make(map[string]domain.Position)
	for _, s := range cfg.Symbols {
		positions[s] = domain.Position{
			Symbol:      s,
			CashBalance: 10000.0, // baseline paper cash
			UpdatedAt:   time.Now(),
		}
	}

	return &TradingAgent{
		cfg:          cfg,
		logger:       logger.Named("trading_agent"),
		llmClient:    llmClient,
		renderer:     renderer,
		tickRepo:     tickRepo,
		decisionRepo: decisionRepo,
		memoryRepo:   memoryRepo,
		positions:    positions,
	}
}

// Observe gathers latest market data, calculates indicators, and compiles state.
func (a *TradingAgent) Observe(ctx context.Context, symbol string) (*domain.Observation, error) {
	ticks, err := a.tickRepo.GetLatest(ctx, symbol, 100)
	if err != nil {
		return nil, fmt.Errorf("fetch recent ticks: %w", err)
	}

	currentPrice := 0.0
	prices := make([]float64, 0, len(ticks))
	// Ticks from DB are sorted timestamp DESC, reverse for indicator calculation
	for i := len(ticks) - 1; i >= 0; i-- {
		prices = append(prices, ticks[i].Price)
	}
	if len(ticks) > 0 {
		currentPrice = ticks[0].Price
	}

	indicators := ComputeIndicators(prices)

	a.mu.RLock()
	pos := a.positions[symbol]
	pos.CurrentPrice = currentPrice
	if pos.Quantity > 0 {
		pos.UnrealizedPnL = (currentPrice - pos.AveragePrice) * pos.Quantity
	}
	a.mu.RUnlock()

	return &domain.Observation{
		Timestamp:    time.Now().UTC(),
		Symbol:       symbol,
		CurrentPrice: currentPrice,
		PriceHistory: ticks,
		Indicators:   indicators,
		Sentiment: domain.SentimentState{
			Score:       0.2, // baseline neutral-bullish
			Label:       "NEUTRAL",
			LastUpdated: time.Now(),
		},
		Position: pos,
	}, nil
}

// Think formulates a Decision using RAG Vector Semantic Search, LLM prompts, and guardrails.
func (a *TradingAgent) Think(ctx context.Context, obs *domain.Observation) (*domain.Decision, error) {
	// 1. Vector Search for Precedents (if memory repository is configured)
	var similarMemories []domain.MarketMemory
	if a.memoryRepo != nil && a.llmClient != nil {
		queryText := fmt.Sprintf("%s price %0.2f RSI %0.2f sentiment %s",
			obs.Symbol, obs.CurrentPrice, obs.Indicators.RSI, obs.Sentiment.Label)

		queryVec, err := a.llmClient.CreateEmbedding(ctx, queryText)
		if err == nil {
			mems, err := a.memoryRepo.SearchSimilar(ctx, obs.Symbol, "", queryVec, 2)
			if err == nil {
				similarMemories = mems
			}
		}
	}

	// 2. Render Prompts
	sysPrompt, err := a.renderer.RenderTradingSystemPrompt()
	if err != nil {
		return nil, fmt.Errorf("render system prompt: %w", err)
	}

	userPrompt, err := a.renderer.RenderTradingUserPrompt(prompts.TradingPromptData{
		Observation:     obs,
		SimilarMemories: similarMemories,
		MaxPositionUSD:  a.cfg.MaxPositionUSD,
		MinConfidence:   a.cfg.MinConfidence,
	})
	if err != nil {
		return nil, fmt.Errorf("render user prompt: %w", err)
	}

	// 3. Call LLM
	chatResp, err := a.llmClient.Chat(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: sysPrompt},
			{Role: "user", Content: userPrompt},
		},
		ResponseJSON: true,
	})
	if err != nil {
		return nil, fmt.Errorf("llm chat: %w", err)
	}

	// 4. Parse Structured Decision JSON
	var parsed struct {
		Action         domain.Action         `json:"action"`
		Quantity       float64               `json:"quantity"`
		Confidence     float64               `json:"confidence"`
		Reasoning      string                `json:"reasoning"`
		RiskAssessment domain.RiskAssessment `json:"risk_assessment"`
	}

	if err := json.Unmarshal([]byte(chatResp.Content), &parsed); err != nil {
		a.logger.Warn("Failed to unmarshal LLM response, fallback to HOLD", zap.Error(err), zap.String("raw", chatResp.Content))
		return a.fallbackHoldDecision(obs.Symbol, "Invalid JSON from LLM brain"), nil
	}

	newID, _ := uuid.NewV7()
	decision := &domain.Decision{
		ID:             newID.String(),
		Symbol:         obs.Symbol,
		Action:         parsed.Action,
		Quantity:       parsed.Quantity,
		Confidence:     parsed.Confidence,
		Reasoning:      parsed.Reasoning,
		RiskAssessment: parsed.RiskAssessment,
		SignalsUsed: map[string]any{
			"rsi":             obs.Indicators.RSI,
			"macd":            obs.Indicators.MACD,
			"current_price":   obs.CurrentPrice,
			"sentiment_score": obs.Sentiment.Score,
			"memories_count":  len(similarMemories),
		},
		CreatedAt: time.Now().UTC(),
	}

	// 5. Apply Strict Risk Guardrails
	a.applyGuardrails(decision, obs)

	// 6. Audit Trail Persistence
	if a.decisionRepo != nil {
		signalsJSON, _ := json.Marshal(decision.SignalsUsed)
		_ = a.decisionRepo.Insert(ctx, domain.AgentDecision{
			ID:          decision.ID,
			Symbol:      decision.Symbol,
			Action:      decision.Action,
			Confidence:  decision.Confidence,
			Reasoning:   decision.Reasoning,
			SignalsUsed: signalsJSON,
			CreatedAt:   decision.CreatedAt,
		})
	}

	return decision, nil
}

// Act simulates paper order execution and balances.
func (a *TradingAgent) Act(ctx context.Context, dec *domain.Decision) (*domain.ActionResult, error) {
	if dec.Action == domain.ActionHold || !a.cfg.EnableTrading {
		return &domain.ActionResult{
			DecisionID: dec.ID,
			Symbol:     dec.Symbol,
			Action:     domain.ActionHold,
			Success:    true,
			ExecutedAt: time.Now().UTC(),
		}, nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	pos := a.positions[dec.Symbol]
	executedPrice := pos.CurrentPrice
	executedQty := dec.Quantity
	fee := (executedPrice * executedQty) * 0.001 // 0.1% exchange fee
	var realizedPnL float64

	if dec.Action == domain.ActionBuy {
		cost := (executedPrice * executedQty) + fee
		if cost > pos.CashBalance {
			return &domain.ActionResult{
				DecisionID:   dec.ID,
				Symbol:       dec.Symbol,
				Action:       dec.Action,
				Success:      false,
				ErrorMessage: "Insufficient cash balance",
				ExecutedAt:   time.Now().UTC(),
			}, nil
		}
		// Weighted average entry price
		totalQty := pos.Quantity + executedQty
		pos.AveragePrice = ((pos.Quantity * pos.AveragePrice) + (executedQty * executedPrice)) / totalQty
		pos.Quantity = totalQty
		pos.CashBalance -= cost
	} else if dec.Action == domain.ActionSell {
		if executedQty > pos.Quantity {
			executedQty = pos.Quantity // sell all available
		}
		if executedQty <= 0 {
			return &domain.ActionResult{
				DecisionID:   dec.ID,
				Symbol:       dec.Symbol,
				Action:       dec.Action,
				Success:      false,
				ErrorMessage: "No position available to sell",
				ExecutedAt:   time.Now().UTC(),
			}, nil
		}
		revenue := (executedPrice * executedQty) - fee
		realizedPnL = (executedPrice - pos.AveragePrice) * executedQty - fee
		pos.Quantity -= executedQty
		pos.CashBalance += revenue
		if pos.Quantity == 0 {
			pos.AveragePrice = 0
		}
		a.tradeCount++
		a.totalPnL += realizedPnL
		if realizedPnL > 0 {
			a.recentWins++
		}
	}

	pos.UpdatedAt = time.Now().UTC()
	a.positions[dec.Symbol] = pos

	tradeID, _ := uuid.NewV7()
	return &domain.ActionResult{
		DecisionID:    dec.ID,
		TradeID:       tradeID.String(),
		Symbol:        dec.Symbol,
		Action:        dec.Action,
		ExecutedPrice: executedPrice,
		ExecutedQty:   executedQty,
		Fee:           fee,
		RealizedPnL:   realizedPnL,
		Success:       true,
		ExecutedAt:    time.Now().UTC(),
	}, nil
}

// Reflect reviews performance and saves distilled lessons to vector store memory.
func (a *TradingAgent) Reflect(ctx context.Context, result *domain.ActionResult) error {
	if a.memoryRepo == nil || a.llmClient == nil || result == nil || !result.Success || result.Action == domain.ActionHold {
		return nil
	}

	a.mu.RLock()
	totalTrades := a.tradeCount
	winRate := 0.0
	if totalTrades > 0 {
		winRate = (float64(a.recentWins) / float64(totalTrades)) * 100.0
	}
	totalPnL := a.totalPnL
	a.mu.RUnlock()

	sysPrompt, _ := a.renderer.RenderReflectionSystemPrompt()
	userPrompt, _ := a.renderer.RenderReflectionUserPrompt(prompts.ReflectionPromptData{
		ActionResult: result,
		WinRate:      winRate,
		TotalPnL:     totalPnL,
	})

	chatResp, err := a.llmClient.Chat(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: sysPrompt},
			{Role: "user", Content: userPrompt},
		},
		ResponseJSON: true,
	})
	if err != nil {
		return err
	}

	var refResult struct {
		KeyFindings         string `json:"key_findings"`
		StrategyAdjustments string `json:"strategy_adjustments"`
		MemorySummary       string `json:"memory_summary"`
	}
	if err := json.Unmarshal([]byte(chatResp.Content), &refResult); err == nil && refResult.MemorySummary != "" {
		embedding, err := a.llmClient.CreateEmbedding(ctx, refResult.MemorySummary)
		if err == nil {
			_ = a.memoryRepo.Insert(ctx, domain.MarketMemory{
				Symbol:    result.Symbol,
				Category:  "TRADE_REFLECTION",
				Content:   refResult.MemorySummary,
				Embedding: embedding,
				CreatedAt: time.Now().UTC(),
			})
			a.logger.Info("Saved post-trade reflection to pgvector memory",
				zap.String("symbol", result.Symbol),
				zap.String("summary", refResult.MemorySummary),
			)
		}
	}

	return nil
}

// Run executes the periodic agent loop.
func (a *TradingAgent) Run(ctx context.Context) error {
	a.logger.Info("Starting Trading Agent decision loop",
		zap.Duration("interval", a.cfg.LoopInterval),
		zap.Bool("enable_trading", a.cfg.EnableTrading),
	)

	ticker := time.NewTicker(a.cfg.LoopInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			a.logger.Info("Shutting down trading agent loop")
			return ctx.Err()
		case <-ticker.C:
			a.cycleCount++
			for _, sym := range a.cfg.Symbols {
				a.processSymbolCycle(ctx, sym)
			}
		}
	}
}

func (a *TradingAgent) processSymbolCycle(ctx context.Context, symbol string) {
	obs, err := a.Observe(ctx, symbol)
	if err != nil {
		a.logger.Warn("Failed to observe market state", zap.String("symbol", symbol), zap.Error(err))
		return
	}

	if obs.CurrentPrice == 0 {
		a.logger.Debug("Waiting for tick data stream...", zap.String("symbol", symbol))
		return
	}

	dec, err := a.Think(ctx, obs)
	if err != nil {
		a.logger.Error("Failed in thinking phase", zap.String("symbol", symbol), zap.Error(err))
		return
	}

	a.logger.Info("Agent formulated decision",
		zap.String("symbol", dec.Symbol),
		zap.String("action", string(dec.Action)),
		zap.Float64("confidence", dec.Confidence),
		zap.String("reasoning", dec.Reasoning),
	)

	res, err := a.Act(ctx, dec)
	if err != nil {
		a.logger.Error("Failed in execution phase", zap.String("symbol", symbol), zap.Error(err))
		return
	}

	if a.cycleCount%a.cfg.ReflectionFrequency == 0 && res.Action != domain.ActionHold {
		_ = a.Reflect(ctx, res)
	}
}

func (a *TradingAgent) applyGuardrails(dec *domain.Decision, obs *domain.Observation) {
	if dec.Confidence < a.cfg.MinConfidence {
		dec.Action = domain.ActionHold
		dec.Reasoning = fmt.Sprintf("Rejected by guardrails: confidence %.2f below threshold %.2f", dec.Confidence, a.cfg.MinConfidence)
		return
	}

	if dec.Action == domain.ActionBuy && obs.Indicators.RSI > 75 {
		dec.Action = domain.ActionHold
		dec.Reasoning = fmt.Sprintf("Rejected by guardrails: RSI %.2f is overbought (>75)", obs.Indicators.RSI)
		return
	}

	if dec.Action == domain.ActionSell && obs.Indicators.RSI < 25 {
		dec.Action = domain.ActionHold
		dec.Reasoning = fmt.Sprintf("Rejected by guardrails: RSI %.2f is oversold (<25)", obs.Indicators.RSI)
		return
	}

	// Max order size check
	orderUSD := dec.Quantity * obs.CurrentPrice
	if dec.Action == domain.ActionBuy && orderUSD > a.cfg.MaxPositionUSD {
		dec.Quantity = a.cfg.MaxPositionUSD / obs.CurrentPrice
		dec.Reasoning += fmt.Sprintf(" (Quantity capped to $%.2f limit)", a.cfg.MaxPositionUSD)
	}
}

func (a *TradingAgent) fallbackHoldDecision(symbol, reason string) *domain.Decision {
	holdID, _ := uuid.NewV7()
	return &domain.Decision{
		ID:         holdID.String(),
		Symbol:     symbol,
		Action:     domain.ActionHold,
		Confidence: 0.0,
		Reasoning:  reason,
		CreatedAt:  time.Now().UTC(),
	}
}
