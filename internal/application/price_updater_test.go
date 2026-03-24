package application

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jmanzanog/stock-tracker/internal/domain"

	"github.com/stretchr/testify/assert"
)

type mockPriceRefresher struct {
	mu               sync.Mutex
	refreshFunc      func(ctx context.Context) error
	getPortfolioFunc func(ctx context.Context) (*domain.Portfolio, error)
	callCount        int
}

func (m *mockPriceRefresher) RefreshPrices(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	if m.refreshFunc != nil {
		return m.refreshFunc(ctx)
	}
	return nil
}

func (m *mockPriceRefresher) GetPortfolioSummary(ctx context.Context) (*domain.Portfolio, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.getPortfolioFunc != nil {
		return m.getPortfolioFunc(ctx)
	}
	return &domain.Portfolio{Positions: []domain.Position{}}, nil
}

func (m *mockPriceRefresher) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

type mockPriceHistoryRepo struct{}

func (m *mockPriceHistoryRepo) SaveBatch(ctx context.Context, history []domain.PriceHistory) error {
	return nil
}

func (m *mockPriceHistoryRepo) GetByISIN(ctx context.Context, isin string, from, to time.Time) ([]domain.PriceHistory, error) {
	return nil, nil
}

func (m *mockPriceHistoryRepo) GetSparkline(ctx context.Context, isin string, days int) ([]domain.PriceHistory, error) {
	return nil, nil
}

func (m *mockPriceHistoryRepo) GetSparklinesBatch(ctx context.Context, requests []domain.SparklineRequest) ([]domain.SparklineResult, error) {
	return []domain.SparklineResult{}, nil
}

func (m *mockPriceHistoryRepo) CleanupOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	return 0, nil
}

func TestPriceUpdater_Start(t *testing.T) {
	t.Run("Refreshes prices on interval", func(t *testing.T) {
		mockRefresher := &mockPriceRefresher{}
		mockHistoryRepo := &mockPriceHistoryRepo{}
		interval := 10 * time.Millisecond
		updater := NewPriceUpdater(mockRefresher, mockHistoryRepo, interval)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go updater.Start(ctx)

		time.Sleep(100 * time.Millisecond)

		updater.Stop()

		assert.GreaterOrEqual(t, mockRefresher.CallCount(), 3)
	})

	t.Run("Stops on Stop() call", func(t *testing.T) {
		mockRefresher := &mockPriceRefresher{}
		mockHistoryRepo := &mockPriceHistoryRepo{}
		updater := NewPriceUpdater(mockRefresher, mockHistoryRepo, 100*time.Millisecond)

		go updater.Start(context.Background())
		time.Sleep(20 * time.Millisecond)

		updater.Stop()
	})

	t.Run("Handles Refresh Error gracefully", func(t *testing.T) {
		mockRefresher := &mockPriceRefresher{
			refreshFunc: func(ctx context.Context) error {
				return errors.New("refresh failed")
			},
		}
		mockHistoryRepo := &mockPriceHistoryRepo{}
		updater := NewPriceUpdater(mockRefresher, mockHistoryRepo, 10*time.Millisecond)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go updater.Start(ctx)
		time.Sleep(30 * time.Millisecond)
		updater.Stop()

		assert.GreaterOrEqual(t, mockRefresher.CallCount(), 1)
	})

	t.Run("Stops on context cancellation", func(t *testing.T) {
		mockRefresher := &mockPriceRefresher{}
		mockHistoryRepo := &mockPriceHistoryRepo{}
		updater := NewPriceUpdater(mockRefresher, mockHistoryRepo, 100*time.Millisecond)

		ctx, cancel := context.WithCancel(context.Background())

		go updater.Start(ctx)
		time.Sleep(20 * time.Millisecond)

		cancel()
	})
}
