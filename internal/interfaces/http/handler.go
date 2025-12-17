package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jmanzanog/stock-tracker/internal/application"
	"github.com/shopspring/decimal"
)

type Handler struct {
	portfolioService *application.PortfolioService
}

func NewHandler(portfolioService *application.PortfolioService) *Handler {
	return &Handler{
		portfolioService: portfolioService,
	}
}

type AddPositionRequest struct {
	ISIN           string          `json:"isin" binding:"required"`
	InvestedAmount decimal.Decimal `json:"invested_amount" binding:"required"`
	Currency       string          `json:"currency" binding:"required"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func (h *Handler) AddPosition(c *gin.Context) {
	var req AddPositionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	position, err := h.portfolioService.AddPosition(c.Request.Context(), req.ISIN, req.InvestedAmount, req.Currency)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, position)
}

func (h *Handler) ListPositions(c *gin.Context) {
	positions, err := h.portfolioService.ListPositions(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, positions)
}

func (h *Handler) GetPosition(c *gin.Context) {
	positionID := c.Param("id")

	position, err := h.portfolioService.GetPosition(c.Request.Context(), positionID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, position)
}

func (h *Handler) DeletePosition(c *gin.Context) {
	positionID := c.Param("id")

	if err := h.portfolioService.RemovePosition(c.Request.Context(), positionID); err != nil {
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

	summary := map[string]interface{}{
		"id":                        portfolio.ID,
		"name":                      portfolio.Name,
		"positions":                 portfolio.Positions,
		"total_value":               portfolio.TotalValue(),
		"total_invested":            portfolio.TotalInvested(),
		"total_profit_loss":         portfolio.TotalProfitLoss(),
		"total_profit_loss_percent": portfolio.TotalProfitLossPercent(),
		"created_at":                portfolio.CreatedAt,
	}

	c.JSON(http.StatusOK, summary)
}

func (h *Handler) RefreshPrices(c *gin.Context) {
	if err := h.portfolioService.RefreshPrices(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "prices refreshed successfully"})
}
