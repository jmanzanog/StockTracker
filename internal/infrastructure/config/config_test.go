package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoad_Success_TwelveData(t *testing.T) {
	// Setup env vars for TwelveData provider (default)
	t.Setenv("TWELVE_DATA_API_KEY", "test-key")
	t.Setenv("DB_DSN", "postgres://user:pass@localhost:5432/db")
	t.Setenv("PRICE_REFRESH_INTERVAL", "10m")
	t.Setenv("DB_DRIVER", "postgres")
	t.Setenv("MARKET_DATA_PROVIDER", "twelvedata")

	cfg, err := Load()
	assert.NoError(t, err)
	assert.Equal(t, "test-key", cfg.TwelveDataAPIKey)
	assert.Equal(t, "postgres://user:pass@localhost:5432/db", cfg.DBDSN)
	assert.Equal(t, "postgres", cfg.DBDriver)
	assert.Equal(t, 10*time.Minute, cfg.PriceRefreshInterval)
	assert.Equal(t, "twelvedata", cfg.MarketDataProvider)
}

func TestLoad_Success_Finnhub(t *testing.T) {
	// Setup env vars for Finnhub provider
	t.Setenv("FINNHUB_API_KEY", "finnhub-test-key")
	t.Setenv("DB_DSN", "postgres://user:pass@localhost:5432/db")
	t.Setenv("MARKET_DATA_PROVIDER", "finnhub")

	cfg, err := Load()
	assert.NoError(t, err)
	assert.Equal(t, "finnhub-test-key", cfg.FinnhubAPIKey)
	assert.Equal(t, "finnhub", cfg.MarketDataProvider)
}

func TestLoad_MissingTwelveDataAPIKey(t *testing.T) {
	// Ensure CLEAN environment
	t.Setenv("TWELVE_DATA_API_KEY", "")
	t.Setenv("DB_DSN", "dsn")
	t.Setenv("MARKET_DATA_PROVIDER", "twelvedata")

	_, err := Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "TWELVE_DATA_API_KEY")
	assert.Contains(t, err.Error(), "twelvedata provider")
}

func TestLoad_MissingFinnhubAPIKey(t *testing.T) {
	t.Setenv("FINNHUB_API_KEY", "")
	t.Setenv("DB_DSN", "dsn")
	t.Setenv("MARKET_DATA_PROVIDER", "finnhub")

	_, err := Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "FINNHUB_API_KEY")
	assert.Contains(t, err.Error(), "finnhub provider")
}

func TestLoad_UnsupportedProvider(t *testing.T) {
	t.Setenv("DB_DSN", "dsn")
	t.Setenv("MARKET_DATA_PROVIDER", "unsupported_provider")

	_, err := Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported MARKET_DATA_PROVIDER")
	assert.Contains(t, err.Error(), "unsupported_provider")
}

func TestLoad_MissingDBDSN(t *testing.T) {
	t.Setenv("TWELVE_DATA_API_KEY", "key")
	t.Setenv("DB_DSN", "") // Missing

	_, err := Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "DB_DSN environment variable is required")
}

func TestLoad_InvalidRefreshInterval(t *testing.T) {
	t.Setenv("TWELVE_DATA_API_KEY", "key")
	t.Setenv("DB_DSN", "dsn")
	t.Setenv("PRICE_REFRESH_INTERVAL", "invalid")

	_, err := Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid PRICE_REFRESH_INTERVAL")
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("TWELVE_DATA_API_KEY", "key")
	t.Setenv("DB_DSN", "dsn")
	// Clear MARKET_DATA_PROVIDER to use default
	t.Setenv("MARKET_DATA_PROVIDER", "")

	cfg, err := Load()
	assert.NoError(t, err)

	// Check defaults
	assert.Equal(t, "8080", cfg.ServerPort)
	assert.Equal(t, "localhost", cfg.ServerHost)
	assert.Equal(t, "postgres", cfg.DBDriver)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "twelvedata", cfg.MarketDataProvider) // Default provider
	assert.Equal(t, 60*time.Second, cfg.PriceRefreshInterval)
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
	assert.Equal(t, "twelvedata", MarketDataProviderTwelveData)
	assert.Equal(t, "finnhub", MarketDataProviderFinnhub)
}
