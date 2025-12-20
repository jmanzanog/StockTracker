package application

import (
	"context"
	"fmt"

	"github.com/jmanzanog/stock-tracker/internal/domain"
	"github.com/jmanzanog/stock-tracker/internal/infrastructure/marketdata"
)

type PortfolioService struct {
	repo             domain.PortfolioRepository
	marketData       marketdata.MarketDataProvider
	defaultPortfolio *domain.Portfolio
}

func NewPortfolioService(repo domain.PortfolioRepository, marketData marketdata.MarketDataProvider) (*PortfolioService, error) {
	defaultPortfolio := domain.NewPortfolio("default")
	// Use background context for initialization since there's no request context yet
	if err := repo.Save(context.Background(), &defaultPortfolio); err != nil {
		return nil, fmt.Errorf("failed to save default portfolio: %w", err)
	}

	return &PortfolioService{
		repo:             repo,
		marketData:       marketData,
		defaultPortfolio: &defaultPortfolio,
	}, nil
}

func (s *PortfolioService) AddPosition(ctx context.Context, isin string, investedAmount domain.Decimal, currency string) (*domain.Position, error) {
	instrument, err := s.marketData.SearchByISIN(ctx, isin)
	if err != nil {
		return nil, fmt.Errorf("failed to find instrument: %w", err)
	}

	position := domain.NewPosition(*instrument, investedAmount, currency)

	quote, err := s.marketData.GetQuote(ctx, instrument.Symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get quote: %w", err)
	}

	// Convert shopspring decimal (from marketdata) to domain decimal
	price, err := domain.NewDecimalFromString(quote.Price.String())
	if err != nil {
		return nil, fmt.Errorf("failed to parse quote price: %w", err)
	}

	if err := position.UpdatePrice(price); err != nil {
		return nil, fmt.Errorf("failed to update position price: %w", err)
	}

	if err := s.defaultPortfolio.AddPosition(position); err != nil {
		return nil, fmt.Errorf("failed to add position: %w", err)
	}

	if err := s.repo.Save(ctx, s.defaultPortfolio); err != nil {
		return nil, fmt.Errorf("failed to save portfolio: %w", err)
	}

	return &position, nil
}

func (s *PortfolioService) RemovePosition(ctx context.Context, positionID string) error {
	if err := s.defaultPortfolio.RemovePosition(positionID); err != nil {
		return fmt.Errorf("failed to remove position: %w", err)
	}

	if err := s.repo.Save(ctx, s.defaultPortfolio); err != nil {
		return fmt.Errorf("failed to save portfolio: %w", err)
	}

	return nil
}

func (s *PortfolioService) GetPosition(ctx context.Context, positionID string) (*domain.Position, error) {
	position, err := s.defaultPortfolio.GetPosition(positionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get position: %w", err)
	}
	return position, nil
}

func (s *PortfolioService) ListPositions(ctx context.Context) ([]domain.Position, error) {
	return s.defaultPortfolio.Positions, nil
}

func (s *PortfolioService) GetPortfolioSummary(ctx context.Context) (*domain.Portfolio, error) {
	return s.defaultPortfolio, nil
}

func (s *PortfolioService) RefreshPrices(ctx context.Context) error {
	for i := range s.defaultPortfolio.Positions {
		pos := &s.defaultPortfolio.Positions[i]

		quote, err := s.marketData.GetQuote(ctx, pos.Instrument.Symbol)
		if err != nil {
			return fmt.Errorf("failed to get quote for %s: %w", pos.Instrument.Symbol, err)
		}

		price, err := domain.NewDecimalFromString(quote.Price.String())
		if err != nil {
			return fmt.Errorf("failed to parse quote price for %s: %w", pos.Instrument.Symbol, err)
		}

		if err := pos.UpdatePrice(price); err != nil {
			return fmt.Errorf("failed to update price for %s: %w", pos.Instrument.Symbol, err)
		}
	}

	if err := s.repo.Save(ctx, s.defaultPortfolio); err != nil {
		return fmt.Errorf("failed to save portfolio: %w", err)
	}

	return nil
}
