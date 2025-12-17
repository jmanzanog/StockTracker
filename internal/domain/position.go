package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Position struct {
	ID               string          `json:"id" gorm:"primaryKey"`
	PortfolioID      string          `json:"-"` // Foreign Key for GORM
	InstrumentISIN   string          `json:"-"` // Foreign Key to Instrument table
	Instrument       Instrument      `json:"instrument" gorm:"foreignKey:InstrumentISIN;references:ISIN"`
	InvestedAmount   decimal.Decimal `json:"invested_amount" gorm:"type:decimal(20,8)"`
	InvestedCurrency string          `json:"invested_currency"`
	Quantity         decimal.Decimal `json:"quantity" gorm:"type:decimal(20,8)"`
	CurrentPrice     decimal.Decimal `json:"current_price" gorm:"type:decimal(20,8)"`
	LastUpdated      time.Time       `json:"last_updated"`
}

func NewPosition(instrument Instrument, investedAmount decimal.Decimal, investedCurrency string) Position {
	return Position{
		ID:               uuid.New().String(),
		Instrument:       instrument,
		InvestedAmount:   investedAmount,
		InvestedCurrency: investedCurrency,
		Quantity:         decimal.Zero,
		CurrentPrice:     decimal.Zero,
		LastUpdated:      time.Now(),
	}
}

func (p *Position) UpdatePrice(price decimal.Decimal) {
	p.CurrentPrice = price
	p.LastUpdated = time.Now()

	if !price.IsZero() && !p.InvestedAmount.IsZero() {
		p.Quantity = p.InvestedAmount.Div(price)
	}
}

func (p Position) CurrentValue() decimal.Decimal {
	if p.CurrentPrice.IsZero() {
		return decimal.Zero
	}
	return p.Quantity.Mul(p.CurrentPrice)
}

func (p Position) ProfitLoss() decimal.Decimal {
	return p.CurrentValue().Sub(p.InvestedAmount)
}

func (p Position) ProfitLossPercent() decimal.Decimal {
	if p.InvestedAmount.IsZero() {
		return decimal.Zero
	}
	return p.ProfitLoss().Div(p.InvestedAmount).Mul(decimal.NewFromInt(100))
}

func (p Position) IsValid() bool {
	return p.ID != "" &&
		p.Instrument.IsValid() &&
		!p.InvestedAmount.IsZero() &&
		p.InvestedCurrency != ""
}
