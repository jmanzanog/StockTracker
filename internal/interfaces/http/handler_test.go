package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jmanzanog/stock-tracker/internal/application"
	"github.com/jmanzanog/stock-tracker/internal/domain"
)

// --- Mock Service ---

type MockPortfolioService struct {
	addPositionFunc         func(ctx context.Context, isin string, amount domain.Decimal, currency string) (*domain.Position, error)
	addPositionsBatchFunc   func(ctx context.Context, requests []application.AddPositionBatchRequest) *application.AddPositionsBatchResult
	removePositionFunc      func(ctx context.Context, id string) error
	getPositionFunc         func(ctx context.Context, id string) (*domain.Position, error)
	listPositionsFunc       func(ctx context.Context) ([]domain.Position, error)
	getPortfolioSummaryFunc func(ctx context.Context) (*domain.Portfolio, error)
	refreshPricesFunc       func(ctx context.Context) error
}

func (m *MockPortfolioService) AddPosition(ctx context.Context, isin string, amount domain.Decimal, currency string) (*domain.Position, error) {
	if m.addPositionFunc != nil {
		return m.addPositionFunc(ctx, isin, amount, currency)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *MockPortfolioService) AddPositionsBatch(ctx context.Context, requests []application.AddPositionBatchRequest) *application.AddPositionsBatchResult {
	if m.addPositionsBatchFunc != nil {
		return m.addPositionsBatchFunc(ctx, requests)
	}
	return &application.AddPositionsBatchResult{
		Successful: make([]application.AddPositionResult, 0),
		Failed:     make([]application.AddPositionResult, 0),
	}
}

func (m *MockPortfolioService) RemovePosition(ctx context.Context, id string) error {
	if m.removePositionFunc != nil {
		return m.removePositionFunc(ctx, id)
	}
	return fmt.Errorf("not implemented")
}

func (m *MockPortfolioService) GetPosition(ctx context.Context, id string) (*domain.Position, error) {
	if m.getPositionFunc != nil {
		return m.getPositionFunc(ctx, id)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *MockPortfolioService) ListPositions(ctx context.Context) ([]domain.Position, error) {
	if m.listPositionsFunc != nil {
		return m.listPositionsFunc(ctx)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *MockPortfolioService) GetPortfolioSummary(ctx context.Context) (*domain.Portfolio, error) {
	if m.getPortfolioSummaryFunc != nil {
		return m.getPortfolioSummaryFunc(ctx)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *MockPortfolioService) RefreshPrices(ctx context.Context) error {
	if m.refreshPricesFunc != nil {
		return m.refreshPricesFunc(ctx)
	}
	return fmt.Errorf("not implemented")
}

// --- Test Setup ---

func setupRouter(handler *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	SetupRoutes(router, handler)
	return router
}

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

// --- AddPosition Tests ---

func TestHandler_AddPosition_Success(t *testing.T) {
	mockService := &MockPortfolioService{
		addPositionFunc: func(ctx context.Context, isin string, amount domain.Decimal, currency string) (*domain.Position, error) {
			instrument := domain.NewInstrument(isin, "AAPL", "Apple Inc.", domain.InstrumentTypeStock, "USD", "NASDAQ")
			position := domain.NewPosition(instrument, amount, currency)
			price := domain.NewDecimalFromInt(150)
			_ = position.UpdatePrice(price)
			return &position, nil
		},
	}

	handler := NewHandler(mockService)
	router := setupRouter(handler)

	reqBody := AddPositionRequest{
		ISIN:           "US0378331005",
		InvestedAmount: domain.NewDecimalFromInt(1000),
		Currency:       "USD",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/positions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var response domain.Position
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response.Instrument.ISIN != "US0378331005" {
		t.Errorf("expected ISIN US0378331005, got %s", response.Instrument.ISIN)
	}
}

func TestHandler_AddPosition_InvalidJSON(t *testing.T) {
	mockService := &MockPortfolioService{}
	handler := NewHandler(mockService)
	router := setupRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/positions", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandler_AddPosition_MissingFields(t *testing.T) {
	mockService := &MockPortfolioService{}
	handler := NewHandler(mockService)
	router := setupRouter(handler)

	testCases := []struct {
		name string
		body map[string]interface{}
	}{
		{
			name: "missing ISIN",
			body: map[string]interface{}{
				"invested_amount": 1000,
				"currency":        "USD",
			},
		},
		{
			name: "missing currency",
			body: map[string]interface{}{
				"isin":            "US0378331005",
				"invested_amount": 1000,
			},
		},
		{
			name: "empty body",
			body: map[string]interface{}{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/positions", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadRequest, w.Code, w.Body.String())
			}
		})
	}
}

func TestHandler_AddPosition_ServiceError(t *testing.T) {
	mockService := &MockPortfolioService{
		addPositionFunc: func(ctx context.Context, isin string, amount domain.Decimal, currency string) (*domain.Position, error) {
			return nil, fmt.Errorf("service error: instrument not found")
		},
	}

	handler := NewHandler(mockService)
	router := setupRouter(handler)

	reqBody := AddPositionRequest{
		ISIN:           "INVALID",
		InvestedAmount: domain.NewDecimalFromInt(1000),
		Currency:       "USD",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/positions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	var errResp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)
	}

	if errResp.Error == "" {
		t.Error("expected non-empty error message")
	}
}

// --- ListPositions Tests ---

func TestHandler_ListPositions_Success(t *testing.T) {
	mockService := &MockPortfolioService{
		listPositionsFunc: func(ctx context.Context) ([]domain.Position, error) {
			instrument := domain.NewInstrument("US0378331005", "AAPL", "Apple Inc.", domain.InstrumentTypeStock, "USD", "NASDAQ")
			position := domain.NewPosition(instrument, domain.NewDecimalFromInt(1000), "USD")
			return []domain.Position{position}, nil
		},
	}

	handler := NewHandler(mockService)
	router := setupRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/positions", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var positions []domain.Position
	if err := json.Unmarshal(w.Body.Bytes(), &positions); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(positions) != 1 {
		t.Errorf("expected 1 position, got %d", len(positions))
	}
}

func TestHandler_ListPositions_Empty(t *testing.T) {
	mockService := &MockPortfolioService{
		listPositionsFunc: func(ctx context.Context) ([]domain.Position, error) {
			return []domain.Position{}, nil
		},
	}

	handler := NewHandler(mockService)
	router := setupRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/positions", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var positions []domain.Position
	if err := json.Unmarshal(w.Body.Bytes(), &positions); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(positions) != 0 {
		t.Errorf("expected 0 positions, got %d", len(positions))
	}
}

func TestHandler_ListPositions_ServiceError(t *testing.T) {
	mockService := &MockPortfolioService{
		listPositionsFunc: func(ctx context.Context) ([]domain.Position, error) {
			return nil, fmt.Errorf("database connection failed")
		},
	}

	handler := NewHandler(mockService)
	router := setupRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/positions", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// --- GetPosition Tests ---

func TestHandler_GetPosition_Success(t *testing.T) {
	mockService := &MockPortfolioService{
		getPositionFunc: func(ctx context.Context, id string) (*domain.Position, error) {
			instrument := domain.NewInstrument("US0378331005", "AAPL", "Apple Inc.", domain.InstrumentTypeStock, "USD", "NASDAQ")
			position := domain.NewPosition(instrument, domain.NewDecimalFromInt(1000), "USD")
			position.ID = id
			return &position, nil
		},
	}

	handler := NewHandler(mockService)
	router := setupRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/positions/test-id", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var position domain.Position
	if err := json.Unmarshal(w.Body.Bytes(), &position); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if position.ID != "test-id" {
		t.Errorf("expected ID test-id, got %s", position.ID)
	}
}

func TestHandler_GetPosition_NotFound(t *testing.T) {
	mockService := &MockPortfolioService{
		getPositionFunc: func(ctx context.Context, id string) (*domain.Position, error) {
			return nil, domain.ErrPositionNotFound
		},
	}

	handler := NewHandler(mockService)
	router := setupRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/positions/non-existent", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}

	var errResp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)
	}

	if errResp.Error == "" {
		t.Error("expected non-empty error message")
	}
}

// --- DeletePosition Tests ---

func TestHandler_DeletePosition_Success(t *testing.T) {
	mockService := &MockPortfolioService{
		removePositionFunc: func(ctx context.Context, id string) error {
			return nil
		},
	}

	handler := NewHandler(mockService)
	router := setupRouter(handler)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/positions/test-id", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, w.Code)
	}
}

func TestHandler_DeletePosition_NotFound(t *testing.T) {
	mockService := &MockPortfolioService{
		removePositionFunc: func(ctx context.Context, id string) error {
			return domain.ErrPositionNotFound
		},
	}

	handler := NewHandler(mockService)
	router := setupRouter(handler)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/positions/non-existent", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// --- GetPortfolio Tests ---

func TestHandler_GetPortfolio_Success(t *testing.T) {
	mockService := &MockPortfolioService{
		getPortfolioSummaryFunc: func(ctx context.Context) (*domain.Portfolio, error) {
			portfolio := domain.NewPortfolio("test-portfolio")
			instrument := domain.NewInstrument("US0378331005", "AAPL", "Apple Inc.", domain.InstrumentTypeStock, "USD", "NASDAQ")
			position := domain.NewPosition(instrument, domain.NewDecimalFromInt(1000), "USD")
			price := domain.NewDecimalFromInt(150)
			_ = position.UpdatePrice(price)
			_ = portfolio.AddPosition(position)
			return &portfolio, nil
		},
	}

	handler := NewHandler(mockService)
	router := setupRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/portfolio", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		t.Logf("Response body: %s", w.Body.String())
	}

	var summary map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &summary); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Verify all expected fields are present
	expectedFields := []string{"id", "name", "positions", "total_value", "total_invested", "total_profit_loss", "total_profit_loss_percent", "created_at"}
	for _, field := range expectedFields {
		if _, ok := summary[field]; !ok {
			t.Errorf("expected field %s in response", field)
		}
	}
}

func TestHandler_GetPortfolio_ServiceError(t *testing.T) {
	mockService := &MockPortfolioService{
		getPortfolioSummaryFunc: func(ctx context.Context) (*domain.Portfolio, error) {
			return nil, fmt.Errorf("database error")
		},
	}

	handler := NewHandler(mockService)
	router := setupRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/portfolio", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// --- RefreshPrices Tests ---

func TestHandler_RefreshPrices_Success(t *testing.T) {
	mockService := &MockPortfolioService{
		refreshPricesFunc: func(ctx context.Context) error {
			return nil
		},
	}

	handler := NewHandler(mockService)
	router := setupRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/portfolio/refresh", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response["message"] != "prices refreshed successfully" {
		t.Errorf("unexpected message: %s", response["message"])
	}
}

func TestHandler_RefreshPrices_ServiceError(t *testing.T) {
	mockService := &MockPortfolioService{
		refreshPricesFunc: func(ctx context.Context) error {
			return fmt.Errorf("market data API unavailable")
		},
	}

	handler := NewHandler(mockService)
	router := setupRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/portfolio/refresh", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// --- NewHandler Tests ---

func TestNewHandler(t *testing.T) {
	mockService := &MockPortfolioService{}
	handler := NewHandler(mockService)

	if handler == nil {
		t.Fatal("expected non-nil handler")
	}

	if handler.portfolioService == nil {
		t.Error("expected non-nil portfolio service")
	}
}
