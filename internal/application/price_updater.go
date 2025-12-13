package application

import (
	"context"
	"log"
	"time"
)

type PriceUpdater struct {
	service  *PortfolioService
	interval time.Duration
	stopChan chan struct{}
}

func NewPriceUpdater(service *PortfolioService, interval time.Duration) *PriceUpdater {
	return &PriceUpdater{
		service:  service,
		interval: interval,
		stopChan: make(chan struct{}),
	}
}

func (u *PriceUpdater) Start(ctx context.Context) {
	ticker := time.NewTicker(u.interval)
	defer ticker.Stop()

	log.Printf("Price updater started with interval: %s", u.interval)

	for {
		select {
		case <-ticker.C:
			if err := u.service.RefreshPrices(ctx); err != nil {
				log.Printf("Error refreshing prices: %v", err)
			} else {
				log.Println("Prices refreshed successfully")
			}
		case <-u.stopChan:
			log.Println("Price updater stopped")
			return
		case <-ctx.Done():
			log.Println("Price updater stopped due to context cancellation")
			return
		}
	}
}

func (u *PriceUpdater) Stop() {
	close(u.stopChan)
}
