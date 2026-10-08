package main

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	"go.uber.org/zap"

	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/agent"
	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/agent/prompts"
	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/domain"
	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/exchange"
	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/llm"
	"github.com/arifrhm/autonomous-crypto-trading-agent/pkg/logger"
)

type demoMemoryStore struct {
	mems []domain.MarketMemory
}

func (s *demoMemoryStore) Insert(ctx context.Context, m domain.MarketMemory) error {
	s.mems = append(s.mems, m)
	return nil
}

func (s *demoMemoryStore) SearchSimilar(ctx context.Context, symbol, category string, vec []float32, limit int) ([]domain.MarketMemory, error) {
	return s.mems, nil
}

type demoLLM struct{}

func (d *demoLLM) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{
		Content: `{
			"action": "BUY",
			"quantity": 0.05,
			"confidence": 0.88,
			"reasoning": "RSI at 32.4 oversold dip with strong historical precedent similarity (0.91). EMA21 dynamic support held.",
			"risk_assessment": {
				"stop_loss_price": 63500.0,
				"take_profit_price": 67800.0,
				"max_loss_usd": 75.0,
				"risk_reward_ratio": 2.2,
				"within_limits": true
			}
		}`,
		PromptTokens:     245,
		CompletionTokens: 85,
		TotalTokens:      330,
		CostUSD:          0.00008,
	}, nil
}

func (d *demoLLM) CreateEmbedding(ctx context.Context, text string) ([]float32, error) {
	return []float32{0.05, 0.12, -0.45}, nil
}

func main() {
	log, _ := logger.New("development", "info")
	defer func() { _ = log.Sync() }()

	fmt.Println("==========================================================================")
	fmt.Println("🚀 AUTONOMOUS CRYPTO TRADING AGENT - LIVE WALKTHROUGH DEMO")
	fmt.Println("   Binance WS Ingestion • pgvector RAG • LLM Brain • Paper Execution")
	fmt.Println("==========================================================================")

	// 1. Setup in-memory episodic memory (pgvector mock)
	memStore := &demoMemoryStore{
		mems: []domain.MarketMemory{
			{
				Category:   "REGIME_HISTORICAL",
				Content:    "Oversold bounce occurred after institutional ETF inflow announced.",
				Similarity: 0.91,
			},
		},
	}

	// 2. Setup Paper Engine ($10,000 cash, 0.1% fee, 0.05% slippage)
	paper := exchange.NewPaperEngine(10000.0, 0.001, 0.0005)

	renderer, _ := prompts.NewPromptRenderer()
	mockBrain := &demoLLM{}

	fmt.Println("📡 [STEP 1: STREAM INGESTION] Simulating Binance trade ticks (BTCUSDT)...")
	basePrice := 64800.0
	var prices []float64

	for i := 1; i <= 30; i++ {
		wave := math.Sin(float64(i)/3.0) * 150.0
		tickPrice := basePrice + wave + (rand.Float64()-0.5)*20.0
		prices = append(prices, tickPrice)
		if i%5 == 0 {
			fmt.Printf("   -> Tick #%02d | Symbol: BTCUSDT | Price: $%.2f | Volume: %.4f BTC\n", i, tickPrice, 0.15+rand.Float64()*0.5)
		}
	}

	// 3. Technical Indicators
	fmt.Println("\n📐 [STEP 2: PERCEPTION & TA] Calculating In-House Technical Indicators...")
	indicators := agent.ComputeIndicators(prices)
	currentPrice := prices[len(prices)-1]
	fmt.Printf("   -> Current Mark Price : $%.2f\n", currentPrice)
	fmt.Printf("   -> In-House RSI (14)   : %.2f (Oversold condition trigger)\n", indicators.RSI)
	fmt.Printf("   -> In-House EMA (9)    : $%.2f\n", indicators.EMA9)
	fmt.Printf("   -> In-House EMA (21)   : $%.2f\n", indicators.EMA21)
	fmt.Printf("   -> In-House MACD Hist  : %.4f\n", indicators.Histogram)

	// 4. Memory RAG Search
	fmt.Println("\n🧠 [STEP 3: EPISODIC MEMORY RAG] Querying pgvector HNSW index for similar regimes...")
	similar, _ := memStore.SearchSimilar(context.Background(), "BTCUSDT", "", nil, 2)
	for _, m := range similar {
		fmt.Printf("   -> Matched Memory [Similarity: %.2f]: %s\n", m.Similarity, m.Content)
	}

	// 5. LLM Brain Thinking
	fmt.Println("\n🤖 [STEP 4: COGNITION & THINKING] Dispatching to LLM Brain (GPT-4o-mini)...")
	obs := &domain.Observation{
		Timestamp:    time.Now().UTC(),
		Symbol:       "BTCUSDT",
		CurrentPrice: currentPrice,
		Indicators:   indicators,
		Sentiment: domain.SentimentState{
			Score: 0.65,
			Label: "BULLISH",
		},
		Position: domain.Position{
			Symbol:      "BTCUSDT",
			CashBalance: 10000.0,
		},
	}

	sysPrompt, _ := renderer.RenderTradingSystemPrompt()
	userPrompt, _ := renderer.RenderTradingUserPrompt(prompts.TradingPromptData{
		Observation:     obs,
		SimilarMemories: similar,
		MaxPositionUSD:  5000.0,
		MinConfidence:   0.75,
	})

	chatResp, _ := mockBrain.Chat(context.Background(), llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: sysPrompt},
			{Role: "user", Content: userPrompt},
		},
	})

	fmt.Printf("   -> Tokens Billed  : %d (Prompt: %d, Completion: %d)\n", chatResp.TotalTokens, chatResp.PromptTokens, chatResp.CompletionTokens)
	fmt.Printf("   -> Inference Cost : $%.6f\n", chatResp.CostUSD)
	fmt.Printf("   -> Decision JSON  :\n%s\n", chatResp.Content)

	// 6. Execution via Paper Trading Engine
	fmt.Println("\n⚡ [STEP 5: ORDER EXECUTION] Routing order to PaperEngine (with 0.05% slippage & 0.1% fee)...")
	trade, err := paper.ExecuteMarketOrder(context.Background(), exchange.OrderRequest{
		Symbol:   "BTCUSDT",
		Side:     domain.SideBuy,
		Type:     exchange.OrderTypeMarket,
		Quantity: 0.05,
		Reason:   "LLM Brain: RSI oversold dip + RAG confirmation",
	}, currentPrice)

	if err != nil {
		log.Fatal("Trade execution failed", zap.Error(err))
	}

	fmt.Printf("   -> Order Side       : %s\n", trade.Side)
	fmt.Printf("   -> Requested Price  : $%.2f\n", currentPrice)
	fmt.Printf("   -> Executed Price   : $%.2f (Slipped by +$%.2f)\n", trade.Price, trade.Slippage)
	fmt.Printf("   -> Quantity Filled  : %.4f BTC\n", trade.Quantity)
	fmt.Printf("   -> Exchange Fee     : $%.4f USDT\n", trade.Fee)
	fmt.Printf("   -> Time-Ordered ID  : %s (UUIDv7 RFC 9562)\n", trade.ID)

	// 7. Portfolio Status After Trade
	bal := paper.GetBalance()
	fmt.Println("\n💼 [STEP 6: PORTFOLIO BALANCE POST-TRADE]")
	fmt.Printf("   -> Available Cash   : $%.2f USDT\n", bal.CashUSDT)
	fmt.Printf("   -> Open Position    : %.4f BTCUSDT @ $%.2f\n", bal.Positions["BTCUSDT"].Quantity, bal.Positions["BTCUSDT"].AveragePrice)
	fmt.Printf("   -> Total Equity     : $%.2f USDT\n", paper.GetTotalEquity(map[string]float64{"BTCUSDT": currentPrice}))

	// 8. Profit Taking Simulation
	fmt.Println("\n📈 [STEP 7: MARKET RALLY & EXIT] Simulating +4% price rally to $67,500...")
	sellPrice := 67500.0
	sellTrade, _ := paper.ExecuteMarketOrder(context.Background(), exchange.OrderRequest{
		Symbol:   "BTCUSDT",
		Side:     domain.SideSell,
		Type:     exchange.OrderTypeMarket,
		Quantity: 0.05,
		Reason:   "Take profit target reached at $67,500",
	}, sellPrice)

	fmt.Printf("   -> Exit Executed    : $%.2f (Net PnL: +$%.2f USDT)\n", sellTrade.Price, sellTrade.PnL)
	finalBal := paper.GetBalance()
	fmt.Printf("   -> Final Cash Balance: $%.2f USDT (Net Growth: +$%.2f)\n", finalBal.CashUSDT, finalBal.CashUSDT-10000.0)

	fmt.Println("==========================================================================")
	fmt.Println("✅ WALKTHROUGH DEMO COMPLETED SUCCESSFULLY!")
	fmt.Println("==========================================================================")
}
