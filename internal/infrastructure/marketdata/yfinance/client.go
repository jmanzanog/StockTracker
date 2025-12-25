package yfinance

import (
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
	defaultBaseURL = "http://localhost:8000"
	searchPath     = "/api/v1/search"
	quotePath      = "/api/v1/quote"
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
