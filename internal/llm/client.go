package llm

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

var (
	metricLLMCalls = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "llm_calls_total",
		Help: "Total number of LLM inference requests made",
	}, []string{"provider", "model", "status"})

	metricLLMTokens = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "llm_tokens_total",
		Help: "Total tokens consumed by LLM operations",
	}, []string{"provider", "type"})

	metricLLMCostUSD = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "llm_cost_usd_total",
		Help: "Total estimated USD cost of LLM calls",
	}, []string{"provider", "model"})

	metricLLMLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "llm_latency_seconds",
		Help:    "Inference latency duration in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"provider", "model"})
)

// Config defines LLM Client options.
type Config struct {
	Provider       string        // "openai" or "anthropic"
	APIKey         string
	Model          string        // default: "gpt-4o-mini"
	EmbeddingModel string        // default: "text-embedding-3-small"
	CacheTTL       time.Duration // default: 5m
	BaseURL        string
}

type ResilientLLMClient struct {
	cfg        Config
	logger     *zap.Logger
	rdb        *redis.Client
	httpClient *http.Client
	cb         *CircuitBreaker
}

// NewClient creates a resilient multi-provider LLM client with retry, breaker, and Redis caching.
func NewClient(cfg Config, rdb *redis.Client, logger *zap.Logger) *ResilientLLMClient {
	if cfg.Model == "" {
		cfg.Model = "gpt-4o-mini"
	}
	if cfg.EmbeddingModel == "" {
		cfg.EmbeddingModel = "text-embedding-3-small"
	}
	if cfg.CacheTTL <= 0 {
		cfg.CacheTTL = 5 * time.Minute
	}
	if cfg.BaseURL == "" {
		if strings.ToLower(cfg.Provider) == "anthropic" {
			cfg.BaseURL = "https://api.anthropic.com/v1"
		} else {
			cfg.BaseURL = "https://api.openai.com/v1"
		}
	}

	return &ResilientLLMClient{
		cfg:        cfg,
		logger:     logger.Named("llm_client"),
		rdb:        rdb,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		cb:         NewCircuitBreaker(5, 1*time.Minute, 5*time.Minute),
	}
}

func (c *ResilientLLMClient) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if err := c.cb.Allow(); err != nil {
		metricLLMCalls.WithLabelValues(c.cfg.Provider, c.cfg.Model, "circuit_broken").Inc()
		return nil, err
	}

	model := c.cfg.Model
	if req.ModelOverride != "" {
		model = req.ModelOverride
	}

	// 1. Check Redis Cache
	cacheKey := c.generateCacheKey(model, req)
	if c.rdb != nil {
		cachedBytes, err := c.rdb.Get(ctx, cacheKey).Bytes()
		if err == nil {
			var resp ChatResponse
			if err := json.Unmarshal(cachedBytes, &resp); err == nil {
				resp.Cached = true
				metricLLMCalls.WithLabelValues(c.cfg.Provider, model, "cached").Inc()
				c.logger.Debug("Serving LLM chat response from Redis cache", zap.String("key", cacheKey))
				return &resp, nil
			}
		}
	}

	// 2. Call LLM with Retries for 429/500/503
	var lastErr error
	var resp *ChatResponse
	backoff := 500 * time.Millisecond

	for attempt := 1; attempt <= 3; attempt++ {
		start := time.Now()
		resp, lastErr = c.doChatRequest(ctx, model, req)
		duration := time.Since(start)

		if lastErr == nil {
			c.cb.RecordSuccess()
			resp.Latency = duration
			c.recordMetrics(c.cfg.Provider, model, resp, duration)

			// Store in Redis Cache
			if c.rdb != nil {
				data, _ := json.Marshal(resp)
				_ = c.rdb.Set(ctx, cacheKey, data, c.cfg.CacheTTL).Err()
			}
			return resp, nil
		}

		c.cb.RecordFailure()
		c.logger.Warn("LLM call attempt failed",
			zap.Int("attempt", attempt),
			zap.Error(lastErr),
			zap.Duration("backoff", backoff),
		)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
		backoff *= 2
	}

	metricLLMCalls.WithLabelValues(c.cfg.Provider, model, "error").Inc()
	return nil, fmt.Errorf("llm chat failed after 3 attempts: %w", lastErr)
}

func (c *ResilientLLMClient) CreateEmbedding(ctx context.Context, text string) ([]float32, error) {
	if err := c.cb.Allow(); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/embeddings", c.cfg.BaseURL)
	payload := map[string]any{
		"model": c.cfg.EmbeddingModel,
		"input": text,
	}

	jsonBytes, _ := json.Marshal(payload)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	res, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.cb.RecordFailure()
		return nil, fmt.Errorf("execute embedding request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		c.cb.RecordFailure()
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("embedding api returned %d: %s", res.StatusCode, string(body))
	}

	c.cb.RecordSuccess()

	var embResp struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}

	if err := json.NewDecoder(res.Body).Decode(&embResp); err != nil {
		return nil, fmt.Errorf("decode embedding response: %w", err)
	}

	if len(embResp.Data) == 0 {
		return nil, fmt.Errorf("no embedding vector in response")
	}

	return embResp.Data[0].Embedding, nil
}

func (c *ResilientLLMClient) doChatRequest(ctx context.Context, model string, req ChatRequest) (*ChatResponse, error) {
	url := fmt.Sprintf("%s/chat/completions", c.cfg.BaseURL)

	bodyMap := map[string]any{
		"model":    model,
		"messages": req.Messages,
	}
	if req.Temperature > 0 {
		bodyMap["temperature"] = req.Temperature
	}
	if req.MaxTokens > 0 {
		bodyMap["max_tokens"] = req.MaxTokens
	}
	if req.ResponseJSON {
		bodyMap["response_format"] = map[string]string{"type": "json_object"}
	}

	bodyBytes, _ := json.Marshal(bodyMap)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("api error status %d: %s", httpResp.StatusCode, string(respBody))
	}

	var openAIResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(httpResp.Body).Decode(&openAIResp); err != nil {
		return nil, fmt.Errorf("decode chat response: %w", err)
	}

	if len(openAIResp.Choices) == 0 {
		return nil, fmt.Errorf("empty completion choices received")
	}

	// Cost Calculation (e.g. gpt-4o-mini: $0.15 / 1M prompt, $0.60 / 1M completion)
	cost := (float64(openAIResp.Usage.PromptTokens)*0.15 + float64(openAIResp.Usage.CompletionTokens)*0.60) / 1000000.0

	return &ChatResponse{
		Content:          openAIResp.Choices[0].Message.Content,
		PromptTokens:     openAIResp.Usage.PromptTokens,
		CompletionTokens: openAIResp.Usage.CompletionTokens,
		TotalTokens:      openAIResp.Usage.TotalTokens,
		CostUSD:          cost,
	}, nil
}

func (c *ResilientLLMClient) generateCacheKey(model string, req ChatRequest) string {
	h := sha256.New()
	h.Write([]byte(model))
	for _, m := range req.Messages {
		h.Write([]byte(m.Role))
		h.Write([]byte(m.Content))
	}
	if req.ResponseJSON {
		h.Write([]byte("json"))
	}
	return fmt.Sprintf("llm:cache:%s", hex.EncodeToString(h.Sum(nil)))
}

func (c *ResilientLLMClient) recordMetrics(provider, model string, resp *ChatResponse, duration time.Duration) {
	metricLLMCalls.WithLabelValues(provider, model, "success").Inc()
	metricLLMTokens.WithLabelValues(provider, "prompt").Add(float64(resp.PromptTokens))
	metricLLMTokens.WithLabelValues(provider, "completion").Add(float64(resp.CompletionTokens))
	metricLLMCostUSD.WithLabelValues(provider, model).Add(resp.CostUSD)
	metricLLMLatency.WithLabelValues(provider, model).Observe(duration.Seconds())
}
