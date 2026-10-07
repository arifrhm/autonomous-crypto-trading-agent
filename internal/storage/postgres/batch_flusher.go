package postgres

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/domain"
)

type BatchFlusherConfig struct {
	BatchSize     int
	FlushInterval time.Duration
}

// TickBatchFlusher collects price ticks and flushes them in batches to PostgreSQL.
type TickBatchFlusher struct {
	repo   PriceTickRepository
	logger *zap.Logger
	cfg    BatchFlusherConfig
	buffer []domain.PriceTick
}

func NewTickBatchFlusher(repo PriceTickRepository, cfg BatchFlusherConfig, logger *zap.Logger) *TickBatchFlusher {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 1 * time.Second
	}

	return &TickBatchFlusher{
		repo:   repo,
		logger: logger.Named("batch_flusher"),
		cfg:    cfg,
		buffer: make([]domain.PriceTick, 0, cfg.BatchSize),
	}
}

// Run consumes trades from incoming channel, buffers them, and writes batches to DB.
func (f *TickBatchFlusher) Run(ctx context.Context, tradeCh <-chan domain.Trade) error {
	ticker := time.NewTicker(f.cfg.FlushInterval)
	defer ticker.Stop()

	defer func() {
		// Final flush on exit
		if len(f.buffer) > 0 {
			f.logger.Info("Flushing remaining buffer on shutdown", zap.Int("count", len(f.buffer)))
			flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := f.flush(flushCtx); err != nil {
				f.logger.Error("Failed final flush", zap.Error(err))
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			f.logger.Info("Stopping batch flusher worker")
			return ctx.Err()

		case trade, ok := <-tradeCh:
			if !ok {
				f.logger.Info("Trade channel closed, stopping flusher")
				return nil
			}

			tick := domain.PriceTick{
				Symbol:       trade.Symbol,
				Price:        trade.Price,
				Volume:       trade.Quantity,
				TradeID:      trade.TradeID,
				IsBuyerMaker: trade.IsBuyerMaker,
				Timestamp:    trade.TradeTime,
			}
			f.buffer = append(f.buffer, tick)

			if len(f.buffer) >= f.cfg.BatchSize {
				if err := f.flush(ctx); err != nil {
					f.logger.Error("Batch size flush failed", zap.Error(err))
				}
			}

		case <-ticker.C:
			if len(f.buffer) > 0 {
				if err := f.flush(ctx); err != nil {
					f.logger.Error("Interval flush failed", zap.Error(err))
				}
			}
		}
	}
}

func (f *TickBatchFlusher) flush(ctx context.Context) error {
	if len(f.buffer) == 0 {
		return nil
	}

	start := time.Now()
	count := len(f.buffer)
	toInsert := make([]domain.PriceTick, count)
	copy(toInsert, f.buffer)
	f.buffer = f.buffer[:0] // Reset slice keeping capacity

	err := f.repo.InsertBatch(ctx, toInsert)
	if err != nil {
		return fmt.Errorf("flush batch of %d items: %w", count, err)
	}

	f.logger.Debug("Flushed price tick batch to PostgreSQL",
		zap.Int("count", count),
		zap.Duration("duration", time.Since(start)),
	)
	return nil
}
