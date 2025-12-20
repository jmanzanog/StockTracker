package twelvedata

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
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
		failConnection bool // Trigger network error
	}{
		{
			name:       "Success - Stock Found",
			isin:       "US0378331005",
			statusCode: http.StatusOK,
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
			name:       "Success - ETF Found",
			isin:       "IE00B3RBWM25",
			statusCode: http.StatusOK,
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
			name:       "Not Found",
			isin:       "INVALIDXXXX",
			statusCode: http.StatusOK,
			mockResponse: `{
				"data": [],
				"status": "ok"
			}`,
			expectedSymbol: "",
			expectError:    true,
		},
		{
			name:           "API Error Response",
			isin:           "US0378331005",
			statusCode:     http.StatusOK, // API returns 200 but status field "error"
			mockResponse:   `{"status": "error", "message": "API Limit Reached"}`,
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
				w.WriteHeader(tt.statusCode)
				_, err := w.Write([]byte(tt.mockResponse))
				if err != nil {
					t.Logf("Error writing response: %v", err)
				}
			}))
			defer server.Close()

			client := NewClient("test-key")
			if tt.failConnection {
				client.baseURL = "http://invalid-url-that-fails" // Invalid URL to trigger Do error
				// Alternatively, simply closing the server client.httpClient could work or invalid transport
				// Simple way:
				client.baseURL = "http://127.0.0.1:0" // Bad port
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
			name:       "Success - No Status Logic",
			symbol:     "AAPL",
			statusCode: http.StatusOK,
			mockResponse: `{
				"symbol": "AAPL",
				"close": "168.22"
			}`,
			expectedPrice: "168.22",
			expectError:   false,
		},
		{
			name:       "Status Error",
			symbol:     "AAPL",
			statusCode: http.StatusOK,
			mockResponse: `{
				"status": "error",
				"message": "Invalid API Key"
			}`,
			expectedPrice: "",
			expectError:   true,
		},
		{
			name:       "Missing Price",
			symbol:     "AAPL",
			statusCode: http.StatusOK,
			mockResponse: `{
				"symbol": "AAPL",
				"close": ""
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
				"close": "invalid-decimal"
			}`,
			expectedPrice: "",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, err := w.Write([]byte(tt.mockResponse))
				if err != nil {
					t.Logf("Error Write %v", err)
				}
			}))
			defer server.Close()

			client := NewClient("test-key")
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
	apiKey := "test-key"
	client := NewClient(apiKey)

	if client.apiKey != apiKey {
		t.Errorf("Expected api key %s, got %s", apiKey, client.apiKey)
	}
	if client.baseURL != defaultBaseURL {
		t.Errorf("Expected base url %s, got %s", defaultBaseURL, client.baseURL)
	}
	if client.httpClient == nil {
		t.Error("Expected http client to be initialized")
	}
}

func TestClient_RequestCreationError(t *testing.T) {
	client := NewClient("test-key")
	// Set baseURL to something with a control character to trigger net/url parse error or http.NewRequest error
	client.baseURL = "http://api.twelvedata.com\x7f"

	_, err := client.SearchByISIN(context.Background(), "US0378331005")
	if err == nil {
		t.Error("Expected error for SearchByISIN with bad URL, got nil")
	}

	_, err = client.GetQuote(context.Background(), "AAPL")
	if err == nil {
		t.Error("Expected error for GetQuote with bad URL, got nil")
	}
}

type errorBody struct{}

func (e *errorBody) Read(_ []byte) (n int, err error) {
	return 0, io.EOF
}

func (e *errorBody) Close() error {
	return context.Canceled // Just an error
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
	client := NewClient("test-key")
	client.httpClient = &http.Client{
		Transport: &errorTransport{},
	}

	// Should not panic or return error, just log warning
	_, _ = client.SearchByISIN(context.Background(), "US123")
	_, _ = client.GetQuote(context.Background(), "AAPL")
}
