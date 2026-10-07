package ingestion

import (
	"context"
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

	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/agent/prompts"
	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/domain"
	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/llm"
	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/storage/postgres"
)

var (
	metricSentimentFetches = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "sentiment_fetch_total",
		Help: "Total count of news sentiment fetch executions",
	}, []string{"symbol", "status"})

	metricSentimentScore = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "sentiment_score",
		Help: "Current parsed market sentiment score (-1.0 to 1.0)",
	}, []string{"symbol"})
)

type CryptoPanicResponse struct {
	Results []struct {
		Title     string `json:"title"`
		PublishedAt string `json:"published_at"`
		Domain    string `json:"domain"`
	} `json:"results"`
}

type SentimentFetcherConfig struct {
	Symbols       []string
	APIKey        string
	Interval      time.Duration
	BaseURL       string
	BatchLimit    int
	CacheTTL      time.Duration
}

type SentimentFetcher struct {
	cfg        SentimentFetcherConfig
	logger     *zap.Logger
	llmClient  llm.Client
	renderer   prompts.PromptRenderer
	rdb        *redis.Client
	memoryRepo postgres.MemoryRepository
	httpClient *http.Client
}

func NewSentimentFetcher(
	cfg SentimentFetcherConfig,
	llmClient llm.Client,
	renderer prompts.PromptRenderer,
	rdb *redis.Client,
	memoryRepo postgres.MemoryRepository,
	logger *zap.Logger,
) *SentimentFetcher {
	if cfg.Interval <= 0 {
		cfg.Interval = 5 * time.Minute
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://cryptopanic.com/api/free/v1/posts/"
	}
	if cfg.BatchLimit <= 0 {
		cfg.BatchLimit = 20
	}
	if cfg.CacheTTL <= 0 {
		cfg.CacheTTL = 10 * time.Minute
	}

	return &SentimentFetcher{
		cfg:        cfg,
		logger:     logger.Named("sentiment_fetcher"),
		llmClient:  llmClient,
		renderer:   renderer,
		rdb:        rdb,
		memoryRepo: memoryRepo,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *SentimentFetcher) Start(ctx context.Context) error {
	s.logger.Info("Starting Sentiment Fetcher pipeline",
		zap.Strings("symbols", s.cfg.Symbols),
		zap.Duration("interval", s.cfg.Interval),
	)

	// Run first fetch immediately
	s.fetchAllSymbols(ctx)

	ticker := time.NewTicker(s.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Stopping Sentiment Fetcher pipeline")
			return ctx.Err()
		case <-ticker.C:
			s.fetchAllSymbols(ctx)
		}
	}
}

func (s *SentimentFetcher) fetchAllSymbols(ctx context.Context) {
	for _, sym := range s.cfg.Symbols {
		if err := s.FetchAndScore(ctx, sym); err != nil {
			s.logger.Warn("Failed sentiment cycle", zap.String("symbol", sym), zap.Error(err))
		}
	}
}

func (s *SentimentFetcher) FetchAndScore(ctx context.Context, symbol string) error {
	headlines, err := s.fetchHeadlines(ctx, symbol)
	if err != nil {
		metricSentimentFetches.WithLabelValues(symbol, "fetch_error").Inc()
		return fmt.Errorf("fetch headlines: %w", err)
	}

	if len(headlines) == 0 {
		s.logger.Debug("No headlines returned", zap.String("symbol", symbol))
		return nil
	}

	// Score via LLM
	scoreState, err := s.scoreHeadlinesWithLLM(ctx, symbol, headlines)
	if err != nil {
		metricSentimentFetches.WithLabelValues(symbol, "score_error").Inc()
		return fmt.Errorf("score headlines with LLM: %w", err)
	}

	metricSentimentFetches.WithLabelValues(symbol, "success").Inc()
	metricSentimentScore.WithLabelValues(symbol).Set(scoreState.Score)

	// 1. Save state in Redis
	if s.rdb != nil {
		data, _ := json.Marshal(scoreState)
		cacheKey := fmt.Sprintf("sentiment:%s", strings.ToLower(symbol))
		if err := s.rdb.Set(ctx, cacheKey, data, s.cfg.CacheTTL).Err(); err != nil {
			s.logger.Warn("Failed to save sentiment to Redis", zap.Error(err))
		}
	}

	// 2. Save significant headlines to pgvector memory if memoryRepo available
	if s.memoryRepo != nil && s.llmClient != nil && (scoreState.Score > 0.4 || scoreState.Score < -0.4) {
		summaryText := fmt.Sprintf("[%s] %s (Score: %.2f)", symbol, strings.Join(headlines[:min(3, len(headlines))], " | "), scoreState.Score)
		vec, err := s.llmClient.CreateEmbedding(ctx, summaryText)
		if err == nil {
			_ = s.memoryRepo.Insert(ctx, domain.MarketMemory{
				Symbol:    symbol,
				Category:  "NEWS",
				Content:   summaryText,
				Embedding: vec,
				CreatedAt: time.Now().UTC(),
			})
		}
	}

	s.logger.Info("Market sentiment updated",
		zap.String("symbol", symbol),
		zap.Float64("score", scoreState.Score),
		zap.String("label", scoreState.Label),
		zap.Int("headlines_count", len(headlines)),
	)
	return nil
}

func (s *SentimentFetcher) fetchHeadlines(ctx context.Context, symbol string) ([]string, error) {
	currency := strings.ToUpper(strings.TrimSuffix(strings.TrimSuffix(symbol, "USDT"), "USD"))
	url := fmt.Sprintf("%s?currencies=%s&kind=news", s.cfg.BaseURL, currency)
	if s.cfg.APIKey != "" {
		url += fmt.Sprintf("&auth_token=%s", s.cfg.APIKey)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("cryptopanic api status %d: %s", resp.StatusCode, string(body))
	}

	var panicResp CryptoPanicResponse
	if err := json.NewDecoder(resp.Body).Decode(&panicResp); err != nil {
		return nil, err
	}

	headlines := make([]string, 0, len(panicResp.Results))
	for i, item := range panicResp.Results {
		if i >= s.cfg.BatchLimit {
			break
		}
		if item.Title != "" {
			headlines = append(headlines, item.Title)
		}
	}
	return headlines, nil
}

func (s *SentimentFetcher) scoreHeadlinesWithLLM(ctx context.Context, symbol string, headlines []string) (*domain.SentimentState, error) {
	sysPrompt, err := s.renderer.RenderSentimentSystemPrompt()
	if err != nil {
		return nil, err
	}

	userPrompt, err := s.renderer.RenderSentimentUserPrompt(prompts.SentimentPromptData{
		Symbol:    symbol,
		Headlines: headlines,
	})
	if err != nil {
		return nil, err
	}

	chatResp, err := s.llmClient.Chat(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: sysPrompt},
			{Role: "user", Content: userPrompt},
		},
		ResponseJSON: true,
	})
	if err != nil {
		return nil, err
	}

	var parsed struct {
		Score     float64 `json:"score"`
		Label     string  `json:"label"`
		Reasoning string  `json:"reasoning"`
	}

	if err := json.Unmarshal([]byte(chatResp.Content), &parsed); err != nil {
		return nil, fmt.Errorf("parse sentiment json %q: %w", chatResp.Content, err)
	}

	return &domain.SentimentState{
		Score:       parsed.Score,
		Label:       parsed.Label,
		Headlines:   headlines,
		LastUpdated: time.Now().UTC(),
	}, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
