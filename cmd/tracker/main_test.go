package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jmanzanog/stock-tracker/internal/application"
	"github.com/jmanzanog/stock-tracker/internal/domain"
	"github.com/jmanzanog/stock-tracker/internal/infrastructure/config"
	"github.com/jmanzanog/stock-tracker/internal/infrastructure/marketdata/yfinance"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestSetupLogger(t *testing.T) {
	originalLogger := slog.Default()
	defer slog.SetDefault(originalLogger)

	logger := setupLogger()

	if logger == nil {
		t.Fatal("setupLogger returned nil logger")
	}

	if slog.Default() != logger {
		t.Error("setupLogger did not set the logger as default")
	}

	logger.Info("test message", "key", "value")
}

func TestInitializeDatabase_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

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

	cfg := &config.Config{
		DBDriver: "postgres",
		DBDSN:    connStr,
	}

	repo, priceHistoryRepo, err := initializeDatabase(cfg)
	if err != nil {
		t.Fatalf("initializeDatabase failed: %v", err)
	}

	if repo == nil {
		t.Fatal("initializeDatabase returned nil repository")
	}
	_ = priceHistoryRepo

	_, err = repo.FindByID(ctx, "test-id")
	if err == nil {
		t.Error("expected error when finding non-existent portfolio, got nil")
	}
}

func TestInitializeDatabase_UnsupportedDriver(t *testing.T) {
	cfg := &config.Config{
		DBDriver: "unsupported",
		DBDSN:    "invalid",
	}

	repo, priceHistoryRepo, err := initializeDatabase(cfg)

	if err == nil {
		t.Fatal("expected error for unsupported driver, got nil")
	}

	if repo != nil {
		t.Error("expected nil repository for unsupported driver")
	}
	_ = priceHistoryRepo

	expectedError := "unsupported database driver"
	if err.Error() != "unsupported database driver: unsupported" {
		t.Errorf("expected error containing '%s', got '%s'", expectedError, err.Error())
	}
}

func TestInitializeDatabase_InvalidDSN(t *testing.T) {
	cfg := &config.Config{
		DBDriver: "postgres",
		DBDSN:    "postgres://invalid:5432/db",
	}

	repo, priceHistoryRepo, err := initializeDatabase(cfg)

	if err == nil {
		t.Fatal("expected error for invalid DSN, got nil")
	}

	if repo != nil {
		t.Error("expected nil repository for invalid DSN")
	}
	_ = priceHistoryRepo
}

func TestBuildServer(t *testing.T) {
	mockRepo := &mockPortfolioRepository{}
	mockPriceHistoryRepo := &mockPriceHistoryRepository{}

	mockMarketData := yfinance.NewClientWithBaseURL("http://localhost:8000")

	portfolioService, err := application.NewPortfolioService(mockRepo, mockMarketData)
	if err != nil {
		t.Fatalf("failed to create portfolio service: %v", err)
	}

	dashboardService := application.NewDashboardService(mockRepo, mockPriceHistoryRepo)

	cfg := &config.Config{
		ServerHost: "localhost",
		ServerPort: "8080",
	}

	server := buildServer(cfg, portfolioService, dashboardService)

	expectedAddr := "localhost:8080"
	if server.Addr != expectedAddr {
		t.Errorf("expected server addr %s, got %s", expectedAddr, server.Addr)
	}

	if server.Handler == nil {
		t.Error("server handler is nil")
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected dashboard page status %d, got %d", http.StatusOK, w.Code)
	}

	if got := w.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("expected html content type, got %q", got)
	}
}

func TestCreateMarketDataClient(t *testing.T) {
	cfg := &config.Config{
		YFinanceBaseURL: "http://localhost:8000",
	}
	client := createMarketDataClient(cfg)

	if _, ok := client.(*yfinance.Client); !ok {
		t.Fatal("expected YFinance client")
	}
}

func TestRun_ConfigLoadError(t *testing.T) {
	t.Setenv("DB_DSN", "")

	if err := run(); err == nil {
		t.Fatal("expected config load error")
	}
}

func TestRun_InitializeDatabaseError(t *testing.T) {
	t.Setenv("DB_DSN", "postgres://invalid")
	t.Setenv("DB_DRIVER", "unsupported")

	if err := run(); err == nil {
		t.Fatal("expected database initialization error")
	}
}

func TestMain_ExitOnError(t *testing.T) {
	originalRun := runFunc
	originalExit := exitFunc
	defer func() {
		runFunc = originalRun
		exitFunc = originalExit
	}()

	var exitCode int
	runFunc = func() error {
		return fmt.Errorf("boom")
	}
	exitFunc = func(code int) {
		exitCode = code
	}

	main()

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
}

func TestApp_Shutdown(t *testing.T) {
	mockRepo := &mockPortfolioRepository{}
	mockPriceHistoryRepo := &mockPriceHistoryRepository{}
	mockMarketData := yfinance.NewClientWithBaseURL("http://localhost:8000")
	portfolioService, _ := application.NewPortfolioService(mockRepo, mockMarketData)
	dashboardService := application.NewDashboardService(mockRepo, mockPriceHistoryRepo)

	cfg := &config.Config{
		ServerHost:           "localhost",
		ServerPort:           "0",
		PriceRefreshInterval: 1 * time.Hour,
	}

	ctx, cancel := context.WithCancel(context.Background())
	priceUpdater := application.NewPriceUpdater(portfolioService, mockPriceHistoryRepo, cfg.PriceRefreshInterval)
	go priceUpdater.Start(ctx)

	server := buildServer(cfg, portfolioService, dashboardService)

	app := &App{
		Server:        server,
		PriceUpdater:  priceUpdater,
		CancelContext: cancel,
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()

	err := app.Shutdown(shutdownCtx)
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

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

type mockPriceHistoryRepository struct{}

func (m *mockPriceHistoryRepository) SaveBatch(_ context.Context, _ []domain.PriceHistory) error {
	return nil
}

func (m *mockPriceHistoryRepository) GetByISIN(_ context.Context, _ string, _, _ time.Time) ([]domain.PriceHistory, error) {
	return nil, nil
}

func (m *mockPriceHistoryRepository) GetSparkline(_ context.Context, _ string, _ int) ([]domain.PriceHistory, error) {
	return nil, nil
}

func (m *mockPriceHistoryRepository) GetSparklinesBatch(_ context.Context, _ []domain.SparklineRequest) ([]domain.SparklineResult, error) {
	return []domain.SparklineResult{}, nil
}

func (m *mockPriceHistoryRepository) CleanupOlderThan(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}

func BenchmarkSetupLogger(b *testing.B) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = setupLogger()
	}
}

func TestFullInitializationFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

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

	cfg := &config.Config{
		DBDriver:             "postgres",
		DBDSN:                connStr,
		ServerHost:           "localhost",
		ServerPort:           "0",
		YFinanceBaseURL:      "http://localhost:8000",
		PriceRefreshInterval: 1 * time.Hour,
	}

	repo, priceHistoryRepo, err := initializeDatabase(cfg)
	if err != nil {
		t.Fatalf("database initialization failed: %v", err)
	}

	marketDataClient := yfinance.NewClientWithBaseURL(cfg.YFinanceBaseURL)

	portfolioService, err := application.NewPortfolioService(repo, marketDataClient)
	if err != nil {
		t.Fatalf("portfolio service creation failed: %v", err)
	}

	dashboardService := application.NewDashboardService(repo, priceHistoryRepo)

	server := buildServer(cfg, portfolioService, dashboardService)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("health check failed: expected 200, got %d", w.Code)
	}

	t.Log("Full initialization flow completed successfully")
}
