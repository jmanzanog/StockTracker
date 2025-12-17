package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	TwelveDataAPIKey     string
	ServerPort           string
	ServerHost           string
	PriceRefreshInterval time.Duration
	LogLevel             string
	DBDriver             string
	DBDSN                string
}

func Load() (*Config, error) {
	apiKey := os.Getenv("TWELVE_DATA_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("TWELVE_DATA_API_KEY environment variable is required")
	}

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

	return &Config{
		TwelveDataAPIKey:     apiKey,
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
