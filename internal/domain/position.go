package domain

import (
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

func (p *Position) UpdatePrice(price Decimal) {
	p.CurrentPrice = price
	p.LastUpdated = time.Now()

	if !price.IsZero() && !p.InvestedAmount.IsZero() {
		p.Quantity = p.InvestedAmount.Div(price)
	}
}

func (p *Position) CurrentValue() Decimal {
	if p.CurrentPrice.IsZero() {
		return Zero
	}
	return p.Quantity.Mul(p.CurrentPrice)
}

func (p *Position) ProfitLoss() Decimal {
	return p.CurrentValue().Sub(p.InvestedAmount)
}

func (p *Position) ProfitLossPercent() Decimal {
	if p.InvestedAmount.IsZero() {
		return Zero
	}
	return p.ProfitLoss().Div(p.InvestedAmount).Mul(NewDecimalFromInt(100))
}

func (p *Position) IsValid() bool {
	return p.ID != "" &&
		p.Instrument.IsValid() &&
		!p.InvestedAmount.IsZero() &&
		p.InvestedCurrency != ""
}
