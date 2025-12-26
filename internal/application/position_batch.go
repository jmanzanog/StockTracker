package application

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/jmanzanog/stock-tracker/internal/domain"
	"github.com/jmanzanog/stock-tracker/internal/infrastructure/marketdata"
)

// AddPositionBatchRequest represents a single position request in a batch.
type AddPositionBatchRequest struct {
	ISIN           string         `json:"isin"`
	InvestedAmount domain.Decimal `json:"invested_amount"`
	Currency       string         `json:"currency"`
}

// AddPositionResult represents the result of adding a single position.
type AddPositionResult struct {
	ISIN     string           `json:"isin"`
	Position *domain.Position `json:"position,omitempty"`
	Error    string           `json:"error,omitempty"`
}

// AddPositionsBatchResult represents the result of a batch position creation.
type AddPositionsBatchResult struct {
	Successful []AddPositionResult `json:"successful"`
	Failed     []AddPositionResult `json:"failed"`
}

// AddPositionsBatch adds multiple positions in batch.
// It prioritizes batch API calls when the provider supports them,
// falling back to concurrent individual calls with goroutines/channels otherwise.
func (s *PortfolioService) AddPositionsBatch(ctx context.Context, requests []AddPositionBatchRequest) *AddPositionsBatchResult {
	result := &AddPositionsBatchResult{
		Successful: make([]AddPositionResult, 0),
		Failed:     make([]AddPositionResult, 0),
	}

	if len(requests) == 0 {
		return result
	}

	// Extract ISINs for batch search
	isins := make([]string, len(requests))
	requestMap := make(map[string]AddPositionBatchRequest)
	for i, req := range requests {
		isins[i] = req.ISIN
		requestMap[req.ISIN] = req
	}

	// Try batch provider first, fall back to concurrent calls
	var instruments map[string]*domain.Instrument
	var instrumentErrors map[string]error

	if batchProvider, ok := s.marketData.(marketdata.BatchProvider); ok {
		slog.InfoContext(ctx, "Using batch provider for instrument search", "count", len(isins))
		instruments, instrumentErrors = s.searchInstrumentsBatch(ctx, batchProvider, isins)
	} else {
		slog.InfoContext(ctx, "Batch provider not available, using concurrent search", "count", len(isins))
		instruments, instrumentErrors = s.searchInstrumentsConcurrent(ctx, isins)
	}

	// Process instruments that failed to be found
	for isin, err := range instrumentErrors {
		result.Failed = append(result.Failed, AddPositionResult{
			ISIN:  isin,
			Error: err.Error(),
		})
		// Remove from requests to process
		delete(requestMap, isin)
	}

	// Get quotes for found instruments
	symbols := make([]string, 0, len(instruments))
	symbolToISIN := make(map[string]string)
	for isin, inst := range instruments {
		symbols = append(symbols, inst.Symbol)
		symbolToISIN[inst.Symbol] = isin
	}

	var quotes map[string]*marketdata.QuoteResult
	var quoteErrors map[string]error

	if batchProvider, ok := s.marketData.(marketdata.BatchProvider); ok {
		slog.InfoContext(ctx, "Using batch provider for quotes", "count", len(symbols))
		quotes, quoteErrors = s.getQuotesBatch(ctx, batchProvider, symbols)
	} else {
		slog.InfoContext(ctx, "Batch provider not available, using concurrent quotes", "count", len(symbols))
		quotes, quoteErrors = s.getQuotesConcurrent(ctx, symbols)
	}

	// Process quote errors
	for symbol, err := range quoteErrors {
		isin := symbolToISIN[symbol]
		result.Failed = append(result.Failed, AddPositionResult{
			ISIN:  isin,
			Error: fmt.Sprintf("failed to get quote: %v", err),
		})
		delete(requestMap, isin)
	}

	// Create positions for successful instruments and quotes
	for isin, req := range requestMap {
		instrument := instruments[isin]
		if instrument == nil {
			continue
		}

		quote := quotes[instrument.Symbol]
		if quote == nil {
			continue
		}

		position := domain.NewPosition(*instrument, req.InvestedAmount, req.Currency)

		price, err := domain.NewDecimalFromString(quote.Price.String())
		if err != nil {
			result.Failed = append(result.Failed, AddPositionResult{
				ISIN:  isin,
				Error: fmt.Sprintf("failed to parse price: %v", err),
			})
			continue
		}

		if err := position.UpdatePrice(price); err != nil {
			result.Failed = append(result.Failed, AddPositionResult{
				ISIN:  isin,
				Error: fmt.Sprintf("failed to update price: %v", err),
			})
			continue
		}

		if err := s.defaultPortfolio.AddPosition(position); err != nil {
			result.Failed = append(result.Failed, AddPositionResult{
				ISIN:  isin,
				Error: fmt.Sprintf("failed to add to portfolio: %v", err),
			})
			continue
		}

		result.Successful = append(result.Successful, AddPositionResult{
			ISIN:     isin,
			Position: &position,
		})
	}

	// Save portfolio if we have any successful positions
	if len(result.Successful) > 0 {
		if err := s.repo.Save(ctx, s.defaultPortfolio); err != nil {
			slog.ErrorContext(ctx, "Failed to save portfolio after batch add", "error", err)
			// Move all successful to failed
			for _, pos := range result.Successful {
				result.Failed = append(result.Failed, AddPositionResult{
					ISIN:  pos.ISIN,
					Error: fmt.Sprintf("failed to save portfolio: %v", err),
				})
			}
			result.Successful = nil
		}
	}

	return result
}

// searchInstrumentsBatch uses the batch provider to search for instruments.
func (s *PortfolioService) searchInstrumentsBatch(ctx context.Context, provider marketdata.BatchProvider, isins []string) (map[string]*domain.Instrument, map[string]error) {
	instruments := make(map[string]*domain.Instrument)
	errors := make(map[string]error)

	results := provider.SearchByISINBatch(ctx, isins)
	for _, r := range results {
		if r.Error != nil {
			errors[r.ISIN] = r.Error
		} else {
			instruments[r.ISIN] = r.Instrument
		}
	}

	return instruments, errors
}

// searchInstrumentsConcurrent searches for instruments concurrently using goroutines and channels.
// This is used as a fallback when the provider doesn't support batch operations.
func (s *PortfolioService) searchInstrumentsConcurrent(ctx context.Context, isins []string) (map[string]*domain.Instrument, map[string]error) {
	instruments := make(map[string]*domain.Instrument)
	errors := make(map[string]error)
	var mu sync.Mutex

	type searchResult struct {
		isin       string
		instrument *domain.Instrument
		err        error
	}

	resultChan := make(chan searchResult, len(isins))
	var wg sync.WaitGroup

	for _, isin := range isins {
		wg.Add(1)
		go func(isin string) {
			defer wg.Done()

			instrument, err := s.marketData.SearchByISIN(ctx, isin)
			resultChan <- searchResult{
				isin:       isin,
				instrument: instrument,
				err:        err,
			}
		}(isin)
	}

	// Close channel when all goroutines complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	for r := range resultChan {
		mu.Lock()
		if r.err != nil {
			errors[r.isin] = r.err
		} else {
			instruments[r.isin] = r.instrument
		}
		mu.Unlock()
	}

	return instruments, errors
}

// getQuotesBatch uses the batch provider to get quotes.
func (s *PortfolioService) getQuotesBatch(ctx context.Context, provider marketdata.BatchProvider, symbols []string) (map[string]*marketdata.QuoteResult, map[string]error) {
	quotes := make(map[string]*marketdata.QuoteResult)
	errors := make(map[string]error)

	results := provider.GetQuoteBatch(ctx, symbols)
	for _, r := range results {
		if r.Error != nil {
			errors[r.Symbol] = r.Error
		} else {
			quotes[r.Symbol] = r.Quote
		}
	}

	return quotes, errors
}

// getQuotesConcurrent gets quotes concurrently using goroutines and channels.
// This is used as a fallback when the provider doesn't support batch operations.
func (s *PortfolioService) getQuotesConcurrent(ctx context.Context, symbols []string) (map[string]*marketdata.QuoteResult, map[string]error) {
	quotes := make(map[string]*marketdata.QuoteResult)
	errors := make(map[string]error)
	var mu sync.Mutex

	type quoteResult struct {
		symbol string
		quote  *marketdata.QuoteResult
		err    error
	}

	resultChan := make(chan quoteResult, len(symbols))
	var wg sync.WaitGroup

	for _, symbol := range symbols {
		wg.Add(1)
		go func(symbol string) {
			defer wg.Done()

			quote, err := s.marketData.GetQuote(ctx, symbol)
			resultChan <- quoteResult{
				symbol: symbol,
				quote:  quote,
				err:    err,
			}
		}(symbol)
	}

	// Close channel when all goroutines complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	for r := range resultChan {
		mu.Lock()
		if r.err != nil {
			errors[r.symbol] = r.err
		} else {
			quotes[r.symbol] = r.quote
		}
		mu.Unlock()
	}

	return quotes, errors
}
