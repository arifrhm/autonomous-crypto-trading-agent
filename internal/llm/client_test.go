package llm_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"

	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/llm"
)

func TestResilientLLMClient_ChatAndRetry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			// First attempt simulates rate limit 429
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error": "rate limit exceeded"}`))
			return
		}

		// Second attempt succeeds
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": `{"action": "BUY", "confidence": 0.85}`,
					},
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     150,
				"completion_tokens": 50,
				"total_tokens":      200,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	logger := zaptest.NewLogger(t)
	cfg := llm.Config{
		Provider: "openai",
		APIKey:   "test-key",
		Model:    "gpt-4o-mini",
		BaseURL:  server.URL,
	}

	client := llm.NewClient(cfg, nil, logger)

	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: "You are a crypto trader."},
			{Role: "user", Content: "Analyze BTC."},
		},
		ResponseJSON: true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Chat(ctx, req)
	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}

	if attempts != 2 {
		t.Errorf("expected 2 attempts due to retry, got %d", attempts)
	}

	if resp.TotalTokens != 200 {
		t.Errorf("expected 200 total tokens, got %d", resp.TotalTokens)
	}

	if resp.Content != `{"action": "BUY", "confidence": 0.85}` {
		t.Errorf("unexpected content: %s", resp.Content)
	}
}

func TestResilientLLMClient_Embedding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": []map[string]any{
				{
					"embedding": []float32{0.123, 0.456, -0.789},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	logger := zaptest.NewLogger(t)
	cfg := llm.Config{
		Provider: "openai",
		APIKey:   "test-key",
		BaseURL:  server.URL,
	}

	client := llm.NewClient(cfg, nil, logger)

	vec, err := client.CreateEmbedding(context.Background(), "Bitcoin breaks resistance")
	if err != nil {
		t.Fatalf("unexpected embedding error: %v", err)
	}

	if len(vec) != 3 || vec[0] != 0.123 {
		t.Errorf("unexpected vector: %+v", vec)
	}
}
