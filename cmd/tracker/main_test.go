package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jmanzanog/stock-tracker/internal/application"
	"github.com/jmanzanog/stock-tracker/internal/domain"
	"github.com/jmanzanog/stock-tracker/internal/infrastructure/config"
	"github.com/jmanzanog/stock-tracker/internal/infrastructure/marketdata/twelvedata"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestSetupLogger(t *testing.T) {
	// Capture the original logger to restore it later
	originalLogger := slog.Default()
	defer slog.SetDefault(originalLogger)

	logger := setupLogger()

	if logger == nil {
		t.Fatal("setupLogger returned nil logger")
	}

	// Verify the logger is set as default
	if slog.Default() != logger {
		t.Error("setupLogger did not set the logger as default")
	}

	// Verify the logger can be used (basic smoke test)
	logger.Info("test message", "key", "value")
}

func TestInitializeDatabase_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	// Start PostgreSQL container
	pgContainer, err := postgres.Run(ctx,
		"postgres:18-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}
	defer func() {
		if err := testcontainers.TerminateContainer(pgContainer); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}()

	// Get connection string
	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	cfg := &config.Config{
		DBDriver: "postgres",
		DBDSN:    connStr,
	}

	repo, err := initializeDatabase(cfg)
	if err != nil {
		t.Fatalf("initializeDatabase failed: %v", err)
	}

	if repo == nil {
		t.Fatal("initializeDatabase returned nil repository")
	}

	// Verify the repository is of the correct type
	// We can't check for specific struct type easily if headers are private or using interface,
	// but we can check if it implements the interface.
	// Since initDB returns the interface, this check is implicit.
	// We can check if it works.

	// Verify we can use the repository (basic query)
	_, err = repo.FindByID(ctx, "test-id")
	// Expect an error (not found), but no panic
	if err == nil {
		t.Error("expected error when finding non-existent portfolio, got nil")
	}
}

func TestInitializeDatabase_UnsupportedDriver(t *testing.T) {
	cfg := &config.Config{
		DBDriver: "unsupported",
		DBDSN:    "invalid",
	}

	repo, err := initializeDatabase(cfg)

	if err == nil {
		t.Fatal("expected error for unsupported driver, got nil")
	}

	if repo != nil {
		t.Error("expected nil repository for unsupported driver")
	}

	expectedError := "unsupported database driver"
	if err.Error() != "unsupported database driver: unsupported" {
		t.Errorf("expected error containing '%s', got '%s'", expectedError, err.Error())
	}
}

func TestInitializeDatabase_InvalidDSN(t *testing.T) {
	cfg := &config.Config{
		DBDriver: "postgres",
		DBDSN:    "postgres://invalid:5432/db", // Valid format but unreachable
	}

	// The current implementation of initializeDatabase pings the DB.
	// So it should fail if DB is unreachable.

	repo, err := initializeDatabase(cfg)

	if err == nil {
		// Wait, pgx might not fail on Open, but fails on Ping.
		// My implementation does Ping.
		t.Fatal("expected error for invalid DSN, got nil")
	}

	if repo != nil {
		t.Error("expected nil repository for invalid DSN")
	}
}

func TestBuildServer(t *testing.T) {
	// Create a mock repository
	mockRepo := &mockPortfolioRepository{}

	// Create a mock market data client
	mockMarketData := twelvedata.NewClient("test-key")

	// Create portfolio service
	portfolioService, err := application.NewPortfolioService(mockRepo, mockMarketData)
	if err != nil {
		t.Fatalf("failed to create portfolio service: %v", err)
	}

	// Create config
	cfg := &config.Config{
		ServerHost: "localhost",
		ServerPort: "8080",
	}

	// Build server
	server := buildServer(cfg, portfolioService)

	if server == nil {
		t.Fatal("buildServer returned nil server")
	}

	expectedAddr := "localhost:8080"
	if server.Addr != expectedAddr {
		t.Errorf("expected server addr %s, got %s", expectedAddr, server.Addr)
	}

	if server.Handler == nil {
		t.Error("server handler is nil")
	}
}

// --- App Tests ---

func TestApp_Shutdown(t *testing.T) {
	// Create mock components
	mockRepo := &mockPortfolioRepository{}
	mockMarketData := twelvedata.NewClient("test-key")
	portfolioService, _ := application.NewPortfolioService(mockRepo, mockMarketData)

	cfg := &config.Config{
		ServerHost:           "localhost",
		ServerPort:           "0", // Use port 0 for automatic assignment
		PriceRefreshInterval: 1 * time.Hour,
	}

	ctx, cancel := context.WithCancel(context.Background())
	priceUpdater := application.NewPriceUpdater(portfolioService, cfg.PriceRefreshInterval)
	go priceUpdater.Start(ctx)

	server := buildServer(cfg, portfolioService)

	app := &App{
		Server:        server,
		PriceUpdater:  priceUpdater,
		CancelContext: cancel,
	}

	// Test shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()

	err := app.Shutdown(shutdownCtx)
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

// --- Mock Repository ---

type mockPortfolioRepository struct{}

func (m *mockPortfolioRepository) Save(_ context.Context, _ *domain.Portfolio) error {
	return nil
}

func (m *mockPortfolioRepository) FindByID(_ context.Context, _ string) (*domain.Portfolio, error) {
	return nil, fmt.Errorf("portfolio not found")
}

func (m *mockPortfolioRepository) FindAll(_ context.Context) ([]*domain.Portfolio, error) {
	return []*domain.Portfolio{}, nil
}

func (m *mockPortfolioRepository) Delete(_ context.Context, _ string) error {
	return nil
}

func (m *mockPortfolioRepository) AutoMigrate() error {
	return nil
}

// --- Benchmark ---

func BenchmarkSetupLogger(b *testing.B) {
	// Suppress output during benchmark
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = setupLogger()
	}
}

// --- Integration Test ---

func TestFullInitializationFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Set environment for testing
	_ = os.Setenv("TWELVE_DATA_API_KEY", "test-key")
	defer func() {
		err := os.Unsetenv("TWELVE_DATA_API_KEY")
		if err != nil {
			t.Logf("failed to unset env var: %v", err)
		}
	}()

	ctx := context.Background()

	// Start PostgreSQL container
	pgContainer, err := postgres.Run(ctx,
		"postgres:18-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}
	defer func() {
		if err := testcontainers.TerminateContainer(pgContainer); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}()

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	// Test complete initialization flow
	cfg := &config.Config{
		DBDriver:             "postgres",
		DBDSN:                connStr,
		ServerHost:           "localhost",
		ServerPort:           "0",
		TwelveDataAPIKey:     "test-key",
		PriceRefreshInterval: 1 * time.Hour,
	}

	// Initialize database
	repo, err := initializeDatabase(cfg)
	if err != nil {
		t.Fatalf("database initialization failed: %v", err)
	}

	// Create market data client
	marketDataClient := twelvedata.NewClient(cfg.TwelveDataAPIKey)

	// Create portfolio service
	portfolioService, err := application.NewPortfolioService(repo, marketDataClient)
	if err != nil {
		t.Fatalf("portfolio service creation failed: %v", err)
	}

	// Build server
	server := buildServer(cfg, portfolioService)
	if server == nil {
		t.Fatal("failed to build server")
	}

	// Verify end-to-end: the server can handle requests
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("health check failed: expected 200, got %d", w.Code)
	}

	t.Log("Full initialization flow completed successfully")
}
