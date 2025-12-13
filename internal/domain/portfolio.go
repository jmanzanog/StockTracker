package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var (
	ErrPositionNotFound     = errors.New("position not found")
	ErrInvalidPosition      = errors.New("invalid position")
	ErrDuplicatePosition    = errors.New("position with same ISIN already exists")
)

type Portfolio struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Positions []Position `json:"positions"`
	CreatedAt time.Time  `json:"created_at"`
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

	for _, existing := range p.Positions {
		if existing.Instrument.ISIN == pos.Instrument.ISIN {
			return ErrDuplicatePosition
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
