package domain

import "context"

// PortfolioRepository defines the interface for portfolio persistence.
// It follows the Domain-Driven Design repository pattern.
// All methods accept context.Context to enable proper timeout handling,
// cancellation propagation, and request-scoped values like tracing IDs.
type PortfolioRepository interface {
	Save(ctx context.Context, portfolio *Portfolio) error
	FindByID(ctx context.Context, id string) (*Portfolio, error)
	FindAll(ctx context.Context) ([]*Portfolio, error)
	Delete(ctx context.Context, id string) error
}
