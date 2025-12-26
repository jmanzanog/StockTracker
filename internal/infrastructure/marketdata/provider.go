package marketdata

import (
	"context"

	"github.com/jmanzanog/stock-tracker/internal/domain"
)

// QuoteResult represents a single quote result from a market data provider.
type QuoteResult struct {
	Symbol   string
	Price    domain.Decimal
	Currency string
	Time     string
}

// SearchResult represents a single search result in a batch operation.
type SearchResult struct {
	Instrument *domain.Instrument
	ISIN       string
	Error      error
}

// QuoteBatchResult represents results from a batch quote operation.
type QuoteBatchResult struct {
	Symbol string
	Quote  *QuoteResult
	Error  error
}

// MDataProvider defines the interface for market data providers.
type MDataProvider interface {
	SearchByISIN(ctx context.Context, isin string) (*domain.Instrument, error)
	GetQuote(ctx context.Context, symbol string) (*QuoteResult, error)
}

// BatchProvider defines optional batch operations for providers that support them.
// Providers like TwelveData and YFinance implement this interface.
// Finnhub does not support batch and will use the fallback concurrent implementation.
type BatchProvider interface {
	MDataProvider
	SearchByISINBatch(ctx context.Context, isins []string) []SearchResult
	GetQuoteBatch(ctx context.Context, symbols []string) []QuoteBatchResult
}
