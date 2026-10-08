package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/agent"
	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/config"
	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/ingestion"
	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/storage/postgres"
	"github.com/arifrhm/autonomous-crypto-trading-agent/pkg/logger"
)

func main() {
	// 1. Load Configurations
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("FATAL: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// 2. Initialize Structured Logger
	log, err := logger.New(cfg.App.Env, "info")
	if err != nil {
		fmt.Printf("FATAL: Failed to initialize zap logger: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = log.Sync() }()

	log.Info("Starting Autonomous Crypto Trading Agent",
		zap.String("env", cfg.App.Env),
		zap.Int("port", cfg.App.Port),
		zap.Strings("symbols", cfg.Binance.Symbols),
	)

	// Root context with graceful termination listener
	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 3. Connect to PostgreSQL
	pgConfig := postgres.Config{
		Host:            cfg.Database.Host,
		Port:            cfg.Database.Port,
		User:            cfg.Database.User,
		Password:        cfg.Database.Password,
		Database:        cfg.Database.Name,
		SSLMode:         cfg.Database.SSLMode,
		MaxConns:        cfg.Database.MaxConns,
		MinConns:        cfg.Database.MinConns,
		MaxConnLifetime: cfg.Database.MaxConnLifetime,
		MaxConnIdleTime: cfg.Database.MaxConnIdleTime,
	}

	pgPool, err := postgres.NewPool(rootCtx, pgConfig)
	if err != nil {
		log.Warn("Failed to connect to PostgreSQL (will attempt reconnect if infra starting)", zap.Error(err))
	} else {
		defer pgPool.Close()
		log.Info("PostgreSQL connected successfully")

		// 4. Run database migrations
		if err := postgres.RunMigrations(rootCtx, pgPool, "migrations", log); err != nil {
			log.Error("Migration run failed", zap.Error(err))
		}
	}

	// 5. Connect to Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer func() { _ = rdb.Close() }()

	pingCtx, cancelPing := context.WithTimeout(rootCtx, 3*time.Second)
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		log.Warn("Redis connection check failed", zap.Error(err))
	} else {
		log.Info("Redis connected successfully")
	}
	cancelPing()

	// 6. Initialize WebSocket Ingestion & Batch Flusher
	wsCfg := ingestion.Config{
		BaseWSURL:   cfg.Binance.WSURL,
		Symbols:     cfg.Binance.Symbols,
		MinBackoff:  1 * time.Second,
		MaxBackoff:  30 * time.Second,
		PingPeriod:  20 * time.Second,
		ChannelSize: 2000,
	}

	wsClient, err := ingestion.NewBinanceClient(wsCfg, log)
	if err != nil {
		log.Fatal("Failed to create Binance client", zap.Error(err))
	}

	var priceRepo postgres.PriceTickRepository
	if pgPool != nil {
		priceRepo = postgres.NewPriceTickRepository(pgPool)
	}

	// 7. Start Background Ingestion Goroutines
	go func() {
		log.Info("Starting Binance WebSocket ingestion worker")
		if err := wsClient.Start(rootCtx); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("Binance WebSocket client encountered fatal error", zap.Error(err))
		}
	}()

	if priceRepo != nil {
		flusher := postgres.NewTickBatchFlusher(priceRepo, postgres.BatchFlusherConfig{
			BatchSize:     100,
			FlushInterval: 1 * time.Second,
		}, log)

		go func() {
			log.Info("Starting TickBatchFlusher worker (batch: 100, interval: 1s)")
			if err := flusher.Run(rootCtx, wsClient.Trades()); err != nil && !errors.Is(err, context.Canceled) {
				log.Error("Batch flusher encountered fatal error", zap.Error(err))
			}
		}()
	} else {
		// Drain channel in memory if DB is unavailable
		go func() {
			for trade := range wsClient.Trades() {
				log.Debug("Trade received (no DB pool)", zap.String("symbol", trade.Symbol), zap.Float64("price", trade.Price))
			}
		}()
	}

	// 8. Setup HTTP Server (/health, /metrics, /api/telemetry)
	var tradeRepo postgres.TradeRepository
	var decisionRepo postgres.DecisionRepository
	if pgPool != nil {
		tradeRepo = postgres.NewTradeRepository(pgPool)
		decisionRepo = postgres.NewDecisionRepository(pgPool)
	}

	telemetryHandler := agent.NewTelemetryHandler(pgPool, rdb, tradeRepo, decisionRepo, priceRepo, log)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/api/telemetry", telemetryHandler)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		status := map[string]any{
			"status":    "healthy",
			"timestamp": time.Now().UTC(),
			"postgres":  checkPostgresHealth(pgPool),
			"redis":     checkRedisHealth(rdb),
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(status)
	})

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.App.Port),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		log.Info("HTTP server running", zap.Int("port", cfg.App.Port))
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("HTTP server failed", zap.Error(err))
		}
	}()

	// 9. Await Shutdown Signal & Graceful Exit
	<-rootCtx.Done()
	log.Info("Shutdown signal received, performing graceful shutdown...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error("HTTP server shutdown error", zap.Error(err))
	}

	log.Info("Autonomous Crypto Trading Agent stopped successfully")
}

func checkPostgresHealth(pool *pgxpool.Pool) string {
	if pool == nil {
		return "disconnected"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		return "unhealthy"
	}
	return "healthy"
}

func checkRedisHealth(rdb *redis.Client) string {
	if rdb == nil {
		return "disconnected"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return "unhealthy"
	}
	return "healthy"
}
