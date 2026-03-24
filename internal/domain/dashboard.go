package domain

import "time"

type DashboardSnapshot struct {
	PortfolioID   string               `json:"portfolio_id"`
	GeneratedAt   time.Time            `json:"generated_at"`
	TotalValue    Decimal              `json:"total_value"`
	TotalInvested Decimal              `json:"total_invested"`
	TotalPnL      Decimal              `json:"total_pnl"`
	PnLPercent    Decimal              `json:"pnl_percent"`
	ByCurrency    []CurrencyAllocation `json:"by_currency"`
	ByType        []TypeAllocation     `json:"by_type"`
	BySector      []SectorAllocation   `json:"by_sector"`
	Positions     []PositionDashboard  `json:"positions"`
}

type CurrencyAllocation struct {
	Currency   string  `json:"currency"`
	TotalValue Decimal `json:"total_value"`
	Percent    Decimal `json:"percent"`
}

type TypeAllocation struct {
	Type       InstrumentType `json:"type"`
	TotalValue Decimal        `json:"total_value"`
	Percent    Decimal        `json:"percent"`
}

type SectorAllocation struct {
	Sector     string  `json:"sector"`
	TotalValue Decimal `json:"total_value"`
	Percent    Decimal `json:"percent"`
}

type PositionDashboard struct {
	ID             string                      `json:"id"`
	ISIN           string                      `json:"isin"`
	Symbol         string                      `json:"symbol"`
	Name           string                      `json:"name"`
	Type           InstrumentType              `json:"type"`
	Sector         string                      `json:"sector"`
	Quantity       Decimal                     `json:"quantity"`
	CurrentPrice   Decimal                     `json:"current_price"`
	CurrentValue   Decimal                     `json:"current_value"`
	InvestedAmount Decimal                     `json:"invested_amount"`
	PnL            Decimal                     `json:"pnl"`
	PnLPercent     Decimal                     `json:"pnl_percent"`
	Currency       string                      `json:"currency"`
	Sparklines     map[string][]SparklinePoint `json:"sparklines"`
}

type SparklinePoint struct {
	Date  time.Time `json:"date"`
	Price Decimal   `json:"price"`
}
