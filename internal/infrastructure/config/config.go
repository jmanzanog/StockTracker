package config

import (
	"fmt"
	"os"
	"time"
)

// MarketDataProviderTwelveData is the constant for TwelveData provider.
const MarketDataProviderTwelveData = "twelvedata"

// MarketDataProviderFinnhub is the constant for Finnhub provider.
const MarketDataProviderFinnhub = "finnhub"

// MarketDataProviderYFinance is the constant for the yfinance-based Market Data Service.
const MarketDataProviderYFinance = "yfinance"

type Config struct {
	TwelveDataAPIKey     string
	FinnhubAPIKey        string
	YFinanceBaseURL      string
	MarketDataProvider   string
	ServerPort           string
	ServerHost           string
	PriceRefreshInterval time.Duration
	LogLevel             string
	DBDriver             string
	DBDSN                string
}

func Load() (*Config, error) {
	port := getEnvOrDefault("SERVER_PORT", "8080")
	host := getEnvOrDefault("SERVER_HOST", "localhost")
	logLevel := getEnvOrDefault("LOG_LEVEL", "info")

	refreshInterval, err := time.ParseDuration(getEnvOrDefault("PRICE_REFRESH_INTERVAL", "60s"))
	if err != nil {
		return nil, fmt.Errorf("invalid PRICE_REFRESH_INTERVAL: %w", err)
	}

	dbDriver := getEnvOrDefault("DB_DRIVER", "postgres")

	dbDSN := os.Getenv("DB_DSN")
	if dbDSN == "" {
		return nil, fmt.Errorf("DB_DSN environment variable is required")
	}

	marketDataProvider := getEnvOrDefault("MARKET_DATA_PROVIDER", MarketDataProviderTwelveData)

	// Validate market data provider API key based on selected provider
	twelveDataAPIKey := os.Getenv("TWELVE_DATA_API_KEY")
	finnhubAPIKey := os.Getenv("FINNHUB_API_KEY")
	yfinanceBaseURL := getEnvOrDefault("YFINANCE_BASE_URL", "http://localhost:8000")

	switch marketDataProvider {
	case MarketDataProviderTwelveData:
		if twelveDataAPIKey == "" {
			return nil, fmt.Errorf("TWELVE_DATA_API_KEY environment variable is required when using twelvedata provider")
		}
	case MarketDataProviderFinnhub:
		if finnhubAPIKey == "" {
			return nil, fmt.Errorf("FINNHUB_API_KEY environment variable is required when using finnhub provider")
		}
	case MarketDataProviderYFinance:
		// yfinance provider uses a self-hosted microservice, no API key required
		// just validate the base URL is set (has default)
	default:
		return nil, fmt.Errorf("unsupported MARKET_DATA_PROVIDER: %s (supported: %s, %s, %s)",
			marketDataProvider, MarketDataProviderTwelveData, MarketDataProviderFinnhub, MarketDataProviderYFinance)
	}

	return &Config{
		TwelveDataAPIKey:     twelveDataAPIKey,
		FinnhubAPIKey:        finnhubAPIKey,
		YFinanceBaseURL:      yfinanceBaseURL,
		MarketDataProvider:   marketDataProvider,
		ServerPort:           port,
		ServerHost:           host,
		PriceRefreshInterval: refreshInterval,
		LogLevel:             logLevel,
		DBDriver:             dbDriver,
		DBDSN:                dbDSN,
	}, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
