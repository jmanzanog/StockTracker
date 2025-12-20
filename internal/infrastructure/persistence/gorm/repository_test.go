package persistence

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/jmanzanog/stock-tracker/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	postgresDriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("user"),
		postgres.WithPassword("password"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %s", err)
	}

	t.Cleanup(func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	})

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %s", err)
	}

	db, err := gorm.Open(postgresDriver.Open(connStr), &gorm.Config{
		Logger: nil,
	})
	if err != nil {
		t.Fatalf("failed to connect to database: %s", err)
	}

	return db
}

// --- Basic CRUD Tests ---

func TestGormRepository_SaveAndFind(t *testing.T) {
	// 1. Setup
	db := setupTestDB(t)

	repo := NewGormRepository(db)
	err := repo.AutoMigrate()
	assert.NoError(t, err)

	// 2. Create Portfolio
	p := domain.NewPortfolio("My Test Portfolio")
	ctx := context.Background()
	err = repo.Save(ctx, &p)
	assert.NoError(t, err)

	// Add a position
	inst := domain.NewInstrument("US123", "TEST", "Test Corp", domain.InstrumentTypeStock, "USD", "NYSE")
	pos := domain.NewPosition(inst, domain.NewDecimalFromInt(100), "USD")
	err = pos.UpdatePrice(domain.NewDecimalFromInt(10))
	if err != nil {
		t.Fatalf("UpdatePrice failed: %v", err)
	}

	err = p.AddPosition(pos)
	assert.NoError(t, err)

	// 3. Save (Update with new position)
	err = repo.Save(ctx, &p)
	assert.NoError(t, err)

	// 4. Find
	found, err := repo.FindByID(ctx, p.ID)
	assert.NoError(t, err)
	assert.NotNil(t, found)
	assert.Equal(t, p.ID, found.ID)
	assert.Equal(t, 1, len(found.Positions))
	assert.Equal(t, "US123", found.Positions[0].Instrument.ISIN)
}

func TestGormRepository_Save_Update(t *testing.T) {
	// Validate "Upsert" logic
	db := setupTestDB(t)
	repo := NewGormRepository(db)
	err := repo.AutoMigrate()
	assert.NoError(t, err)
	slog.SetDefault(slog.Default()) // Ensure logger exists

	p := domain.NewPortfolio("Updates")
	ctx := context.Background()
	err = repo.Save(ctx, &p)
	if err != nil {
		t.Fatalf("Error %v", err)
	}

	// Modify
	p.Name = "Updated Name"
	p.LastUpdated = time.Now()

	// Save again
	err = repo.Save(ctx, &p)
	assert.NoError(t, err)

	found, err := repo.FindByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}
	assert.Equal(t, "Updated Name", found.Name)
}

func TestGormRepository_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := NewGormRepository(db)
	err := repo.AutoMigrate()
	if err != nil {
		t.Fatalf("Error %v", err)
	}

	ctx := context.Background()
	_, err = repo.FindByID(ctx, "non-existent-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "portfolio not found")
}

// --- FindAll Tests ---

func TestGormRepository_FindAll_Empty(t *testing.T) {
	db := setupTestDB(t)
	repo := NewGormRepository(db)
	_ = repo.AutoMigrate()

	ctx := context.Background()
	portfolios, err := repo.FindAll(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, portfolios)
	assert.Equal(t, 0, len(portfolios))
}

func TestGormRepository_FindAll_Multiple(t *testing.T) {
	db := setupTestDB(t)
	repo := NewGormRepository(db)
	_ = repo.AutoMigrate()

	ctx := context.Background()

	// Create multiple portfolios
	p1 := domain.NewPortfolio("Portfolio 1")
	p2 := domain.NewPortfolio("Portfolio 2")
	p3 := domain.NewPortfolio("Portfolio 3")

	_ = repo.Save(ctx, &p1)
	_ = repo.Save(ctx, &p2)
	_ = repo.Save(ctx, &p3)

	// Find all
	portfolios, err := repo.FindAll(ctx)

	assert.NoError(t, err)
	assert.Equal(t, 3, len(portfolios))
}

// --- Delete Tests ---

func TestGormRepository_Delete_Success(t *testing.T) {
	db := setupTestDB(t)
	repo := NewGormRepository(db)
	_ = repo.AutoMigrate()

	ctx := context.Background()

	// Create a portfolio
	p := domain.NewPortfolio("To Delete")
	err := repo.Save(ctx, &p)
	assert.NoError(t, err)

	// Delete it
	err = repo.Delete(ctx, p.ID)
	assert.NoError(t, err)

	// Verify it's gone
	_, err = repo.FindByID(ctx, p.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "portfolio not found")
}

func TestGormRepository_Delete_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := NewGormRepository(db)
	_ = repo.AutoMigrate()

	ctx := context.Background()

	// Try to delete non-existent portfolio
	err := repo.Delete(ctx, "non-existent-id")

	// Current implementation does not return error when deleting non-existent record
	assert.NoError(t, err)
}

func TestGormRepository_Delete_WithPositions(t *testing.T) {
	db := setupTestDB(t)
	repo := NewGormRepository(db)
	_ = repo.AutoMigrate()

	ctx := context.Background()

	// Create portfolio with positions
	p := domain.NewPortfolio("Portfolio with Positions")
	inst := domain.NewInstrument("US001", "AAPL", "Apple", domain.InstrumentTypeStock, "USD", "NASDAQ")
	pos := domain.NewPosition(inst, domain.NewDecimalFromInt(1000), "USD")
	_ = pos.UpdatePrice(domain.NewDecimalFromInt(150))
	_ = p.AddPosition(pos)

	err := repo.Save(ctx, &p)
	assert.NoError(t, err)

	// Delete portfolio (should also delete positions via cascade)
	err = repo.Delete(ctx, p.ID)
	assert.NoError(t, err)

	// Verify it's gone
	_, err = repo.FindByID(ctx, p.ID)
	assert.Error(t, err)
}

// --- Save with Multiple Positions Tests ---

func TestGormRepository_Save_MultiplePositions(t *testing.T) {
	db := setupTestDB(t)
	repo := NewGormRepository(db)
	_ = repo.AutoMigrate()

	ctx := context.Background()

	p := domain.NewPortfolio("Multi Position Portfolio")

	// Add multiple positions
	inst1 := domain.NewInstrument("US001", "AAPL", "Apple", domain.InstrumentTypeStock, "USD", "NASDAQ")
	pos1 := domain.NewPosition(inst1, domain.NewDecimalFromInt(1000), "USD")
	_ = pos1.UpdatePrice(domain.NewDecimalFromInt(150))
	_ = p.AddPosition(pos1)

	inst2 := domain.NewInstrument("US002", "GOOGL", "Google", domain.InstrumentTypeStock, "USD", "NASDAQ")
	pos2 := domain.NewPosition(inst2, domain.NewDecimalFromInt(2000), "USD")
	_ = pos2.UpdatePrice(domain.NewDecimalFromInt(2800))
	_ = p.AddPosition(pos2)

	// Save
	err := repo.Save(ctx, &p)
	assert.NoError(t, err)

	// Find and verify
	found, err := repo.FindByID(ctx, p.ID)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(found.Positions))

	// Verify instruments are loaded
	assert.NotEmpty(t, found.Positions[0].Instrument.ISIN)
	assert.NotEmpty(t, found.Positions[1].Instrument.ISIN)
}

// --- Save Update Position Tests ---

func TestGormRepository_Save_UpdatePosition(t *testing.T) {
	db := setupTestDB(t)
	repo := NewGormRepository(db)
	_ = repo.AutoMigrate()

	ctx := context.Background()

	p := domain.NewPortfolio("Portfolio")

	// Add a position
	inst := domain.NewInstrument("US001", "AAPL", "Apple", domain.InstrumentTypeStock, "USD", "NASDAQ")
	pos := domain.NewPosition(inst, domain.NewDecimalFromInt(1000), "USD")
	_ = pos.UpdatePrice(domain.NewDecimalFromInt(150))
	_ = p.AddPosition(pos)

	// Save
	err := repo.Save(ctx, &p)
	assert.NoError(t, err)

	// Update price
	err = p.UpdatePositionPrice(pos.ID, domain.NewDecimalFromInt(200))
	assert.NoError(t, err)

	// Save again
	err = repo.Save(ctx, &p)
	assert.NoError(t, err)

	// Find and verify price updated
	found, err := repo.FindByID(ctx, p.ID)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(found.Positions))

	expectedPrice := domain.NewDecimalFromInt(200)
	if !found.Positions[0].CurrentPrice.Equal(expectedPrice) {
		t.Errorf("expected price %s, got %s", expectedPrice, found.Positions[0].CurrentPrice)
	}
}

// --- Instrument Deduplication Tests ---

func TestGormRepository_Save_SameInstrument_MultiplePositions(t *testing.T) {
	db := setupTestDB(t)
	repo := NewGormRepository(db)
	_ = repo.AutoMigrate()

	ctx := context.Background()

	// Create two portfolios with the same instrument
	inst := domain.NewInstrument("US001", "AAPL", "Apple", domain.InstrumentTypeStock, "USD", "NASDAQ")

	p1 := domain.NewPortfolio("Portfolio 1")
	pos1 := domain.NewPosition(inst, domain.NewDecimalFromInt(1000), "USD")
	_ = pos1.UpdatePrice(domain.NewDecimalFromInt(150))
	_ = p1.AddPosition(pos1)

	p2 := domain.NewPortfolio("Portfolio 2")
	pos2 := domain.NewPosition(inst, domain.NewDecimalFromInt(2000), "USD")
	_ = pos2.UpdatePrice(domain.NewDecimalFromInt(150))
	_ = p2.AddPosition(pos2)

	// Save both
	_ = repo.Save(ctx, &p1)
	_ = repo.Save(ctx, &p2)

	// Find both and verify same instrument (by ISIN)
	found1, _ := repo.FindByID(ctx, p1.ID)
	found2, _ := repo.FindByID(ctx, p2.ID)

	assert.Equal(t, found1.Positions[0].Instrument.ISIN, found2.Positions[0].Instrument.ISIN)
}

// --- AutoMigrate Tests ---

func TestGormRepository_AutoMigrate(t *testing.T) {
	db := setupTestDB(t)
	repo := NewGormRepository(db)

	err := repo.AutoMigrate()
	assert.NoError(t, err)

	// Verify tables exist by doing a simple query
	ctx := context.Background()
	_, err = repo.FindAll(ctx)
	assert.NoError(t, err)
}

// --- NewGormRepository Tests ---

func TestNewGormRepository(t *testing.T) {
	db := setupTestDB(t)

	repo := NewGormRepository(db)

	assert.NotNil(t, repo)
	assert.NotNil(t, repo.db)
}
