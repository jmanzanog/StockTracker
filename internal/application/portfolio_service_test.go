package application

import (
	"context"
	"fmt"
	"testing"

	"github.com/jmanzanog/stock-tracker/internal/domain"
	"github.com/jmanzanog/stock-tracker/internal/infrastructure/marketdata"
)

// --- Mocks ---

type MockRepository struct {
	portfolio   *domain.Portfolio
	saveError   error
	findError   error
	deleteError error
}

func (m *MockRepository) Save(_ context.Context, p *domain.Portfolio) error {
	if m.saveError != nil {
		return m.saveError
	}
	m.portfolio = p
	return nil
}

func (m *MockRepository) FindByID(_ context.Context, _ string) (*domain.Portfolio, error) {
	if m.findError != nil {
		return nil, m.findError
	}
	return m.portfolio, nil
}

func (m *MockRepository) FindAll(_ context.Context) ([]*domain.Portfolio, error) {
	if m.findError != nil {
		return nil, m.findError
	}
	if m.portfolio == nil {
		return []*domain.Portfolio{}, nil
	}
	return []*domain.Portfolio{m.portfolio}, nil
}

func (m *MockRepository) Delete(_ context.Context, _ string) error {
	if m.deleteError != nil {
		return m.deleteError
	}
	return nil
}

type MockMarketData struct {
	searchError error
	quoteError  error
}

func (m *MockMarketData) SearchByISIN(_ context.Context, isin string) (*domain.Instrument, error) {
	if m.searchError != nil {
		return nil, m.searchError
	}
	inst := domain.NewInstrument(
		isin,
		"TESTSYM",
		"Test Stock",
		domain.InstrumentTypeStock,
		"USD",
		"NASDAQ",
	)
	return &inst, nil
}

func (m *MockMarketData) GetQuote(_ context.Context, symbol string) (*marketdata.QuoteResult, error) {
	if m.quoteError != nil {
		return nil, m.quoteError
	}
	return &marketdata.QuoteResult{
		Symbol:   symbol,
		Price:    domain.NewDecimalFromInt(150),
		Currency: "USD",
		Time:     "2023-01-01",
	}, nil
}

// --- Tests ---

func TestNewPortfolioService_Success(t *testing.T) {
	repo := &MockRepository{}
	marketData := &MockMarketData{}

	service, err := NewPortfolioService(repo, marketData)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if service == nil {
		t.Fatal("expected non-nil service")
	}

	if service.defaultPortfolio == nil {
		t.Error("expected non-nil default portfolio")
	}
}

func TestNewPortfolioService_RepositoryError(t *testing.T) {
	repo := &MockRepository{
		saveError: fmt.Errorf("database connection failed"),
	}
	marketData := &MockMarketData{}

	service, err := NewPortfolioService(repo, marketData)

	if err == nil {
		t.Fatal("expected error when repository fails")
	}

	if service != nil {
		t.Error("expected nil service when initialization fails")
	}
}

func TestAddPosition_Success(t *testing.T) {
	repo := &MockRepository{}
	marketData := &MockMarketData{}
	service, _ := NewPortfolioService(repo, marketData)
	ctx := context.Background()

	isin := "US0000000001"
	amount := domain.NewDecimalFromInt(1000)
	currency := "USD"

	pos, err := service.AddPosition(ctx, isin, amount, currency)

	if err != nil {
		t.Fatalf("AddPosition failed: %v", err)
	}

	if pos.Instrument.ISIN != isin {
		t.Errorf("expected ISIN %s, got %s", isin, pos.Instrument.ISIN)
	}

	if !pos.CurrentPrice.Equal(domain.NewDecimalFromInt(150)) {
		t.Errorf("expected price 150, got %s", pos.CurrentPrice)
	}
}

func TestAddPosition_InstrumentNotFound(t *testing.T) {
	repo := &MockRepository{}
	marketData := &MockMarketData{
		searchError: fmt.Errorf("instrument not found"),
	}
	service, _ := NewPortfolioService(repo, marketData)
	ctx := context.Background()

	_, err := service.AddPosition(ctx, "INVALID", domain.NewDecimalFromInt(1000), "USD")

	if err == nil {
		t.Fatal("expected error when instrument not found")
	}
}

func TestAddPosition_QuoteFetchError(t *testing.T) {
	repo := &MockRepository{}
	marketData := &MockMarketData{
		quoteError: fmt.Errorf("market data API unavailable"),
	}
	service, _ := NewPortfolioService(repo, marketData)
	ctx := context.Background()

	_, err := service.AddPosition(ctx, "US0000000001", domain.NewDecimalFromInt(1000), "USD")

	if err == nil {
		t.Fatal("expected error when quote fetch fails")
	}
}

func TestAddPosition_RepositorySaveError(t *testing.T) {
	repo := &MockRepository{
		saveError: fmt.Errorf("database write failed"),
	}
	marketData := &MockMarketData{}
	// Need to create service differently to avoid initial save error
	service := &PortfolioService{
		repo:             repo,
		marketData:       marketData,
		defaultPortfolio: &domain.Portfolio{ID: "test"},
	}
	ctx := context.Background()

	// Reset the error to only affect AddPosition call
	repo.saveError = fmt.Errorf("database write failed")

	_, err := service.AddPosition(ctx, "US0000000001", domain.NewDecimalFromInt(1000), "USD")

	if err == nil {
		t.Fatal("expected error when repository save fails")
	}
}

func TestRemovePosition_Success(t *testing.T) {
	repo := &MockRepository{}
	marketData := &MockMarketData{}
	service, _ := NewPortfolioService(repo, marketData)
	ctx := context.Background()

	// First add a position
	pos, _ := service.AddPosition(ctx, "US0000000001", domain.NewDecimalFromInt(1000), "USD")

	// Then remove it
	err := service.RemovePosition(ctx, pos.ID)

	if err != nil {
		t.Fatalf("RemovePosition failed: %v", err)
	}
}

func TestRemovePosition_NotFound(t *testing.T) {
	repo := &MockRepository{}
	marketData := &MockMarketData{}
	service, _ := NewPortfolioService(repo, marketData)
	ctx := context.Background()

	err := service.RemovePosition(ctx, "non-existent-id")

	if err == nil {
		t.Fatal("expected error when removing non-existent position")
	}
}

func TestRemovePosition_RepositoryError(t *testing.T) {
	repo := &MockRepository{}
	marketData := &MockMarketData{}
	service, _ := NewPortfolioService(repo, marketData)
	ctx := context.Background()

	// Add a position first
	pos, _ := service.AddPosition(ctx, "US0000000001", domain.NewDecimalFromInt(1000), "USD")

	// Set repository error
	repo.saveError = fmt.Errorf("database error")

	err := service.RemovePosition(ctx, pos.ID)

	if err == nil {
		t.Fatal("expected error when repository fails")
	}
}

func TestGetPosition_Success(t *testing.T) {
	repo := &MockRepository{}
	marketData := &MockMarketData{}
	service, _ := NewPortfolioService(repo, marketData)
	ctx := context.Background()

	// Add a position first
	addedPos, _ := service.AddPosition(ctx, "US0000000001", domain.NewDecimalFromInt(1000), "USD")

	// Retrieve it
	pos, err := service.GetPosition(ctx, addedPos.ID)

	if err != nil {
		t.Fatalf("GetPosition failed: %v", err)
	}

	if pos.ID != addedPos.ID {
		t.Errorf("expected ID %s, got %s", addedPos.ID, pos.ID)
	}
}

func TestGetPosition_NotFound(t *testing.T) {
	repo := &MockRepository{}
	marketData := &MockMarketData{}
	service, _ := NewPortfolioService(repo, marketData)
	ctx := context.Background()

	_, err := service.GetPosition(ctx, "non-existent-id")

	if err == nil {
		t.Fatal("expected error when position not found")
	}
}

func TestListPositions_Success(t *testing.T) {
	repo := &MockRepository{}
	marketData := &MockMarketData{}
	service, _ := NewPortfolioService(repo, marketData)
	ctx := context.Background()

	// Initially empty
	positions, err := service.ListPositions(ctx)
	if err != nil {
		t.Fatalf("ListPositions failed: %v", err)
	}
	if len(positions) != 0 {
		t.Errorf("expected 0 positions, got %d", len(positions))
	}

	// Add some positions
	_, err = service.AddPosition(ctx, "US0000000001", domain.NewDecimalFromInt(1000), "USD")
	if err != nil {
		t.Fatalf("AddPosition failed: %v", err)
	}
	_, err = service.AddPosition(ctx, "US0000000002", domain.NewDecimalFromInt(2000), "USD")
	if err != nil {
		t.Fatalf("AddPosition failed: %v", err)
	}

	positions, err = service.ListPositions(ctx)
	if err != nil {
		t.Fatalf("ListPositions failed: %v", err)
	}
	if len(positions) != 2 {
		t.Errorf("expected 2 positions, got %d", len(positions))
	}
}

func TestGetPortfolioSummary_Success(t *testing.T) {
	repo := &MockRepository{}
	marketData := &MockMarketData{}
	service, _ := NewPortfolioService(repo, marketData)
	ctx := context.Background()

	portfolio, err := service.GetPortfolioSummary(ctx)

	if err != nil {
		t.Fatalf("GetPortfolioSummary failed: %v", err)
	}

	if portfolio == nil {
		t.Fatal("expected non-nil portfolio")
	}

	if portfolio.ID == "" {
		t.Error("expected non-empty portfolio ID")
	}
}

func TestRefreshPrices_Success(t *testing.T) {
	repo := &MockRepository{}
	marketData := &MockMarketData{}
	service, _ := NewPortfolioService(repo, marketData)
	ctx := context.Background()

	// Add a position
	_, err := service.AddPosition(ctx, "US0000000001", domain.NewDecimalFromInt(1000), "USD")
	if err != nil {
		t.Fatalf("AddPosition failed: %v", err)
	}

	err = service.RefreshPrices(ctx)

	if err != nil {
		t.Fatalf("RefreshPrices failed: %v", err)
	}
}

func TestRefreshPrices_EmptyPortfolio(t *testing.T) {
	repo := &MockRepository{}
	marketData := &MockMarketData{}
	service, _ := NewPortfolioService(repo, marketData)
	ctx := context.Background()

	// No positions
	err := service.RefreshPrices(ctx)

	if err != nil {
		t.Fatalf("RefreshPrices should succeed with empty portfolio: %v", err)
	}
}

func TestRefreshPrices_MarketDataError(t *testing.T) {
	repo := &MockRepository{}
	marketData := &MockMarketData{
		quoteError: fmt.Errorf("API rate limit exceeded"),
	}
	service, _ := NewPortfolioService(repo, marketData)
	ctx := context.Background()

	// Add a position first (before setting quote error)
	marketData.quoteError = nil
	_, err := service.AddPosition(ctx, "US0000000001", domain.NewDecimalFromInt(1000), "USD")
	if err != nil {
		t.Fatalf("AddPosition failed: %v", err)
	}

	// Now set the error
	marketData.quoteError = fmt.Errorf("API rate limit exceeded")

	err = service.RefreshPrices(ctx)

	if err == nil {
		t.Fatal("expected error when market data fetch fails")
	}
}

func TestRefreshPrices_RepositoryError(t *testing.T) {
	repo := &MockRepository{}
	marketData := &MockMarketData{}
	service, _ := NewPortfolioService(repo, marketData)
	ctx := context.Background()

	// Add a position
	_, err := service.AddPosition(ctx, "US0000000001", domain.NewDecimalFromInt(1000), "USD")
	if err != nil {
		t.Fatalf("AddPosition failed: %v", err)
	}

	// Set repository error
	repo.saveError = fmt.Errorf("database connection lost")

	err = service.RefreshPrices(ctx)

	if err == nil {
		t.Fatal("expected error when repository save fails")
	}
}
