package ingestion

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap/zaptest"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func TestBinanceClient_StreamTrades(t *testing.T) {
	samplePayload := `{
		"stream": "btcusdt@trade",
		"data": {
			"e": "trade",
			"E": 1672531199000,
			"s": "BTCUSDT",
			"t": 1234567,
			"p": "42350.50",
			"q": "0.15000",
			"b": 8888,
			"a": 9999,
			"T": 1672531198500,
			"m": true
		}
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade error: %v", err)
			return
		}
		defer conn.Close()

		// Send sample trade message
		err = conn.WriteMessage(websocket.TextMessage, []byte(samplePayload))
		if err != nil {
			t.Logf("write error: %v", err)
			return
		}

		// Keep connection open briefly then exit cleanly
		time.Sleep(200 * time.Millisecond)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	logger := zaptest.NewLogger(t)
	cfg := Config{
		BaseWSURL:   wsURL,
		Symbols:     []string{"btcusdt"},
		MinBackoff:  50 * time.Millisecond,
		MaxBackoff:  100 * time.Millisecond,
		PingPeriod:  100 * time.Millisecond,
		ChannelSize: 10,
	}

	client, err := NewBinanceClient(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	go func() {
		_ = client.Start(ctx)
	}()

	select {
	case trade, ok := <-client.Trades():
		if !ok {
			t.Fatal("trade channel closed unexpectedly")
		}
		if trade.Symbol != "BTCUSDT" {
			t.Errorf("expected BTCUSDT, got %s", trade.Symbol)
		}
		if trade.Price != 42350.50 {
			t.Errorf("expected 42350.50, got %f", trade.Price)
		}
		if trade.Quantity != 0.15 {
			t.Errorf("expected 0.15, got %f", trade.Quantity)
		}
		if trade.TradeID != 1234567 {
			t.Errorf("expected 1234567, got %d", trade.TradeID)
		}
		if !trade.IsBuyerMaker {
			t.Errorf("expected IsBuyerMaker=true, got %v", trade.IsBuyerMaker)
		}
	case <-time.After(800 * time.Millisecond):
		t.Fatal("timed out waiting for trade event from mock websocket server")
	}
}

func TestBinanceClient_Validation(t *testing.T) {
	logger := zaptest.NewLogger(t)
	_, err := NewBinanceClient(Config{Symbols: []string{}}, logger)
	if err == nil {
		t.Error("expected error when symbols slice is empty, got nil")
	}
}
