package marketdata

import (
	"context"

	"github.com/jmanzanog/stock-tracker/internal/domain"
)

type QuoteResult struct {
	Symbol   string
	Price    domain.Decimal
	Currency string
	Time     string
}

type MarketDataProvider interface {
	SearchByISIN(ctx context.Context, isin string) (*domain.Instrument, error)
	GetQuote(ctx context.Context, symbol string) (*QuoteResult, error)
}
