package sqldb

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmanzanog/stock-tracker/internal/domain"
	_ "github.com/sijms/go-ora/v2"

	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupTestDB(t *testing.T) *DB {
	dbType := os.Getenv("TEST_DB")
	if dbType == "oracle" {
		return setupOracle(t)
	}
	return setupPostgres(t)
}

func setupPostgres(t *testing.T) *DB {
	ctx := context.Background()
	pgContainer, err := postgres.Run(ctx,
		"postgres:17-alpine",
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

	// Print logs if failed
	// t.Cleanup(...)
	t.Cleanup(func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %s", err)
		}
	})

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %s", err)
	}

	rawDB, err := sql.Open("pgx", connStr)
	if err != nil {
		t.Fatalf("failed to open db: %s", err)
	}

	db := New(rawDB, &PostgresDialect{})

	if err := db.Dialect.Migrate(ctx, rawDB); err != nil {
		t.Fatalf("failed to migrate: %s", err)
	}

	return db
}

func setupOracle(t *testing.T) *DB {
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		// Use a light, fast start image
		Image:        "gvenzl/oracle-free:23.3-slim-faststart",
		ExposedPorts: []string{"1521/tcp"},
		Env:          map[string]string{"ORACLE_PASSWORD": "password"},
		WaitingFor:   wait.ForLog("DATABASE IS READY TO USE").WithStartupTimeout(120 * time.Second),
	}

	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start oracle container: %s", err)
	}
	t.Cleanup(func() {
		c.Terminate(ctx)
	})

	port, err := c.MappedPort(ctx, "1521")
	if err != nil {
		t.Fatalf("failed to get port: %v", err)
	}
	host, err := c.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get host: %v", err)
	}

	// DSN for go-ora: oracle://user:password@host:port/service
	dsn := fmt.Sprintf("oracle://system:password@%s:%s/FREE", host, port.Port())

	rawDB, err := sql.Open("oracle", dsn)
	if err != nil {
		t.Fatalf("failed to open db: %s", err)
	}

	db := New(rawDB, &OracleDialect{})
	if err := db.Dialect.Migrate(ctx, rawDB); err != nil {
		t.Fatalf("failed to migrate: %s", err)
	}

	return db
}

// --- Basic CRUD Tests (Ported) ---

func TestRepository_SaveAndFind(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	p := domain.NewPortfolio("My Test Portfolio")
	ctx := context.Background()
	err := repo.Save(ctx, &p)
	assert.NoError(t, err)

	inst := domain.NewInstrument("US123", "TEST", "Test Corp", domain.InstrumentTypeStock, "USD", "NYSE")
	pos := domain.NewPosition(inst, domain.NewDecimalFromInt(100), "USD")
	err = pos.UpdatePrice(domain.NewDecimalFromInt(10))
	assert.NoError(t, err)

	err = p.AddPosition(pos)
	assert.NoError(t, err)

	err = repo.Save(ctx, &p)
	assert.NoError(t, err)

	found, err := repo.FindByID(ctx, p.ID)
	assert.NoError(t, err)
	assert.NotNil(t, found)
	assert.Equal(t, p.ID, found.ID)
	assert.Equal(t, 1, len(found.Positions))
	assert.Equal(t, "US123", found.Positions[0].Instrument.ISIN)
}

func TestRepository_Save_Update(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	p := domain.NewPortfolio("Updates")
	ctx := context.Background()
	err := repo.Save(ctx, &p)
	assert.NoError(t, err)

	p.Name = "Updated Name"
	p.LastUpdated = time.Now()

	err = repo.Save(ctx, &p)
	assert.NoError(t, err)

	found, err := repo.FindByID(ctx, p.ID)
	assert.NoError(t, err)
	assert.Equal(t, "Updated Name", found.Name)
}

func TestRepository_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	ctx := context.Background()
	_, err := repo.FindByID(ctx, "non-existent-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "portfolio not found")
}

func TestRepository_FindAll_Empty(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	ctx := context.Background()
	portfolios, err := repo.FindAll(ctx)

	assert.NoError(t, err)
	// assert.NotNil(t, portfolios) // Empty slice might be nil or empty, depends on impl
	assert.Equal(t, 0, len(portfolios))
}

func TestRepository_FindAll_Multiple(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	ctx := context.Background()

	p1 := domain.NewPortfolio("Portfolio 1")
	p2 := domain.NewPortfolio("Portfolio 2")
	p3 := domain.NewPortfolio("Portfolio 3")

	_ = repo.Save(ctx, &p1)
	_ = repo.Save(ctx, &p2)
	_ = repo.Save(ctx, &p3)

	portfolios, err := repo.FindAll(ctx)

	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(portfolios), 3)
}

func TestRepository_Delete_Success(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	ctx := context.Background()

	p := domain.NewPortfolio("To Delete")
	err := repo.Save(ctx, &p)
	assert.NoError(t, err)

	err = repo.Delete(ctx, p.ID)
	assert.NoError(t, err)

	_, err = repo.FindByID(ctx, p.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "portfolio not found")
}

func TestRepository_Delete_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	ctx := context.Background()
	err := repo.Delete(ctx, "non-existent-id")
	assert.NoError(t, err)
}

func TestRepository_Delete_WithPositions(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	ctx := context.Background()

	p := domain.NewPortfolio("Portfolio with Positions")
	inst := domain.NewInstrument("US001", "AAPL", "Apple", domain.InstrumentTypeStock, "USD", "NASDAQ")
	pos := domain.NewPosition(inst, domain.NewDecimalFromInt(1000), "USD")
	_ = pos.UpdatePrice(domain.NewDecimalFromInt(150))
	_ = p.AddPosition(pos)

	err := repo.Save(ctx, &p)
	assert.NoError(t, err)

	err = repo.Delete(ctx, p.ID)
	assert.NoError(t, err)

	_, err = repo.FindByID(ctx, p.ID)
	assert.Error(t, err)
}

func TestRepository_Save_MultiplePositions(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	ctx := context.Background()

	p := domain.NewPortfolio("Multi Position Portfolio")

	inst1 := domain.NewInstrument("US001", "AAPL", "Apple", domain.InstrumentTypeStock, "USD", "NASDAQ")
	pos1 := domain.NewPosition(inst1, domain.NewDecimalFromInt(1000), "USD")
	_ = pos1.UpdatePrice(domain.NewDecimalFromInt(150))
	_ = p.AddPosition(pos1)

	inst2 := domain.NewInstrument("US002", "GOOGL", "Google", domain.InstrumentTypeStock, "USD", "NASDAQ")
	pos2 := domain.NewPosition(inst2, domain.NewDecimalFromInt(2000), "USD")
	_ = pos2.UpdatePrice(domain.NewDecimalFromInt(2800))
	_ = p.AddPosition(pos2)

	err := repo.Save(ctx, &p)
	assert.NoError(t, err)

	found, err := repo.FindByID(ctx, p.ID)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(found.Positions))
}

func TestRepository_Save_UpdatePosition(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	ctx := context.Background()

	p := domain.NewPortfolio("Portfolio")

	inst := domain.NewInstrument("US001", "AAPL", "Apple", domain.InstrumentTypeStock, "USD", "NASDAQ")
	pos := domain.NewPosition(inst, domain.NewDecimalFromInt(1000), "USD")
	_ = pos.UpdatePrice(domain.NewDecimalFromInt(150))
	_ = p.AddPosition(pos)

	err := repo.Save(ctx, &p)
	assert.NoError(t, err)

	err = p.UpdatePositionPrice(pos.ID, domain.NewDecimalFromInt(200))
	assert.NoError(t, err)

	err = repo.Save(ctx, &p)
	assert.NoError(t, err)

	found, err := repo.FindByID(ctx, p.ID)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(found.Positions))

	expectedPrice := domain.NewDecimalFromInt(200)
	assert.True(t, found.Positions[0].CurrentPrice.Equal(expectedPrice))
}

func TestRepository_Save_SameInstrument_MultiplePositions(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	ctx := context.Background()

	inst := domain.NewInstrument("US001", "AAPL", "Apple", domain.InstrumentTypeStock, "USD", "NASDAQ")

	p1 := domain.NewPortfolio("Portfolio 1")
	pos1 := domain.NewPosition(inst, domain.NewDecimalFromInt(1000), "USD")
	_ = pos1.UpdatePrice(domain.NewDecimalFromInt(150))
	_ = p1.AddPosition(pos1)

	p2 := domain.NewPortfolio("Portfolio 2")
	pos2 := domain.NewPosition(inst, domain.NewDecimalFromInt(2000), "USD")
	_ = pos2.UpdatePrice(domain.NewDecimalFromInt(150))
	_ = p2.AddPosition(pos2)

	_ = repo.Save(ctx, &p1)
	_ = repo.Save(ctx, &p2)

	found1, _ := repo.FindByID(ctx, p1.ID)
	found2, _ := repo.FindByID(ctx, p2.ID)

	assert.Equal(t, found1.Positions[0].Instrument.ISIN, found2.Positions[0].Instrument.ISIN)
}
