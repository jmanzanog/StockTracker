package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// SaleTransaction represents a partial or full sale of a position.
// It records the details of the sale for audit and analytics purposes.
type SaleTransaction struct {
	ID              string    `json:"id"`
	PositionID      string    `json:"position_id"`
	ISIN            string    `json:"isin"`
	Symbol          string    `json:"symbol"`
	QuantitySold    Decimal   `json:"quantity_sold"`
	SalePrice       Decimal   `json:"sale_price"`
	TotalProceeds   Decimal   `json:"total_proceeds"`
	InvestedSold    Decimal   `json:"invested_sold"`
	ProfitLoss      Decimal   `json:"profit_loss"`
	ProfitLossPct   Decimal   `json:"profit_loss_pct"`
	Currency        string    `json:"currency"`
	SoldAt          time.Time `json:"sold_at"`
	RemainingQty    Decimal   `json:"remaining_qty"`
	RemainingInvest Decimal   `json:"remaining_invest"`
	IsFullSale      bool      `json:"is_full_sale"`
}

// NewSaleTransaction creates a new sale transaction record.
// It calculates the P/L and remaining values based on the sale.
func NewSaleTransaction(
	positionID, isin, symbol string,
	quantitySold, salePrice, totalProceeds, investedSold, profitLoss Decimal,
	currency string,
	remainingQty, remainingInvest Decimal,
	isFullSale bool,
) *SaleTransaction {
	profitLossPct := Zero
	if !investedSold.IsZero() {
		pct, err := profitLoss.Div(investedSold)
		if err == nil {
			profitLossPct, _ = pct.Mul(NewDecimalFromInt(100))
		}
	}

	return &SaleTransaction{
		ID:              uuid.New().String(),
		PositionID:      positionID,
		ISIN:            isin,
		Symbol:          symbol,
		QuantitySold:    quantitySold,
		SalePrice:       salePrice,
		TotalProceeds:   totalProceeds,
		InvestedSold:    investedSold,
		ProfitLoss:      profitLoss,
		ProfitLossPct:   profitLossPct,
		Currency:        currency,
		SoldAt:          time.Now(),
		RemainingQty:    remainingQty,
		RemainingInvest: remainingInvest,
		IsFullSale:      isFullSale,
	}
}

// SaleRepository defines the interface for sale transaction persistence.
type SaleRepository interface {
	Save(ctx context.Context, sale *SaleTransaction) error
	FindByPositionID(ctx context.Context, positionID string) ([]*SaleTransaction, error)
	FindAll(ctx context.Context) ([]*SaleTransaction, error)
}
