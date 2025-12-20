package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestAddPosition_New(t *testing.T) {
	p := NewPortfolio("Test Portfolio")
	inst := NewInstrument("US123", "AAPL", "Apple", InstrumentTypeStock, "USD", "NASDAQ")
	pos := NewPosition(inst, decimal.NewFromInt(1000), "USD")
	pos.UpdatePrice(decimal.NewFromInt(100)) // Quantity = 10

	err := p.AddPosition(pos)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(p.Positions) != 1 {
		t.Errorf("Expected 1 position, got %d", len(p.Positions))
	}
	if !p.Positions[0].Quantity.Equal(decimal.NewFromInt(10)) {
		t.Errorf("Expected quantity 10, got %s", p.Positions[0].Quantity)
	}
}

func TestAddPosition_Merge(t *testing.T) {
	p := NewPortfolio("Test Portfolio")
	inst := NewInstrument("US123", "AAPL", "Apple", InstrumentTypeStock, "USD", "NASDAQ")

	// 1. Add first position
	pos1 := NewPosition(inst, decimal.NewFromInt(1000), "USD") // 1000 USD
	pos1.UpdatePrice(decimal.NewFromInt(100))                  // Price 100 -> Qty 10
	_ = p.AddPosition(pos1)                                    // Intentionally ignoring error in test setup

	// 2. Add second position (same ISIN)
	pos2 := NewPosition(inst, decimal.NewFromInt(500), "USD") // 500 USD
	pos2.UpdatePrice(decimal.NewFromInt(125))                 // Price 125 -> Qty 4

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
	expectedInvested := decimal.NewFromInt(1500)
	if !merged.InvestedAmount.Equal(expectedInvested) {
		t.Errorf("Expected invested %s, got %s", expectedInvested, merged.InvestedAmount)
	}

	// Total Quantity: 10 + 4 = 14
	expectedQty := decimal.NewFromInt(14)
	if !merged.Quantity.Equal(expectedQty) {
		t.Errorf("Expected quantity %s, got %s", expectedQty, merged.Quantity)
	}

	// Current Price should be latest (125)
	expectedPrice := decimal.NewFromInt(125)
	if !merged.CurrentPrice.Equal(expectedPrice) {
		t.Errorf("Expected current price %s, got %s", expectedPrice, merged.CurrentPrice)
	}
}

func TestAddPosition_Invalid(t *testing.T) {
	p := NewPortfolio("Test P")
	// Invalid position (empty ISIN)
	inst := NewInstrument("", "", "", InstrumentTypeStock, "USD", "")
	pos := NewPosition(inst, decimal.Zero, "USD")

	err := p.AddPosition(pos)
	if err != ErrInvalidPosition {
		t.Errorf("Expected ErrInvalidPosition, got %v", err)
	}
}
