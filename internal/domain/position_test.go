package domain

import (
	"testing"
)

func TestPosition_UpdatePrice(t *testing.T) {
	instrument := NewInstrument("US0378331005", "AAPL", "Apple Inc.", InstrumentTypeStock, "USD", "NASDAQ")
	position := NewPosition(instrument, NewDecimalFromInt(10000), "USD")

	price := NewDecimalFromInt(150)
	position.UpdatePrice(price)

	if !position.CurrentPrice.Equal(price) {
		t.Errorf("Expected current price %s, got %s", price, position.CurrentPrice)
	}

	expectedQuantity := NewDecimalFromInt(10000).Div(price)
	if !position.Quantity.Equal(expectedQuantity) {
		t.Errorf("Expected quantity %s, got %s", expectedQuantity, position.Quantity)
	}
}

func TestPosition_CurrentValue(t *testing.T) {
	instrument := NewInstrument("US0378331005", "AAPL", "Apple Inc.", InstrumentTypeStock, "USD", "NASDAQ")
	position := NewPosition(instrument, NewDecimalFromInt(10000), "USD")
	position.UpdatePrice(NewDecimalFromInt(100))

	currentValue := position.CurrentValue()
	expected := NewDecimalFromInt(10000)

	if !currentValue.Equal(expected) {
		t.Errorf("Expected current value %s, got %s", expected, currentValue)
	}
}

func TestPosition_ProfitLoss(t *testing.T) {
	instrument := NewInstrument("US0378331005", "AAPL", "Apple Inc.", InstrumentTypeStock, "USD", "NASDAQ")
	position := NewPosition(instrument, NewDecimalFromInt(10000), "USD")

	initialPrice := NewDecimalFromInt(100)
	position.UpdatePrice(initialPrice)

	quantity := position.Quantity

	newPrice := NewDecimalFromInt(120)
	position.Quantity = quantity
	position.CurrentPrice = newPrice

	profitLoss := position.ProfitLoss().Round(2)
	expected := NewDecimalFromInt(2000)

	if !profitLoss.Equal(expected) {
		t.Errorf("Expected P/L %s, got %s", expected, profitLoss)
	}
}

func TestPosition_ProfitLossPercent(t *testing.T) {
	instrument := NewInstrument("US0378331005", "AAPL", "Apple Inc.", InstrumentTypeStock, "USD", "NASDAQ")
	position := NewPosition(instrument, NewDecimalFromInt(10000), "USD")

	initialPrice := NewDecimalFromInt(100)
	position.UpdatePrice(initialPrice)

	quantity := position.Quantity

	newPrice := NewDecimalFromInt(120)
	position.Quantity = quantity
	position.CurrentPrice = newPrice

	profitLossPercent := position.ProfitLossPercent().Round(0)
	expected := NewDecimalFromInt(20)

	if !profitLossPercent.Equal(expected) {
		t.Errorf("Expected P/L%% %s, got %s", expected, profitLossPercent)
	}
}
