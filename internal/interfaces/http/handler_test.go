package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cockroachdb/apd/v3"
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

func withDomainContext(t *testing.T, ctx *apd.Context, fn func()) {
	t.Helper()

	original := domain.DefaultContext
	domain.DefaultContext = ctx

	t.Cleanup(func() {
		domain.DefaultContext = original
	})

	fn()
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
			instrument := domain.NewInstrument(isin, "AAPL", "Apple Inc.", domain.InstrumentTypeStock, "USD", "NASDAQ", "Technology")
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

// --- AddPositionsBatch Tests ---

func TestHandler_AddPositionsBatch_Success(t *testing.T) {
	mockService := &MockPortfolioService{
		addPositionsBatchFunc: func(ctx context.Context, requests []application.AddPositionBatchRequest) *application.AddPositionsBatchResult {
			instrument := domain.NewInstrument(requests[0].ISIN, "AAPL", "Apple Inc.", domain.InstrumentTypeStock, "USD", "NASDAQ", "Technology")
			position := domain.NewPosition(instrument, requests[0].InvestedAmount, requests[0].Currency)

			return &application.AddPositionsBatchResult{
				Successful: []application.AddPositionResult{
					{ISIN: requests[0].ISIN, Position: &position},
				},
				Failed: []application.AddPositionResult{},
			}
		},
	}

	handler := NewHandler(mockService)
	router := setupRouter(handler)

	reqBody := []application.AddPositionBatchRequest{
		{ISIN: "US0378331005", InvestedAmount: domain.NewDecimalFromInt(1000), Currency: "USD"},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/positions/batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var result application.AddPositionsBatchResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(result.Successful) != 1 {
		t.Errorf("expected 1 successful position, got %d", len(result.Successful))
	}
}

func TestHandler_AddPositionsBatch_PartialSuccess(t *testing.T) {
	mockService := &MockPortfolioService{
		addPositionsBatchFunc: func(ctx context.Context, requests []application.AddPositionBatchRequest) *application.AddPositionsBatchResult {
			instrument := domain.NewInstrument(requests[0].ISIN, "AAPL", "Apple Inc.", domain.InstrumentTypeStock, "USD", "NASDAQ", "Technology")
			position := domain.NewPosition(instrument, requests[0].InvestedAmount, requests[0].Currency)

			return &application.AddPositionsBatchResult{
				Successful: []application.AddPositionResult{
					{ISIN: requests[0].ISIN, Position: &position},
				},
				Failed: []application.AddPositionResult{
					{ISIN: "INVALID", Error: "instrument not found"},
				},
			}
		},
	}

	handler := NewHandler(mockService)
	router := setupRouter(handler)

	reqBody := []application.AddPositionBatchRequest{
		{ISIN: "US0378331005", InvestedAmount: domain.NewDecimalFromInt(1000), Currency: "USD"},
		{ISIN: "INVALID", InvestedAmount: domain.NewDecimalFromInt(2000), Currency: "USD"},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/positions/batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusMultiStatus {
		t.Errorf("expected status %d (Multi-Status), got %d", http.StatusMultiStatus, w.Code)
	}

	var result application.AddPositionsBatchResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(result.Successful) != 1 || len(result.Failed) != 1 {
		t.Errorf("expected 1 successful and 1 failed, got %d and %d", len(result.Successful), len(result.Failed))
	}
}

func TestHandler_AddPositionsBatch_EmptyArray(t *testing.T) {
	mockService := &MockPortfolioService{}
	handler := NewHandler(mockService)
	router := setupRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/positions/batch", bytes.NewReader([]byte("[]")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandler_AddPositionsBatch_InvalidJSON(t *testing.T) {
	mockService := &MockPortfolioService{}
	handler := NewHandler(mockService)
	router := setupRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/positions/batch", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandler_AddPositionsBatch_AllFailed(t *testing.T) {
	mockService := &MockPortfolioService{
		addPositionsBatchFunc: func(ctx context.Context, requests []application.AddPositionBatchRequest) *application.AddPositionsBatchResult {
			return &application.AddPositionsBatchResult{
				Successful: []application.AddPositionResult{},
				Failed: []application.AddPositionResult{
					{ISIN: requests[0].ISIN, Error: "failed"},
				},
			}
		},
	}

	handler := NewHandler(mockService)
	router := setupRouter(handler)

	reqBody := []application.AddPositionBatchRequest{
		{ISIN: "US0378331005", InvestedAmount: domain.NewDecimalFromInt(1000), Currency: "USD"},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/positions/batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusMultiStatus {
		t.Errorf("expected status %d, got %d", http.StatusMultiStatus, w.Code)
	}
}

// --- ListPositions Tests ---

func TestHandler_ListPositions_Success(t *testing.T) {
	mockService := &MockPortfolioService{
		listPositionsFunc: func(ctx context.Context) ([]domain.Position, error) {
			instrument := domain.NewInstrument("US0378331005", "AAPL", "Apple Inc.", domain.InstrumentTypeStock, "USD", "NASDAQ", "Technology")
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
			instrument := domain.NewInstrument("US0378331005", "AAPL", "Apple Inc.", domain.InstrumentTypeStock, "USD", "NASDAQ", "Technology")
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
			instrument := domain.NewInstrument("US0378331005", "AAPL", "Apple Inc.", domain.InstrumentTypeStock, "USD", "NASDAQ", "Technology")
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

func TestHandler_GetPortfolio_TotalValueError(t *testing.T) {
	ctx := apd.BaseContext.WithPrecision(1)
	ctx.Traps = apd.Inexact | apd.Rounded

	withDomainContext(t, ctx, func() {
		mockService := &MockPortfolioService{
			getPortfolioSummaryFunc: func(ctx context.Context) (*domain.Portfolio, error) {
				portfolio := domain.NewPortfolio("test-portfolio")
				instrument := domain.NewInstrument("US0378331005", "AAPL", "Apple Inc.", domain.InstrumentTypeStock, "USD", "NASDAQ", "Technology")
				position := domain.NewPosition(instrument, domain.NewDecimalFromInt(1), "USD")
				position.Quantity, _ = domain.NewDecimalFromString("1.1")
				position.CurrentPrice, _ = domain.NewDecimalFromString("1.1")
				_ = portfolio.AddPosition(position)
				return &portfolio, nil
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
	})
}

func TestHandler_GetPortfolio_TotalInvestedError(t *testing.T) {
	ctx := apd.BaseContext.WithPrecision(1)
	ctx.Traps = apd.Inexact | apd.Rounded

	withDomainContext(t, ctx, func() {
		mockService := &MockPortfolioService{
			getPortfolioSummaryFunc: func(ctx context.Context) (*domain.Portfolio, error) {
				portfolio := domain.NewPortfolio("test-portfolio")
				instrument := domain.NewInstrument("US0378331005", "AAPL", "Apple Inc.", domain.InstrumentTypeStock, "USD", "NASDAQ", "Technology")
				position := domain.NewPosition(instrument, domain.NewDecimalFromInt(1), "USD")
				position.InvestedAmount, _ = domain.NewDecimalFromString("1.03")
				position.CurrentPrice = domain.Zero
				_ = portfolio.AddPosition(position)
				return &portfolio, nil
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
	})
}

func TestHandler_GetPortfolio_TotalProfitLossError(t *testing.T) {
	ctx := apd.BaseContext.WithPrecision(1)
	ctx.Traps = apd.Inexact | apd.Rounded

	withDomainContext(t, ctx, func() {
		mockService := &MockPortfolioService{
			getPortfolioSummaryFunc: func(ctx context.Context) (*domain.Portfolio, error) {
				portfolio := domain.NewPortfolio("test-portfolio")
				instrument := domain.NewInstrument("US0378331005", "AAPL", "Apple Inc.", domain.InstrumentTypeStock, "USD", "NASDAQ", "Technology")
				position := domain.NewPosition(instrument, domain.NewDecimalFromInt(1), "USD")
				position.InvestedAmount, _ = domain.NewDecimalFromString("0.03")
				position.Quantity = domain.NewDecimalFromInt(1)
				position.CurrentPrice = domain.NewDecimalFromInt(1)
				_ = portfolio.AddPosition(position)
				return &portfolio, nil
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
	})
}

func TestHandler_GetPortfolio_TotalProfitLossPercentError(t *testing.T) {
	ctx := apd.BaseContext.WithPrecision(10)
	ctx.MaxExponent = 1
	ctx.Traps = apd.Overflow

	withDomainContext(t, ctx, func() {
		mockService := &MockPortfolioService{
			getPortfolioSummaryFunc: func(ctx context.Context) (*domain.Portfolio, error) {
				portfolio := domain.NewPortfolio("test-portfolio")
				instrument := domain.NewInstrument("US0378331005", "AAPL", "Apple Inc.", domain.InstrumentTypeStock, "USD", "NASDAQ", "Technology")
				position := domain.NewPosition(instrument, domain.NewDecimalFromInt(1), "USD")
				position.Quantity = domain.NewDecimalFromInt(2)
				position.CurrentPrice = domain.NewDecimalFromInt(1)
				_ = portfolio.AddPosition(position)
				return &portfolio, nil
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
	})
}

func TestHandler_HealthCheck(t *testing.T) {
	mockService := &MockPortfolioService{}
	handler := NewHandler(mockService)
	router := setupRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
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

	if handler.portfolioService == nil {
		t.Error("expected non-nil portfolio service")
	}
}

func TestNewHandlerWithDashboard(t *testing.T) {
	mockService := &MockPortfolioService{}
	dashboardService := application.NewDashboardService(&mockPortfolioRepoForDashboard{}, &mockPriceHistoryRepoForDashboard{})

	handler := NewHandlerWithDashboard(mockService, dashboardService)

	if handler.portfolioService == nil {
		t.Error("expected non-nil portfolio service")
	}
	if handler.dashboardService == nil {
		t.Error("expected non-nil dashboard service")
	}
}

func TestGetDashboard_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := &MockPortfolioService{}
	dashboardService := application.NewDashboardService(&mockPortfolioRepoForDashboard{}, &mockPriceHistoryRepoForDashboard{})

	handler := NewHandlerWithDashboard(mockService, dashboardService)
	router := setupRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard?sparklines=7,30", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response["portfolio_id"] != "default" {
		t.Errorf("expected portfolio_id 'default', got %v", response["portfolio_id"])
	}
}

func TestGetDashboard_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := &MockPortfolioService{}
	dashboardService := application.NewDashboardService(&mockPortfolioRepoWithError{}, &mockPriceHistoryRepoForDashboard{})

	handler := NewHandlerWithDashboard(mockService, dashboardService)
	router := setupRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestGetDashboard_NoDashboardService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := &MockPortfolioService{}
	handler := NewHandlerWithDashboard(mockService, nil)
	router := setupRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
	}
}

func TestDashboardPageRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := &MockPortfolioService{}
	handler := NewHandler(mockService)
	router := setupRouter(handler)

	for _, path := range []string{"/"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
			}

			if got := w.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
				t.Fatalf("expected html content type, got %q", got)
			}

			body := w.Body.String()
			if !strings.Contains(body, "Stock Tracker Dashboard") {
				t.Fatalf("expected dashboard title in response body")
			}

			if !strings.Contains(body, "api/v1/dashboard?sparklines=7") {
				t.Fatalf("expected dashboard API bootstrap URL in response body")
			}
		})
	}
}

func TestParseSparklineDays(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []int
	}{
		{"valid single value", "7", []int{7}},
		{"valid multiple values", "7,30,90", []int{7, 30, 90}},
		{"with spaces", "7, 30, 90", []int{7, 30, 90}},
		{"exceeds max ranges", "1,2,3,4,5,6", []int{1, 2, 3, 4, 5}},
		{"exceeds max days", "400", []int{7, 30, 90}},
		{"negative value", "-7", []int{7, 30, 90}},
		{"invalid value", "abc", []int{7, 30, 90}},
		{"empty string", "", []int{7, 30, 90}},
		{"mixed valid and invalid", "7,abc,90", []int{7, 90}},
		{"zero value", "0", []int{7, 30, 90}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := parseSparklineDays(tc.input)
			if len(result) != len(tc.expected) {
				t.Errorf("expected length %d, got %d", len(tc.expected), len(result))
				return
			}
			for i, v := range result {
				if v != tc.expected[i] {
					t.Errorf("expected[%d]=%d, got %d", i, tc.expected[i], v)
				}
			}
		})
	}
}

type mockPortfolioRepoForDashboard struct{}

func (m *mockPortfolioRepoForDashboard) FindByID(ctx context.Context, id string) (*domain.Portfolio, error) {
	return &domain.Portfolio{
		ID: "default",
		Positions: []domain.Position{
			domain.NewPosition(
				domain.NewInstrument("US0378331005", "AAPL", "Apple Inc.", domain.InstrumentTypeStock, "USD", "NASDAQ", "Technology"),
				domain.NewDecimalFromInt(10000),
				"USD",
			),
		},
	}, nil
}

func (m *mockPortfolioRepoForDashboard) Save(ctx context.Context, p *domain.Portfolio) error {
	return nil
}

func (m *mockPortfolioRepoForDashboard) FindAll(ctx context.Context) ([]*domain.Portfolio, error) {
	return []*domain.Portfolio{}, nil
}

func (m *mockPortfolioRepoForDashboard) Delete(ctx context.Context, id string) error {
	return nil
}

type mockPriceHistoryRepoForDashboard struct{}

func (m *mockPriceHistoryRepoForDashboard) SaveBatch(ctx context.Context, history []domain.PriceHistory) error {
	return nil
}

func (m *mockPriceHistoryRepoForDashboard) GetByISIN(ctx context.Context, isin string, from, to time.Time) ([]domain.PriceHistory, error) {
	return nil, nil
}

func (m *mockPriceHistoryRepoForDashboard) GetSparkline(ctx context.Context, isin string, days int) ([]domain.PriceHistory, error) {
	return nil, nil
}

func (m *mockPriceHistoryRepoForDashboard) GetSparklinesBatch(ctx context.Context, requests []domain.SparklineRequest) ([]domain.SparklineResult, error) {
	return []domain.SparklineResult{}, nil
}

func (m *mockPriceHistoryRepoForDashboard) CleanupOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	return 0, nil
}

type mockPortfolioRepoWithError struct{}

func (m *mockPortfolioRepoWithError) FindByID(ctx context.Context, id string) (*domain.Portfolio, error) {
	return nil, fmt.Errorf("database error")
}

func (m *mockPortfolioRepoWithError) Save(ctx context.Context, p *domain.Portfolio) error {
	return nil
}

func (m *mockPortfolioRepoWithError) FindAll(ctx context.Context) ([]*domain.Portfolio, error) {
	return nil, nil
}

func (m *mockPortfolioRepoWithError) Delete(ctx context.Context, id string) error {
	return nil
}
