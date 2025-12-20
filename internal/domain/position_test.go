package domain

import (
	"testing"
)

func TestPosition_UpdatePrice(t *testing.T) {
	instrument := NewInstrument("US0378331005", "AAPL", "Apple Inc.", InstrumentTypeStock, "USD", "NASDAQ")
	position := NewPosition(instrument, NewDecimalFromInt(10000), "USD")

	price := NewDecimalFromInt(150)
	err := position.UpdatePrice(price)
	if err != nil {
		t.Fatalf("UpdatePrice failed: %v", err)
	}

	if !position.CurrentPrice.Equal(price) {
		t.Errorf("Expected current price %s, got %s", price, position.CurrentPrice)
	}

	expectedQuantity, err := NewDecimalFromInt(10000).Div(price)
	if err != nil {
		t.Fatalf("Division failed: %v", err)
	}
	if !position.Quantity.Equal(expectedQuantity) {
		t.Errorf("Expected quantity %s, got %s", expectedQuantity, position.Quantity)
	}
}

func TestPosition_CurrentValue(t *testing.T) {
	instrument := NewInstrument("US0378331005", "AAPL", "Apple Inc.", InstrumentTypeStock, "USD", "NASDAQ")
	position := NewPosition(instrument, NewDecimalFromInt(10000), "USD")
	err := position.UpdatePrice(NewDecimalFromInt(100))
	if err != nil {
		t.Fatalf("UpdatePrice failed: %v", err)
	}

	currentValue, err := position.CurrentValue()
	if err != nil {
		t.Fatalf("CurrentValue failed: %v", err)
	}
	expected := NewDecimalFromInt(10000)

	if !currentValue.Equal(expected) {
		t.Errorf("Expected current value %s, got %s", expected, currentValue)
	}
}

func TestPosition_ProfitLoss(t *testing.T) {
	instrument := NewInstrument("US0378331005", "AAPL", "Apple Inc.", InstrumentTypeStock, "USD", "NASDAQ")
	position := NewPosition(instrument, NewDecimalFromInt(10000), "USD")

	initialPrice := NewDecimalFromInt(100)
	err := position.UpdatePrice(initialPrice)
	if err != nil {
		t.Fatalf("UpdatePrice failed: %v", err)
	}

	quantity := position.Quantity

	newPrice := NewDecimalFromInt(120)
	position.Quantity = quantity
	position.CurrentPrice = newPrice

	profitLoss, err := position.ProfitLoss()
	if err != nil {
		t.Fatalf("ProfitLoss failed: %v", err)
	}
	roundedProfitLoss, err := profitLoss.Round(2)
	if err != nil {
		t.Fatalf("Round failed: %v", err)
	}
	expected := NewDecimalFromInt(2000)

	if !roundedProfitLoss.Equal(expected) {
		t.Errorf("Expected P/L %s, got %s", expected, roundedProfitLoss)
	}
}

func TestPosition_ProfitLossPercent(t *testing.T) {
	instrument := NewInstrument("US0378331005", "AAPL", "Apple Inc.", InstrumentTypeStock, "USD", "NASDAQ")
	position := NewPosition(instrument, NewDecimalFromInt(10000), "USD")

	initialPrice := NewDecimalFromInt(100)
	err := position.UpdatePrice(initialPrice)
	if err != nil {
		t.Fatalf("UpdatePrice failed: %v", err)
	}

	quantity := position.Quantity

	newPrice := NewDecimalFromInt(120)
	position.Quantity = quantity
	position.CurrentPrice = newPrice

	profitLossPercent, err := position.ProfitLossPercent()
	if err != nil {
		t.Fatalf("ProfitLossPercent failed: %v", err)
	}
	roundedPercent, err := profitLossPercent.Round(0)
	if err != nil {
		t.Fatalf("Round failed: %v", err)
	}
	expected := NewDecimalFromInt(20)

	if !roundedPercent.Equal(expected) {
		t.Errorf("Expected P/L%% %s, got %s", expected, roundedPercent)
	}
}
