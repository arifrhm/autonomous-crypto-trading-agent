package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/domain"
	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/storage/postgres"
)

// MockPriceTickRepo is an in-memory test double verifying interface contract
type MockPriceTickRepo struct {
	ticks []domain.PriceTick
}

func (m *MockPriceTickRepo) Insert(ctx context.Context, tick domain.PriceTick) error {
	m.ticks = append(m.ticks, tick)
	return nil
}

func (m *MockPriceTickRepo) InsertBatch(ctx context.Context, ticks []domain.PriceTick) error {
	m.ticks = append(m.ticks, ticks...)
	return nil
}

func (m *MockPriceTickRepo) GetRange(ctx context.Context, symbol string, from, to time.Time) ([]domain.PriceTick, error) {
	var res []domain.PriceTick
	for _, t := range m.ticks {
		if t.Symbol == symbol && !t.Timestamp.Before(from) && !t.Timestamp.After(to) {
			res = append(res, t)
		}
	}
	return res, nil
}

func (m *MockPriceTickRepo) GetLatest(ctx context.Context, symbol string, limit int) ([]domain.PriceTick, error) {
	var res []domain.PriceTick
	for i := len(m.ticks) - 1; i >= 0 && len(res) < limit; i-- {
		if m.ticks[i].Symbol == symbol {
			res = append(res, m.ticks[i])
		}
	}
	return res, nil
}

// Compile-time assertion that MockPriceTickRepo implements postgres.PriceTickRepository
var _ postgres.PriceTickRepository = (*MockPriceTickRepo)(nil)

func TestMockPriceTickRepo(t *testing.T) {
	repo := &MockPriceTickRepo{}
	ctx := context.Background()

	ticks := []domain.PriceTick{
		{Symbol: "BTCUSDT", Price: 65000, Volume: 1.2, Timestamp: time.Now()},
		{Symbol: "BTCUSDT", Price: 65100, Volume: 0.5, Timestamp: time.Now().Add(time.Second)},
		{Symbol: "ETHUSDT", Price: 3500, Volume: 10, Timestamp: time.Now()},
	}

	if err := repo.InsertBatch(ctx, ticks); err != nil {
		t.Fatalf("failed to insert batch: %v", err)
	}

	latest, err := repo.GetLatest(ctx, "BTCUSDT", 2)
	if err != nil {
		t.Fatalf("failed to get latest: %v", err)
	}

	if len(latest) != 2 {
		t.Fatalf("expected 2 latest ticks, got %d", len(latest))
	}

	if latest[0].Price != 65100 {
		t.Errorf("expected latest price 65100, got %f", latest[0].Price)
	}
}
