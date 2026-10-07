package agent_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap/zaptest"

	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/agent"
	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/agent/prompts"
	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/domain"
	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/ingestion"
)

var e2eUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func TestEndToEnd_PipelineSimulation(t *testing.T) {
	// 1. Mock WebSocket Server streaming trades
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := e2eUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		price := 60000.0
		for i := 0; i < 10; i++ {
			price += 10.0
			payload := fmt.Sprintf(`{
				"stream": "btcusdt@trade",
				"data": {
					"e": "trade",
					"E": 1672531199000,
					"s": "BTCUSDT",
					"t": %d,
					"p": "%.2f",
					"q": "0.15000",
					"b": 8888,
					"a": 9999,
					"T": 1672531198500,
					"m": true
				}
			}`, 1000+i, price)
			_ = conn.WriteMessage(websocket.TextMessage, []byte(payload))
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer server.Close()

	logger := zaptest.NewLogger(t)
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	wsClient, err := ingestion.NewBinanceClient(ingestion.Config{
		BaseWSURL:  wsURL,
		Symbols:    []string{"btcusdt"},
		MinBackoff: 50 * time.Millisecond,
		MaxBackoff: 100 * time.Millisecond,
	}, logger)
	if err != nil {
		t.Fatalf("create ws client failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		_ = wsClient.Start(ctx)
	}()

	// 2. Mock Repositories in memory
	tickRepo := &mockTickRepo{}
	renderer, _ := prompts.NewPromptRenderer()
	mockLLM := &mockLLMClient{
		chatResponse: `{"action": "BUY", "quantity": 0.05, "confidence": 0.85, "reasoning": "Breakout confirmed"}`,
	}

	cfg := agent.TradingAgentConfig{
		Symbols:        []string{"BTCUSDT"},
		EnableTrading:  true,
		MaxPositionUSD: 5000.0,
		MinConfidence:  0.75,
	}

	trader := agent.NewTradingAgent(cfg, mockLLM, renderer, tickRepo, nil, nil, logger)

	// Consume trades and simulate ingestion into tickRepo
	receivedCount := 0
	doneCh := make(chan struct{})

	go func() {
		for trade := range wsClient.Trades() {
			receivedCount++
			tick := domain.PriceTick{
				Symbol:    trade.Symbol,
				Price:     trade.Price,
				Volume:    trade.Quantity,
				Timestamp: trade.TradeTime,
			}
			tickRepo.ticks = append(tickRepo.ticks, tick)
			if receivedCount >= 5 {
				close(doneCh)
				return
			}
		}
	}()

	select {
	case <-doneCh:
	case <-time.After(1500 * time.Millisecond):
		t.Fatal("timed out waiting for trades from WS stream")
	}

	// 3. Run Agent cycle on ingested data
	obs, err := trader.Observe(ctx, "BTCUSDT")
	if err != nil {
		t.Fatalf("observe failed: %v", err)
	}
	if obs.CurrentPrice <= 0 {
		t.Errorf("expected positive price, got %f", obs.CurrentPrice)
	}

	dec, err := trader.Think(ctx, obs)
	if err != nil {
		t.Fatalf("think failed: %v", err)
	}
	if dec.Action != domain.ActionBuy {
		t.Errorf("expected BUY decision, got %s", dec.Action)
	}

	res, err := trader.Act(ctx, dec)
	if err != nil {
		t.Fatalf("act failed: %v", err)
	}
	if !res.Success {
		t.Errorf("action execution failed: %s", res.ErrorMessage)
	}

	t.Logf("E2E simulation passed! Executed %s of %.4f %s at $%.2f", res.Action, res.ExecutedQty, res.Symbol, res.ExecutedPrice)
}
