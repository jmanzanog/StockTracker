package persistence

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/jmanzanog/stock-tracker/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GormRepository implements domain.PortfolioRepository using GORM.
type GormRepository struct {
	db *gorm.DB
}

func NewGormRepository(db *gorm.DB) *GormRepository {
	return &GormRepository{db: db}
}

// AutoMigrate applies schema changes to the database
func (r *GormRepository) AutoMigrate() error {
	// We need to register the models that GORM should manage.
	// Note: We are using domain models directly. Ideally, we would have separate DB models
	// if the mapping becomes complex, to keep domain pure.
	// For now, GORM handles simple structs well.
	return r.db.AutoMigrate(&domain.Portfolio{}, &domain.Position{}, &domain.Instrument{})
}

func (r *GormRepository) Save(portfolio *domain.Portfolio) error {
	// GORM's Save updates if ID exists, creates if not.
	// We use session with FullSaveAssociations to ensure positions are updated naturally.
	// We use OnConflict clause to handle cases where nested entities (like Instruments or Positions) already exist
	// UpdateAll: true ensures that if it exists, it updates it.
	if err := r.db.Session(&gorm.Session{FullSaveAssociations: true}).
		Clauses(clause.OnConflict{UpdateAll: true}).
		Save(portfolio).Error; err != nil {
		slog.Error("Failed to save portfolio", "portfolio_id", portfolio.ID, "error", err)
		return fmt.Errorf("failed to save portfolio: %w", err)
	}
	return nil
}

func (r *GormRepository) FindByID(id string) (*domain.Portfolio, error) {
	var portfolio domain.Portfolio
	// Preload loads the related positions automatically (Like Eager Loading in Hibernate)
	if err := r.db.Preload("Positions").Preload("Positions.Instrument").First(&portfolio, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Debug("Portfolio not found", "id", id)
			return nil, fmt.Errorf("portfolio not found: %w", err)
		}
		slog.Error("Failed to find portfolio", "id", id, "error", err)
		return nil, err
	}
	return &portfolio, nil
}

func (r *GormRepository) FindAll() ([]*domain.Portfolio, error) {
	var portfolios []*domain.Portfolio
	if err := r.db.Preload("Positions").Preload("Positions.Instrument").Find(&portfolios).Error; err != nil {
		return nil, err
	}
	return portfolios, nil
}

func (r *GormRepository) Delete(id string) error {
	if err := r.db.Delete(&domain.Portfolio{}, "id = ?", id).Error; err != nil {
		return err
	}
	return nil
}
