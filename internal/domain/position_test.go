package domain

import (
	"testing"
)

// --- NewPosition Tests ---

func TestNewPosition(t *testing.T) {
	instrument := NewInstrument("US0378331005", "AAPL", "Apple Inc.", InstrumentTypeStock, "USD", "NASDAQ")
	investedAmount := NewDecimalFromInt(1000)
	currency := "USD"

	position := NewPosition(instrument, investedAmount, currency)

	if position.ID == "" {
		t.Error("expected non-empty position ID")
	}

	if position.Instrument.ISIN != "US0378331005" {
		t.Errorf("expected ISIN US0378331005, got %s", position.Instrument.ISIN)
	}

	if !position.InvestedAmount.Equal(investedAmount) {
		t.Errorf("expected invested amount %s, got %s", investedAmount, position.InvestedAmount)
	}

	if position.InvestedCurrency != currency {
		t.Errorf("expected currency %s, got %s", currency, position.InvestedCurrency)
	}

	if !position.CurrentPrice.IsZero() {
		t.Errorf("expected zero current price initially, got %s", position.CurrentPrice)
	}

	if !position.Quantity.IsZero() {
		t.Errorf("expected zero quantity initially, got %s", position.Quantity)
	}
}

// --- UpdatePrice Tests ---

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

func TestPosition_UpdatePrice_Zero(t *testing.T) {
	instrument := NewInstrument("US0378331005", "AAPL", "Apple Inc.", InstrumentTypeStock, "USD", "NASDAQ")
	position := NewPosition(instrument, NewDecimalFromInt(1000), "USD")

	err := position.UpdatePrice(Zero)

	// UpdatePrice doesn't error on zero, it just doesn't calculate quantity
	if err != nil {
		t.Fatalf("UpdatePrice should not error on zero price: %v", err)
	}

	if !position.CurrentPrice.IsZero() {
		t.Errorf("expected zero current price, got %s", position.CurrentPrice)
	}

	if !position.Quantity.IsZero() {
		t.Errorf("expected zero quantity, got %s", position.Quantity)
	}
}

func TestPosition_UpdatePrice_Negative(t *testing.T) {
	instrument := NewInstrument("US0378331005", "AAPL", "Apple Inc.", InstrumentTypeStock, "USD", "NASDAQ")
	position := NewPosition(instrument, NewDecimalFromInt(1000), "USD")

	negativePrice := NewDecimalFromInt(-100)
	err := position.UpdatePrice(negativePrice)

	// UpdatePrice doesn't validate for negative values
	if err != nil {
		t.Fatalf("UpdatePrice should not error on negative price: %v", err)
	}

	if !position.CurrentPrice.Equal(negativePrice) {
		t.Errorf("expected current price %s, got %s", negativePrice, position.CurrentPrice)
	}
}

// --- CurrentValue Tests ---

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

func TestPosition_CurrentValue_WithPriceChange(t *testing.T) {
	instrument := NewInstrument("US0378331005", "AAPL", "Apple Inc.", InstrumentTypeStock, "USD", "NASDAQ")
	position := NewPosition(instrument, NewDecimalFromInt(1000), "USD")

	// Initial price: $100, Quantity: 10
	_ = position.UpdatePrice(NewDecimalFromInt(100))

	// Price increases to $150, Value should be $1500
	position.CurrentPrice = NewDecimalFromInt(150)

	currentValue, err := position.CurrentValue()
	if err != nil {
		t.Fatalf("CurrentValue failed: %v", err)
	}

	expected := NewDecimalFromInt(1500)
	if !currentValue.Equal(expected) {
		t.Errorf("Expected current value %s, got %s", expected, currentValue)
	}
}

// --- ProfitLoss Tests ---

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

func TestPosition_ProfitLoss_Loss(t *testing.T) {
	instrument := NewInstrument("US0378331005", "AAPL", "Apple Inc.", InstrumentTypeStock, "USD", "NASDAQ")
	position := NewPosition(instrument, NewDecimalFromInt(1000), "USD")

	// Initial price: $100, Quantity: 10
	_ = position.UpdatePrice(NewDecimalFromInt(100))

	// Price drops to $50
	position.CurrentPrice = NewDecimalFromInt(50)

	profitLoss, err := position.ProfitLoss()
	if err != nil {
		t.Fatalf("ProfitLoss failed: %v", err)
	}

	expected := NewDecimalFromInt(-500)
	if !profitLoss.Equal(expected) {
		t.Errorf("Expected P/L %s, got %s", expected, profitLoss)
	}
}

func TestPosition_ProfitLoss_NoChange(t *testing.T) {
	instrument := NewInstrument("US0378331005", "AAPL", "Apple Inc.", InstrumentTypeStock, "USD", "NASDAQ")
	position := NewPosition(instrument, NewDecimalFromInt(1000), "USD")

	// Price stays the same
	_ = position.UpdatePrice(NewDecimalFromInt(100))

	profitLoss, err := position.ProfitLoss()
	if err != nil {
		t.Fatalf("ProfitLoss failed: %v", err)
	}

	if !profitLoss.IsZero() {
		t.Errorf("Expected zero P/L, got %s", profitLoss)
	}
}

// --- ProfitLossPercent Tests ---

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

func TestPosition_ProfitLossPercent_Loss(t *testing.T) {
	instrument := NewInstrument("US0378331005", "AAPL", "Apple Inc.", InstrumentTypeStock, "USD", "NASDAQ")
	position := NewPosition(instrument, NewDecimalFromInt(1000), "USD")

	// Initial price: $100
	_ = position.UpdatePrice(NewDecimalFromInt(100))

	// Price drops to $75 (-25%)
	position.CurrentPrice = NewDecimalFromInt(75)

	profitLossPercent, err := position.ProfitLossPercent()
	if err != nil {
		t.Fatalf("ProfitLossPercent failed: %v", err)
	}

	expected := NewDecimalFromInt(-25)
	roundedPercent, err := profitLossPercent.Round(0)
	if err != nil {
		t.Fatalf("Round failed: %v", err)
	}

	if !roundedPercent.Equal(expected) {
		t.Errorf("Expected P/L%% %s, got %s", expected, roundedPercent)
	}
}

func TestPosition_ProfitLossPercent_ZeroInvestment(t *testing.T) {
	instrument := NewInstrument("US0378331005", "AAPL", "Apple Inc.", InstrumentTypeStock, "USD", "NASDAQ")
	position := NewPosition(instrument, Zero, "USD")

	_ = position.UpdatePrice(NewDecimalFromInt(100))

	percent, err := position.ProfitLossPercent()
	if err != nil {
		t.Fatalf("ProfitLossPercent failed: %v", err)
	}

	// With zero investment, percent should be zero
	if !percent.IsZero() {
		t.Errorf("Expected zero P/L%% for zero investment, got %s", percent)
	}
}

// --- IsValid Tests ---

func TestPosition_IsValid(t *testing.T) {
	testCases := []struct {
		name     string
		position Position
		expected bool
	}{
		{
			name: "valid position",
			position: Position{
				ID:               "test-id",
				Instrument:       NewInstrument("US001", "AAPL", "Apple", InstrumentTypeStock, "USD", "NASDAQ"),
				InvestedAmount:   NewDecimalFromInt(1000),
				InvestedCurrency: "USD",
			},
			expected: true,
		},
		{
			name: "empty ID",
			position: Position{
				ID:               "",
				Instrument:       NewInstrument("US001", "AAPL", "Apple", InstrumentTypeStock, "USD", "NASDAQ"),
				InvestedAmount:   NewDecimalFromInt(1000),
				InvestedCurrency: "USD",
			},
			expected: false,
		},
		{
			name: "empty ISIN",
			position: Position{
				ID:               "test-id",
				Instrument:       NewInstrument("", "AAPL", "Apple", InstrumentTypeStock, "USD", "NASDAQ"),
				InvestedAmount:   NewDecimalFromInt(1000),
				InvestedCurrency: "USD",
			},
			expected: false,
		},
		{
			name: "zero invested amount",
			position: Position{
				ID:               "test-id",
				Instrument:       NewInstrument("US001", "AAPL", "Apple", InstrumentTypeStock, "USD", "NASDAQ"),
				InvestedAmount:   Zero,
				InvestedCurrency: "USD",
			},
			expected: false,
		},
		{
			name: "empty InvestedCurrency",
			position: Position{
				ID:               "test-id",
				Instrument:       NewInstrument("US001", "AAPL", "Apple", InstrumentTypeStock, "USD", "NASDAQ"),
				InvestedAmount:   NewDecimalFromInt(1000),
				InvestedCurrency: "",
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.position.IsValid()
			if result != tc.expected {
				t.Errorf("expected IsValid() = %v, got %v", tc.expected, result)
			}
		})
	}
}

// --- NewInstrument Tests ---

func TestNewInstrument(t *testing.T) {
	isin := "US0378331005"
	symbol := "AAPL"
	name := "Apple Inc."
	instrumentType := InstrumentTypeStock
	currency := "USD"
	exchange := "NASDAQ"

	instrument := NewInstrument(isin, symbol, name, instrumentType, currency, exchange)

	if instrument.ISIN != isin {
		t.Errorf("expected ISIN %s, got %s", isin, instrument.ISIN)
	}
	if instrument.Symbol != symbol {
		t.Errorf("expected Symbol %s, got %s", symbol, instrument.Symbol)
	}
	if instrument.Name != name {
		t.Errorf("expected Name %s, got %s", name, instrument.Name)
	}
	if instrument.Type != instrumentType {
		t.Errorf("expected Type %s, got %s", instrumentType, instrument.Type)
	}
	if instrument.Currency != currency {
		t.Errorf("expected Currency %s, got %s", currency, instrument.Currency)
	}
	if instrument.Exchange != exchange {
		t.Errorf("expected Exchange %s, got %s", exchange, instrument.Exchange)
	}
}

func TestInstrument_DifferentTypes(t *testing.T) {
	types := []InstrumentType{
		InstrumentTypeStock,
		InstrumentTypeETF,
	}

	for _, instType := range types {
		inst := NewInstrument("TEST", "TST", "Test", instType, "USD", "NYSE")
		if inst.Type != instType {
			t.Errorf("expected type %s, got %s", instType, inst.Type)
		}
	}
}
