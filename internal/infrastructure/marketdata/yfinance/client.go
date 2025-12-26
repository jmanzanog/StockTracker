package yfinance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/jmanzanog/stock-tracker/internal/domain"
	"github.com/jmanzanog/stock-tracker/internal/infrastructure/marketdata"
)

const (
	defaultBaseURL  = "http://localhost:8000"
	searchPath      = "/api/v1/search"
	quotePath       = "/api/v1/quote"
	searchBatchPath = "/api/v1/search/batch"
	quoteBatchPath  = "/api/v1/quote/batch"
)

// Client implements the MDataProvider interface using the yfinance-based Market Data Service.
// This is a lightweight Python microservice that provides stock market data via REST API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new yfinance Market Data Service client with default settings.
func NewClient() *Client {
	return &Client{
		baseURL: defaultBaseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// NewClientWithBaseURL creates a new client with a custom base URL (useful for K8s deployments).
func NewClientWithBaseURL(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// NewClientWithHTTPClient creates a new client with a custom HTTP client (for testing).
func NewClientWithHTTPClient(httpClient *http.Client) *Client {
	return &Client{
		baseURL:    defaultBaseURL,
		httpClient: httpClient,
	}
}

// SetBaseURL sets the base URL for the API (useful for testing).
func (c *Client) SetBaseURL(baseURL string) {
	c.baseURL = baseURL
}

// searchResponse represents the response from the search endpoint.
type searchResponse struct {
	ISIN     string `json:"isin"`
	Symbol   string `json:"symbol"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Currency string `json:"currency"`
	Exchange string `json:"exchange"`
}

// quoteResponse represents the response from the quote endpoint.
type quoteResponse struct {
	Symbol   string `json:"symbol"`
	Price    string `json:"price"`
	Currency string `json:"currency"`
	Time     string `json:"time"`
}

// errorResponse represents an error response from the API.
type errorResponse struct {
	Detail string `json:"detail"`
}

// SearchByISIN searches for an instrument by its ISIN using the Market Data Service.
func (c *Client) SearchByISIN(ctx context.Context, isin string) (*domain.Instrument, error) {
	reqURL := fmt.Sprintf("%s%s/%s", c.baseURL, searchPath, isin)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Warn("failed to close response body", "error", closeErr, "url", reqURL)
		}
	}()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("no instrument found for ISIN: %s", isin)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		var errResp errorResponse
		if json.Unmarshal(body, &errResp) == nil && errResp.Detail != "" {
			return nil, fmt.Errorf("API error: %s", errResp.Detail)
		}
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var searchResp searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	instrumentType := mapInstrumentType(searchResp.Type)

	instrument := domain.NewInstrument(
		searchResp.ISIN,
		searchResp.Symbol,
		searchResp.Name,
		instrumentType,
		searchResp.Currency,
		searchResp.Exchange,
	)

	return &instrument, nil
}

// GetQuote retrieves the current quote for a symbol using the Market Data Service.
func (c *Client) GetQuote(ctx context.Context, symbol string) (*marketdata.QuoteResult, error) {
	reqURL := fmt.Sprintf("%s%s/%s", c.baseURL, quotePath, symbol)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Warn("failed to close response body", "error", closeErr, "url", reqURL)
		}
	}()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("no quote found for symbol: %s", symbol)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		var errResp errorResponse
		if json.Unmarshal(body, &errResp) == nil && errResp.Detail != "" {
			return nil, fmt.Errorf("API error: %s", errResp.Detail)
		}
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var quoteResp quoteResponse
	if err := json.NewDecoder(resp.Body).Decode(&quoteResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if quoteResp.Price == "" {
		return nil, fmt.Errorf("quote request returned no price data for symbol: %s", symbol)
	}

	price, err := domain.NewDecimalFromString(quoteResp.Price)
	if err != nil {
		return nil, fmt.Errorf("failed to parse price: %w", err)
	}

	return &marketdata.QuoteResult{
		Symbol:   quoteResp.Symbol,
		Price:    price,
		Currency: quoteResp.Currency,
		Time:     quoteResp.Time,
	}, nil
}

// mapInstrumentType maps the API type string to domain InstrumentType.
func mapInstrumentType(apiType string) domain.InstrumentType {
	switch apiType {
	case "etf", "ETF":
		return domain.InstrumentTypeETF
	default:
		return domain.InstrumentTypeStock
	}
}

// Batch request/response types for the yfinance microservice.

// searchBatchRequest is the request body for the batch search endpoint.
type searchBatchRequest struct {
	ISINs []string `json:"isins"`
}

// searchBatchResponse is the response from the batch search endpoint.
type searchBatchResponse struct {
	Results []searchResponse   `json:"results"`
	Errors  []searchBatchError `json:"errors"`
}

// searchBatchError represents an error for a single ISIN in a batch search.
type searchBatchError struct {
	ISIN  string `json:"isin"`
	Error string `json:"error"`
}

// quoteBatchRequest is the request body for the batch quote endpoint.
type quoteBatchRequest struct {
	Symbols []string `json:"symbols"`
}

// quoteBatchResponse is the response from the batch quote endpoint.
type quoteBatchResponse struct {
	Results []quoteResponse   `json:"results"`
	Errors  []quoteBatchError `json:"errors"`
}

// quoteBatchError represents an error for a single symbol in a batch quote.
type quoteBatchError struct {
	Symbol string `json:"symbol"`
	Error  string `json:"error"`
}

// SearchByISINBatch searches for multiple instruments by their ISINs in a single request.
// This is more efficient than calling SearchByISIN multiple times.
func (c *Client) SearchByISINBatch(ctx context.Context, isins []string) []marketdata.SearchResult {
	results := make([]marketdata.SearchResult, 0, len(isins))

	if len(isins) == 0 {
		return results
	}

	reqBody := searchBatchRequest{ISINs: isins}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		// Return all as errors if we can't marshal the request
		for _, isin := range isins {
			results = append(results, marketdata.SearchResult{
				ISIN:  isin,
				Error: fmt.Errorf("failed to marshal request: %w", err),
			})
		}
		return results
	}

	reqURL := fmt.Sprintf("%s%s", c.baseURL, searchBatchPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(jsonBody))
	if err != nil {
		for _, isin := range isins {
			results = append(results, marketdata.SearchResult{
				ISIN:  isin,
				Error: fmt.Errorf("failed to create request: %w", err),
			})
		}
		return results
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		for _, isin := range isins {
			results = append(results, marketdata.SearchResult{
				ISIN:  isin,
				Error: fmt.Errorf("failed to execute request: %w", err),
			})
		}
		return results
	}

	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Warn("failed to close response body", "error", closeErr, "url", reqURL)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		for _, isin := range isins {
			results = append(results, marketdata.SearchResult{
				ISIN:  isin,
				Error: fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body)),
			})
		}
		return results
	}

	var batchResp searchBatchResponse
	if err := json.NewDecoder(resp.Body).Decode(&batchResp); err != nil {
		for _, isin := range isins {
			results = append(results, marketdata.SearchResult{
				ISIN:  isin,
				Error: fmt.Errorf("failed to decode response: %w", err),
			})
		}
		return results
	}

	// Process successful results
	for _, sr := range batchResp.Results {
		instrumentType := mapInstrumentType(sr.Type)
		instrument := domain.NewInstrument(
			sr.ISIN,
			sr.Symbol,
			sr.Name,
			instrumentType,
			sr.Currency,
			sr.Exchange,
		)
		results = append(results, marketdata.SearchResult{
			ISIN:       sr.ISIN,
			Instrument: &instrument,
		})
	}

	// Process errors
	for _, e := range batchResp.Errors {
		results = append(results, marketdata.SearchResult{
			ISIN:  e.ISIN,
			Error: fmt.Errorf("%s", e.Error),
		})
	}

	return results
}

// GetQuoteBatch retrieves quotes for multiple symbols in a single request.
// This is more efficient than calling GetQuote multiple times.
func (c *Client) GetQuoteBatch(ctx context.Context, symbols []string) []marketdata.QuoteBatchResult {
	results := make([]marketdata.QuoteBatchResult, 0, len(symbols))

	if len(symbols) == 0 {
		return results
	}

	reqBody := quoteBatchRequest{Symbols: symbols}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		for _, symbol := range symbols {
			results = append(results, marketdata.QuoteBatchResult{
				Symbol: symbol,
				Error:  fmt.Errorf("failed to marshal request: %w", err),
			})
		}
		return results
	}

	reqURL := fmt.Sprintf("%s%s", c.baseURL, quoteBatchPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(jsonBody))
	if err != nil {
		for _, symbol := range symbols {
			results = append(results, marketdata.QuoteBatchResult{
				Symbol: symbol,
				Error:  fmt.Errorf("failed to create request: %w", err),
			})
		}
		return results
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		for _, symbol := range symbols {
			results = append(results, marketdata.QuoteBatchResult{
				Symbol: symbol,
				Error:  fmt.Errorf("failed to execute request: %w", err),
			})
		}
		return results
	}

	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Warn("failed to close response body", "error", closeErr, "url", reqURL)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		for _, symbol := range symbols {
			results = append(results, marketdata.QuoteBatchResult{
				Symbol: symbol,
				Error:  fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body)),
			})
		}
		return results
	}

	var batchResp quoteBatchResponse
	if err := json.NewDecoder(resp.Body).Decode(&batchResp); err != nil {
		for _, symbol := range symbols {
			results = append(results, marketdata.QuoteBatchResult{
				Symbol: symbol,
				Error:  fmt.Errorf("failed to decode response: %w", err),
			})
		}
		return results
	}

	// Process successful results
	for _, qr := range batchResp.Results {
		price, err := domain.NewDecimalFromString(qr.Price)
		if err != nil {
			results = append(results, marketdata.QuoteBatchResult{
				Symbol: qr.Symbol,
				Error:  fmt.Errorf("failed to parse price: %w", err),
			})
			continue
		}

		results = append(results, marketdata.QuoteBatchResult{
			Symbol: qr.Symbol,
			Quote: &marketdata.QuoteResult{
				Symbol:   qr.Symbol,
				Price:    price,
				Currency: qr.Currency,
				Time:     qr.Time,
			},
		})
	}

	// Process errors
	for _, e := range batchResp.Errors {
		results = append(results, marketdata.QuoteBatchResult{
			Symbol: e.Symbol,
			Error:  fmt.Errorf("%s", e.Error),
		})
	}

	return results
}

// Compile-time check that Client implements BatchProvider.
var _ marketdata.BatchProvider = (*Client)(nil)
