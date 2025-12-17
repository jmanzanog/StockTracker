package application

import (
	"context"
	"testing"

	"github.com/jmanzanog/stock-tracker/internal/domain"
	"github.com/jmanzanog/stock-tracker/internal/infrastructure/marketdata"
	"github.com/shopspring/decimal"
)

// --- Mocks ---

type MockRepository struct {
	portfolio *domain.Portfolio
}

func (m *MockRepository) Save(p *domain.Portfolio) error {
	m.portfolio = p
	return nil
}

func (m *MockRepository) FindByID(id string) (*domain.Portfolio, error) {
	return m.portfolio, nil
}

func (m *MockRepository) FindAll() ([]*domain.Portfolio, error) {
	return []*domain.Portfolio{m.portfolio}, nil
}

func (m *MockRepository) Delete(id string) error {
	return nil
}

type MockMarketData struct{}

func (m *MockMarketData) SearchByISIN(ctx context.Context, isin string) (*domain.Instrument, error) {
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

func (m *MockMarketData) GetQuote(ctx context.Context, symbol string) (*marketdata.QuoteResult, error) {
	return &marketdata.QuoteResult{
		Symbol:   symbol,
		Price:    decimal.NewFromInt(150),
		Currency: "USD",
		Time:     "2023-01-01",
	}, nil
}

// --- Tests ---

func TestAddAndListPosition(t *testing.T) {
	// 1. Setup
	repo := &MockRepository{}
	marketData := &MockMarketData{}
	service := NewPortfolioService(repo, marketData)
	ctx := context.Background()

	// 2. Action: Add Position
	isin := "US0000000001"
	amount := decimal.NewFromInt(1000)
	currency := "USD"

	addedPos, err := service.AddPosition(ctx, isin, amount, currency)
	if err != nil {
		t.Fatalf("AddPosition failed: %v", err)
	}

	// 3. Verify return value
	if addedPos.Instrument.ISIN != isin {
		t.Errorf("Expected ISIN %s, got %s", isin, addedPos.Instrument.ISIN)
	}
	if !addedPos.CurrentPrice.Equal(decimal.NewFromInt(150)) {
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
