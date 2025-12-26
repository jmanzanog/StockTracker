package application

import (
	"context"
	"fmt"
	"testing"

	"github.com/jmanzanog/stock-tracker/internal/domain"
	"github.com/jmanzanog/stock-tracker/internal/infrastructure/marketdata"
)

// mockBatchMarketData implements both MDataProvider and BatchProvider
type mockBatchMarketData struct {
	MockMarketData
	searchByISINBatchFunc func(ctx context.Context, isins []string) []marketdata.SearchResult
	getQuoteBatchFunc     func(ctx context.Context, symbols []string) []marketdata.QuoteBatchResult
}

func (m *mockBatchMarketData) SearchByISINBatch(ctx context.Context, isins []string) []marketdata.SearchResult {
	if m.searchByISINBatchFunc != nil {
		return m.searchByISINBatchFunc(ctx, isins)
	}
	return nil
}

func (m *mockBatchMarketData) GetQuoteBatch(ctx context.Context, symbols []string) []marketdata.QuoteBatchResult {
	if m.getQuoteBatchFunc != nil {
		return m.getQuoteBatchFunc(ctx, symbols)
	}
	return nil
}

func TestAddPositionsBatch_EmptyRequest(t *testing.T) {
	repo := &MockRepository{}
	provider := &MockMarketData{}

	service, err := NewPortfolioService(repo, provider)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	result := service.AddPositionsBatch(context.Background(), []AddPositionBatchRequest{})

	if len(result.Successful) != 0 {
		t.Errorf("expected 0 successful, got %d", len(result.Successful))
	}
	if len(result.Failed) != 0 {
		t.Errorf("expected 0 failed, got %d", len(result.Failed))
	}
}

func TestAddPositionsBatch_WithBatchProvider_Success(t *testing.T) {
	repo := &MockRepository{}

	instrument := domain.NewInstrument("US0378331005", "AAPL", "Apple Inc.", domain.InstrumentTypeStock, "USD", "NASDAQ")

	provider := &mockBatchMarketData{
		searchByISINBatchFunc: func(ctx context.Context, isins []string) []marketdata.SearchResult {
			results := make([]marketdata.SearchResult, 0, len(isins))
			for _, isin := range isins {
				inst := instrument
				inst.ISIN = isin
				results = append(results, marketdata.SearchResult{
					ISIN:       isin,
					Instrument: &inst,
				})
			}
			return results
		},
		getQuoteBatchFunc: func(ctx context.Context, symbols []string) []marketdata.QuoteBatchResult {
			results := make([]marketdata.QuoteBatchResult, 0, len(symbols))
			for _, symbol := range symbols {
				results = append(results, marketdata.QuoteBatchResult{
					Symbol: symbol,
					Quote: &marketdata.QuoteResult{
						Symbol:   symbol,
						Price:    domain.NewDecimalFromInt(150),
						Currency: "USD",
						Time:     "2024-01-01",
					},
				})
			}
			return results
		},
	}

	service, err := NewPortfolioService(repo, provider)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	requests := []AddPositionBatchRequest{
		{ISIN: "US0378331005", InvestedAmount: domain.NewDecimalFromInt(1000), Currency: "USD"},
		{ISIN: "US5949181045", InvestedAmount: domain.NewDecimalFromInt(2000), Currency: "USD"},
	}

	result := service.AddPositionsBatch(context.Background(), requests)

	if len(result.Successful) != 2 {
		t.Errorf("expected 2 successful, got %d", len(result.Successful))
	}
	if len(result.Failed) != 0 {
		t.Errorf("expected 0 failed, got %d", len(result.Failed))
	}
}

func TestAddPositionsBatch_WithBatchProvider_PartialFailure(t *testing.T) {
	repo := &MockRepository{}

	instrument := domain.NewInstrument("US0378331005", "AAPL", "Apple Inc.", domain.InstrumentTypeStock, "USD", "NASDAQ")

	provider := &mockBatchMarketData{
		searchByISINBatchFunc: func(ctx context.Context, isins []string) []marketdata.SearchResult {
			results := make([]marketdata.SearchResult, 0, len(isins))
			for _, isin := range isins {
				if isin == "INVALID" {
					results = append(results, marketdata.SearchResult{
						ISIN:  isin,
						Error: fmt.Errorf("instrument not found"),
					})
				} else {
					inst := instrument
					inst.ISIN = isin
					results = append(results, marketdata.SearchResult{
						ISIN:       isin,
						Instrument: &inst,
					})
				}
			}
			return results
		},
		getQuoteBatchFunc: func(ctx context.Context, symbols []string) []marketdata.QuoteBatchResult {
			results := make([]marketdata.QuoteBatchResult, 0, len(symbols))
			for _, symbol := range symbols {
				results = append(results, marketdata.QuoteBatchResult{
					Symbol: symbol,
					Quote: &marketdata.QuoteResult{
						Symbol:   symbol,
						Price:    domain.NewDecimalFromInt(150),
						Currency: "USD",
						Time:     "2024-01-01",
					},
				})
			}
			return results
		},
	}

	service, err := NewPortfolioService(repo, provider)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	requests := []AddPositionBatchRequest{
		{ISIN: "US0378331005", InvestedAmount: domain.NewDecimalFromInt(1000), Currency: "USD"},
		{ISIN: "INVALID", InvestedAmount: domain.NewDecimalFromInt(2000), Currency: "USD"},
	}

	result := service.AddPositionsBatch(context.Background(), requests)

	if len(result.Successful) != 1 {
		t.Errorf("expected 1 successful, got %d", len(result.Successful))
	}
	if len(result.Failed) != 1 {
		t.Errorf("expected 1 failed, got %d", len(result.Failed))
	}
}

func TestAddPositionsBatch_WithConcurrentFallback(t *testing.T) {
	repo := &MockRepository{}
	// MockMarketData does NOT implement BatchProvider, so it will use concurrent fallback
	provider := &MockMarketData{}

	service, err := NewPortfolioService(repo, provider)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	requests := []AddPositionBatchRequest{
		{ISIN: "US0378331005", InvestedAmount: domain.NewDecimalFromInt(1000), Currency: "USD"},
		{ISIN: "US5949181045", InvestedAmount: domain.NewDecimalFromInt(2000), Currency: "USD"},
	}

	result := service.AddPositionsBatch(context.Background(), requests)

	if len(result.Successful) != 2 {
		t.Errorf("expected 2 successful, got %d", len(result.Successful))
	}
	if len(result.Failed) != 0 {
		t.Errorf("expected 0 failed, got %d", len(result.Failed))
	}
}

func TestAddPositionsBatch_SaveError(t *testing.T) {
	repo := &MockRepository{}

	instrument := domain.NewInstrument("US0378331005", "AAPL", "Apple Inc.", domain.InstrumentTypeStock, "USD", "NASDAQ")

	provider := &mockBatchMarketData{
		searchByISINBatchFunc: func(ctx context.Context, isins []string) []marketdata.SearchResult {
			results := make([]marketdata.SearchResult, 0, len(isins))
			for _, isin := range isins {
				inst := instrument
				inst.ISIN = isin
				results = append(results, marketdata.SearchResult{
					ISIN:       isin,
					Instrument: &inst,
				})
			}
			return results
		},
		getQuoteBatchFunc: func(ctx context.Context, symbols []string) []marketdata.QuoteBatchResult {
			results := make([]marketdata.QuoteBatchResult, 0, len(symbols))
			for _, symbol := range symbols {
				results = append(results, marketdata.QuoteBatchResult{
					Symbol: symbol,
					Quote: &marketdata.QuoteResult{
						Symbol:   symbol,
						Price:    domain.NewDecimalFromInt(150),
						Currency: "USD",
						Time:     "2024-01-01",
					},
				})
			}
			return results
		},
	}

	service, err := NewPortfolioService(repo, provider)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	// Set the save error AFTER creating the service
	repo.saveError = fmt.Errorf("database error")

	requests := []AddPositionBatchRequest{
		{ISIN: "US0378331005", InvestedAmount: domain.NewDecimalFromInt(1000), Currency: "USD"},
	}

	result := service.AddPositionsBatch(context.Background(), requests)

	// When save fails, all successful positions should become failed
	if len(result.Successful) != 0 {
		t.Errorf("expected 0 successful after save error, got %d", len(result.Successful))
	}
	if len(result.Failed) != 1 {
		t.Errorf("expected 1 failed after save error, got %d", len(result.Failed))
	}
}

func TestAddPositionsBatch_QuoteError(t *testing.T) {
	repo := &MockRepository{}

	instrument := domain.NewInstrument("US0378331005", "AAPL", "Apple Inc.", domain.InstrumentTypeStock, "USD", "NASDAQ")

	provider := &mockBatchMarketData{
		searchByISINBatchFunc: func(ctx context.Context, isins []string) []marketdata.SearchResult {
			results := make([]marketdata.SearchResult, 0, len(isins))
			for _, isin := range isins {
				inst := instrument
				inst.ISIN = isin
				results = append(results, marketdata.SearchResult{
					ISIN:       isin,
					Instrument: &inst,
				})
			}
			return results
		},
		getQuoteBatchFunc: func(ctx context.Context, symbols []string) []marketdata.QuoteBatchResult {
			results := make([]marketdata.QuoteBatchResult, 0, len(symbols))
			for _, symbol := range symbols {
				results = append(results, marketdata.QuoteBatchResult{
					Symbol: symbol,
					Error:  fmt.Errorf("quote API unavailable"),
				})
			}
			return results
		},
	}

	service, err := NewPortfolioService(repo, provider)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	requests := []AddPositionBatchRequest{
		{ISIN: "US0378331005", InvestedAmount: domain.NewDecimalFromInt(1000), Currency: "USD"},
	}

	result := service.AddPositionsBatch(context.Background(), requests)

	if len(result.Successful) != 0 {
		t.Errorf("expected 0 successful when quote fails, got %d", len(result.Successful))
	}
	if len(result.Failed) != 1 {
		t.Errorf("expected 1 failed when quote fails, got %d", len(result.Failed))
	}
}
