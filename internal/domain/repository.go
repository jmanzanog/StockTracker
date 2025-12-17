package domain

// PortfolioRepository defines the interface for portfolio persistence.
// It follows the Domain-Driven Design repository pattern.
type PortfolioRepository interface {
	Save(portfolio *Portfolio) error
	FindByID(id string) (*Portfolio, error)
	FindAll() ([]*Portfolio, error)
	Delete(id string) error
}
