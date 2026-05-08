package http

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmanzanog/stock-tracker/internal/application"
	"github.com/jmanzanog/stock-tracker/internal/domain"
	"github.com/jmanzanog/stock-tracker/internal/interfaces/http/web"
)

// PortfolioService defines the interface for portfolio operations
type PortfolioService interface {
	AddPosition(ctx context.Context, isin string, amount domain.Decimal, currency string) (*domain.Position, error)
	AddPositionsBatch(ctx context.Context, requests []application.AddPositionBatchRequest) *application.AddPositionsBatchResult
	RemovePosition(ctx context.Context, id string) error
	GetPosition(ctx context.Context, id string) (*domain.Position, error)
	ListPositions(ctx context.Context) ([]domain.Position, error)
	GetPortfolioSummary(ctx context.Context) (*domain.Portfolio, error)
	RefreshPrices(ctx context.Context) error
	SellPartial(ctx context.Context, positionID, quantityStr, priceStr string) (*domain.SellPartialResult, error)
}

type Handler struct {
	portfolioService PortfolioService
	dashboardService *application.DashboardService
}

func NewHandler(portfolioService PortfolioService) *Handler {
	return &Handler{
		portfolioService: portfolioService,
	}
}

func NewHandlerWithDashboard(portfolioService PortfolioService, dashboardService *application.DashboardService) *Handler {
	return &Handler{
		portfolioService: portfolioService,
		dashboardService: dashboardService,
	}
}

type AddPositionRequest struct {
	ISIN           string         `json:"isin" binding:"required"`
	InvestedAmount domain.Decimal `json:"invested_amount" binding:"required"`
	Currency       string         `json:"currency" binding:"required"`
}

type SellPartialRequest struct {
	Quantity domain.Decimal `json:"quantity" binding:"required"`
	Price    domain.Decimal `json:"price" binding:"required"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type DashboardResponse struct {
	PortfolioID   string                       `json:"portfolio_id"`
	GeneratedAt   time.Time                    `json:"generated_at"`
	TotalValue    string                       `json:"total_value"`
	TotalInvested string                       `json:"total_invested"`
	TotalPnL      string                       `json:"total_pnl"`
	PnLPercent    string                       `json:"pnl_percent"`
	ByCurrency    []CurrencyAllocationResponse `json:"by_currency"`
	ByType        []TypeAllocationResponse     `json:"by_type"`
	BySector      []SectorAllocationResponse   `json:"by_sector"`
	Positions     []PositionDashboardResponse  `json:"positions"`
}

type CurrencyAllocationResponse struct {
	Currency   string `json:"currency"`
	TotalValue string `json:"total_value"`
	Percent    string `json:"percent"`
}

type TypeAllocationResponse struct {
	Type       string `json:"type"`
	TotalValue string `json:"total_value"`
	Percent    string `json:"percent"`
}

type SectorAllocationResponse struct {
	Sector     string `json:"sector"`
	TotalValue string `json:"total_value"`
	Percent    string `json:"percent"`
}

type PositionDashboardResponse struct {
	ID             string                              `json:"id"`
	ISIN           string                              `json:"isin"`
	Symbol         string                              `json:"symbol"`
	Name           string                              `json:"name"`
	Type           string                              `json:"type"`
	Sector         string                              `json:"sector"`
	Quantity       string                              `json:"quantity"`
	CurrentPrice   string                              `json:"current_price"`
	CurrentValue   string                              `json:"current_value"`
	InvestedAmount string                              `json:"invested_amount"`
	PnL            string                              `json:"pnl"`
	PnLPercent     string                              `json:"pnl_percent"`
	Currency       string                              `json:"currency"`
	Sparklines     map[string][]SparklinePointResponse `json:"sparklines"`
}

type SparklinePointResponse struct {
	Date  time.Time `json:"date"`
	Price string    `json:"price"`
}

type SellPartialResponse struct {
	Sale       SaleTransactionResponse `json:"sale"`
	Position   *PositionResponse       `json:"position,omitempty"`
	IsFullSale bool                    `json:"is_full_sale"`
}

type SaleTransactionResponse struct {
	ID              string    `json:"id"`
	PositionID      string    `json:"position_id"`
	ISIN            string    `json:"isin"`
	Symbol          string    `json:"symbol"`
	QuantitySold    string    `json:"quantity_sold"`
	SalePrice       string    `json:"sale_price"`
	TotalProceeds   string    `json:"total_proceeds"`
	InvestedSold    string    `json:"invested_sold"`
	ProfitLoss      string    `json:"profit_loss"`
	ProfitLossPct   string    `json:"profit_loss_pct"`
	Currency        string    `json:"currency"`
	SoldAt          time.Time `json:"sold_at"`
	RemainingQty    string    `json:"remaining_qty"`
	RemainingInvest string    `json:"remaining_invest"`
	IsFullSale      bool      `json:"is_full_sale"`
}

type PositionResponse struct {
	ID             string `json:"id"`
	ISIN           string `json:"isin"`
	Symbol         string `json:"symbol"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	Sector         string `json:"sector"`
	Quantity       string `json:"quantity"`
	CurrentPrice   string `json:"current_price"`
	CurrentValue   string `json:"current_value"`
	InvestedAmount string `json:"invested_amount"`
	PnL            string `json:"pnl"`
	PnLPercent     string `json:"pnl_percent"`
	Currency       string `json:"currency"`
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

func (h *Handler) SellPartial(c *gin.Context) {
	positionID := c.Param("id")

	var req SellPartialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(c.Request.Context(), "Invalid sale request body", "error", err)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	result, err := h.portfolioService.SellPartial(c.Request.Context(), positionID, req.Quantity.String(), req.Price.String())
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "Failed to sell position", "position_id", positionID, "error", err)
		statusCode := http.StatusInternalServerError
		if err == domain.ErrPositionNotFound {
			statusCode = http.StatusNotFound
		} else if err == domain.ErrInsufficientShares || err == domain.ErrInvalidSaleQuantity || err == domain.ErrInvalidSalePrice {
			statusCode = http.StatusBadRequest
		}
		c.JSON(statusCode, ErrorResponse{Error: err.Error()})
		return
	}

	response := toSellPartialResponse(result)
	c.JSON(http.StatusOK, response)
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

// AddPositionsBatch handles batch creation of positions.
// It accepts an array of positions directly and returns successful and failed results.
func (h *Handler) AddPositionsBatch(c *gin.Context) {
	var positions []application.AddPositionBatchRequest
	if err := c.ShouldBindJSON(&positions); err != nil {
		slog.ErrorContext(c.Request.Context(), "Invalid batch request body", "error", err)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	if len(positions) == 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "positions array cannot be empty"})
		return
	}

	result := h.portfolioService.AddPositionsBatch(c.Request.Context(), positions)

	// Determine response status based on results
	statusCode := http.StatusCreated
	if len(result.Successful) == 0 && len(result.Failed) > 0 {
		// All failed
		statusCode = http.StatusMultiStatus
	} else if len(result.Failed) > 0 {
		// Partial success
		statusCode = http.StatusMultiStatus
	}

	c.JSON(statusCode, result)
}

func (h *Handler) GetDashboard(c *gin.Context) {
	if h.dashboardService == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "dashboard service not available"})
		return
	}

	sparklineStr := c.DefaultQuery("sparklines", "7,30,90")
	days := parseSparklineDays(sparklineStr)

	snapshot, err := h.dashboardService.GetDashboard(c.Request.Context(), application.GetDashboardRequest{SparklineDays: days})
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "Failed to get dashboard", "error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	response := toDashboardResponse(snapshot)
	c.JSON(http.StatusOK, response)
}

func (h *Handler) DashboardPage(c *gin.Context) {
	content, err := web.DashboardPage()
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "Failed to load dashboard page", "error", err)
		c.String(http.StatusInternalServerError, "dashboard page unavailable")
		return
	}

	c.Data(http.StatusOK, "text/html; charset=utf-8", content)
}

func toDashboardResponse(s *domain.DashboardSnapshot) DashboardResponse {
	positions := make([]PositionDashboardResponse, 0, len(s.Positions))
	for _, pos := range s.Positions {
		sparklines := make(map[string][]SparklinePointResponse)
		for key, points := range pos.Sparklines {
			sparklinePoints := make([]SparklinePointResponse, 0, len(points))
			for _, p := range points {
				sparklinePoints = append(sparklinePoints, SparklinePointResponse{
					Date:  p.Date,
					Price: p.Price.String(),
				})
			}
			sparklines[key] = sparklinePoints
		}
		positions = append(positions, PositionDashboardResponse{
			ID:             pos.ID,
			ISIN:           pos.ISIN,
			Symbol:         pos.Symbol,
			Name:           pos.Name,
			Type:           string(pos.Type),
			Sector:         pos.Sector,
			Quantity:       pos.Quantity.String(),
			CurrentPrice:   pos.CurrentPrice.String(),
			CurrentValue:   pos.CurrentValue.String(),
			InvestedAmount: pos.InvestedAmount.String(),
			PnL:            pos.PnL.String(),
			PnLPercent:     pos.PnLPercent.String(),
			Currency:       pos.Currency,
			Sparklines:     sparklines,
		})
	}

	byCurrency := make([]CurrencyAllocationResponse, 0, len(s.ByCurrency))
	for _, alloc := range s.ByCurrency {
		byCurrency = append(byCurrency, CurrencyAllocationResponse{
			Currency:   alloc.Currency,
			TotalValue: alloc.TotalValue.String(),
			Percent:    alloc.Percent.String(),
		})
	}

	byType := make([]TypeAllocationResponse, 0, len(s.ByType))
	for _, alloc := range s.ByType {
		byType = append(byType, TypeAllocationResponse{
			Type:       string(alloc.Type),
			TotalValue: alloc.TotalValue.String(),
			Percent:    alloc.Percent.String(),
		})
	}

	bySector := make([]SectorAllocationResponse, 0, len(s.BySector))
	for _, alloc := range s.BySector {
		bySector = append(bySector, SectorAllocationResponse{
			Sector:     alloc.Sector,
			TotalValue: alloc.TotalValue.String(),
			Percent:    alloc.Percent.String(),
		})
	}

	return DashboardResponse{
		PortfolioID:   s.PortfolioID,
		GeneratedAt:   s.GeneratedAt,
		TotalValue:    s.TotalValue.String(),
		TotalInvested: s.TotalInvested.String(),
		TotalPnL:      s.TotalPnL.String(),
		PnLPercent:    s.PnLPercent.String(),
		ByCurrency:    byCurrency,
		ByType:        byType,
		BySector:      bySector,
		Positions:     positions,
	}
}

const (
	maxSparklineDays   = 365
	maxSparklineRanges = 5
)

func parseSparklineDays(s string) []int {
	parts := strings.Split(s, ",")
	if len(parts) > maxSparklineRanges {
		parts = parts[:maxSparklineRanges]
	}

	days := make([]int, 0, len(parts))
	for _, p := range parts {
		if d, err := strconv.Atoi(strings.TrimSpace(p)); err == nil && d > 0 && d <= maxSparklineDays {
			days = append(days, d)
		}
	}
	if len(days) == 0 {
		days = []int{7, 30, 90}
	}
	return days
}

func toSellPartialResponse(result *domain.SellPartialResult) SellPartialResponse {
	sale := result.Sale
	saleResp := SaleTransactionResponse{
		ID:              sale.ID,
		PositionID:      sale.PositionID,
		ISIN:            sale.ISIN,
		Symbol:          sale.Symbol,
		QuantitySold:    sale.QuantitySold.String(),
		SalePrice:       sale.SalePrice.String(),
		TotalProceeds:   sale.TotalProceeds.String(),
		InvestedSold:    sale.InvestedSold.String(),
		ProfitLoss:      sale.ProfitLoss.String(),
		ProfitLossPct:   sale.ProfitLossPct.String(),
		Currency:        sale.Currency,
		SoldAt:          sale.SoldAt,
		RemainingQty:    sale.RemainingQty.String(),
		RemainingInvest: sale.RemainingInvest.String(),
		IsFullSale:      sale.IsFullSale,
	}

	var posResp *PositionResponse
	if result.Position != nil {
		pos := result.Position
		currentValue, _ := pos.CurrentValue()
		pnl, _ := pos.ProfitLoss()
		pnlPct, _ := pos.ProfitLossPercent()
		posResp = &PositionResponse{
			ID:             pos.ID,
			ISIN:           pos.Instrument.ISIN,
			Symbol:         pos.Instrument.Symbol,
			Name:           pos.Instrument.Name,
			Type:           string(pos.Instrument.Type),
			Sector:         pos.Instrument.Sector,
			Quantity:       pos.Quantity.String(),
			CurrentPrice:   pos.CurrentPrice.String(),
			CurrentValue:   currentValue.String(),
			InvestedAmount: pos.InvestedAmount.String(),
			PnL:            pnl.String(),
			PnLPercent:     pnlPct.String(),
			Currency:       pos.InvestedCurrency,
		}
	}

	return SellPartialResponse{
		Sale:       saleResp,
		Position:   posResp,
		IsFullSale: result.IsFullSale,
	}
}
