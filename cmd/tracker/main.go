package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmanzanog/stock-tracker/internal/application"
	"github.com/jmanzanog/stock-tracker/internal/infrastructure/config"
	"github.com/jmanzanog/stock-tracker/internal/infrastructure/marketdata/twelvedata"
	"github.com/jmanzanog/stock-tracker/internal/infrastructure/persistence/memory"
	httpHandler "github.com/jmanzanog/stock-tracker/internal/interfaces/http"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	marketDataClient := twelvedata.NewClient(cfg.TwelveDataAPIKey)
	portfolioRepo := memory.NewPortfolioRepository()
	portfolioService := application.NewPortfolioService(portfolioRepo, marketDataClient)

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
		log.Printf("Server starting on %s:%s", cfg.ServerHost, cfg.ServerPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	priceUpdater.Stop()
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
