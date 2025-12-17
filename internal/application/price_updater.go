package application

import (
	"context"
	"log/slog"
	"time"
)

type PriceRefresher interface {
	RefreshPrices(ctx context.Context) error
}

type PriceUpdater struct {
	service  PriceRefresher
	interval time.Duration
	stopChan chan struct{}
}

func NewPriceUpdater(service PriceRefresher, interval time.Duration) *PriceUpdater {
	return &PriceUpdater{
		service:  service,
		interval: interval,
		stopChan: make(chan struct{}),
	}
}

func (u *PriceUpdater) Start(ctx context.Context) {
	ticker := time.NewTicker(u.interval)
	defer ticker.Stop()

	slog.Info("Price updater started", "interval", u.interval)

	for {
		select {
		case <-ticker.C:
			if err := u.service.RefreshPrices(ctx); err != nil {
				slog.Error("Error refreshing prices", "error", err)
			} else {
				slog.Info("Prices refreshed successfully")
			}
		case <-u.stopChan:
			slog.Info("Price updater stopped")
			return
		case <-ctx.Done():
			slog.Info("Price updater stopped due to context cancellation")
			return
		}
	}
}

func (u *PriceUpdater) Stop() {
	close(u.stopChan)
}
