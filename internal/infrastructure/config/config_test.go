package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoad_Success(t *testing.T) {
	// Setup env vars
	t.Setenv("TWELVE_DATA_API_KEY", "test-key")
	t.Setenv("DB_DSN", "postgres://user:pass@localhost:5432/db")
	t.Setenv("PRICE_REFRESH_INTERVAL", "10m")
	t.Setenv("DB_DRIVER", "postgres")

	cfg, err := Load()
	assert.NoError(t, err)
	assert.Equal(t, "test-key", cfg.TwelveDataAPIKey)
	assert.Equal(t, "postgres://user:pass@localhost:5432/db", cfg.DBDSN)
	assert.Equal(t, "postgres", cfg.DBDriver)
	assert.Equal(t, 10*time.Minute, cfg.PriceRefreshInterval)
}

func TestLoad_MissingAPIKey(t *testing.T) {
	// Ensure CLEAN environment
	t.Setenv("TWELVE_DATA_API_KEY", "")

	_, err := Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "TWELVE_DATA_API_KEY")
}

func TestLoad_MissingDBDSN(t *testing.T) {
	t.Setenv("TWELVE_DATA_API_KEY", "key")
	t.Setenv("DB_DSN", "") // Missing

	_, err := Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "DB_DSN environment variable is required")
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("TWELVE_DATA_API_KEY", "key")
	t.Setenv("DB_DSN", "dsn")

	cfg, err := Load()
	assert.NoError(t, err)

	// Check defaults
	assert.Equal(t, "8080", cfg.ServerPort)
	assert.Equal(t, "postgres", cfg.DBDriver)
	assert.Equal(t, "info", cfg.LogLevel)
}
