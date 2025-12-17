package main

import (
	"context"
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

func main() {
	// Setup Structured Logging with Source Info
	opts := &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, opts))
	slog.SetDefault(logger)

	if err := godotenv.Load(); err != nil {
		slog.Warn("No .env file found, using environment variables")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	marketDataClient := twelvedata.NewClient(cfg.TwelveDataAPIKey)

	// Database Setup
	var dialector gorm.Dialector

	switch cfg.DBDriver {
	case "postgres":
		dialector = postgres.Open(cfg.DBDSN)
	default:
		log.Fatalf("Unsupported database driver: %s", cfg.DBDriver)
	}

	db, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		slog.Error("failed to connect database", "error", err)
		os.Exit(1)
	}

	// Initialize Repository
	portfolioRepo := persistence.NewGormRepository(db)
	if err := portfolioRepo.AutoMigrate(); err != nil {
		slog.Error("failed to migrate database", "error", err)
		os.Exit(1)
	}

	// For compatibility, if the interface expects domain.PortfolioRepository,
	// NewGormRepository returns *GormRepository which implements it.
	// Cast explicitly if needed, but Go interface satisfaction is implicit.
	var repo domain.PortfolioRepository = portfolioRepo

	portfolioService := application.NewPortfolioService(repo, marketDataClient)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	priceUpdater := application.NewPriceUpdater(portfolioService, cfg.PriceRefreshInterval)
	go priceUpdater.Start(ctx)

	router := gin.Default()
	handler := httpHandler.NewHandler(portfolioService)
	httpHandler.SetupRoutes(router, handler)

	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%s", cfg.ServerHost, cfg.ServerPort),
		Handler: router,
	}

	go func() {
		slog.Info("Server starting", "host", cfg.ServerHost, "port", cfg.ServerPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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
