package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	ErrPositionNotFound = errors.New("position not found")
	ErrInvalidPosition  = errors.New("invalid position")
)

type Portfolio struct {
	ID          string     `json:"id" gorm:"primaryKey"`
	Name        string     `json:"name"`
	Positions   []Position `json:"positions" gorm:"foreignKey:PortfolioID"`
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
			newInvestedAmount, err := p.Positions[i].InvestedAmount.Add(pos.InvestedAmount)
			if err != nil {
				return fmt.Errorf("failed to add invested amount: %w", err)
			}
			p.Positions[i].InvestedAmount = newInvestedAmount

			newQuantity, err := p.Positions[i].Quantity.Add(pos.Quantity)
			if err != nil {
				return fmt.Errorf("failed to add quantity: %w", err)
			}
			p.Positions[i].Quantity = newQuantity

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

func (p *Portfolio) UpdatePositionPrice(id string, price Decimal) error {
	pos, err := p.GetPosition(id)
	if err != nil {
		return err
	}
	if err := pos.UpdatePrice(price); err != nil {
		return fmt.Errorf("failed to update price: %w", err)
	}
	return nil
}

func (p *Portfolio) TotalValue() (Decimal, error) {
	total := Zero
	for _, pos := range p.Positions {
		currentValue, err := pos.CurrentValue()
		if err != nil {
			return Zero, fmt.Errorf("failed to calculate current value: %w", err)
		}
		newTotal, err := total.Add(currentValue)
		if err != nil {
			return Zero, fmt.Errorf("failed to add to total: %w", err)
		}
		total = newTotal
	}
	return total, nil
}

func (p *Portfolio) TotalInvested() (Decimal, error) {
	total := Zero
	for _, pos := range p.Positions {
		newTotal, err := total.Add(pos.InvestedAmount)
		if err != nil {
			return Zero, fmt.Errorf("failed to add invested amount: %w", err)
		}
		total = newTotal
	}
	return total, nil
}

func (p *Portfolio) TotalProfitLoss() (Decimal, error) {
	totalValue, err := p.TotalValue()
	if err != nil {
		return Zero, fmt.Errorf("failed to calculate total value: %w", err)
	}
	totalInvested, err := p.TotalInvested()
	if err != nil {
		return Zero, fmt.Errorf("failed to calculate total invested: %w", err)
	}
	result, err := totalValue.Sub(totalInvested)
	if err != nil {
		return Zero, fmt.Errorf("failed to subtract: %w", err)
	}
	return result, nil
}

func (p *Portfolio) TotalProfitLossPercent() (Decimal, error) {
	invested, err := p.TotalInvested()
	if err != nil {
		return Zero, fmt.Errorf("failed to calculate total invested: %w", err)
	}
	if invested.IsZero() {
		return Zero, nil
	}
	profitLoss, err := p.TotalProfitLoss()
	if err != nil {
		return Zero, fmt.Errorf("failed to calculate profit/loss: %w", err)
	}
	percentage, err := profitLoss.Div(invested)
	if err != nil {
		return Zero, fmt.Errorf("failed to divide: %w", err)
	}
	hundred := NewDecimalFromInt(100)
	result, err := percentage.Mul(hundred)
	if err != nil {
		return Zero, fmt.Errorf("failed to multiply by 100: %w", err)
	}
	return result, nil
}
