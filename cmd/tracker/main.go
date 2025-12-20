package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmanzanog/stock-tracker/internal/application"
	"github.com/jmanzanog/stock-tracker/internal/domain"
	"github.com/jmanzanog/stock-tracker/internal/infrastructure/config"
	"github.com/jmanzanog/stock-tracker/internal/infrastructure/marketdata/twelvedata"
	persistence "github.com/jmanzanog/stock-tracker/internal/infrastructure/persistence/gorm"
	httpHandler "github.com/jmanzanog/stock-tracker/internal/interfaces/http"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
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
	var dialector gorm.Dialector

	switch cfg.DBDriver {
	case "postgres":
		dialector = postgres.Open(cfg.DBDSN)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.DBDriver)
	}

	db, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}

	portfolioRepo := persistence.NewGormRepository(db)
	if err := portfolioRepo.AutoMigrate(); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return portfolioRepo, nil
}

// buildServer creates and configures the HTTP server with all routes and handlers
func buildServer(cfg *config.Config, portfolioService *application.PortfolioService) *http.Server {
	router := gin.Default()
	handler := httpHandler.NewHandler(portfolioService)
	httpHandler.SetupRoutes(router, handler)

	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%s", cfg.ServerHost, cfg.ServerPort),
		Handler: router,
	}

	return server
}

func main() {
	setupLogger()

	if err := godotenv.Load(); err != nil {
		slog.Warn("No .env file found, using environment variables")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	marketDataClient := twelvedata.NewClient(cfg.TwelveDataAPIKey)

	repo, err := initializeDatabase(cfg)
	if err != nil {
		slog.Error("database initialization failed", "error", err)
		os.Exit(1)
	}

	portfolioService, err := application.NewPortfolioService(repo, marketDataClient)
	if err != nil {
		slog.Error("failed to create portfolio service", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	priceUpdater := application.NewPriceUpdater(portfolioService, cfg.PriceRefreshInterval)
	go priceUpdater.Start(ctx)

	server := buildServer(cfg, portfolioService)

	go func() {
		slog.Info("Server starting", "host", cfg.ServerHost, "port", cfg.ServerPort)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Failed to start server", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down server...")

	priceUpdater.Stop()
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("Server exited")
}
