package http

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jmanzanog/stock-tracker/internal/domain"
)

// PortfolioService defines the interface for portfolio operations
type PortfolioService interface {
	AddPosition(ctx context.Context, isin string, amount domain.Decimal, currency string) (*domain.Position, error)
	RemovePosition(ctx context.Context, id string) error
	GetPosition(ctx context.Context, id string) (*domain.Position, error)
	ListPositions(ctx context.Context) ([]domain.Position, error)
	GetPortfolioSummary(ctx context.Context) (*domain.Portfolio, error)
	RefreshPrices(ctx context.Context) error
}

type Handler struct {
	portfolioService PortfolioService
}

func NewHandler(portfolioService PortfolioService) *Handler {
	return &Handler{
		portfolioService: portfolioService,
	}
}

type AddPositionRequest struct {
	ISIN           string         `json:"isin" binding:"required"`
	InvestedAmount domain.Decimal `json:"invested_amount" binding:"required"`
	Currency       string         `json:"currency" binding:"required"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func (h *Handler) AddPosition(c *gin.Context) {
	var req AddPositionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(c.Request.Context(), "Invalid request body", "error", err)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	position, err := h.portfolioService.AddPosition(c.Request.Context(), req.ISIN, req.InvestedAmount, req.Currency)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "Failed to add position", "isin", req.ISIN, "error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, position)
}

func (h *Handler) ListPositions(c *gin.Context) {
	positions, err := h.portfolioService.ListPositions(c.Request.Context())
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "Failed to list positions", "error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, positions)
}

func (h *Handler) GetPosition(c *gin.Context) {
	positionID := c.Param("id")

	position, err := h.portfolioService.GetPosition(c.Request.Context(), positionID)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "Failed to get position", "position_id", positionID, "error", err)
		c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, position)
}

func (h *Handler) DeletePosition(c *gin.Context) {
	positionID := c.Param("id")

	if err := h.portfolioService.RemovePosition(c.Request.Context(), positionID); err != nil {
		slog.ErrorContext(c.Request.Context(), "Failed to delete position", "position_id", positionID, "error", err)
		c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

func (h *Handler) GetPortfolio(c *gin.Context) {
	portfolio, err := h.portfolioService.GetPortfolioSummary(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	totalValue, err := portfolio.TotalValue()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	totalInvested, err := portfolio.TotalInvested()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	totalProfitLoss, err := portfolio.TotalProfitLoss()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	totalProfitLossPercent, err := portfolio.TotalProfitLossPercent()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	summary := map[string]interface{}{
		"id":                        portfolio.ID,
		"name":                      portfolio.Name,
		"positions":                 portfolio.Positions,
		"total_value":               totalValue,
		"total_invested":            totalInvested,
		"total_profit_loss":         totalProfitLoss,
		"total_profit_loss_percent": totalProfitLossPercent,
		"created_at":                portfolio.CreatedAt,
	}

	c.JSON(http.StatusOK, summary)
}

func (h *Handler) RefreshPrices(c *gin.Context) {
	if err := h.portfolioService.RefreshPrices(c.Request.Context()); err != nil {
		slog.ErrorContext(c.Request.Context(), "Failed to refresh prices", "error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "prices refreshed successfully"})
}
