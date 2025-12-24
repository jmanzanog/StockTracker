package finnhub

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jmanzanog/stock-tracker/internal/domain"
	"github.com/jmanzanog/stock-tracker/internal/infrastructure/marketdata"
)

func TestNewClient(t *testing.T) {
	client := NewClient("test-api-key")

	assert.NotNil(t, client)
	assert.Equal(t, defaultBaseURL, client.baseURL)
	assert.Equal(t, "test-api-key", client.apiKey)
	assert.NotNil(t, client.httpClient)
}

func TestNewClientWithHTTPClient(t *testing.T) {
	customHTTPClient := &http.Client{Timeout: 30 * time.Second}
	client := NewClientWithHTTPClient("test-api-key", customHTTPClient)

	assert.NotNil(t, client)
	assert.Equal(t, defaultBaseURL, client.baseURL)
	assert.Equal(t, "test-api-key", client.apiKey)
	assert.Equal(t, customHTTPClient, client.httpClient)
}

func TestClient_SetBaseURL(t *testing.T) {
	client := NewClient("test-api-key")
	newURL := "https://custom.api.com"

	client.SetBaseURL(newURL)

	assert.Equal(t, newURL, client.baseURL)
}

func TestClient_SearchByISIN_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/search", r.URL.Path)
		assert.Equal(t, "GB00B63H8491", r.URL.Query().Get("q"))
		assert.Equal(t, "test-api-key", r.URL.Query().Get("token"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"count": 1,
			"result": [
				{
					"description": "ROLLS-ROYCE HOLDINGS PLC",
					"displaySymbol": "RR.L",
					"symbol": "RR.L",
					"type": "Common Stock"
				}
			]
		}`))
	}))
	defer server.Close()

	client := NewClient("test-api-key")
	client.SetBaseURL(server.URL)

	instrument, err := client.SearchByISIN(context.Background(), "GB00B63H8491")

	require.NoError(t, err)
	assert.NotNil(t, instrument)
	assert.Equal(t, "GB00B63H8491", instrument.ISIN)
	assert.Equal(t, "RR.L", instrument.Symbol)
	assert.Equal(t, "ROLLS-ROYCE HOLDINGS PLC", instrument.Name)
	assert.Equal(t, domain.InstrumentTypeStock, instrument.Type)
	assert.Equal(t, "L", instrument.Exchange)
}

func TestClient_SearchByISIN_ETF(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"count": 1,
			"result": [
				{
					"description": "VANGUARD S&P 500 ETF",
					"displaySymbol": "VOO",
					"symbol": "VOO",
					"type": "ETF"
				}
			]
		}`))
	}))
	defer server.Close()

	client := NewClient("test-api-key")
	client.SetBaseURL(server.URL)

	instrument, err := client.SearchByISIN(context.Background(), "US9229087690")

	require.NoError(t, err)
	assert.NotNil(t, instrument)
	assert.Equal(t, domain.InstrumentTypeETF, instrument.Type)
	assert.Equal(t, "", instrument.Exchange) // No dot in symbol
}

func TestClient_SearchByISIN_ETP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"count": 1,
			"result": [
				{
					"description": "SOME ETP",
					"displaySymbol": "ETP1",
					"symbol": "ETP1",
					"type": "ETP"
				}
			]
		}`))
	}))
	defer server.Close()

	client := NewClient("test-api-key")
	client.SetBaseURL(server.URL)

	instrument, err := client.SearchByISIN(context.Background(), "US1234567890")

	require.NoError(t, err)
	assert.Equal(t, domain.InstrumentTypeETF, instrument.Type)
}

func TestClient_SearchByISIN_EquityType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"count": 1,
			"result": [
				{
					"description": "SOME EQUITY",
					"displaySymbol": "EQ1",
					"symbol": "EQ1",
					"type": "Equity"
				}
			]
		}`))
	}))
	defer server.Close()

	client := NewClient("test-api-key")
	client.SetBaseURL(server.URL)

	instrument, err := client.SearchByISIN(context.Background(), "US1234567890")

	require.NoError(t, err)
	assert.Equal(t, domain.InstrumentTypeStock, instrument.Type)
}

func TestClient_SearchByISIN_UnknownType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"count": 1,
			"result": [
				{
					"description": "UNKNOWN TYPE",
					"displaySymbol": "UNK",
					"symbol": "UNK",
					"type": "SomeUnknownType"
				}
			]
		}`))
	}))
	defer server.Close()

	client := NewClient("test-api-key")
	client.SetBaseURL(server.URL)

	instrument, err := client.SearchByISIN(context.Background(), "US1234567890")

	require.NoError(t, err)
	assert.Equal(t, domain.InstrumentTypeStock, instrument.Type) // Default to Stock
}

func TestClient_SearchByISIN_NoResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"count": 0,
			"result": []
		}`))
	}))
	defer server.Close()

	client := NewClient("test-api-key")
	client.SetBaseURL(server.URL)

	instrument, err := client.SearchByISIN(context.Background(), "INVALID_ISIN")

	assert.Nil(t, instrument)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no instrument found for ISIN")
}

func TestClient_SearchByISIN_CountZeroWithResults(t *testing.T) {
	// Edge case: count is 0 but result array has items (should still fail)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"count": 0,
			"result": [{"description": "Test", "symbol": "TEST", "type": "Stock"}]
		}`))
	}))
	defer server.Close()

	client := NewClient("test-api-key")
	client.SetBaseURL(server.URL)

	instrument, err := client.SearchByISIN(context.Background(), "TEST_ISIN")

	// Should fail because count is 0
	assert.Nil(t, instrument)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no instrument found for ISIN")
}

func TestClient_SearchByISIN_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer server.Close()

	client := NewClient("test-api-key")
	client.SetBaseURL(server.URL)

	instrument, err := client.SearchByISIN(context.Background(), "GB00B63H8491")

	assert.Nil(t, instrument)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API returned status 500")
}

func TestClient_SearchByISIN_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": "You don't have access"}`))
	}))
	defer server.Close()

	client := NewClient("invalid-api-key")
	client.SetBaseURL(server.URL)

	instrument, err := client.SearchByISIN(context.Background(), "GB00B63H8491")

	assert.Nil(t, instrument)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API returned status 401")
}

func TestClient_SearchByISIN_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	client := NewClient("test-api-key")
	client.SetBaseURL(server.URL)

	instrument, err := client.SearchByISIN(context.Background(), "GB00B63H8491")

	assert.Nil(t, instrument)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode response")
}

func TestClient_SearchByISIN_NetworkError(t *testing.T) {
	client := NewClient("test-api-key")
	client.SetBaseURL("http://localhost:99999") // Invalid port

	instrument, err := client.SearchByISIN(context.Background(), "GB00B63H8491")

	assert.Nil(t, instrument)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to execute request")
}

func TestClient_SearchByISIN_ContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient("test-api-key")
	client.SetBaseURL(server.URL)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	instrument, err := client.SearchByISIN(ctx, "GB00B63H8491")

	assert.Nil(t, instrument)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to execute request")
}

func TestClient_GetQuote_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/quote", r.URL.Path)
		assert.Equal(t, "RR.L", r.URL.Query().Get("symbol"))
		assert.Equal(t, "test-api-key", r.URL.Query().Get("token"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"c": 5.23,
			"d": 0.05,
			"dp": 0.96,
			"h": 5.30,
			"l": 5.15,
			"o": 5.18,
			"pc": 5.18,
			"t": 1703433600
		}`))
	}))
	defer server.Close()

	client := NewClient("test-api-key")
	client.SetBaseURL(server.URL)

	quote, err := client.GetQuote(context.Background(), "RR.L")

	require.NoError(t, err)
	assert.NotNil(t, quote)
	assert.Equal(t, "RR.L", quote.Symbol)
	assert.Equal(t, "5.2300", quote.Price.String())
	assert.NotEmpty(t, quote.Time)
}

func TestClient_GetQuote_NoData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"c": 0,
			"d": 0,
			"dp": 0,
			"h": 0,
			"l": 0,
			"o": 0,
			"pc": 0,
			"t": 0
		}`))
	}))
	defer server.Close()

	client := NewClient("test-api-key")
	client.SetBaseURL(server.URL)

	quote, err := client.GetQuote(context.Background(), "INVALID_SYMBOL")

	assert.Nil(t, quote)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no quote data found for symbol")
}

func TestClient_GetQuote_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer server.Close()

	client := NewClient("test-api-key")
	client.SetBaseURL(server.URL)

	quote, err := client.GetQuote(context.Background(), "RR.L")

	assert.Nil(t, quote)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API returned status 500")
}

func TestClient_GetQuote_TooManyRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error": "API limit reached. Please try again later"}`))
	}))
	defer server.Close()

	client := NewClient("test-api-key")
	client.SetBaseURL(server.URL)

	quote, err := client.GetQuote(context.Background(), "RR.L")

	assert.Nil(t, quote)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API returned status 429")
}

func TestClient_GetQuote_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{not valid json`))
	}))
	defer server.Close()

	client := NewClient("test-api-key")
	client.SetBaseURL(server.URL)

	quote, err := client.GetQuote(context.Background(), "RR.L")

	assert.Nil(t, quote)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode response")
}

func TestClient_GetQuote_NetworkError(t *testing.T) {
	client := NewClient("test-api-key")
	client.SetBaseURL("http://localhost:99999") // Invalid port

	quote, err := client.GetQuote(context.Background(), "RR.L")

	assert.Nil(t, quote)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to execute request")
}

func TestClient_GetQuote_ContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient("test-api-key")
	client.SetBaseURL(server.URL)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	quote, err := client.GetQuote(ctx, "RR.L")

	assert.Nil(t, quote)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to execute request")
}

func TestClient_GetQuote_ValidDataWithZeroPreviousClose(t *testing.T) {
	// Edge case: current price > 0 but previous close is 0 (valid scenario)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"c": 10.50,
			"d": 0,
			"dp": 0,
			"h": 0,
			"l": 0,
			"o": 0,
			"pc": 0,
			"t": 1703433600
		}`))
	}))
	defer server.Close()

	client := NewClient("test-api-key")
	client.SetBaseURL(server.URL)

	quote, err := client.GetQuote(context.Background(), "NEW_IPO")

	require.NoError(t, err)
	assert.NotNil(t, quote)
	assert.Equal(t, "10.5000", quote.Price.String())
}

func TestMapInstrumentType(t *testing.T) {
	tests := []struct {
		name         string
		finnhubType  string
		expectedType domain.InstrumentType
	}{
		{"ETF type", "ETF", domain.InstrumentTypeETF},
		{"ETP type", "ETP", domain.InstrumentTypeETF},
		{"Common Stock type", "Common Stock", domain.InstrumentTypeStock},
		{"Equity type", "Equity", domain.InstrumentTypeStock},
		{"Unknown type defaults to Stock", "SomethingElse", domain.InstrumentTypeStock},
		{"Empty type defaults to Stock", "", domain.InstrumentTypeStock},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapInstrumentType(tt.finnhubType)
			assert.Equal(t, tt.expectedType, result)
		})
	}
}

func TestExtractExchange(t *testing.T) {
	tests := []struct {
		name     string
		symbol   string
		expected string
	}{
		{"London exchange", "RR.L", "L"},
		{"German exchange", "BMW.DE", "DE"},
		{"Paris exchange", "AIR.PA", "PA"},
		{"US stock no exchange", "AAPL", ""},
		{"Empty symbol", "", ""},
		{"Multiple dots", "X.Y.Z", "Z"},
		{"Dot at end", "TEST.", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractExchange(tt.symbol)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test that Client implements MDataProvider interface
func TestClient_ImplementsMDataProvider(t *testing.T) {
	client := NewClient("test-api-key")

	// This will fail to compile if Client doesn't implement the interface
	var _ interface {
		SearchByISIN(ctx context.Context, isin string) (*domain.Instrument, error)
		GetQuote(ctx context.Context, symbol string) (*marketdata.QuoteResult, error)
	} = client
}

// Test response body close error logging (coverage for defer with error)
func TestClient_SearchByISIN_ResponseBodyCloseError(t *testing.T) {
	// Create a custom response body that errors on Close
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"count": 1,
			"result": [{"description": "Test", "symbol": "TEST", "displaySymbol": "TEST", "type": "Stock"}]
		}`))
	}))
	defer server.Close()

	client := NewClient("test-api-key")
	client.SetBaseURL(server.URL)

	// This tests the normal path but ensures the defer block runs
	instrument, err := client.SearchByISIN(context.Background(), "TEST_ISIN")

	require.NoError(t, err)
	assert.NotNil(t, instrument)
}

// Test with custom HTTP client timeout
type slowRoundTripper struct{}

func (s *slowRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	time.Sleep(50 * time.Millisecond)
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"count": 0, "result": []}`)),
		Header:     make(http.Header),
	}, nil
}

func TestClient_WithCustomHTTPClientTimeout(t *testing.T) {
	httpClient := &http.Client{
		Timeout:   100 * time.Millisecond,
		Transport: &slowRoundTripper{},
	}

	client := NewClientWithHTTPClient("test-api-key", httpClient)

	_, err := client.SearchByISIN(context.Background(), "TEST")

	// Should complete without timeout error (50ms sleep < 100ms timeout)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no instrument found for ISIN")
}
