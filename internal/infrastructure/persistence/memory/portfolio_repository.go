package memory

import (
	"context"
	"errors"
	"sync"

	"github.com/jmanzanog/stock-tracker/internal/domain"
)

var ErrPortfolioNotFound = errors.New("portfolio not found")

type PortfolioRepository struct {
	mu         sync.RWMutex
	portfolios map[string]*domain.Portfolio
}

func NewPortfolioRepository() *PortfolioRepository {
	return &PortfolioRepository{
		portfolios: make(map[string]*domain.Portfolio),
	}
}

func (r *PortfolioRepository) Save(ctx context.Context, portfolio *domain.Portfolio) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.portfolios[portfolio.ID] = portfolio
	return nil
}

func (r *PortfolioRepository) FindByID(ctx context.Context, id string) (*domain.Portfolio, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	portfolio, exists := r.portfolios[id]
	if !exists {
		return nil, ErrPortfolioNotFound
	}

	return portfolio, nil
}

func (r *PortfolioRepository) FindAll(ctx context.Context) ([]*domain.Portfolio, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	portfolios := make([]*domain.Portfolio, 0, len(r.portfolios))
	for _, p := range r.portfolios {
		portfolios = append(portfolios, p)
	}

	return portfolios, nil
}

func (r *PortfolioRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.portfolios[id]; !exists {
		return ErrPortfolioNotFound
	}

	delete(r.portfolios, id)
	return nil
}
