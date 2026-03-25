package config

import (
	"fmt"
	"os"
	"time"
)

// MarketDataProviderYFinance is the constant for the yfinance-based Market Data Service.
const MarketDataProviderYFinance = "yfinance"

type Config struct {
	YFinanceBaseURL      string
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

	yfinanceBaseURL := getEnvOrDefault("YFINANCE_BASE_URL", "http://localhost:8000")

	return &Config{
		YFinanceBaseURL:      yfinanceBaseURL,
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
