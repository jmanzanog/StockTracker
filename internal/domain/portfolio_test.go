package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestPortfolio_AddPosition(t *testing.T) {
	portfolio := NewPortfolio("test")
	instrument := NewInstrument("US0378331005", "AAPL", "Apple Inc.", InstrumentTypeStock, "USD", "NASDAQ")
	position := NewPosition(instrument, decimal.NewFromInt(10000), "USD")

	err := portfolio.AddPosition(position)
	if err != nil {
		t.Fatalf("Failed to add position: %v", err)
	}

	if len(portfolio.Positions) != 1 {
		t.Errorf("Expected 1 position, got %d", len(portfolio.Positions))
	}
}

func TestPortfolio_AddDuplicatePosition(t *testing.T) {
	portfolio := NewPortfolio("test")
	instrument := NewInstrument("US0378331005", "AAPL", "Apple Inc.", InstrumentTypeStock, "USD", "NASDAQ")
	position1 := NewPosition(instrument, decimal.NewFromInt(10000), "USD")
	position2 := NewPosition(instrument, decimal.NewFromInt(5000), "USD")

	portfolio.AddPosition(position1)
	err := portfolio.AddPosition(position2)

	if err != ErrDuplicatePosition {
		t.Errorf("Expected ErrDuplicatePosition, got %v", err)
	}
}

func TestPortfolio_RemovePosition(t *testing.T) {
	portfolio := NewPortfolio("test")
	instrument := NewInstrument("US0378331005", "AAPL", "Apple Inc.", InstrumentTypeStock, "USD", "NASDAQ")
	position := NewPosition(instrument, decimal.NewFromInt(10000), "USD")

	portfolio.AddPosition(position)
	err := portfolio.RemovePosition(position.ID)

	if err != nil {
		t.Fatalf("Failed to remove position: %v", err)
	}

	if len(portfolio.Positions) != 0 {
		t.Errorf("Expected 0 positions, got %d", len(portfolio.Positions))
	}
}

func TestPortfolio_TotalValue(t *testing.T) {
	portfolio := NewPortfolio("test")

	instrument1 := NewInstrument("US0378331005", "AAPL", "Apple Inc.", InstrumentTypeStock, "USD", "NASDAQ")
	position1 := NewPosition(instrument1, decimal.NewFromInt(10000), "USD")
	position1.UpdatePrice(decimal.NewFromInt(100))

	instrument2 := NewInstrument("IE00B4L5Y983", "IWDA", "iShares Core MSCI World", InstrumentTypeETF, "USD", "XETRA")
	position2 := NewPosition(instrument2, decimal.NewFromInt(5000), "USD")
	position2.UpdatePrice(decimal.NewFromInt(50))

	portfolio.AddPosition(position1)
	portfolio.AddPosition(position2)

	totalValue := portfolio.TotalValue()
	expected := decimal.NewFromInt(15000)

	if !totalValue.Equal(expected) {
		t.Errorf("Expected total value %s, got %s", expected, totalValue)
	}
}

func TestPortfolio_TotalProfitLoss(t *testing.T) {
	portfolio := NewPortfolio("test")

	instrument := NewInstrument("US0378331005", "AAPL", "Apple Inc.", InstrumentTypeStock, "USD", "NASDAQ")
	position := NewPosition(instrument, decimal.NewFromInt(10000), "USD")

	initialPrice := decimal.NewFromInt(100)
	position.UpdatePrice(initialPrice)

	quantity := position.Quantity
	newPrice := decimal.NewFromInt(120)
	position.Quantity = quantity
	position.CurrentPrice = newPrice

	portfolio.AddPosition(position)

	profitLoss := portfolio.TotalProfitLoss().Round(2)
	expected := decimal.NewFromInt(2000)

	if !profitLoss.Equal(expected) {
		t.Errorf("Expected total P/L %s, got %s", expected, profitLoss)
	}
}
