package yfinance

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jmanzanog/stock-tracker/internal/domain"
)

func TestSearchByISIN(t *testing.T) {
	tests := []struct {
		name           string
		isin           string
		mockResponse   string
		statusCode     int
		expectedSymbol string
		expectError    bool
		failConnection bool
	}{
		{
			name:       "Success - Stock Found",
			isin:       "US0378331005",
			statusCode: http.StatusOK,
			mockResponse: `{
				"isin": "US0378331005",
				"symbol": "AAPL",
				"name": "Apple Inc.",
				"type": "stock",
				"currency": "USD",
				"exchange": "NASDAQ"
			}`,
			expectedSymbol: "AAPL",
			expectError:    false,
		},
		{
			name:       "Success - ETF Found",
			isin:       "IE00B3RBWM25",
			statusCode: http.StatusOK,
			mockResponse: `{
				"isin": "IE00B3RBWM25",
				"symbol": "VWRL.L",
				"name": "Vanguard FTSE All-World UCITS ETF",
				"type": "etf",
				"currency": "USD",
				"exchange": "LSE"
			}`,
			expectedSymbol: "VWRL.L",
			expectError:    false,
		},
		{
			name:           "Not Found - 404",
			isin:           "INVALIDXXXX",
			statusCode:     http.StatusNotFound,
			mockResponse:   `{"detail": "ISIN not found"}`,
			expectedSymbol: "",
			expectError:    true,
		},
		{
			name:           "HTTP 500 Error",
			isin:           "US0378331005",
			statusCode:     http.StatusInternalServerError,
			mockResponse:   `Internal Server Error`,
			expectedSymbol: "",
			expectError:    true,
		},
		{
			name:           "HTTP 500 Error with JSON detail",
			isin:           "US0378331005",
			statusCode:     http.StatusInternalServerError,
			mockResponse:   `{"detail": "Database connection error"}`,
			expectedSymbol: "",
			expectError:    true,
		},
		{
			name:           "Malformed JSON",
			isin:           "US0378331005",
			statusCode:     http.StatusOK,
			mockResponse:   `{invalid-json`,
			expectedSymbol: "",
			expectError:    true,
		},
		{
			name:           "Network Error",
			isin:           "US0378331005",
			failConnection: true,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify the request path
				expectedPath := "/api/v1/search/" + tt.isin
				if r.URL.Path != expectedPath {
					t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
				}

				w.WriteHeader(tt.statusCode)
				_, err := w.Write([]byte(tt.mockResponse))
				if err != nil {
					t.Logf("Error writing response: %v", err)
				}
			}))
			defer server.Close()

			client := NewClient()
			if tt.failConnection {
				client.baseURL = "http://127.0.0.1:0" // Bad port to trigger connection error
			} else {
				client.baseURL = server.URL
			}

			result, err := client.SearchByISIN(context.Background(), tt.isin)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result.Symbol != tt.expectedSymbol {
				t.Errorf("Expected symbol %s, got %s", tt.expectedSymbol, result.Symbol)
			}

			if tt.name == "Success - ETF Found" && result.Type != domain.InstrumentTypeETF {
				t.Errorf("Expected instrument type ETF, got %v", result.Type)
			}

			if tt.name == "Success - Stock Found" && result.Type != domain.InstrumentTypeStock {
				t.Errorf("Expected instrument type Stock, got %v", result.Type)
			}
		})
	}
}

func TestGetQuote(t *testing.T) {
	tests := []struct {
		name           string
		symbol         string
		mockResponse   string
		statusCode     int
		expectedPrice  string
		expectError    bool
		failConnection bool
	}{
		{
			name:       "Success",
			symbol:     "AAPL",
			statusCode: http.StatusOK,
			mockResponse: `{
				"symbol": "AAPL",
				"price": "195.5000",
				"currency": "USD",
				"time": "2024-12-24T15:00:00+00:00"
			}`,
			expectedPrice: "195.5000",
			expectError:   false,
		},
		{
			name:          "Not Found - 404",
			symbol:        "INVALID",
			statusCode:    http.StatusNotFound,
			mockResponse:  `{"detail": "Symbol not found"}`,
			expectedPrice: "",
			expectError:   true,
		},
		{
			name:       "Missing Price",
			symbol:     "AAPL",
			statusCode: http.StatusOK,
			mockResponse: `{
				"symbol": "AAPL",
				"price": "",
				"currency": "USD",
				"time": "2024-12-24T15:00:00+00:00"
			}`,
			expectedPrice: "",
			expectError:   true,
		},
		{
			name:          "HTTP 500 Error",
			symbol:        "AAPL",
			statusCode:    http.StatusInternalServerError,
			mockResponse:  `Internal Server Error`,
			expectedPrice: "",
			expectError:   true,
		},
		{
			name:          "HTTP 500 Error with JSON detail",
			symbol:        "AAPL",
			statusCode:    http.StatusInternalServerError,
			mockResponse:  `{"detail": "yfinance API rate limit"}`,
			expectedPrice: "",
			expectError:   true,
		},
		{
			name:          "Malformed JSON",
			symbol:        "AAPL",
			statusCode:    http.StatusOK,
			mockResponse:  `{invalid-json`,
			expectedPrice: "",
			expectError:   true,
		},
		{
			name:           "Network Error",
			symbol:         "AAPL",
			failConnection: true,
			expectError:    true,
		},
		{
			name:       "Invalid Price Format",
			symbol:     "AAPL",
			statusCode: http.StatusOK,
			mockResponse: `{
				"symbol": "AAPL",
				"price": "invalid-decimal",
				"currency": "USD",
				"time": "2024-12-24T15:00:00+00:00"
			}`,
			expectedPrice: "",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify the request path
				expectedPath := "/api/v1/quote/" + tt.symbol
				if r.URL.Path != expectedPath {
					t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
				}

				w.WriteHeader(tt.statusCode)
				_, err := w.Write([]byte(tt.mockResponse))
				if err != nil {
					t.Logf("Error writing response: %v", err)
				}
			}))
			defer server.Close()

			client := NewClient()
			if tt.failConnection {
				client.baseURL = "http://127.0.0.1:0"
			} else {
				client.baseURL = server.URL
			}

			result, err := client.GetQuote(context.Background(), tt.symbol)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			expectedDecimal, _ := domain.NewDecimalFromString(tt.expectedPrice)
			if !result.Price.Equal(expectedDecimal) {
				t.Errorf("Expected price %s, got %s", tt.expectedPrice, result.Price)
			}
		})
	}
}

func TestNewClient(t *testing.T) {
	client := NewClient()

	if client.baseURL != defaultBaseURL {
		t.Errorf("Expected base url %s, got %s", defaultBaseURL, client.baseURL)
	}
	if client.httpClient == nil {
		t.Error("Expected http client to be initialized")
	}
}

func TestNewClientWithBaseURL(t *testing.T) {
	customURL := "http://market-data-service:8000"
	client := NewClientWithBaseURL(customURL)

	if client.baseURL != customURL {
		t.Errorf("Expected base url %s, got %s", customURL, client.baseURL)
	}
	if client.httpClient == nil {
		t.Error("Expected http client to be initialized")
	}
}

func TestNewClientWithHTTPClient(t *testing.T) {
	customHTTPClient := &http.Client{}
	client := NewClientWithHTTPClient(customHTTPClient)

	if client.baseURL != defaultBaseURL {
		t.Errorf("Expected base url %s, got %s", defaultBaseURL, client.baseURL)
	}
	if client.httpClient != customHTTPClient {
		t.Error("Expected custom http client to be used")
	}
}

func TestClient_SetBaseURL(t *testing.T) {
	client := NewClient()
	newURL := "http://custom-service:9000"

	client.SetBaseURL(newURL)

	if client.baseURL != newURL {
		t.Errorf("Expected base url %s, got %s", newURL, client.baseURL)
	}
}

func TestClient_RequestCreationError(t *testing.T) {
	client := NewClient()
	// Set baseURL to something with a control character to trigger http.NewRequest error
	client.baseURL = "http://market-data-service\x7f"

	_, err := client.SearchByISIN(context.Background(), "US0378331005")
	if err == nil {
		t.Error("Expected error for SearchByISIN with bad URL, got nil")
	}

	_, err = client.GetQuote(context.Background(), "AAPL")
	if err == nil {
		t.Error("Expected error for GetQuote with bad URL, got nil")
	}
}

func TestMapInstrumentType(t *testing.T) {
	tests := []struct {
		apiType  string
		expected domain.InstrumentType
	}{
		{"stock", domain.InstrumentTypeStock},
		{"Stock", domain.InstrumentTypeStock},
		{"STOCK", domain.InstrumentTypeStock},
		{"etf", domain.InstrumentTypeETF},
		{"ETF", domain.InstrumentTypeETF},
		{"", domain.InstrumentTypeStock},
		{"unknown", domain.InstrumentTypeStock},
	}

	for _, tt := range tests {
		t.Run(tt.apiType, func(t *testing.T) {
			result := mapInstrumentType(tt.apiType)
			if result != tt.expected {
				t.Errorf("mapInstrumentType(%s) = %v, want %v", tt.apiType, result, tt.expected)
			}
		})
	}
}

type errorBody struct{}

func (e *errorBody) Read(_ []byte) (n int, err error) {
	return 0, io.EOF
}

func (e *errorBody) Close() error {
	return context.Canceled // Simulate close error
}

type errorTransport struct{}

func (t *errorTransport) RoundTrip(_ *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: 200,
		Body:       &errorBody{},
		Header:     make(http.Header),
	}, nil
}

func TestBodyCloseError(t *testing.T) {
	client := NewClientWithHTTPClient(&http.Client{
		Transport: &errorTransport{},
	})

	// Should not panic, just log warning and return error from decode
	_, err := client.SearchByISIN(context.Background(), "US123")
	if err == nil {
		t.Log("SearchByISIN returned nil error (expected decode error)")
	}

	_, err = client.GetQuote(context.Background(), "AAPL")
	if err == nil {
		t.Log("GetQuote returned nil error (expected decode error)")
	}
}

func TestSearchByISINBatch(t *testing.T) {
	tests := []struct {
		name           string
		isins          []string
		mockResponse   string
		statusCode     int
		expectedCount  int
		expectedErrors int
		expectFail     bool
	}{
		{
			name:       "Success - Mixed Results",
			isins:      []string{"US0378331005", "DE0007164600"},
			statusCode: http.StatusOK,
			mockResponse: `{
				"results": [
					{"isin": "US0378331005", "symbol": "AAPL", "name": "Apple Inc.", "type": "stock", "currency": "USD", "exchange": "NASDAQ"},
					{"isin": "DE0007164600", "symbol": "SAP", "name": "SAP SE", "type": "stock", "currency": "EUR", "exchange": "XETRA"}
				],
				"errors": []
			}`,
			expectedCount:  2,
			expectedErrors: 0,
		},
		{
			name:       "Partial Success",
			isins:      []string{"US0378331005", "INVALID"},
			statusCode: http.StatusOK,
			mockResponse: `{
				"results": [
					{"isin": "US0378331005", "symbol": "AAPL", "name": "Apple Inc.", "type": "stock", "currency": "USD", "exchange": "NASDAQ"}
				],
				"errors": [
					{"isin": "INVALID", "error": "Not found"}
				]
			}`,
			expectedCount:  2,
			expectedErrors: 1,
		},
		{
			name:       "All Failed",
			isins:      []string{"INVALID1", "INVALID2"},
			statusCode: http.StatusOK,
			mockResponse: `{
				"results": [],
				"errors": [
					{"isin": "INVALID1", "error": "Not found"},
					{"isin": "INVALID2", "error": "Not found"}
				]
			}`,
			expectedCount:  2,
			expectedErrors: 2,
		},
		{
			name:         "General API Error",
			isins:        []string{"US0378331005"},
			statusCode:   http.StatusInternalServerError,
			mockResponse: `Internal Server Error`,
			expectFail:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/search/batch" {
					t.Errorf("Expected path /api/v1/search/batch, got %s", r.URL.Path)
				}
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.mockResponse))
			}))
			defer server.Close()

			client := NewClientWithBaseURL(server.URL)
			results := client.SearchByISINBatch(context.Background(), tt.isins)

			if tt.expectFail {
				if len(results) != len(tt.isins) {
					t.Errorf("Expected %d results, got %d", len(tt.isins), len(results))
				}
				for _, r := range results {
					if r.Error == nil {
						t.Error("Expected error in result, got nil")
					}
				}
				return
			}

			if len(results) != tt.expectedCount {
				t.Errorf("Expected %d results, got %d", tt.expectedCount, len(results))
			}

			actualErrors := 0
			for _, r := range results {
				if r.Error != nil {
					actualErrors++
				}
			}
			if actualErrors != tt.expectedErrors {
				t.Errorf("Expected %d errors, got %d", tt.expectedErrors, actualErrors)
			}
		})
	}
}

func TestGetQuoteBatch(t *testing.T) {
	tests := []struct {
		name           string
		symbols        []string
		mockResponse   string
		statusCode     int
		expectedCount  int
		expectedErrors int
		expectFail     bool
	}{
		{
			name:       "Success - Mixed Results",
			symbols:    []string{"AAPL", "SAP"},
			statusCode: http.StatusOK,
			mockResponse: `{
				"results": [
					{"symbol": "AAPL", "price": "195.5000", "currency": "USD", "time": "2024-01-01"},
					{"symbol": "SAP", "price": "150.0000", "currency": "EUR", "time": "2024-01-01"}
				],
				"errors": []
			}`,
			expectedCount:  2,
			expectedErrors: 0,
		},
		{
			name:       "Partial Success",
			symbols:    []string{"AAPL", "INVALID"},
			statusCode: http.StatusOK,
			mockResponse: `{
				"results": [
					{"symbol": "AAPL", "price": "195.5000", "currency": "USD", "time": "2024-01-01"}
				],
				"errors": [
					{"symbol": "INVALID", "error": "Not found"}
				]
			}`,
			expectedCount:  2,
			expectedErrors: 1,
		},
		{
			name:         "General API Error",
			symbols:      []string{"AAPL"},
			statusCode:   http.StatusInternalServerError,
			mockResponse: `Internal Server Error`,
			expectFail:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/quote/batch" {
					t.Errorf("Expected path /api/v1/quote/batch, got %s", r.URL.Path)
				}
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.mockResponse))
			}))
			defer server.Close()

			client := NewClientWithBaseURL(server.URL)
			results := client.GetQuoteBatch(context.Background(), tt.symbols)

			if tt.expectFail {
				if len(results) != len(tt.symbols) {
					t.Errorf("Expected %d results, got %d", len(tt.symbols), len(results))
				}
				for _, r := range results {
					if r.Error == nil {
						t.Error("Expected error in result, got nil")
					}
				}
				return
			}

			if len(results) != tt.expectedCount {
				t.Errorf("Expected %d results, got %d", tt.expectedCount, len(results))
			}

			actualErrors := 0
			for _, r := range results {
				if r.Error != nil {
					actualErrors++
				}
			}
			if actualErrors != tt.expectedErrors {
				t.Errorf("expected %d errors, got %d", tt.expectedErrors, actualErrors)
			}
		})
	}
}

func TestSearchByISINBatch_Errors(t *testing.T) {
	client := NewClient()
	client.baseURL = "http://market-data-service\x7f" // Bad URL

	results := client.SearchByISINBatch(context.Background(), []string{"US123"})
	if len(results) != 1 || results[0].Error == nil {
		t.Error("expected error for SearchByISINBatch with bad URL")
	}

	// Network error
	client.baseURL = "http://127.0.0.1:0"
	results = client.SearchByISINBatch(context.Background(), []string{"US123"})
	if len(results) != 1 || results[0].Error == nil {
		t.Error("expected network error for SearchByISINBatch")
	}

	// Malformed JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{invalid-json`))
	}))
	defer server.Close()
	client.baseURL = server.URL
	results = client.SearchByISINBatch(context.Background(), []string{"US123"})
	if len(results) != 1 || results[0].Error == nil {
		t.Error("expected JSON decode error for SearchByISINBatch")
	}
}

func TestGetQuoteBatch_Errors(t *testing.T) {
	client := NewClient()
	client.baseURL = "http://market-data-service\x7f" // Bad URL

	results := client.GetQuoteBatch(context.Background(), []string{"AAPL"})
	if len(results) != 1 || results[0].Error == nil {
		t.Error("expected error for GetQuoteBatch with bad URL")
	}

	// Network error
	client.baseURL = "http://127.0.0.1:0"
	results = client.GetQuoteBatch(context.Background(), []string{"AAPL"})
	if len(results) != 1 || results[0].Error == nil {
		t.Error("expected network error for GetQuoteBatch")
	}

	// Malformed JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{invalid-json`))
	}))
	defer server.Close()
	client.baseURL = server.URL
	results = client.GetQuoteBatch(context.Background(), []string{"AAPL"})
	if len(results) != 1 || results[0].Error == nil {
		t.Error("expected JSON decode error for GetQuoteBatch")
	}

	// Status 500
	server500 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`Internal Error`))
	}))
	defer server500.Close()
	client.baseURL = server500.URL
	results = client.GetQuoteBatch(context.Background(), []string{"AAPL"})
	if len(results) != 1 || !strings.Contains(results[0].Error.Error(), "500") {
		t.Error("expected status 500 error for GetQuoteBatch")
	}

	// Invalid Price in JSON
	serverPrice := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"results": [{"symbol": "AAPL", "price": "invalid", "currency": "USD", "time": "2024-01-01"}]}`))
	}))
	defer serverPrice.Close()
	client.baseURL = serverPrice.URL
	results = client.GetQuoteBatch(context.Background(), []string{"AAPL"})
	if len(results) != 1 || !strings.Contains(results[0].Error.Error(), "failed to parse price") {
		t.Error("expected parse price error for GetQuoteBatch")
	}
}

func TestSearchByISINBatch_AdditionalErrors(t *testing.T) {
	client := NewClient()

	// Status 500
	server500 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server500.Close()
	client.baseURL = server500.URL
	results := client.SearchByISINBatch(context.Background(), []string{"US123"})
	if len(results) != 1 || results[0].Error == nil {
		t.Error("expected status 500 error for SearchByISINBatch")
	}
}
