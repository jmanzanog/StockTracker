package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var (
	ErrPositionNotFound  = errors.New("position not found")
	ErrInvalidPosition   = errors.New("invalid position")
	ErrDuplicatePosition = errors.New("position with same ISIN already exists")
)

type Portfolio struct {
	ID          string     `json:"id" gorm:"primaryKey"`
	Name        string     `json:"name"`
	Positions   []Position `json:"positions"`
	LastUpdated time.Time  `json:"last_updated"`
	CreatedAt   time.Time  `json:"created_at"`
}

func NewPortfolio(name string) Portfolio {
	return Portfolio{
		ID:        uuid.New().String(),
		Name:      name,
		Positions: make([]Position, 0),
		CreatedAt: time.Now(),
	}
}

func (p *Portfolio) AddPosition(pos Position) error {
	if !pos.IsValid() {
		return ErrInvalidPosition
	}

	for i, existing := range p.Positions {
		if existing.ID == pos.ID || (existing.Instrument.ISIN == pos.Instrument.ISIN && existing.Instrument.ISIN != "") {
			// Merge Logic: Update existing position
			p.Positions[i].InvestedAmount = p.Positions[i].InvestedAmount.Add(pos.InvestedAmount)
			p.Positions[i].Quantity = p.Positions[i].Quantity.Add(pos.Quantity)
			// We keep the latest price update
			p.Positions[i].CurrentPrice = pos.CurrentPrice
			p.Positions[i].LastUpdated = time.Now()
			return nil
		}
	}

	p.Positions = append(p.Positions, pos)
	return nil
}

func (p *Portfolio) RemovePosition(id string) error {
	for i, pos := range p.Positions {
		if pos.ID == id {
			p.Positions = append(p.Positions[:i], p.Positions[i+1:]...)
			return nil
		}
	}
	return ErrPositionNotFound
}

func (p *Portfolio) GetPosition(id string) (*Position, error) {
	for i := range p.Positions {
		if p.Positions[i].ID == id {
			return &p.Positions[i], nil
		}
	}
	return nil, ErrPositionNotFound
}

func (p *Portfolio) UpdatePositionPrice(id string, price decimal.Decimal) error {
	pos, err := p.GetPosition(id)
	if err != nil {
		return err
	}
	pos.UpdatePrice(price)
	return nil
}

func (p Portfolio) TotalValue() decimal.Decimal {
	total := decimal.Zero
	for _, pos := range p.Positions {
		total = total.Add(pos.CurrentValue())
	}
	return total
}

func (p Portfolio) TotalInvested() decimal.Decimal {
	total := decimal.Zero
	for _, pos := range p.Positions {
		total = total.Add(pos.InvestedAmount)
	}
	return total
}

func (p Portfolio) TotalProfitLoss() decimal.Decimal {
	return p.TotalValue().Sub(p.TotalInvested())
}

func (p Portfolio) TotalProfitLossPercent() decimal.Decimal {
	invested := p.TotalInvested()
	if invested.IsZero() {
		return decimal.Zero
	}
	return p.TotalProfitLoss().Div(invested).Mul(decimal.NewFromInt(100))
}
