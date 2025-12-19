package twelvedata

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
	"github.com/shopspring/decimal"
)

const (
	defaultBaseURL   = "https://api.twelvedata.com"
	symbolSearchPath = "/symbol_search"
	quotePath        = "/quote"
)

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		baseURL: defaultBaseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type symbolSearchResponse struct {
	Data []struct {
		Symbol         string `json:"symbol"`
		InstrumentName string `json:"instrument_name"`
		Exchange       string `json:"exchange"`
		Currency       string `json:"currency"`
		InstrumentType string `json:"instrument_type"`
	} `json:"data"`
	Status string `json:"status"`
}

type quoteResponse struct {
	Symbol   string `json:"symbol"`
	Name     string `json:"name"`
	Exchange string `json:"exchange"`
	Currency string `json:"currency"`
	Datetime string `json:"datetime"`
	Close    string `json:"close"`
	Status   string `json:"status"`
	Message  string `json:"message"`
}

func (c *Client) SearchByISIN(ctx context.Context, isin string) (*domain.Instrument, error) {
	params := url.Values{}
	params.Add("symbol", isin)
	params.Add("apikey", c.apiKey)

	reqURL := fmt.Sprintf("%s%s?%s", c.baseURL, symbolSearchPath, params.Encode())

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

	var searchResp symbolSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if searchResp.Status != "ok" || len(searchResp.Data) == 0 {
		return nil, fmt.Errorf("no instrument found for ISIN: %s", isin)
	}

	data := searchResp.Data[0]
	instrumentType := domain.InstrumentTypeStock
	if data.InstrumentType == "ETF" {
		instrumentType = domain.InstrumentTypeETF
	}

	instrument := domain.NewInstrument(
		isin,
		data.Symbol,
		data.InstrumentName,
		instrumentType,
		data.Currency,
		data.Exchange,
	)

	return &instrument, nil
}

func (c *Client) GetQuote(ctx context.Context, symbol string) (*marketdata.QuoteResult, error) {
	params := url.Values{}
	params.Add("symbol", symbol)
	params.Add("apikey", c.apiKey)

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

	if quoteResp.Status == "error" {
		return nil, fmt.Errorf("quote request failed for symbol %s: %s", symbol, quoteResp.Message)
	}

	if quoteResp.Close == "" {
		return nil, fmt.Errorf("quote request returned no price data for symbol: %s", symbol)
	}

	price, err := decimal.NewFromString(quoteResp.Close)
	if err != nil {
		return nil, fmt.Errorf("failed to parse price: %w", err)
	}

	return &marketdata.QuoteResult{
		Symbol:   quoteResp.Symbol,
		Price:    price,
		Currency: quoteResp.Currency,
		Time:     quoteResp.Datetime,
	}, nil
}
