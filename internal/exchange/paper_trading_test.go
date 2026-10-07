package exchange_test

import (
	"context"
	"testing"

	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/domain"
	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/exchange"
)

func TestPaperEngine_ExecutionAndPnL(t *testing.T) {
	ctx := context.Background()
	engine := exchange.NewPaperEngine(10000.0, 0.001, 0.0005) // $10k cash, 0.1% fee, 0.05% slippage

	// 1. Buy 0.1 BTC at market price 50,000
	buyTrade, err := engine.ExecuteMarketOrder(ctx, exchange.OrderRequest{
		Symbol:   "BTCUSDT",
		Side:     domain.SideBuy,
		Type:     exchange.OrderTypeMarket,
		Quantity: 0.1,
		Reason:   "Golden cross entry",
	}, 50000.0)

	if err != nil {
		t.Fatalf("buy order failed: %v", err)
	}

	// Execution price should have slipped higher
	if buyTrade.Price <= 50000.0 {
		t.Errorf("expected buy price > 50000 due to slippage, got %f", buyTrade.Price)
	}

	bal := engine.GetBalance()
	if bal.Positions["BTCUSDT"].Quantity != 0.1 {
		t.Errorf("expected 0.1 BTC, got %f", bal.Positions["BTCUSDT"].Quantity)
	}

	// 2. Sell 0.1 BTC at market price 55,000 (profit scenario)
	sellTrade, err := engine.ExecuteMarketOrder(ctx, exchange.OrderRequest{
		Symbol:   "BTCUSDT",
		Side:     domain.SideSell,
		Type:     exchange.OrderTypeMarket,
		Quantity: 0.1,
		Reason:   "Take profit reached",
	}, 55000.0)

	if err != nil {
		t.Fatalf("sell order failed: %v", err)
	}

	// Realized PnL should be roughly (55,000 - 50,000) * 0.1 = $500 minus fees & slippage
	if sellTrade.PnL < 450.0 || sellTrade.PnL > 500.0 {
		t.Errorf("expected realized PnL between 450 and 500, got %f", sellTrade.PnL)
	}

	balAfter := engine.GetBalance()
	if balAfter.CashUSDT <= 10000.0 {
		t.Errorf("expected total cash > 10000 after winning trade, got %f", balAfter.CashUSDT)
	}
	if balAfter.Positions["BTCUSDT"].Quantity != 0.0 {
		t.Errorf("expected 0 BTC remaining, got %f", balAfter.Positions["BTCUSDT"].Quantity)
	}
}

func TestPaperEngine_InsufficientFunds(t *testing.T) {
	ctx := context.Background()
	engine := exchange.NewPaperEngine(100.0, 0.001, 0.0005) // only $100 cash

	_, err := engine.ExecuteMarketOrder(ctx, exchange.OrderRequest{
		Symbol:   "BTCUSDT",
		Side:     domain.SideBuy,
		Quantity: 1.0, // $50,000 required
	}, 50000.0)

	if err == nil {
		t.Error("expected error for insufficient funds, got nil")
	}
}
