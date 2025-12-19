package domain

import (
	"testing"
)

func TestAddPosition_New(t *testing.T) {
	p := NewPortfolio("Test Portfolio")
	inst := NewInstrument("US123", "AAPL", "Apple", InstrumentTypeStock, "USD", "NASDAQ")
	pos := NewPosition(inst, NewDecimalFromInt(1000), "USD")
	pos.UpdatePrice(NewDecimalFromInt(100)) // Quantity = 10

	err := p.AddPosition(pos)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(p.Positions) != 1 {
		t.Errorf("Expected 1 position, got %d", len(p.Positions))
	}
	if !p.Positions[0].Quantity.Equal(NewDecimalFromInt(10)) {
		t.Errorf("Expected quantity 10, got %s", p.Positions[0].Quantity)
	}
}

func TestAddPosition_Merge(t *testing.T) {
	p := NewPortfolio("Test Portfolio")
	inst := NewInstrument("US123", "AAPL", "Apple", InstrumentTypeStock, "USD", "NASDAQ")

	// 1. Add first position
	pos1 := NewPosition(inst, NewDecimalFromInt(1000), "USD") // 1000 USD
	pos1.UpdatePrice(NewDecimalFromInt(100))                  // Price 100 -> Qty 10

	if err := p.AddPosition(pos1); err != nil {
		t.Errorf("Expected no error on first add, got %v", err)
	}

	// 2. Add second position (same ISIN)
	pos2 := NewPosition(inst, NewDecimalFromInt(500), "USD") // 500 USD
	pos2.UpdatePrice(NewDecimalFromInt(125))                 // Price 125 -> Qty 4

	err := p.AddPosition(pos2)
	if err != nil {
		t.Fatalf("Expected no error on merge, got %v", err)
	}

	// VALIDATIONS
	if len(p.Positions) != 1 {
		t.Fatalf("Expected merged into 1 position, got %d", len(p.Positions))
	}

	merged := p.Positions[0]

	// Total Invested: 1000 + 500 = 1500
	expectedInvested := NewDecimalFromInt(1500)
	if !merged.InvestedAmount.Equal(expectedInvested) {
		t.Errorf("Expected invested %s, got %s", expectedInvested, merged.InvestedAmount)
	}

	// Total Quantity: 10 + 4 = 14
	expectedQty := NewDecimalFromInt(14)
	if !merged.Quantity.Equal(expectedQty) {
		t.Errorf("Expected quantity %s, got %s", expectedQty, merged.Quantity)
	}

	// Current Price should be latest (125)
	expectedPrice := NewDecimalFromInt(125)
	if !merged.CurrentPrice.Equal(expectedPrice) {
		t.Errorf("Expected current price %s, got %s", expectedPrice, merged.CurrentPrice)
	}
}

func TestAddPosition_Invalid(t *testing.T) {
	p := NewPortfolio("Test P")
	// Invalid position (empty ISIN)
	inst := NewInstrument("", "", "", InstrumentTypeStock, "USD", "")
	pos := NewPosition(inst, Zero, "USD")

	err := p.AddPosition(pos)
	if err != ErrInvalidPosition {
		t.Errorf("Expected ErrInvalidPosition, got %v", err)
	}
}
