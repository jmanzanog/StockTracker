package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Position struct {
	ID               string     `json:"id" gorm:"primaryKey"`
	PortfolioID      string     `json:"-"` // Foreign Key for GORM
	InstrumentISIN   string     `json:"-"` // Foreign Key to Instrument table
	Instrument       Instrument `json:"instrument" gorm:"foreignKey:InstrumentISIN;references:ISIN"`
	InvestedAmount   Decimal    `json:"invested_amount" gorm:"type:numeric"`
	InvestedCurrency string     `json:"invested_currency"`
	Quantity         Decimal    `json:"quantity" gorm:"type:numeric"`
	CurrentPrice     Decimal    `json:"current_price" gorm:"type:numeric"`
	LastUpdated      time.Time  `json:"last_updated"`
}

func NewPosition(instrument Instrument, investedAmount Decimal, investedCurrency string) Position {
	return Position{
		ID:               uuid.New().String(),
		Instrument:       instrument,
		InvestedAmount:   investedAmount,
		InvestedCurrency: investedCurrency,
		Quantity:         Zero,
		CurrentPrice:     Zero,
		LastUpdated:      time.Now(),
	}
}

func (p *Position) UpdatePrice(price Decimal) error {
	p.CurrentPrice = price
	p.LastUpdated = time.Now()

	if !price.IsZero() && !p.InvestedAmount.IsZero() {
		quantity, err := p.InvestedAmount.Div(price)
		if err != nil {
			return fmt.Errorf("failed to calculate quantity: %w", err)
		}
		p.Quantity = quantity
	}
	return nil
}

func (p *Position) CurrentValue() (Decimal, error) {
	if p.CurrentPrice.IsZero() {
		return Zero, nil
	}
	value, err := p.Quantity.Mul(p.CurrentPrice)
	if err != nil {
		return Zero, fmt.Errorf("failed to calculate current value: %w", err)
	}
	return value, nil
}

func (p *Position) ProfitLoss() (Decimal, error) {
	currentValue, err := p.CurrentValue()
	if err != nil {
		return Zero, fmt.Errorf("failed to get current value: %w", err)
	}
	result, err := currentValue.Sub(p.InvestedAmount)
	if err != nil {
		return Zero, fmt.Errorf("failed to calculate profit/loss: %w", err)
	}
	return result, nil
}

func (p *Position) ProfitLossPercent() (Decimal, error) {
	if p.InvestedAmount.IsZero() {
		return Zero, nil
	}
	profitLoss, err := p.ProfitLoss()
	if err != nil {
		return Zero, fmt.Errorf("failed to calculate profit/loss: %w", err)
	}
	percentage, err := profitLoss.Div(p.InvestedAmount)
	if err != nil {
		return Zero, fmt.Errorf("failed to divide profit/loss: %w", err)
	}
	hundred := NewDecimalFromInt(100)
	result, err := percentage.Mul(hundred)
	if err != nil {
		return Zero, fmt.Errorf("failed to multiply by 100: %w", err)
	}
	return result, nil
}

func (p *Position) IsValid() bool {
	return p.ID != "" &&
		p.Instrument.IsValid() &&
		!p.InvestedAmount.IsZero() &&
		p.InvestedCurrency != ""
}
