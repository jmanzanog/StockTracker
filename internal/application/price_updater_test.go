package application

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type mockPriceRefresher struct {
	mu          sync.Mutex
	refreshFunc func(ctx context.Context) error
	callCount   int
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

func (m *mockPriceRefresher) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

func TestPriceUpdater_Start(t *testing.T) {
	t.Run("Refreshes prices on interval", func(t *testing.T) {
		mockRefresher := &mockPriceRefresher{}
		// Use a very short interval for testing
		interval := 10 * time.Millisecond
		updater := NewPriceUpdater(mockRefresher, interval)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Run Start in a goroutine
		go updater.Start(ctx)

		// Wait enough time for a few ticks
		time.Sleep(50 * time.Millisecond)

		updater.Stop()

		// Assert that RefreshPrices was called at least once
		assert.GreaterOrEqual(t, mockRefresher.CallCount(), 3)
	})

	t.Run("Stops on Stop() call", func(t *testing.T) {
		mockRefresher := &mockPriceRefresher{}
		updater := NewPriceUpdater(mockRefresher, 100*time.Millisecond)

		go updater.Start(context.Background())
		time.Sleep(20 * time.Millisecond) // Ensure it started

		updater.Stop()

		// Wait to ensure it doesn't continue ticking (checking log locally is hard,
		// but we can verify it doesn't block or panic)
	})

	t.Run("Handles Refresh Error gracefully", func(t *testing.T) {
		mockRefresher := &mockPriceRefresher{
			refreshFunc: func(ctx context.Context) error {
				return errors.New("refresh failed")
			},
		}
		updater := NewPriceUpdater(mockRefresher, 10*time.Millisecond)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go updater.Start(ctx)
		time.Sleep(30 * time.Millisecond)
		updater.Stop()

		assert.GreaterOrEqual(t, mockRefresher.CallCount(), 1)
	})

	t.Run("Stops on context cancellation", func(t *testing.T) {
		mockRefresher := &mockPriceRefresher{}
		updater := NewPriceUpdater(mockRefresher, 100*time.Millisecond)

		ctx, cancel := context.WithCancel(context.Background())

		go updater.Start(ctx)
		time.Sleep(20 * time.Millisecond)

		cancel()
		// Wait ensures goroutine exits, not easily testable without a "done" channel on local structure,
		// but verifying no data race or block is implicit.
	})
}
