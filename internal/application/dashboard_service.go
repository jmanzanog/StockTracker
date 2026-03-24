package application

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jmanzanog/stock-tracker/internal/domain"
)

type DashboardService struct {
	portfolioRepo    domain.PortfolioRepository
	priceHistoryRepo domain.PriceHistoryRepository
}

func NewDashboardService(portfolioRepo domain.PortfolioRepository, priceHistoryRepo domain.PriceHistoryRepository) *DashboardService {
	return &DashboardService{
		portfolioRepo:    portfolioRepo,
		priceHistoryRepo: priceHistoryRepo,
	}
}

type GetDashboardRequest struct {
	SparklineDays []int
}

func (s *DashboardService) GetDashboard(ctx context.Context, req GetDashboardRequest) (*domain.DashboardSnapshot, error) {
	portfolio, err := s.portfolioRepo.FindByID(ctx, "default")
	if err != nil {
		return nil, fmt.Errorf("failed to get portfolio: %w", err)
	}

	totalValue := domain.NewDecimalFromInt(0)
	totalInvested := domain.NewDecimalFromInt(0)

	positionDashboards := make([]domain.PositionDashboard, 0, len(portfolio.Positions))

	sparklineRequests := make([]domain.SparklineRequest, 0)
	sparklineIndex := make(map[string]int)

	for _, pos := range portfolio.Positions {
		for _, days := range req.SparklineDays {
			key := fmt.Sprintf("%s_%dd", pos.Instrument.ISIN, days)
			sparklineRequests = append(sparklineRequests, domain.SparklineRequest{ISIN: pos.Instrument.ISIN, Days: days})
			sparklineIndex[key] = len(sparklineRequests) - 1
		}
	}

	sparklineResults := make(map[string][]domain.PriceHistory)
	if len(sparklineRequests) > 0 {
		results, err := s.priceHistoryRepo.GetSparklinesBatch(ctx, sparklineRequests)
		if err != nil {
			slog.Warn("failed to get sparklines batch", "error", err)
		} else {
			for _, result := range results {
				key := fmt.Sprintf("%s_%dd", result.ISIN, result.Days)
				sparklineResults[key] = result.Points
			}
		}
	}

	for _, pos := range portfolio.Positions {
		currentValue, err := pos.CurrentPrice.Mul(pos.Quantity)
		if err != nil {
			slog.Warn("failed to calculate current value", "position_id", pos.ID, "error", err)
			currentValue = domain.NewDecimalFromInt(0)
		}
		pnl, err := currentValue.Sub(pos.InvestedAmount)
		if err != nil {
			slog.Warn("failed to calculate PnL", "position_id", pos.ID, "error", err)
			pnl = domain.NewDecimalFromInt(0)
		}
		pnlPercent := domain.NewDecimalFromInt(0)
		if !pos.InvestedAmount.IsZero() {
			divResult, err := pnl.Div(pos.InvestedAmount)
			if err != nil {
				slog.Warn("failed to calculate PnL percent", "position_id", pos.ID, "error", err)
			} else {
				mulResult, err := divResult.Mul(domain.NewDecimalFromInt(100))
				if err != nil {
					slog.Warn("failed to calculate PnL percent", "position_id", pos.ID, "error", err)
				} else {
					pnlPercent, err = mulResult.Round(2)
					if err != nil {
						slog.Warn("failed to round PnL percent", "position_id", pos.ID, "error", err)
						pnlPercent = domain.NewDecimalFromInt(0)
					}
				}
			}
		}

		pd := domain.PositionDashboard{
			ID:             pos.ID,
			ISIN:           pos.Instrument.ISIN,
			Symbol:         pos.Instrument.Symbol,
			Name:           pos.Instrument.Name,
			Type:           pos.Instrument.Type,
			Sector:         pos.Instrument.Sector,
			Quantity:       pos.Quantity,
			CurrentPrice:   pos.CurrentPrice,
			CurrentValue:   currentValue,
			InvestedAmount: pos.InvestedAmount,
			PnL:            pnl,
			PnLPercent:     pnlPercent,
			Currency:       pos.Instrument.Currency,
			Sparklines:     make(map[string][]domain.SparklinePoint),
		}

		for _, days := range req.SparklineDays {
			key := fmt.Sprintf("%dd", days)
			sparkKey := fmt.Sprintf("%s_%dd", pos.Instrument.ISIN, days)
			if history, ok := sparklineResults[sparkKey]; ok {
				points := make([]domain.SparklinePoint, 0, len(history))
				for _, h := range history {
					points = append(points, domain.SparklinePoint{
						Date:  h.RecordedAt,
						Price: h.Price,
					})
				}
				pd.Sparklines[key] = points
			} else {
				pd.Sparklines[key] = []domain.SparklinePoint{}
			}
		}

		positionDashboards = append(positionDashboards, pd)

		newTotalValue, err := totalValue.Add(currentValue)
		if err != nil {
			slog.Warn("failed to add to total value", "position_id", pos.ID, "error", err)
		} else {
			totalValue = newTotalValue
		}
		newTotalInvested, err := totalInvested.Add(pos.InvestedAmount)
		if err != nil {
			slog.Warn("failed to add to total invested", "position_id", pos.ID, "error", err)
		} else {
			totalInvested = newTotalInvested
		}
	}

	totalPnL, err := totalValue.Sub(totalInvested)
	if err != nil {
		slog.Warn("failed to calculate total PnL", "error", err)
		totalPnL = domain.NewDecimalFromInt(0)
	}
	pnlPercent := domain.NewDecimalFromInt(0)
	if !totalInvested.IsZero() {
		divResult, err := totalPnL.Div(totalInvested)
		if err != nil {
			slog.Warn("failed to calculate total PnL percent", "error", err)
		} else {
			mulResult, err := divResult.Mul(domain.NewDecimalFromInt(100))
			if err != nil {
				slog.Warn("failed to calculate total PnL percent", "error", err)
			} else {
				pnlPercent, err = mulResult.Round(2)
				if err != nil {
					slog.Warn("failed to round total PnL percent", "error", err)
					pnlPercent = domain.NewDecimalFromInt(0)
				}
			}
		}
	}

	byCurrency := s.calculateByCurrency(positionDashboards, totalValue)
	byType := s.calculateByType(positionDashboards, totalValue)
	bySector := s.calculateBySector(positionDashboards, totalValue)

	return &domain.DashboardSnapshot{
		PortfolioID:   portfolio.ID,
		GeneratedAt:   time.Now(),
		TotalValue:    totalValue,
		TotalInvested: totalInvested,
		TotalPnL:      totalPnL,
		PnLPercent:    pnlPercent,
		ByCurrency:    byCurrency,
		ByType:        byType,
		BySector:      bySector,
		Positions:     positionDashboards,
	}, nil
}

func (s *DashboardService) calculateByCurrency(positions []domain.PositionDashboard, totalValue domain.Decimal) []domain.CurrencyAllocation {
	currencyMap := make(map[string]domain.Decimal)

	for _, pos := range positions {
		existing := currencyMap[pos.Currency]
		newVal, err := existing.Add(pos.CurrentValue)
		if err != nil {
			slog.Warn("failed to add currency value", "currency", pos.Currency, "error", err)
			continue
		}
		currencyMap[pos.Currency] = newVal
	}

	allocations := make([]domain.CurrencyAllocation, 0, len(currencyMap))
	for currency, value := range currencyMap {
		percent := domain.NewDecimalFromInt(0)
		if !totalValue.IsZero() {
			divResult, err := value.Div(totalValue)
			if err != nil {
				slog.Warn("failed to calculate currency percent", "currency", currency, "error", err)
				continue
			}
			mulResult, err := divResult.Mul(domain.NewDecimalFromInt(100))
			if err != nil {
				slog.Warn("failed to calculate currency percent", "currency", currency, "error", err)
				continue
			}
			percent, err = mulResult.Round(2)
			if err != nil {
				slog.Warn("failed to round currency percent", "currency", currency, "error", err)
				percent = domain.NewDecimalFromInt(0)
			}
		}
		allocations = append(allocations, domain.CurrencyAllocation{
			Currency:   currency,
			TotalValue: value,
			Percent:    percent,
		})
	}
	return allocations
}

func (s *DashboardService) calculateByType(positions []domain.PositionDashboard, totalValue domain.Decimal) []domain.TypeAllocation {
	typeMap := make(map[domain.InstrumentType]domain.Decimal)

	for _, pos := range positions {
		existing := typeMap[pos.Type]
		newVal, err := existing.Add(pos.CurrentValue)
		if err != nil {
			slog.Warn("failed to add type value", "type", pos.Type, "error", err)
			continue
		}
		typeMap[pos.Type] = newVal
	}

	allocations := make([]domain.TypeAllocation, 0, len(typeMap))
	for itype, value := range typeMap {
		percent := domain.NewDecimalFromInt(0)
		if !totalValue.IsZero() {
			divResult, err := value.Div(totalValue)
			if err != nil {
				slog.Warn("failed to calculate type percent", "type", itype, "error", err)
				continue
			}
			mulResult, err := divResult.Mul(domain.NewDecimalFromInt(100))
			if err != nil {
				slog.Warn("failed to calculate type percent", "type", itype, "error", err)
				continue
			}
			percent, err = mulResult.Round(2)
			if err != nil {
				slog.Warn("failed to round type percent", "type", itype, "error", err)
				percent = domain.NewDecimalFromInt(0)
			}
		}
		allocations = append(allocations, domain.TypeAllocation{
			Type:       itype,
			TotalValue: value,
			Percent:    percent,
		})
	}
	return allocations
}

func (s *DashboardService) calculateBySector(positions []domain.PositionDashboard, totalValue domain.Decimal) []domain.SectorAllocation {
	sectorMap := make(map[string]domain.Decimal)

	for _, pos := range positions {
		sector := pos.Sector
		if sector == "" {
			sector = "Unknown"
		}
		existing := sectorMap[sector]
		newVal, err := existing.Add(pos.CurrentValue)
		if err != nil {
			slog.Warn("failed to add sector value", "sector", sector, "error", err)
			continue
		}
		sectorMap[sector] = newVal
	}

	allocations := make([]domain.SectorAllocation, 0, len(sectorMap))
	for sector, value := range sectorMap {
		percent := domain.NewDecimalFromInt(0)
		if !totalValue.IsZero() {
			divResult, err := value.Div(totalValue)
			if err != nil {
				slog.Warn("failed to calculate sector percent", "sector", sector, "error", err)
				continue
			}
			mulResult, err := divResult.Mul(domain.NewDecimalFromInt(100))
			if err != nil {
				slog.Warn("failed to calculate sector percent", "sector", sector, "error", err)
				continue
			}
			percent, err = mulResult.Round(2)
			if err != nil {
				slog.Warn("failed to round sector percent", "sector", sector, "error", err)
				percent = domain.NewDecimalFromInt(0)
			}
		}
		allocations = append(allocations, domain.SectorAllocation{
			Sector:     sector,
			TotalValue: value,
			Percent:    percent,
		})
	}
	return allocations
}
