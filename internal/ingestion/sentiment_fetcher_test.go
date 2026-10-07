package ingestion_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"

	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/agent/prompts"
	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/ingestion"
	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/llm"
)

type mockLLMSentimentClient struct{}

func (m *mockLLMSentimentClient) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{
		Content: `{"score": 0.75, "label": "BULLISH", "reasoning": "Strong inflow news"}`,
	}, nil
}

func (m *mockLLMSentimentClient) CreateEmbedding(ctx context.Context, text string) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3}, nil
}

func TestSentimentFetcher_FetchAndScore(t *testing.T) {
	// Mock CryptoPanic server
	cryptoPanicServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ingestion.CryptoPanicResponse{
			Results: []struct {
				Title       string `json:"title"`
				PublishedAt string `json:"published_at"`
				Domain      string `json:"domain"`
			}{
				{Title: "Bitcoin ETF sees record inflows", PublishedAt: "2026-10-07T10:00:00Z", Domain: "coindesk.com"},
				{Title: "Institutional demand drives market higher", PublishedAt: "2026-10-07T10:10:00Z", Domain: "cointelegraph.com"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer cryptoPanicServer.Close()

	logger := zaptest.NewLogger(t)
	renderer, err := prompts.NewPromptRenderer()
	if err != nil {
		t.Fatalf("failed to init prompt renderer: %v", err)
	}

	cfg := ingestion.SentimentFetcherConfig{
		Symbols:  []string{"BTCUSDT"},
		BaseURL:  cryptoPanicServer.URL,
		Interval: 1 * time.Minute,
	}

	mockLLM := &mockLLMSentimentClient{}
	fetcher := ingestion.NewSentimentFetcher(cfg, mockLLM, renderer, nil, nil, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = fetcher.FetchAndScore(ctx, "BTCUSDT")
	if err != nil {
		t.Fatalf("failed to fetch and score: %v", err)
	}
}
