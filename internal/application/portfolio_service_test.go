package application

import (
	"context"
	"testing"

	"github.com/jmanzanog/stock-tracker/internal/domain"
	"github.com/jmanzanog/stock-tracker/internal/infrastructure/marketdata"
)

// --- Mocks ---

type MockRepository struct {
	portfolio *domain.Portfolio
}

func (m *MockRepository) Save(ctx context.Context, p *domain.Portfolio) error {
	m.portfolio = p
	return nil
}

func (m *MockRepository) FindByID(ctx context.Context, id string) (*domain.Portfolio, error) {
	return m.portfolio, nil
}

func (m *MockRepository) FindAll(ctx context.Context) ([]*domain.Portfolio, error) {
	return []*domain.Portfolio{m.portfolio}, nil
}

func (m *MockRepository) Delete(ctx context.Context, id string) error {
	return nil
}

type MockMarketData struct{}

func (m *MockMarketData) SearchByISIN(_ context.Context, isin string) (*domain.Instrument, error) {
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
	return &marketdata.QuoteResult{
		Symbol:   symbol,
		Price:    domain.NewDecimalFromInt(150),
		Currency: "USD",
		Time:     "2023-01-01",
	}, nil
}

// --- Tests ---

func TestAddAndListPosition(t *testing.T) {
	// 1. Setup
	repo := &MockRepository{}
	marketData := &MockMarketData{}
	service, err := NewPortfolioService(repo, marketData)
	if err != nil {
		t.Fatalf("NewPortfolioService failed: %v", err)
	}
	ctx := context.Background()

	// 2. Action: Add Position
	isin := "US0000000001"
	amount := domain.NewDecimalFromInt(1000)
	currency := "USD"

	addedPos, err := service.AddPosition(ctx, isin, amount, currency)
	if err != nil {
		t.Fatalf("AddPosition failed: %v", err)
	}

	// 3. Verify return value
	if addedPos.Instrument.ISIN != isin {
		t.Errorf("Expected ISIN %s, got %s", isin, addedPos.Instrument.ISIN)
	}
	if !addedPos.CurrentPrice.Equal(domain.NewDecimalFromInt(150)) {
		t.Errorf("Expected price 150, got %s", addedPos.CurrentPrice)
	}

	// 4. Action: List Positions
	positions, err := service.ListPositions(ctx)
	if err != nil {
		t.Fatalf("ListPositions failed: %v", err)
	}

	// 5. Verify persistence
	if len(positions) != 1 {
		t.Fatalf("Expected 1 position, got %d", len(positions))
	}

	if positions[0].Instrument.ISIN != isin {
		t.Errorf("List: Expected ISIN %s, got %s", isin, positions[0].Instrument.ISIN)
	}
}
