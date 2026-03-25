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
	allocated     bool                 `json:"-"`
}

func (d *DashboardSnapshot) CalculateAllocations() {
	if d.allocated {
		return
	}
	d.ByCurrency = d.calculateByCurrency()
	d.ByType = d.calculateByType()
	d.BySector = d.calculateBySector()
	d.allocated = true
}

func (d *DashboardSnapshot) calculateByCurrency() []CurrencyAllocation {
	currencyMap := make(map[string]Decimal)

	for _, pos := range d.Positions {
		existing := currencyMap[pos.Currency]
		newVal, _ := existing.Add(pos.CurrentValue)
		currencyMap[pos.Currency] = newVal
	}

	allocations := make([]CurrencyAllocation, 0, len(currencyMap))
	for currency, value := range currencyMap {
		percent := NewDecimalFromInt(0)
		if !d.TotalValue.IsZero() {
			divResult, _ := value.Div(d.TotalValue)
			mulResult, _ := divResult.Mul(NewDecimalFromInt(100))
			percent, _ = mulResult.Round(2)
		}
		allocations = append(allocations, CurrencyAllocation{
			Currency:   currency,
			TotalValue: value,
			Percent:    percent,
		})
	}
	return allocations
}

func (d *DashboardSnapshot) calculateByType() []TypeAllocation {
	typeMap := make(map[InstrumentType]Decimal)

	for _, pos := range d.Positions {
		existing := typeMap[pos.Type]
		newVal, _ := existing.Add(pos.CurrentValue)
		typeMap[pos.Type] = newVal
	}

	allocations := make([]TypeAllocation, 0, len(typeMap))
	for itype, value := range typeMap {
		percent := NewDecimalFromInt(0)
		if !d.TotalValue.IsZero() {
			divResult, _ := value.Div(d.TotalValue)
			mulResult, _ := divResult.Mul(NewDecimalFromInt(100))
			percent, _ = mulResult.Round(2)
		}
		allocations = append(allocations, TypeAllocation{
			Type:       itype,
			TotalValue: value,
			Percent:    percent,
		})
	}
	return allocations
}

func (d *DashboardSnapshot) calculateBySector() []SectorAllocation {
	sectorMap := make(map[string]Decimal)

	for _, pos := range d.Positions {
		sector := pos.Sector
		if sector == "" {
			sector = "N/A"
		}
		existing := sectorMap[sector]
		newVal, _ := existing.Add(pos.CurrentValue)
		sectorMap[sector] = newVal
	}

	allocations := make([]SectorAllocation, 0, len(sectorMap))
	for sector, value := range sectorMap {
		percent := NewDecimalFromInt(0)
		if !d.TotalValue.IsZero() {
			divResult, _ := value.Div(d.TotalValue)
			mulResult, _ := divResult.Mul(NewDecimalFromInt(100))
			percent, _ = mulResult.Round(2)
		}
		allocations = append(allocations, SectorAllocation{
			Sector:     sector,
			TotalValue: value,
			Percent:    percent,
		})
	}
	return allocations
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
