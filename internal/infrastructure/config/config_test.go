package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoad_Success_YFinance(t *testing.T) {
	t.Setenv("DB_DSN", "postgres://user:pass@localhost:5432/db")
	t.Setenv("YFINANCE_BASE_URL", "http://market-data-service:8000")

	cfg, err := Load()
	assert.NoError(t, err)
	assert.Equal(t, "http://market-data-service:8000", cfg.YFinanceBaseURL)
}

func TestLoad_Success_YFinance_DefaultURL(t *testing.T) {
	t.Setenv("DB_DSN", "postgres://user:pass@localhost:5432/db")
	t.Setenv("YFINANCE_BASE_URL", "")

	cfg, err := Load()
	assert.NoError(t, err)
	assert.Equal(t, "http://localhost:8000", cfg.YFinanceBaseURL)
}

func TestLoad_MissingDBDSN(t *testing.T) {
	t.Setenv("DB_DSN", "")

	_, err := Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "DB_DSN environment variable is required")
}

func TestLoad_InvalidRefreshInterval(t *testing.T) {
	t.Setenv("DB_DSN", "dsn")
	t.Setenv("PRICE_REFRESH_INTERVAL", "invalid")

	_, err := Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid PRICE_REFRESH_INTERVAL")
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("DB_DSN", "dsn")

	cfg, err := Load()
	assert.NoError(t, err)

	assert.Equal(t, "8080", cfg.ServerPort)
	assert.Equal(t, "localhost", cfg.ServerHost)
	assert.Equal(t, "postgres", cfg.DBDriver)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, 60*time.Second, cfg.PriceRefreshInterval)
	assert.Equal(t, "http://localhost:8000", cfg.YFinanceBaseURL)
}

func TestGetEnvOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		expected     string
	}{
		{
			name:         "returns env value when set",
			key:          "TEST_KEY_1",
			defaultValue: "default",
			envValue:     "custom",
			expected:     "custom",
		},
		{
			name:         "returns default when env not set",
			key:          "TEST_KEY_2",
			defaultValue: "default",
			envValue:     "",
			expected:     "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.key, tt.envValue)
			result := getEnvOrDefault(tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMarketDataProviderConstants(t *testing.T) {
	assert.Equal(t, "yfinance", MarketDataProviderYFinance)
}
