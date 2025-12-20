package persistence

import (
	"context"
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

func (r *GormRepository) Save(ctx context.Context, portfolio *domain.Portfolio) error {
	// Use a transaction to ensure atomicity
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Step 1: Save/Update the Portfolio first (without associations)
	portfolioCopy := *portfolio
	portfolioCopy.Positions = nil // Temporarily remove positions

	if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).
		Save(&portfolioCopy).Error; err != nil {
		tx.Rollback()
		slog.Error("Failed to save portfolio", "portfolio_id", portfolio.ID, "error", err)
		return fmt.Errorf("failed to save portfolio: %w", err)
	}

	// Step 2: Save/Update Instruments (to ensure they exist)
	for i := range portfolio.Positions {
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).
			Save(&portfolio.Positions[i].Instrument).Error; err != nil {
			tx.Rollback()
			slog.Error("Failed to save instrument", "isin", portfolio.Positions[i].Instrument.ISIN, "error", err)
			return fmt.Errorf("failed to save instrument: %w", err)
		}
	}

	// Step 3: Save/Update Positions (now that portfolio exists)
	for i := range portfolio.Positions {
		portfolio.Positions[i].PortfolioID = portfolio.ID
		if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).
			Save(&portfolio.Positions[i]).Error; err != nil {
			tx.Rollback()
			slog.Error("Failed to save position", "position_id", portfolio.Positions[i].ID, "error", err)
			return fmt.Errorf("failed to save position: %w", err)
		}
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		slog.Error("Failed to commit transaction", "portfolio_id", portfolio.ID, "error", err)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (r *GormRepository) FindByID(ctx context.Context, id string) (*domain.Portfolio, error) {
	var portfolio domain.Portfolio
	// Preload loads the related positions automatically (Like Eager Loading in Hibernate)
	if err := r.db.WithContext(ctx).Preload("Positions").Preload("Positions.Instrument").First(&portfolio, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Debug("Portfolio not found", "id", id)
			return nil, fmt.Errorf("portfolio not found: %w", err)
		}
		slog.Error("Failed to find portfolio", "id", id, "error", err)
		return nil, err
	}
	return &portfolio, nil
}

func (r *GormRepository) FindAll(ctx context.Context) ([]*domain.Portfolio, error) {
	var portfolios []*domain.Portfolio
	if err := r.db.WithContext(ctx).Preload("Positions").Preload("Positions.Instrument").Find(&portfolios).Error; err != nil {
		return nil, err
	}
	return portfolios, nil
}

func (r *GormRepository) Delete(ctx context.Context, id string) error {
	// Use a transaction to ensure atomicity
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Step 1: Delete all positions associated with this portfolio
	if err := tx.Where("portfolio_id = ?", id).Delete(&domain.Position{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete positions: %w", err)
	}

	// Step 2: Delete the portfolio
	if err := tx.Delete(&domain.Portfolio{}, "id = ?", id).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete portfolio: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
