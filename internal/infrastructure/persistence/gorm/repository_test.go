package persistence

import (
	"log/slog"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/jmanzanog/stock-tracker/internal/domain"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func setupTestDB() (*gorm.DB, error) {
	// Use in-memory SQLite for testing
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		// Silence logger for cleaner test output
		Logger: nil,
	})
	if err != nil {
		return nil, err
	}
	return db, nil
}

func TestGormRepository_SaveAndFind(t *testing.T) {
	// 1. Setup
	db, err := setupTestDB()
	assert.NoError(t, err)

	repo := NewGormRepository(db)
	err = repo.AutoMigrate()
	assert.NoError(t, err)

	// 2. Create Portfolio
	p := domain.NewPortfolio("My Test Portfolio")

	// Add a position
	inst := domain.NewInstrument("US123", "TEST", "Test Corp", domain.InstrumentTypeStock, "USD", "NYSE")
	pos := domain.NewPosition(inst, decimal.NewFromInt(100), "USD")
	pos.UpdatePrice(decimal.NewFromInt(10))

	err = p.AddPosition(pos)
	assert.NoError(t, err)

	// 3. Save
	err = repo.Save(&p)
	assert.NoError(t, err)

	// 4. Find
	found, err := repo.FindByID(p.ID)
	assert.NoError(t, err)
	assert.NotNil(t, found)
	assert.Equal(t, p.ID, found.ID)
	assert.Equal(t, 1, len(found.Positions))
	assert.Equal(t, "US123", found.Positions[0].Instrument.ISIN)
}

func TestGormRepository_Save_Update(t *testing.T) {
	// Validate "Upsert" logic
	db, err := setupTestDB()
	assert.NoError(t, err)
	repo := NewGormRepository(db)
	repo.AutoMigrate()
	slog.SetDefault(slog.Default()) // Ensure logger exists

	p := domain.NewPortfolio("Updates")
	repo.Save(&p)

	// Modify
	p.Name = "Updated Name"
	p.LastUpdated = time.Now()

	// Save again
	err = repo.Save(&p)
	assert.NoError(t, err)

	found, _ := repo.FindByID(p.ID)
	assert.Equal(t, "Updated Name", found.Name)
}

func TestGormRepository_NotFound(t *testing.T) {
	db, _ := setupTestDB()
	repo := NewGormRepository(db)
	repo.AutoMigrate()

	_, err := repo.FindByID("non-existent-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "portfolio not found")
}
