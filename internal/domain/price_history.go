package domain

import (
	"context"
	"time"
)

type PriceHistory struct {
	ID             string    `json:"id"`
	InstrumentISIN string    `json:"instrument_isin"`
	Price          Decimal   `json:"price"`
	Currency       string    `json:"currency"`
	RecordedAt     time.Time `json:"recorded_at"`
	CreatedAt      time.Time `json:"created_at"`
}

type SparklineRequest struct {
	ISIN string
	Days int
}

type SparklineResult struct {
	ISIN   string
	Days   int
	Points []PriceHistory
}

type PriceHistoryRepository interface {
	SaveBatch(ctx context.Context, history []PriceHistory) error
	GetByISIN(ctx context.Context, isin string, from, to time.Time) ([]PriceHistory, error)
	GetSparkline(ctx context.Context, isin string, days int) ([]PriceHistory, error)
	GetSparklinesBatch(ctx context.Context, requests []SparklineRequest) ([]SparklineResult, error)
	CleanupOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}
