package domain

import (
	"errors"
	"testing"
)

// --- SellPartial Tests ---

func TestSellPartial_Success(t *testing.T) {
	p := NewPortfolio("test")

	// Add a position: 10 shares @ $100 = $1000 invested
	instrument := Instrument{
		ISIN:   "US0378331005",
		Symbol: "AAPL",
		Name:   "Apple Inc",
		Type:   InstrumentTypeStock,
		Sector: "Technology",
	}
	pos := NewPosition(instrument, NewDecimalFromInt(1000), "USD")
	pos.Quantity = NewDecimalFromInt(10)
	pos.CurrentPrice = NewDecimalFromInt(100)

	if err := p.AddPosition(pos); err != nil {
		t.Fatalf("Failed to add position: %v", err)
	}

	// Sell 3 shares @ $120
	result, err := p.SellPartial(pos.ID, "3", "120")
	if err != nil {
		t.Fatalf("SellPartial failed: %v", err)
	}

	// Verify sale transaction
	if result.Sale.QuantitySold.String() != "3" {
		t.Errorf("Expected quantity sold = 3, got %s", result.Sale.QuantitySold.String())
	}
	if result.Sale.SalePrice.String() != "120" {
		t.Errorf("Expected sale price = 120, got %s", result.Sale.SalePrice.String())
	}

	// Total proceeds = 3 * 120 = 360
	expectedProceeds := NewDecimalFromInt(360)
	if result.Sale.TotalProceeds.Cmp(expectedProceeds) != 0 {
		t.Errorf("Expected proceeds = %s, got %s", expectedProceeds.String(), result.Sale.TotalProceeds.String())
	}

	// Invested sold = 1000 * (3/10) = 300
	expectedInvestedSold := NewDecimalFromInt(300)
	if result.Sale.InvestedSold.Cmp(expectedInvestedSold) != 0 {
		t.Errorf("Expected invested sold = %s, got %s", expectedInvestedSold.String(), result.Sale.InvestedSold.String())
	}

	// P/L = 360 - 300 = +60
	expectedPL := NewDecimalFromInt(60)
	if result.Sale.ProfitLoss.Cmp(expectedPL) != 0 {
		t.Errorf("Expected P/L = %s, got %s", expectedPL.String(), result.Sale.ProfitLoss.String())
	}

	// Verify remaining position
	if result.Position.Quantity.String() != "7" {
		t.Errorf("Expected remaining quantity = 7, got %s", result.Position.Quantity.String())
	}

	// Remaining invested = 1000 - 300 = 700
	expectedRemainingInvest := NewDecimalFromInt(700)
	if result.Position.InvestedAmount.Cmp(expectedRemainingInvest) != 0 {
		t.Errorf("Expected remaining invested = %s, got %s", expectedRemainingInvest.String(), result.Position.InvestedAmount.String())
	}

	if result.IsFullSale {
		t.Error("Expected IsFullSale = false")
	}
}

func TestSellPartial_FullSale(t *testing.T) {
	p := NewPortfolio("test")

	instrument := Instrument{
		ISIN:   "US0378331005",
		Symbol: "AAPL",
		Name:   "Apple Inc",
		Type:   InstrumentTypeStock,
		Sector: "Technology",
	}
	pos := NewPosition(instrument, NewDecimalFromInt(1000), "USD")
	pos.Quantity = NewDecimalFromInt(10)
	pos.CurrentPrice = NewDecimalFromInt(100)

	if err := p.AddPosition(pos); err != nil {
		t.Fatalf("Failed to add position: %v", err)
	}

	// Sell all 10 shares @ $150
	result, err := p.SellPartial(pos.ID, "10", "150")
	if err != nil {
		t.Fatalf("SellPartial failed: %v", err)
	}

	if !result.IsFullSale {
		t.Error("Expected IsFullSale = true for full sale")
	}

	if result.Position != nil {
		t.Error("Expected Position = nil for full sale")
	}

	// Verify position was removed from portfolio
	if len(p.Positions) != 0 {
		t.Errorf("Expected 0 positions after full sale, got %d", len(p.Positions))
	}

	// P/L = (150 * 10) - 1000 = 500
	expectedPL := NewDecimalFromInt(500)
	if result.Sale.ProfitLoss.Cmp(expectedPL) != 0 {
		t.Errorf("Expected P/L = %s, got %s", expectedPL.String(), result.Sale.ProfitLoss.String())
	}
}

func TestSellPartial_InsufficientShares(t *testing.T) {
	p := NewPortfolio("test")

	instrument := Instrument{
		ISIN:   "US0378331005",
		Symbol: "AAPL",
		Name:   "Apple Inc",
		Type:   InstrumentTypeStock,
		Sector: "Technology",
	}
	pos := NewPosition(instrument, NewDecimalFromInt(1000), "USD")
	pos.Quantity = NewDecimalFromInt(10)
	pos.CurrentPrice = NewDecimalFromInt(100)

	if err := p.AddPosition(pos); err != nil {
		t.Fatalf("Failed to add position: %v", err)
	}

	// Try to sell 15 shares (only have 10)
	_, err := p.SellPartial(pos.ID, "15", "120")
	if err == nil {
		t.Fatal("Expected error for insufficient shares")
	}

	if !errors.Is(err, ErrInsufficientShares) {
		t.Errorf("Expected ErrInsufficientShares, got %v", err)
	}
}

func TestSellPartial_PositionNotFound(t *testing.T) {
	p := NewPortfolio("test")

	_, err := p.SellPartial("non-existent-id", "5", "100")
	if err == nil {
		t.Fatal("Expected error for non-existent position")
	}

	if err != ErrPositionNotFound {
		t.Errorf("Expected ErrPositionNotFound, got %v", err)
	}
}

func TestSellPartial_InvalidQuantity(t *testing.T) {
	p := NewPortfolio("test")

	instrument := Instrument{
		ISIN:   "US0378331005",
		Symbol: "AAPL",
		Name:   "Apple Inc",
		Type:   InstrumentTypeStock,
		Sector: "Technology",
	}
	pos := NewPosition(instrument, NewDecimalFromInt(1000), "USD")
	pos.Quantity = NewDecimalFromInt(10)

	if err := p.AddPosition(pos); err != nil {
		t.Fatalf("Failed to add position: %v", err)
	}

	// Invalid quantity string
	_, err := p.SellPartial(pos.ID, "invalid", "100")
	if err == nil {
		t.Fatal("Expected error for invalid quantity")
	}

	// Negative quantity
	_, err = p.SellPartial(pos.ID, "-5", "100")
	if err == nil {
		t.Fatal("Expected error for negative quantity")
	}
}

func TestSellPartial_InvalidPrice(t *testing.T) {
	p := NewPortfolio("test")

	instrument := Instrument{
		ISIN:   "US0378331005",
		Symbol: "AAPL",
		Name:   "Apple Inc",
		Type:   InstrumentTypeStock,
		Sector: "Technology",
	}
	pos := NewPosition(instrument, NewDecimalFromInt(1000), "USD")
	pos.Quantity = NewDecimalFromInt(10)

	if err := p.AddPosition(pos); err != nil {
		t.Fatalf("Failed to add position: %v", err)
	}

	// Invalid price string
	_, err := p.SellPartial(pos.ID, "5", "invalid")
	if err == nil {
		t.Fatal("Expected error for invalid price")
	}

	// Negative price
	_, err = p.SellPartial(pos.ID, "5", "-100")
	if err == nil {
		t.Fatal("Expected error for negative price")
	}
}

func TestSellPartial_MultiplePositions(t *testing.T) {
	p := NewPortfolio("test")

	// Add two positions
	instrument1 := Instrument{ISIN: "AAPL", Symbol: "AAPL", Name: "Apple", Type: InstrumentTypeStock, Sector: "Technology"}
	instrument2 := Instrument{ISIN: "GOOGL", Symbol: "GOOGL", Name: "Google", Type: InstrumentTypeStock, Sector: "Technology"}

	pos1 := NewPosition(instrument1, NewDecimalFromInt(1000), "USD")
	pos1.Quantity = NewDecimalFromInt(10)
	pos2 := NewPosition(instrument2, NewDecimalFromInt(2000), "USD")
	pos2.Quantity = NewDecimalFromInt(20)

	if err := p.AddPosition(pos1); err != nil {
		t.Fatalf("Failed to add position 1: %v", err)
	}
	if err := p.AddPosition(pos2); err != nil {
		t.Fatalf("Failed to add position 2: %v", err)
	}

	// Sell from position 1 only
	result, err := p.SellPartial(pos1.ID, "5", "110")
	if err != nil {
		t.Fatalf("SellPartial failed: %v", err)
	}

	// Verify position 1 was updated
	if result.Position.Quantity.String() != "5" {
		t.Errorf("Expected position 1 quantity = 5, got %s", result.Position.Quantity.String())
	}

	// Verify position 2 was not affected
	pos2InPortfolio, err := p.GetPosition(pos2.ID)
	if err != nil {
		t.Fatalf("Failed to get position 2: %v", err)
	}
	if pos2InPortfolio.Quantity.String() != "20" {
		t.Errorf("Expected position 2 quantity = 20, got %s", pos2InPortfolio.Quantity.String())
	}

	// Verify portfolio still has 2 positions
	if len(p.Positions) != 2 {
		t.Errorf("Expected 2 positions, got %d", len(p.Positions))
	}
}

func TestSellPartial_ProfitLossCalculation(t *testing.T) {
	p := NewPortfolio("test")

	instrument := Instrument{
		ISIN:   "US0378331005",
		Symbol: "AAPL",
		Name:   "Apple Inc",
		Type:   InstrumentTypeStock,
		Sector: "Technology",
	}
	pos := NewPosition(instrument, NewDecimalFromInt(1000), "USD")
	pos.Quantity = NewDecimalFromInt(10)
	pos.CurrentPrice = NewDecimalFromInt(100)

	if err := p.AddPosition(pos); err != nil {
		t.Fatalf("Failed to add position: %v", err)
	}

	// Test case 1: Sell at profit (price > avg cost)
	// Avg cost = 1000/10 = 100, sell at 120
	result, err := p.SellPartial(pos.ID, "5", "120")
	if err != nil {
		t.Fatalf("SellPartial failed: %v", err)
	}

	// P/L should be positive: (120-100) * 5 = 100
	if result.Sale.ProfitLoss.Cmp(NewDecimalFromInt(100)) != 0 {
		t.Errorf("Expected profit = 100, got %s", result.Sale.ProfitLoss.String())
	}

	// Add another position to test loss scenario
	p2 := NewPortfolio("test2")
	pos2 := NewPosition(instrument, NewDecimalFromInt(1500), "USD") // Avg cost = 150
	pos2.Quantity = NewDecimalFromInt(10)

	if err := p2.AddPosition(pos2); err != nil {
		t.Fatalf("Failed to add position: %v", err)
	}

	// Test case 2: Sell at loss (price < avg cost)
	// Avg cost = 150, sell at 120
	result2, err := p2.SellPartial(pos2.ID, "5", "120")
	if err != nil {
		t.Fatalf("SellPartial failed: %v", err)
	}

	// P/L should be negative: (120-150) * 5 = -150
	expectedLoss := NewDecimalFromInt(-150)
	if result2.Sale.ProfitLoss.Cmp(expectedLoss) != 0 {
		t.Errorf("Expected loss = %s, got %s", expectedLoss.String(), result2.Sale.ProfitLoss.String())
	}
}
