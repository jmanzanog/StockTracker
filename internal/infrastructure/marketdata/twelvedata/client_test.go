package twelvedata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jmanzanog/stock-tracker/internal/domain"
	"github.com/shopspring/decimal"
)

func TestSearchByISIN(t *testing.T) {
	tests := []struct {
		name           string
		isin           string
		mockResponse   string
		expectedSymbol string
		expectError    bool
	}{
		{
			name: "Success - Stock Found",
			isin: "US0378331005",
			mockResponse: `{
				"data": [
					{
						"symbol": "AAPL",
						"instrument_name": "Apple Inc",
						"exchange": "NASDAQ",
						"currency": "USD",
						"instrument_type": "Common Stock"
					}
				],
				"status": "ok"
			}`,
			expectedSymbol: "AAPL",
			expectError:    false,
		},
		{
			name: "Success - ETF Found",
			isin: "IE00B3RBWM25",
			mockResponse: `{
				"data": [
					{
						"symbol": "VWRL",
						"instrument_name": "Vanguard FTSE All-World",
						"exchange": "LSE",
						"currency": "USD",
						"instrument_type": "ETF"
					}
				],
				"status": "ok"
			}`,
			expectedSymbol: "VWRL",
			expectError:    false,
		},
		{
			name: "Not Found",
			isin: "INVALIDXXXX",
			mockResponse: `{
				"data": [],
				"status": "ok"
			}`,
			expectedSymbol: "",
			expectError:    true,
		},
		{
			name:           "API Error",
			isin:           "US0378331005",
			mockResponse:   `{"status": "error", "message": "API Limit Reached"}`,
			expectedSymbol: "",
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(tt.mockResponse))
			}))
			defer server.Close()

			client := NewClient("test-key")
			client.baseURL = server.URL // Override URL for testing

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

			// Additional check for ETF type mapping
			if tt.name == "Success - ETF Found" && result.Type != domain.InstrumentTypeETF {
				t.Errorf("Expected instrument type ETF, got %v", result.Type)
			}
		})
	}
}

func TestGetQuote(t *testing.T) {
	tests := []struct {
		name          string
		symbol        string
		mockResponse  string
		expectedPrice string
		expectError   bool
	}{
		{
			name:   "Success",
			symbol: "AAPL",
			mockResponse: `{
				"symbol": "AAPL",
				"name": "Apple Inc",
				"exchange": "NASDAQ",
				"currency": "USD",
				"datetime": "2023-10-27",
				"close": "168.22",
				"status": "ok"
			}`,
			expectedPrice: "168.22",
			expectError:   false,
		},
		{
			name:   "Success - No Status Logic", // Testing the logic update we just made
			symbol: "AAPL",
			mockResponse: `{
				"symbol": "AAPL",
				"close": "168.22"
			}`,
			expectedPrice: "168.22",
			expectError:   false,
		},
		{
			name:   "Status Error",
			symbol: "AAPL",
			mockResponse: `{
				"status": "error",
				"message": "Invalid API Key"
			}`,
			expectedPrice: "",
			expectError:   true,
		},
		{
			name:   "Missing Price",
			symbol: "AAPL",
			mockResponse: `{
				"symbol": "AAPL",
				"close": ""
			}`,
			expectedPrice: "",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(tt.mockResponse))
			}))
			defer server.Close()

			client := NewClient("test-key")
			client.baseURL = server.URL

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

			expectedDecimal, _ := decimal.NewFromString(tt.expectedPrice)
			if !result.Price.Equal(expectedDecimal) {
				t.Errorf("Expected price %s, got %s", tt.expectedPrice, result.Price)
			}
		})
	}
}
