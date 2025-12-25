package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmanzanog/stock-tracker/internal/application"
	"github.com/jmanzanog/stock-tracker/internal/domain"
	"github.com/jmanzanog/stock-tracker/internal/infrastructure/config"
	"github.com/jmanzanog/stock-tracker/internal/infrastructure/marketdata"
	"github.com/jmanzanog/stock-tracker/internal/infrastructure/marketdata/finnhub"
	"github.com/jmanzanog/stock-tracker/internal/infrastructure/marketdata/twelvedata"
	"github.com/jmanzanog/stock-tracker/internal/infrastructure/marketdata/yfinance"
	"github.com/jmanzanog/stock-tracker/internal/infrastructure/persistence/sqldb"
	httpHandler "github.com/jmanzanog/stock-tracker/internal/interfaces/http"
	"github.com/joho/godotenv"
	_ "github.com/sijms/go-ora/v2"
)

// setupLogger configures and returns a structured logger with source information
func setupLogger() *slog.Logger {
	opts := &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, opts))
	slog.SetDefault(logger)
	return logger
}

// initializeDatabase sets up the database connection and runs migrations
func initializeDatabase(cfg *config.Config) (domain.PortfolioRepository, error) {
	var db *sql.DB
	var dialect sqldb.Dialect
	var err error

	switch cfg.DBDriver {
	case "postgres":
		db, err = sql.Open("pgx", cfg.DBDSN)
		dialect = &sqldb.PostgresDialect{}
	case "oracle":
		db, err = sql.Open("oracle", cfg.DBDSN)
		dialect = &sqldb.OracleDialect{}
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.DBDriver)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}

	if err := db.Ping(); err != nil {
		_ = db.Close() // Close connection if ping fails
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	wrapper := sqldb.New(db, dialect)

	// Run migrations
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := wrapper.Dialect.Migrate(ctx, db); err != nil {
		_ = db.Close() // Close connection if migration fails
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return sqldb.NewRepository(wrapper), nil
}

// buildServer creates and configures the HTTP server with all routes and handlers
func buildServer(cfg *config.Config, portfolioService *application.PortfolioService) *http.Server {
	router := gin.Default()
	handler := httpHandler.NewHandler(portfolioService)
	httpHandler.SetupRoutes(router, handler)

	server := &http.Server{
		Addr:              fmt.Sprintf("%s:%s", cfg.ServerHost, cfg.ServerPort),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	return server
}

// createMarketDataClient creates the appropriate market data client based on configuration
func createMarketDataClient(cfg *config.Config) marketdata.MDataProvider {
	switch cfg.MarketDataProvider {
	case config.MarketDataProviderFinnhub:
		return finnhub.NewClient(cfg.FinnhubAPIKey)
	case config.MarketDataProviderYFinance:
		return yfinance.NewClientWithBaseURL(cfg.YFinanceBaseURL)
	default:
		return twelvedata.NewClient(cfg.TwelveDataAPIKey)
	}
}

// App wraps the application components for easier testing
type App struct {
	Server        *http.Server
	PriceUpdater  *application.PriceUpdater
	CancelContext context.CancelFunc
}

// Shutdown gracefully shuts down the application
func (a *App) Shutdown(ctx context.Context) error {
	slog.Info("Shutting down application...")

	a.PriceUpdater.Stop()
	a.CancelContext()

	if err := a.Server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown error: %w", err)
	}

	return nil
}

// run contains the main application logic without os.Exit calls
// This makes it testeable
func run() error {
	setupLogger()

	if err := godotenv.Load(); err != nil {
		slog.Warn("No .env file found, using environment variables")
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	marketDataClient := createMarketDataClient(cfg)
	slog.Info("Using market data provider", "provider", cfg.MarketDataProvider)

	repo, err := initializeDatabase(cfg)
	if err != nil {
		return fmt.Errorf("database initialization failed: %w", err)
	}

	portfolioService, err := application.NewPortfolioService(repo, marketDataClient)
	if err != nil {
		return fmt.Errorf("failed to create portfolio service: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	priceUpdater := application.NewPriceUpdater(portfolioService, cfg.PriceRefreshInterval)
	go priceUpdater.Start(ctx)

	server := buildServer(cfg, portfolioService)

	// Create app wrapper
	app := &App{
		Server:        server,
		PriceUpdater:  priceUpdater,
		CancelContext: cancel,
	}

	// Start server in goroutine
	serverErrors := make(chan error, 1)
	go func() {
		slog.Info("Server starting", "host", cfg.ServerHost, "port", cfg.ServerPort)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	// Wait for termination signal or server error
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		return fmt.Errorf("server error: %w", err)
	case <-quit:
		slog.Info("Received shutdown signal")
	}

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := app.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown failed: %w", err)
	}

	slog.Info("Server exited gracefully")
	return nil
}

func main() {
	if err := run(); err != nil {
		slog.Error("Application error", "error", err)
		os.Exit(1)
	}
}
