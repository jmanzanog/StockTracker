package application

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/jmanzanog/stock-tracker/internal/domain"
)

type PriceRefresher interface {
	RefreshPrices(ctx context.Context) error
	GetPortfolioSummary(ctx context.Context) (*domain.Portfolio, error)
}

type PriceUpdater struct {
	service          PriceRefresher
	priceHistoryRepo domain.PriceHistoryRepository
	interval         time.Duration
	stopChan         chan struct{}
}

func NewPriceUpdater(service PriceRefresher, priceHistoryRepo domain.PriceHistoryRepository, interval time.Duration) *PriceUpdater {
	return &PriceUpdater{
		service:          service,
		priceHistoryRepo: priceHistoryRepo,
		interval:         interval,
		stopChan:         make(chan struct{}),
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
				if err := u.captureHistory(ctx); err != nil {
					slog.Warn("Failed to capture price history", "error", err)
				}
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

func (u *PriceUpdater) captureHistory(ctx context.Context) error {
	portfolio, err := u.service.GetPortfolioSummary(ctx)
	if err != nil {
		return err
	}

	if len(portfolio.Positions) == 0 {
		return nil
	}

	now := time.Now().UTC().Truncate(time.Minute)
	history := make([]domain.PriceHistory, 0, len(portfolio.Positions))

	for _, pos := range portfolio.Positions {
		history = append(history, domain.PriceHistory{
			ID:             uuid.New().String(),
			InstrumentISIN: pos.Instrument.ISIN,
			Price:          pos.CurrentPrice,
			Currency:       pos.Instrument.Currency,
			RecordedAt:     now,
			CreatedAt:      now,
		})
	}

	return u.priceHistoryRepo.SaveBatch(ctx, history)
}

func (u *PriceUpdater) Stop() {
	close(u.stopChan)
}
