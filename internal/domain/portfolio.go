package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	ErrPositionNotFound    = errors.New("position not found")
	ErrInvalidPosition     = errors.New("invalid position")
	ErrInsufficientShares  = errors.New("insufficient shares to sell")
	ErrInvalidSaleQuantity = errors.New("invalid sale quantity")
	ErrInvalidSalePrice    = errors.New("invalid sale price")
)

type Portfolio struct {
	ID          string     `json:"id"`
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

// SellPartialResult contains the result of a partial sale operation.
type SellPartialResult struct {
	Sale       *SaleTransaction `json:"sale"`
	Position   *Position        `json:"position"`
	IsFullSale bool             `json:"is_full_sale"`
}

// SellPartial sells a portion (or all) of a position.
// It calculates the proportional invested amount to remove, records the P/L,
// and returns a SaleTransaction for audit purposes.
//
// Parameters:
//   - positionID: ID of the position to sell
//   - quantityStr: quantity to sell (as string for decimal parsing)
//   - salePriceStr: sale price per share (as string for decimal parsing)
//
// Returns:
//   - SellPartialResult with the sale record and updated position
//   - Error if validation fails or position not found
func (p *Portfolio) SellPartial(positionID, quantityStr, salePriceStr string) (*SellPartialResult, error) {
	// Parse input values
	quantityToSell, err := NewDecimalFromString(quantityStr)
	if err != nil || quantityToSell.Cmp(Zero) < 0 {
		return nil, fmt.Errorf("invalid quantity: %w", ErrInvalidSaleQuantity)
	}

	salePrice, err := NewDecimalFromString(salePriceStr)
	if err != nil || salePrice.Cmp(Zero) < 0 {
		return nil, fmt.Errorf("invalid price: %w", ErrInvalidSalePrice)
	}

	// Find the position
	posIdx := -1
	for i, pos := range p.Positions {
		if pos.ID == positionID {
			posIdx = i
			break
		}
	}
	if posIdx == -1 {
		return nil, ErrPositionNotFound
	}

	pos := &p.Positions[posIdx]

	// Validate: cannot sell more than owned
	if quantityToSell.Cmp(pos.Quantity) > 0 {
		return nil, fmt.Errorf("trying to sell %s but only own %s: %w",
			quantityToSell.String(), pos.Quantity.String(), ErrInsufficientShares)
	}

	// Calculate total proceeds from sale
	totalProceeds, err := quantityToSell.Mul(salePrice)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate proceeds: %w", err)
	}

	// Calculate proportional invested amount to remove
	// Formula: investedSold = investedAmount * (qtySold / qtyTotal)
	saleRatio, err := quantityToSell.Div(pos.Quantity)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate sale ratio: %w", err)
	}

	investedSold, err := pos.InvestedAmount.Mul(saleRatio)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate invested sold: %w", err)
	}

	// Calculate P/L on the sold portion
	profitLoss, err := totalProceeds.Sub(investedSold)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate profit/loss: %w", err)
	}

	// Calculate remaining values
	remainingQty, err := pos.Quantity.Sub(quantityToSell)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate remaining quantity: %w", err)
	}

	remainingInvest, err := pos.InvestedAmount.Sub(investedSold)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate remaining invested: %w", err)
	}

	// Determine if this is a full sale
	isFullSale := remainingQty.IsZero() || remainingQty.Cmp(NewDecimalFromInt(0)) == 0

	// Create sale transaction record
	sale := NewSaleTransaction(
		pos.ID,
		pos.Instrument.ISIN,
		pos.Instrument.Symbol,
		quantityToSell,
		salePrice,
		totalProceeds,
		investedSold,
		profitLoss,
		pos.InvestedCurrency,
		remainingQty,
		remainingInvest,
		isFullSale,
	)

	// Update the position
	pos.Quantity = remainingQty
	pos.InvestedAmount = remainingInvest
	if isFullSale {
		// Remove position entirely if fully sold
		p.Positions = append(p.Positions[:posIdx], p.Positions[posIdx+1:]...)
		pos = nil // Position no longer exists
	} else {
		pos.CurrentPrice = salePrice // Update to latest sale price
		pos.LastUpdated = time.Now()
	}

	p.LastUpdated = time.Now()

	return &SellPartialResult{
		Sale:       sale,
		Position:   pos,
		IsFullSale: isFullSale,
	}, nil
}

// Clone creates a deep copy of the portfolio and its positions.
// This is essential for thread safety, ensuring that read operations (like calculating totals
// for a report) don't conflict with concurrent write operations (like background price updates).
func (p *Portfolio) Clone() Portfolio {
	clone := *p
	clone.Positions = make([]Position, len(p.Positions))
	copy(clone.Positions, p.Positions)
	return clone
}
