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
	pos.UpdatePrice(domain.NewDecimalFromInt(10))

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

	found, _ := repo.FindByID(ctx, p.ID)
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
