package finnhub

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/jmanzanog/stock-tracker/internal/domain"
	"github.com/jmanzanog/stock-tracker/internal/infrastructure/marketdata"
)

const (
	defaultBaseURL = "https://finnhub.io/api/v1"
	searchPath     = "/search"
	quotePath      = "/quote"
	profilePath    = "/stock/profile2"
)

// Client implements the MDataProvider interface using Finnhub API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new Finnhub API client.
func NewClient(apiKey string) *Client {
	return &Client{
		baseURL: defaultBaseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// NewClientWithHTTPClient creates a new Finnhub client with a custom HTTP client (for testing).
func NewClientWithHTTPClient(apiKey string, httpClient *http.Client) *Client {
	return &Client{
		baseURL:    defaultBaseURL,
		apiKey:     apiKey,
		httpClient: httpClient,
	}
}

// SetBaseURL sets the base URL for the API (useful for testing).
func (c *Client) SetBaseURL(baseURL string) {
	c.baseURL = baseURL
}

// searchResponse represents the Finnhub symbol search response.
type searchResponse struct {
	Count  int            `json:"count"`
	Result []searchResult `json:"result"`
}

// searchResult represents a single search result from Finnhub.
type searchResult struct {
	Description   string `json:"description"`
	DisplaySymbol string `json:"displaySymbol"`
	Symbol        string `json:"symbol"`
	Type          string `json:"type"`
}

// quoteResponse represents the Finnhub quote response.
type quoteResponse struct {
	Current       float64 `json:"c"`  // Current price
	Change        float64 `json:"d"`  // Change
	PercentChange float64 `json:"dp"` // Percent change
	High          float64 `json:"h"`  // High price of the day
	Low           float64 `json:"l"`  // Low price of the day
	Open          float64 `json:"o"`  // Open price of the day
	PreviousClose float64 `json:"pc"` // Previous close price
	Timestamp     int64   `json:"t"`  // Timestamp
}

// profileResponse represents the Finnhub company profile response.
type profileResponse struct {
	Country              string  `json:"country"`
	Currency             string  `json:"currency"`
	Exchange             string  `json:"exchange"`
	FinnhubIndustry      string  `json:"finnhubIndustry"`
	IPO                  string  `json:"ipo"`
	Logo                 string  `json:"logo"`
	MarketCapitalization float64 `json:"marketCapitalization"`
	Name                 string  `json:"name"`
	Phone                string  `json:"phone"`
	ShareOutstanding     float64 `json:"shareOutstanding"`
	Ticker               string  `json:"ticker"`
	Weburl               string  `json:"weburl"`
}

// SearchByISIN searches for an instrument by its ISIN.
func (c *Client) SearchByISIN(ctx context.Context, isin string) (*domain.Instrument, error) {
	params := url.Values{}
	params.Add("q", isin)
	params.Add("token", c.apiKey)

	reqURL := fmt.Sprintf("%s%s?%s", c.baseURL, searchPath, params.Encode())

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

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var searchResp searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if searchResp.Count == 0 || len(searchResp.Result) == 0 {
		return nil, fmt.Errorf("no instrument found for ISIN: %s", isin)
	}

	result := searchResp.Result[0]
	instrumentType := mapInstrumentType(result.Type)

	// Get company profile to obtain currency and exchange
	profile, err := c.getProfile(ctx, result.Symbol)

	var currency, exchange string
	if err != nil {
		// Log warning but continue with extracted exchange (best effort)
		slog.Warn("failed to get company profile, using fallback values",
			"symbol", result.Symbol, "error", err)
		exchange = extractExchange(result.Symbol)
		currency = "USD" // Default fallback
	} else {
		currency = profile.Currency
		exchange = profile.Exchange
		// Use profile name if available (usually more complete)
		if profile.Name != "" {
			result.Description = profile.Name
		}
	}

	instrument := domain.NewInstrument(
		isin,
		result.Symbol,
		result.Description,
		instrumentType,
		currency,
		exchange,
	)

	return &instrument, nil
}

// getProfile fetches the company profile for a symbol.
func (c *Client) getProfile(ctx context.Context, symbol string) (*profileResponse, error) {
	params := url.Values{}
	params.Add("symbol", symbol)
	params.Add("token", c.apiKey)

	reqURL := fmt.Sprintf("%s%s?%s", c.baseURL, profilePath, params.Encode())

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

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var profileResp profileResponse
	if err := json.NewDecoder(resp.Body).Decode(&profileResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Check if we got valid data (empty response means symbol not found)
	if profileResp.Currency == "" {
		return nil, fmt.Errorf("no profile data found for symbol: %s", symbol)
	}

	return &profileResp, nil
}

// GetQuote retrieves the current quote for a symbol.
func (c *Client) GetQuote(ctx context.Context, symbol string) (*marketdata.QuoteResult, error) {
	params := url.Values{}
	params.Add("symbol", symbol)
	params.Add("token", c.apiKey)

	reqURL := fmt.Sprintf("%s%s?%s", c.baseURL, quotePath, params.Encode())

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

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var quoteResp quoteResponse
	if err := json.NewDecoder(resp.Body).Decode(&quoteResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Check if we got valid data (Finnhub returns 0 for all fields if symbol not found)
	if quoteResp.Current == 0 && quoteResp.PreviousClose == 0 && quoteResp.Timestamp == 0 {
		return nil, fmt.Errorf("no quote data found for symbol: %s", symbol)
	}

	price, err := domain.NewDecimalFromString(fmt.Sprintf("%.4f", quoteResp.Current))
	if err != nil {
		return nil, fmt.Errorf("failed to parse price: %w", err)
	}

	return &marketdata.QuoteResult{
		Symbol:   symbol,
		Price:    price,
		Currency: "", // Finnhub quote endpoint doesn't return currency
		Time:     time.Unix(quoteResp.Timestamp, 0).Format(time.RFC3339),
	}, nil
}

// mapInstrumentType maps Finnhub security types to domain instrument types.
func mapInstrumentType(finnhubType string) domain.InstrumentType {
	switch finnhubType {
	case "ETP", "ETF":
		return domain.InstrumentTypeETF
	case "Common Stock", "Equity":
		return domain.InstrumentTypeStock
	default:
		return domain.InstrumentTypeStock
	}
}

// extractExchange extracts the exchange code from a Finnhub symbol.
// For example: "RR.L" -> "L", "AAPL" -> ""
func extractExchange(symbol string) string {
	for i := len(symbol) - 1; i >= 0; i-- {
		if symbol[i] == '.' {
			return symbol[i+1:]
		}
	}
	return ""
}
