package marketdata

import "github.com/jmanzanog/stock-tracker/internal/domain"

// Type aliases re-export domain types for backward compatibility.
// Canonical definitions now live in the domain layer (domain/marketdata.go).

// QuoteResult is a market data quote — canonical definition in domain.
type QuoteResult = domain.QuoteResult

// SearchResult is a batch search result — canonical definition in domain.
type SearchResult = domain.SearchResult

// QuoteBatchResult is a batch quote result — canonical definition in domain.
type QuoteBatchResult = domain.QuoteBatchResult

// MDataProvider is the market data provider interface — canonical definition in domain.
type MDataProvider = domain.MDataProvider

// BatchProvider extends MDataProvider with batch operations — canonical definition in domain.
type BatchProvider = domain.BatchProvider
