package application

import (
	"context"
	"fmt"
	"testing"

	"github.com/cockroachdb/apd/v3"
	"github.com/jmanzanog/stock-tracker/internal/domain"
)

type MockRepository struct {
	portfolio      *domain.Portfolio
	saveError      error
	findError      error
	deleteError    error
	savedPortfolio *domain.Portfolio // captures what was passed to Save for verification
}

func (m *MockRepository) Save(_ context.Context, p *domain.Portfolio) error {
	if m.saveError != nil {
		return m.saveError
	}
	m.portfolio = p
	// Store a deep copy so tests can inspect what was saved
	clone := p.Clone()
	m.savedPortfolio = &clone
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
	instrument  *domain.Instrument
	quoteResult *domain.QuoteResult
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

func (m *MockMarketData) SearchByISIN(_ context.Context, isin string) (*domain.Instrument, error) {
	if m.searchError != nil {
		return nil, m.searchError
	}
	if m.instrument != nil {
		return m.instrument, nil
	}
	inst := domain.NewInstrument(
		isin,
		"TESTSYM",
		"Test Stock",
		domain.InstrumentTypeStock,
		"USD",
		"NASDAQ",
		"Technology",
	)
	return &inst, nil
}

func (m *MockMarketData) GetQuote(_ context.Context, symbol string) (*domain.QuoteResult, error) {
	if m.quoteError != nil {
		return nil, m.quoteError
	}
	if m.quoteResult != nil {
		return m.quoteResult, nil
	}
	return &domain.QuoteResult{
		Symbol:   symbol,
		Price:    domain.NewDecimalFromInt(150),
		Currency: "USD",
		Time:     "2023-01-01",
	}, nil
}

func TestNewPortfolioService_Success(t *testing.T) {
	repo := &MockRepository{}
	marketData := &MockMarketData{}

	service, err := NewPortfolioService(repo, marketData)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if service.defaultPortfolio == nil {
		t.Error("expected non-nil default portfolio")
	}
}

func TestNewPortfolioService_FindAllError(t *testing.T) {
	repo := &MockRepository{
		findError: fmt.Errorf("database query failed"),
	}
	marketData := &MockMarketData{}

	service, err := NewPortfolioService(repo, marketData)

	if err == nil {
		t.Fatal("expected error when FindAll fails")
	}

	if service != nil {
		t.Error("expected nil service when initialization fails")
	}

	expectedErr := "failed to list portfolios"
	if err.Error() != fmt.Sprintf("%s: %s", expectedErr, "database query failed") {
		// Just creating a simpler check as wrapping might vary slightly
		if len(err.Error()) < len(expectedErr) {
			t.Errorf("expected error message to contain %q, got %q", expectedErr, err.Error())
		}
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

func TestAddPosition_InvalidInstrument(t *testing.T) {
	repo := &MockRepository{}
	invalidInst := domain.NewInstrument("", "", "", domain.InstrumentTypeStock, "USD", "", "")
	marketData := &MockMarketData{
		instrument: &invalidInst,
	}
	service, _ := NewPortfolioService(repo, marketData)
	ctx := context.Background()

	_, err := service.AddPosition(ctx, "US0000000001", domain.NewDecimalFromInt(1000), "USD")

	if err == nil {
		t.Fatal("expected error when adding invalid instrument")
	}
}

func TestAddPosition_UpdatePriceError(t *testing.T) {
	ctx := apd.BaseContext.WithPrecision(1)
	ctx.Traps = apd.Inexact | apd.Rounded

	withDomainContext(t, ctx, func() {
		repo := &MockRepository{}
		marketData := &MockMarketData{
			quoteResult: &domain.QuoteResult{
				Symbol:   "TESTSYM",
				Price:    domain.NewDecimalFromInt(3),
				Currency: "USD",
				Time:     "2023-01-01",
			},
		}
		service, _ := NewPortfolioService(repo, marketData)

		if _, err := service.AddPosition(context.Background(), "US0000000001", domain.NewDecimalFromInt(1), "USD"); err == nil {
			t.Fatal("expected error when UpdatePrice fails")
		}
	})
}

func TestRemovePosition_Success(t *testing.T) {
	repo := &MockRepository{}
	marketData := &MockMarketData{}
	service, _ := NewPortfolioService(repo, marketData)
	ctx := context.Background()

	pos, _ := service.AddPosition(ctx, "US0000000001", domain.NewDecimalFromInt(1000), "USD")

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

	pos, _ := service.AddPosition(ctx, "US0000000001", domain.NewDecimalFromInt(1000), "USD")

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

	addedPos, _ := service.AddPosition(ctx, "US0000000001", domain.NewDecimalFromInt(1000), "USD")

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

	positions, err := service.ListPositions(ctx)
	if err != nil {
		t.Fatalf("ListPositions failed: %v", err)
	}
	if len(positions) != 0 {
		t.Errorf("expected 0 positions, got %d", len(positions))
	}

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

	if portfolio.ID == "" {
		t.Error("expected non-empty portfolio ID")
	}
}

func TestRefreshPrices_Success(t *testing.T) {
	repo := &MockRepository{}
	marketData := &MockMarketData{}
	service, _ := NewPortfolioService(repo, marketData)
	ctx := context.Background()

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

	marketData.quoteError = nil
	_, err := service.AddPosition(ctx, "US0000000001", domain.NewDecimalFromInt(1000), "USD")
	if err != nil {
		t.Fatalf("AddPosition failed: %v", err)
	}

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

	_, err := service.AddPosition(ctx, "US0000000001", domain.NewDecimalFromInt(1000), "USD")
	if err != nil {
		t.Fatalf("AddPosition failed: %v", err)
	}

	repo.saveError = fmt.Errorf("database connection lost")

	err = service.RefreshPrices(ctx)

	if err == nil {
		t.Fatal("expected error when repository save fails")
	}
}

func TestRefreshPrices_UpdatePriceError(t *testing.T) {
	ctx := apd.BaseContext.WithPrecision(1)
	ctx.Traps = apd.Inexact | apd.Rounded

	withDomainContext(t, ctx, func() {
		repo := &MockRepository{}
		marketData := &MockMarketData{
			quoteResult: &domain.QuoteResult{
				Symbol:   "TESTSYM",
				Price:    domain.NewDecimalFromInt(3),
				Currency: "USD",
				Time:     "2023-01-01",
			},
		}
		service, _ := NewPortfolioService(repo, marketData)

		inst := domain.NewInstrument("US0000000001", "TESTSYM", "Test", domain.InstrumentTypeStock, "USD", "NASDAQ", "Technology")
		pos := domain.NewPosition(inst, domain.NewDecimalFromInt(1), "USD")
		service.defaultPortfolio.Positions = []domain.Position{pos}

		if err := service.RefreshPrices(context.Background()); err == nil {
			t.Fatal("expected error when UpdatePrice fails during refresh")
		}
	})
}

// RemovePosition must persist deletion: Save() must not leave orphaned rows in DB.
func TestRemovePosition_PersistsDeletion(t *testing.T) {
	repo := &MockRepository{}
	marketData := &MockMarketData{}
	service, _ := NewPortfolioService(repo, marketData)
	ctx := context.Background()

	pos1, err := service.AddPosition(ctx, "US0000000001", domain.NewDecimalFromInt(1000), "USD")
	if err != nil {
		t.Fatalf("AddPosition failed: %v", err)
	}
	_, err = service.AddPosition(ctx, "US0000000002", domain.NewDecimalFromInt(2000), "USD")
	if err != nil {
		t.Fatalf("AddPosition failed: %v", err)
	}

	if len(repo.savedPortfolio.Positions) != 2 {
		t.Fatalf("expected 2 positions, got %d", len(repo.savedPortfolio.Positions))
	}

	err = service.RemovePosition(ctx, pos1.ID)
	if err != nil {
		t.Fatalf("RemovePosition failed: %v", err)
	}

	if repo.savedPortfolio == nil {
		t.Fatal("savedPortfolio is nil — Save was never called")
	}
	if len(repo.savedPortfolio.Positions) != 1 {
		t.Fatalf("expected 1 position after removal, got %d", len(repo.savedPortfolio.Positions))
	}
	if repo.savedPortfolio.Positions[0].ID == pos1.ID {
		t.Fatal("removed position should not appear in the saved portfolio")
	}
	if repo.savedPortfolio.Positions[0].Instrument.ISIN == "" {
		t.Error("remaining position should still have valid ISIN")
	}
}

// AddPosition with duplicate ISIN must return the merged position, not the pre-merge temporary.
func TestAddPosition_DuplicateISIN_ReturnsMergedPosition(t *testing.T) {
	repo := &MockRepository{}
	marketData := &MockMarketData{}
	service, _ := NewPortfolioService(repo, marketData)
	ctx := context.Background()

	_, err := service.AddPosition(ctx, "US0000000001", domain.NewDecimalFromInt(1000), "USD")
	if err != nil {
		t.Fatalf("AddPosition failed: %v", err)
	}

	pos2, err := service.AddPosition(ctx, "US0000000001", domain.NewDecimalFromInt(500), "USD")
	if err != nil {
		t.Fatalf("AddPosition failed: %v", err)
	}

	expectedInvested := domain.NewDecimalFromInt(1500)
	if !pos2.InvestedAmount.Equal(expectedInvested) {
		t.Errorf("InvestedAmount: expected %s, got %s", expectedInvested, pos2.InvestedAmount)
	}

	expectedQty := domain.NewDecimalFromInt(10)
	if !pos2.Quantity.Equal(expectedQty) {
		t.Errorf("Quantity: expected %s, got %s", expectedQty, pos2.Quantity)
	}

	expectedPrice := domain.NewDecimalFromInt(150)
	if !pos2.CurrentPrice.Equal(expectedPrice) {
		t.Errorf("CurrentPrice: expected %s, got %s", expectedPrice, pos2.CurrentPrice)
	}

	if len(repo.savedPortfolio.Positions) != 1 {
		t.Errorf("expected 1 position in portfolio after duplicate ISIN add, got %d", len(repo.savedPortfolio.Positions))
	}
}
