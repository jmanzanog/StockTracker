package marketdata

import (
	"context"

	"github.com/josemanzano/stock-tracker/internal/domain"
	"github.com/shopspring/decimal"
)

type QuoteResult struct {
	Symbol   string
	Price    decimal.Decimal
	Currency string
	Time     string
}

type MarketDataProvider interface {
	SearchByISIN(ctx context.Context, isin string) (*domain.Instrument, error)
	GetQuote(ctx context.Context, symbol string) (*QuoteResult, error)
}
