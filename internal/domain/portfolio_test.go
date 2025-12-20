package domain

import (
	"errors"
	"testing"
)

// --- NewPortfolio Tests ---

func TestNewPortfolio(t *testing.T) {
	name := "Test Portfolio"
	p := NewPortfolio(name)

	if p.Name != name {
		t.Errorf("expected name %s, got %s", name, p.Name)
	}

	if p.ID == "" {
		t.Error("expected non-empty ID")
	}

	if p.Positions == nil {
		t.Error("expected non-nil positions slice")
	}

	if len(p.Positions) != 0 {
		t.Errorf("expected empty positions, got %d", len(p.Positions))
	}
}

// --- AddPosition Tests ---

func TestAddPosition_New(t *testing.T) {
	p := NewPortfolio("Test Portfolio")
	inst := NewInstrument("US123", "AAPL", "Apple", InstrumentTypeStock, "USD", "NASDAQ")
	pos := NewPosition(inst, NewDecimalFromInt(1000), "USD")
	err := pos.UpdatePrice(NewDecimalFromInt(100)) // Quantity = 10
	if err != nil {
		t.Fatalf("UpdatePrice failed: %v", err)
	}

	err = p.AddPosition(pos)
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
	err := pos1.UpdatePrice(NewDecimalFromInt(100))           // Price 100 -> Qty 10
	if err != nil {
		t.Fatalf("UpdatePrice failed: %v", err)
	}

	if err := p.AddPosition(pos1); err != nil {
		t.Errorf("Expected no error on first add, got %v", err)
	}

	// 2. Add second position (same ISIN)
	pos2 := NewPosition(inst, NewDecimalFromInt(500), "USD") // 500 USD
	err = pos2.UpdatePrice(NewDecimalFromInt(125))           // Price 125 -> Qty 4
	if err != nil {
		t.Fatalf("UpdatePrice failed: %v", err)
	}

	err = p.AddPosition(pos2)
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
	if !errors.Is(err, ErrInvalidPosition) {
		t.Errorf("Expected ErrInvalidPosition, got %v", err)
	}
}

func TestAddPosition_MultipleDistinct(t *testing.T) {
	p := NewPortfolio("Test Portfolio")

	// Add two different instruments
	inst1 := NewInstrument("US001", "AAPL", "Apple", InstrumentTypeStock, "USD", "NASDAQ")
	pos1 := NewPosition(inst1, NewDecimalFromInt(1000), "USD")
	_ = pos1.UpdatePrice(NewDecimalFromInt(150))

	inst2 := NewInstrument("US002", "GOOGL", "Google", InstrumentTypeStock, "USD", "NASDAQ")
	pos2 := NewPosition(inst2, NewDecimalFromInt(2000), "USD")
	_ = pos2.UpdatePrice(NewDecimalFromInt(2800))

	_ = p.AddPosition(pos1)
	_ = p.AddPosition(pos2)

	if len(p.Positions) != 2 {
		t.Errorf("expected 2 positions, got %d", len(p.Positions))
	}
}

// --- RemovePosition Tests ---

func TestRemovePosition_Success(t *testing.T) {
	p := NewPortfolio("Test Portfolio")
	inst := NewInstrument("US123", "AAPL", "Apple", InstrumentTypeStock, "USD", "NASDAQ")
	pos := NewPosition(inst, NewDecimalFromInt(1000), "USD")
	_ = pos.UpdatePrice(NewDecimalFromInt(100))
	_ = p.AddPosition(pos)

	err := p.RemovePosition(pos.ID)

	if err != nil {
		t.Fatalf("RemovePosition failed: %v", err)
	}

	if len(p.Positions) != 0 {
		t.Errorf("expected 0 positions after removal, got %d", len(p.Positions))
	}
}

func TestRemovePosition_NotFound(t *testing.T) {
	p := NewPortfolio("Test Portfolio")

	err := p.RemovePosition("non-existent-id")

	if !errors.Is(err, ErrPositionNotFound) {
		t.Errorf("expected ErrPositionNotFound, got %v", err)
	}
}

func TestRemovePosition_MultiplePositions(t *testing.T) {
	p := NewPortfolio("Test Portfolio")

	inst1 := NewInstrument("US001", "AAPL", "Apple", InstrumentTypeStock, "USD", "NASDAQ")
	pos1 := NewPosition(inst1, NewDecimalFromInt(1000), "USD")
	_ = pos1.UpdatePrice(NewDecimalFromInt(150))

	inst2 := NewInstrument("US002", "GOOGL", "Google", InstrumentTypeStock, "USD", "NASDAQ")
	pos2 := NewPosition(inst2, NewDecimalFromInt(2000), "USD")
	_ = pos2.UpdatePrice(NewDecimalFromInt(2800))

	_ = p.AddPosition(pos1)
	_ = p.AddPosition(pos2)

	// Remove first position
	err := p.RemovePosition(pos1.ID)

	if err != nil {
		t.Fatalf("RemovePosition failed: %v", err)
	}

	if len(p.Positions) != 1 {
		t.Errorf("expected 1 position after removal, got %d", len(p.Positions))
	}

	if p.Positions[0].ID != pos2.ID {
		t.Errorf("expected remaining position to be %s, got %s", pos2.ID, p.Positions[0].ID)
	}
}

// --- GetPosition Tests ---

func TestGetPosition_Success(t *testing.T) {
	p := NewPortfolio("Test Portfolio")
	inst := NewInstrument("US123", "AAPL", "Apple", InstrumentTypeStock, "USD", "NASDAQ")
	pos := NewPosition(inst, NewDecimalFromInt(1000), "USD")
	_ = p.AddPosition(pos)

	retrieved, err := p.GetPosition(pos.ID)

	if err != nil {
		t.Fatalf("GetPosition failed: %v", err)
	}

	if retrieved.ID != pos.ID {
		t.Errorf("expected ID %s, got %s", pos.ID, retrieved.ID)
	}
}

func TestGetPosition_NotFound(t *testing.T) {
	p := NewPortfolio("Test Portfolio")

	_, err := p.GetPosition("non-existent-id")

	if !errors.Is(err, ErrPositionNotFound) {
		t.Errorf("expected ErrPositionNotFound, got %v", err)
	}
}

// --- UpdatePositionPrice Tests ---

func TestUpdatePositionPrice_Success(t *testing.T) {
	p := NewPortfolio("Test Portfolio")
	inst := NewInstrument("US123", "AAPL", "Apple", InstrumentTypeStock, "USD", "NASDAQ")
	pos := NewPosition(inst, NewDecimalFromInt(1000), "USD")
	_ = pos.UpdatePrice(NewDecimalFromInt(100))
	_ = p.AddPosition(pos)

	newPrice := NewDecimalFromInt(200)
	err := p.UpdatePositionPrice(pos.ID, newPrice)

	if err != nil {
		t.Fatalf("UpdatePositionPrice failed: %v", err)
	}

	updated, _ := p.GetPosition(pos.ID)
	if !updated.CurrentPrice.Equal(newPrice) {
		t.Errorf("expected price %s, got %s", newPrice, updated.CurrentPrice)
	}
}

func TestUpdatePositionPrice_NotFound(t *testing.T) {
	p := NewPortfolio("Test Portfolio")

	err := p.UpdatePositionPrice("non-existent-id", NewDecimalFromInt(100))

	if !errors.Is(err, ErrPositionNotFound) {
		t.Errorf("expected ErrPositionNotFound, got %v", err)
	}
}

// --- TotalValue Tests ---

func TestTotalValue_EmptyPortfolio(t *testing.T) {
	p := NewPortfolio("Test Portfolio")

	total, err := p.TotalValue()

	if err != nil {
		t.Fatalf("TotalValue failed: %v", err)
	}

	if !total.IsZero() {
		t.Errorf("expected zero total value, got %s", total)
	}
}

func TestTotalValue_MultiplePositions(t *testing.T) {
	p := NewPortfolio("Test Portfolio")

	inst1 := NewInstrument("US001", "AAPL", "Apple", InstrumentTypeStock, "USD", "NASDAQ")
	pos1 := NewPosition(inst1, NewDecimalFromInt(1000), "USD") // Invested 1000
	_ = pos1.UpdatePrice(NewDecimalFromInt(100))               // Price 100, Qty 10, Value = 1000

	inst2 := NewInstrument("US002", "GOOGL", "Google", InstrumentTypeStock, "USD", "NASDAQ")
	pos2 := NewPosition(inst2, NewDecimalFromInt(2000), "USD") // Invested 2000
	_ = pos2.UpdatePrice(NewDecimalFromInt(2000))              // Price 2000, Qty 1, Value = 2000

	_ = p.AddPosition(pos1)
	_ = p.AddPosition(pos2)

	total, err := p.TotalValue()

	if err != nil {
		t.Fatalf("TotalValue failed: %v", err)
	}

	expected := NewDecimalFromInt(3000)
	if !total.Equal(expected) {
		t.Errorf("expected total value %s, got %s", expected, total)
	}
}

// --- TotalInvested Tests ---

func TestTotalInvested_EmptyPortfolio(t *testing.T) {
	p := NewPortfolio("Test Portfolio")

	total, err := p.TotalInvested()

	if err != nil {
		t.Fatalf("TotalInvested failed: %v", err)
	}

	if !total.IsZero() {
		t.Errorf("expected zero total invested, got %s", total)
	}
}

func TestTotalInvested_MultiplePositions(t *testing.T) {
	p := NewPortfolio("Test Portfolio")

	inst1 := NewInstrument("US001", "AAPL", "Apple", InstrumentTypeStock, "USD", "NASDAQ")
	pos1 := NewPosition(inst1, NewDecimalFromInt(1000), "USD")
	_ = pos1.UpdatePrice(NewDecimalFromInt(100))

	inst2 := NewInstrument("US002", "GOOGL", "Google", InstrumentTypeStock, "USD", "NASDAQ")
	pos2 := NewPosition(inst2, NewDecimalFromInt(2500), "USD")
	_ = pos2.UpdatePrice(NewDecimalFromInt(2000))

	_ = p.AddPosition(pos1)
	_ = p.AddPosition(pos2)

	total, err := p.TotalInvested()

	if err != nil {
		t.Fatalf("TotalInvested failed: %v", err)
	}

	expected := NewDecimalFromInt(3500)
	if !total.Equal(expected) {
		t.Errorf("expected total invested %s, got %s", expected, total)
	}
}

// --- TotalProfitLoss Tests ---

func TestTotalProfitLoss_EmptyPortfolio(t *testing.T) {
	p := NewPortfolio("Test Portfolio")

	profitLoss, err := p.TotalProfitLoss()

	if err != nil {
		t.Fatalf("TotalProfitLoss failed: %v", err)
	}

	if !profitLoss.IsZero() {
		t.Errorf("expected zero profit/loss, got %s", profitLoss)
	}
}

func TestTotalProfitLoss_Profit(t *testing.T) {
	p := NewPortfolio("Test Portfolio")

	// Add a position with profit: Invested 1000, Current value 1500
	inst := NewInstrument("US001", "AAPL", "Apple", InstrumentTypeStock, "USD", "NASDAQ")
	pos := NewPosition(inst, NewDecimalFromInt(1000), "USD") // Invested 1000
	_ = pos.UpdatePrice(NewDecimalFromInt(150))              // Price 150, Qty ~6.67, Value ~1000
	_ = p.AddPosition(pos)

	// Get position and manually update to simulate price change
	position, _ := p.GetPosition(pos.ID)
	position.CurrentPrice = NewDecimalFromInt(200) // Value now ~1333.4

	profitLoss, err := p.TotalProfitLoss()

	if err != nil {
		t.Fatalf("TotalProfitLoss failed: %v", err)
	}

	// Should have profit (> 0)
	if profitLoss.Cmp(Zero) <= 0 {
		t.Errorf("expected positive profit, got %s", profitLoss)
	}
}

func TestTotalProfitLoss_Loss(t *testing.T) {
	p := NewPortfolio("Test Portfolio")

	inst := NewInstrument("US001", "AAPL", "Apple", InstrumentTypeStock, "USD", "NASDAQ")
	pos := NewPosition(inst, NewDecimalFromInt(1000), "USD") // Invested 1000
	_ = pos.UpdatePrice(NewDecimalFromInt(200))              // Price 200, Qty 5, Value 1000
	_ = p.AddPosition(pos)

	// Get position and manually update to simulate price drop
	position, _ := p.GetPosition(pos.ID)
	position.CurrentPrice = NewDecimalFromInt(150) // Value now 750

	profitLoss, err := p.TotalProfitLoss()

	if err != nil {
		t.Fatalf("TotalProfitLoss failed: %v", err)
	}

	expected := NewDecimalFromInt(-250)
	if !profitLoss.Equal(expected) {
		t.Errorf("expected loss %s, got %s", expected, profitLoss)
	}
}

// --- TotalProfitLossPercent Tests ---

func TestTotalProfitLossPercent_EmptyPortfolio(t *testing.T) {
	p := NewPortfolio("Test Portfolio")

	percent, err := p.TotalProfitLossPercent()

	if err != nil {
		t.Fatalf("TotalProfitLossPercent failed: %v", err)
	}

	if !percent.IsZero() {
		t.Errorf("expected zero percent, got %s", percent)
	}
}

func TestTotalProfitLossPercent_Profit(t *testing.T) {
	p := NewPortfolio("Test Portfolio")

	inst := NewInstrument("US001", "AAPL", "Apple", InstrumentTypeStock, "USD", "NASDAQ")
	pos := NewPosition(inst, NewDecimalFromInt(1000), "USD") // Invested 1000
	_ = pos.UpdatePrice(NewDecimalFromInt(100))              // Price 100, Qty 10, Value 1000
	_ = p.AddPosition(pos)

	// Get position and manually update to +50%
	position, _ := p.GetPosition(pos.ID)
	position.CurrentPrice = NewDecimalFromInt(150) // Value now 1500

	percent, err := p.TotalProfitLossPercent()

	if err != nil {
		t.Fatalf("TotalProfitLossPercent failed: %v", err)
	}

	expected := NewDecimalFromInt(50) // 50%
	if !percent.Equal(expected) {
		t.Errorf("expected %s%%, got %s%%", expected, percent)
	}
}

func TestTotalProfitLossPercent_Loss(t *testing.T) {
	p := NewPortfolio("Test Portfolio")

	inst := NewInstrument("US001", "AAPL", "Apple", InstrumentTypeStock, "USD", "NASDAQ")
	pos := NewPosition(inst, NewDecimalFromInt(1000), "USD") // Invested 1000
	_ = pos.UpdatePrice(NewDecimalFromInt(100))              // Price 100, Qty 10, Value 1000
	_ = p.AddPosition(pos)

	// Get position and manually update to -25%
	position, _ := p.GetPosition(pos.ID)
	position.CurrentPrice = NewDecimalFromInt(75) // Value now 750

	percent, err := p.TotalProfitLossPercent()

	if err != nil {
		t.Fatalf("TotalProfitLossPercent failed: %v", err)
	}

	expected := NewDecimalFromInt(-25) // -25%
	if !percent.Equal(expected) {
		t.Errorf("expected %s%%, got %s%%", expected, percent)
	}
}
