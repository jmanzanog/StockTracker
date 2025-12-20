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
	persistence "github.com/jmanzanog/stock-tracker/internal/infrastructure/persistence/gorm"
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
	// We can't easily test the output without capturing stdout,
	// but we can at least verify it doesn't panic
	logger.Info("test message", "key", "value")
}

func TestInitializeDatabase_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	// Start PostgreSQL container
	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
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
	_, ok := repo.(*persistence.GormRepository)
	if !ok {
		t.Errorf("expected *persistence.GormRepository, got %T", repo)
	}

	// Verify we can use the repository (basic query)
	_, err = repo.FindByID(ctx, "test-id")
	if err != nil {
		// Expected error for non-existent ID, repository is still functional
		t.Logf("expected error for non-existent portfolio: %v", err)
	}
}

func TestInitializeDatabase_UnsupportedDriver(t *testing.T) {
	cfg := &config.Config{
		DBDriver: "mysql", // Unsupported driver
		DBDSN:    "some-connection-string",
	}

	repo, err := initializeDatabase(cfg)

	if err == nil {
		t.Fatal("expected error for unsupported driver, got nil")
	}

	if repo != nil {
		t.Errorf("expected nil repository, got %v", repo)
	}

	expectedErrMsg := "unsupported database driver: mysql"
	if err.Error() != expectedErrMsg {
		t.Errorf("expected error message %q, got %q", expectedErrMsg, err.Error())
	}
}

func TestInitializeDatabase_InvalidDSN(t *testing.T) {
	cfg := &config.Config{
		DBDriver: "postgres",
		DBDSN:    "invalid-connection-string",
	}

	repo, err := initializeDatabase(cfg)

	if err == nil {
		t.Fatal("expected error for invalid DSN, got nil")
	}

	if repo != nil {
		t.Errorf("expected nil repository, got %v", repo)
	}
}

func TestBuildServer(t *testing.T) {
	// Suppress Gin debug output during test
	gin := os.Getenv("GIN_MODE")
	if err := os.Setenv("GIN_MODE", "release"); err != nil {
		t.Fatalf("failed to set GIN_MODE: %v", err)
	}
	defer func() {
		if err := os.Setenv("GIN_MODE", gin); err != nil {
			t.Logf("failed to restore GIN_MODE: %v", err)
		}
	}()

	// Create a mock repository
	mockRepo := &mockPortfolioRepository{}

	// Create a mock market data client
	mockClient := twelvedata.NewClient("test-api-key")

	portfolioService, err := application.NewPortfolioService(mockRepo, mockClient)
	if err != nil {
		t.Fatalf("failed to create portfolio service: %v", err)
	}

	cfg := &config.Config{
		ServerHost: "localhost",
		ServerPort: "8080",
	}

	server := buildServer(cfg, portfolioService)

	if server == nil {
		t.Fatal("buildServer returned nil server")
	}

	expectedAddr := "localhost:8080"
	if server.Addr != expectedAddr {
		t.Errorf("expected server address %q, got %q", expectedAddr, server.Addr)
	}

	if server.Handler == nil {
		t.Fatal("server handler is nil")
	}

	// Test that the server can handle a basic request
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	server.Handler.ServeHTTP(w, req)

	// We expect a 200 OK for the health endpoint
	if w.Code != http.StatusOK {
		t.Errorf("expected status code 200, got %d", w.Code)
	}
}

func TestBuildServer_DifferentPorts(t *testing.T) {
	// Suppress Gin debug output during test
	gin := os.Getenv("GIN_MODE")
	if err := os.Setenv("GIN_MODE", "release"); err != nil {
		t.Fatalf("failed to set GIN_MODE: %v", err)
	}
	defer func() {
		if err := os.Setenv("GIN_MODE", gin); err != nil {
			t.Logf("failed to restore GIN_MODE: %v", err)
		}
	}()

	testCases := []struct {
		name string
		host string
		port string
		want string
	}{
		{
			name: "default localhost",
			host: "localhost",
			port: "8080",
			want: "localhost:8080",
		},
		{
			name: "all interfaces",
			host: "0.0.0.0",
			port: "3000",
			want: "0.0.0.0:3000",
		},
		{
			name: "custom port",
			host: "127.0.0.1",
			port: "9090",
			want: "127.0.0.1:9090",
		},
	}

	mockRepo := &mockPortfolioRepository{}
	mockClient := twelvedata.NewClient("test-api-key")
	portfolioService, _ := application.NewPortfolioService(mockRepo, mockClient)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{
				ServerHost: tc.host,
				ServerPort: tc.port,
			}

			server := buildServer(cfg, portfolioService)

			if server.Addr != tc.want {
				t.Errorf("expected server address %q, got %q", tc.want, server.Addr)
			}
		})
	}
}

// mockPortfolioRepository is a minimal mock implementation for testing
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

// TestMain is a special test function that runs before all tests
// We use it to setup global test configuration
func TestMain(m *testing.M) {
	// Suppress all logging during tests to reduce noise
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	// Run all tests
	exitCode := m.Run()

	// Exit with the test result code
	os.Exit(exitCode)
}

// BenchmarkSetupLogger benchmarks the logger setup
func BenchmarkSetupLogger(b *testing.B) {
	for i := 0; i < b.N; i++ {
		setupLogger()
	}
}

// Integration test helper to create a test database configuration
func createTestDBConfig(t *testing.T) (*config.Config, func()) {
	t.Helper()

	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
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

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	cfg := &config.Config{
		DBDriver: "postgres",
		DBDSN:    connStr,
	}

	cleanup := func() {
		if err := testcontainers.TerminateContainer(pgContainer); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}

	return cfg, cleanup
}

// TestFullInitializationFlow tests the complete initialization flow
func TestFullInitializationFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Step 1: Setup logger
	logger := setupLogger()
	if logger == nil {
		t.Fatal("failed to setup logger")
	}

	// Step 2: Initialize database
	cfg, cleanup := createTestDBConfig(t)
	defer cleanup()

	repo, err := initializeDatabase(cfg)
	if err != nil {
		t.Fatalf("failed to initialize database: %v", err)
	}

	// Step 3: Create portfolio service
	mockClient := twelvedata.NewClient("test-api-key")
	portfolioService, err := application.NewPortfolioService(repo, mockClient)
	if err != nil {
		t.Fatalf("failed to create portfolio service: %v", err)
	}

	// Step 4: Build server
	cfg.ServerHost = "localhost"
	cfg.ServerPort = "0" // random port

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
