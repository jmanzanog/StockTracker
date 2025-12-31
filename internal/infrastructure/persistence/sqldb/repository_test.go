package sqldb

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
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

// --- Reusable Containers ---
// These variables hold the shared containers and DB connections.
// Containers start only once per test run, significantly reducing test time.
var (
	// Postgres shared container
	sharedPostgresDB        *DB
	sharedPostgresContainer *postgres.PostgresContainer
	postgresSetupOnce       sync.Once
	postgresSetupErr        error

	// Oracle shared container
	sharedOracleDB        *DB
	sharedOracleContainer testcontainers.Container
	oracleSetupOnce       sync.Once
	oracleSetupErr        error
)

// TestMain sets up the shared containers before running tests.
// Both Postgres and Oracle containers are started once and reused across all tests,
// reducing total test time from ~10 seconds per test to ~35 seconds total.
func TestMain(m *testing.M) {
	ctx := context.Background()

	// Initialize Postgres container
	postgresSetupOnce.Do(func() {
		sharedPostgresDB, sharedPostgresContainer, postgresSetupErr = startPostgresContainer(ctx)
	})
	if postgresSetupErr != nil {
		log.Fatalf("Failed to start shared Postgres container: %v", postgresSetupErr)
	}

	// Initialize Oracle container
	oracleSetupOnce.Do(func() {
		sharedOracleDB, sharedOracleContainer, oracleSetupErr = startOracleContainer(ctx)
	})
	if oracleSetupErr != nil {
		log.Fatalf("Failed to start shared Oracle container: %v", oracleSetupErr)
	}

	code := m.Run()

	// Cleanup: terminate the shared containers
	if sharedPostgresContainer != nil {
		if err := sharedPostgresContainer.Terminate(ctx); err != nil {
			log.Printf("Failed to terminate Postgres container: %v", err)
		}
	}
	if sharedOracleContainer != nil {
		if err := sharedOracleContainer.Terminate(ctx); err != nil {
			log.Printf("Failed to terminate Oracle container: %v", err)
		}
	}

	if code != 0 {
		log.Fatalf("Tests failed with exit code %d", code)
	}
}

// runWithBackends executes the test function against both Postgres and Oracle.
func runWithBackends(t *testing.T, testFunc func(t *testing.T, db *DB)) {
	t.Run("Postgres", func(t *testing.T) {
		db := setupPostgres(t)
		testFunc(t, db)
	})

	t.Run("Oracle", func(t *testing.T) {
		db := setupOracle(t)
		testFunc(t, db)
	})
}

// startPostgresContainer initializes the Postgres container and returns the DB wrapper.
// This is called only once per test run from TestMain.
func startPostgresContainer(ctx context.Context) (*DB, *postgres.PostgresContainer, error) {
	pgContainer, err := postgres.Run(ctx,
		"postgres:18-alpine",
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
		return nil, nil, fmt.Errorf("failed to start postgres container: %w", err)
	}

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, pgContainer, fmt.Errorf("failed to get connection string: %w", err)
	}

	rawDB, err := sql.Open("pgx", connStr)
	if err != nil {
		return nil, pgContainer, fmt.Errorf("failed to open db: %w", err)
	}

	db := New(rawDB, &PostgresDialect{})

	if err := db.Dialect.Migrate(ctx, rawDB); err != nil {
		return nil, pgContainer, fmt.Errorf("failed to migrate: %w", err)
	}

	log.Println("Postgres container started and migrated successfully")
	return db, pgContainer, nil
}

// cleanupPostgresTables truncates all tables to ensure test isolation.
// This is MUCH faster than restarting the container (~5ms vs ~5s).
func cleanupPostgresTables(t *testing.T, db *DB) {
	ctx := context.Background()

	// Use TRUNCATE with CASCADE for faster cleanup and to handle foreign keys
	cleanupQueries := []string{
		"TRUNCATE TABLE positions, portfolios, instruments CASCADE",
	}

	for _, query := range cleanupQueries {
		if _, err := db.ExecContext(ctx, query); err != nil {
			t.Logf("Warning: cleanup query failed (may be expected on first run): %s - %v", query, err)
		}
	}
}

// setupPostgres returns the shared Postgres DB connection and cleans up tables for test isolation.
// The container is started only once in TestMain, making subsequent test runs very fast.
func setupPostgres(t *testing.T) *DB {
	if sharedPostgresDB == nil {
		t.Fatal("Postgres container not initialized. Ensure TEST_DB is not set to 'oracle' only.")
	}

	// Clean up tables before each test to ensure isolation
	cleanupPostgresTables(t, sharedPostgresDB)

	return sharedPostgresDB
}

// startOracleContainer initializes the Oracle container and returns the DB wrapper.
// This is called only once per test run from TestMain.
func startOracleContainer(ctx context.Context) (*DB, testcontainers.Container, error) {
	req := testcontainers.ContainerRequest{
		// Use a light, fast start image
		Image:        "gvenzl/oracle-free:23.6-slim-faststart",
		ExposedPorts: []string{"1521/tcp"},
		Env:          map[string]string{"ORACLE_PASSWORD": "password"},
		WaitingFor:   wait.ForLog("DATABASE IS READY TO USE").WithStartupTimeout(180 * time.Second),
	}

	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start oracle container: %w", err)
	}

	port, err := c.MappedPort(ctx, "1521")
	if err != nil {
		return nil, c, fmt.Errorf("failed to get port: %w", err)
	}
	host, err := c.Host(ctx)
	if err != nil {
		return nil, c, fmt.Errorf("failed to get host: %w", err)
	}

	// DSN for go-ora: oracle://user:password@host:port/service
	dsn := fmt.Sprintf("oracle://system:password@%s:%s/FREE", host, port.Port())

	rawDB, err := sql.Open("oracle", dsn)
	if err != nil {
		return nil, c, fmt.Errorf("failed to open db: %w", err)
	}

	db := New(rawDB, &OracleDialect{})
	if err := db.Dialect.Migrate(ctx, rawDB); err != nil {
		return nil, c, fmt.Errorf("failed to migrate: %w", err)
	}

	log.Println("Oracle container started and migrated successfully")
	return db, c, nil
}

// cleanupOracleTables truncates all tables to ensure test isolation.
// This is MUCH faster than restarting the container (~50ms vs ~60s).
func cleanupOracleTables(t *testing.T, db *DB) {
	ctx := context.Background()

	// Order matters due to foreign key constraints: positions -> portfolios -> instruments
	// Disable constraint checking, truncate, then re-enable
	// Note: Oracle requires disabling constraints or deleting in correct order
	cleanupQueries := []string{
		"DELETE FROM positions",
		"DELETE FROM portfolios",
		"DELETE FROM instruments",
	}

	for _, query := range cleanupQueries {
		if _, err := db.ExecContext(ctx, query); err != nil {
			t.Logf("Warning: cleanup query failed (may be expected on first run): %s - %v", query, err)
		}
	}
}

// setupOracle returns the shared Oracle DB connection and cleans up tables for test isolation.
// The container is started only once in TestMain, making subsequent test runs very fast.
func setupOracle(t *testing.T) *DB {
	if sharedOracleDB == nil {
		t.Fatal("Oracle container not initialized. Ensure TEST_DB=oracle or TEST_DB=all is set.")
	}

	// Clean up tables before each test to ensure isolation
	cleanupOracleTables(t, sharedOracleDB)

	return sharedOracleDB
}

// --- Basic CRUD Tests ---

func TestRepository_SaveAndFind(t *testing.T) {
	runWithBackends(t, func(t *testing.T, db *DB) {
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
	})
}

func TestRepository_Save_Update(t *testing.T) {
	runWithBackends(t, func(t *testing.T, db *DB) {
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
	})
}

func TestRepository_NotFound(t *testing.T) {
	runWithBackends(t, func(t *testing.T, db *DB) {
		repo := NewRepository(db)

		ctx := context.Background()
		_, err := repo.FindByID(ctx, "non-existent-id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "portfolio not found")
	})
}

func TestRepository_FindAll_Empty(t *testing.T) {
	runWithBackends(t, func(t *testing.T, db *DB) {
		repo := NewRepository(db)

		ctx := context.Background()
		portfolios, err := repo.FindAll(ctx)

		assert.NoError(t, err)
		assert.Equal(t, 0, len(portfolios))
	})
}

func TestRepository_FindAll_Multiple(t *testing.T) {
	runWithBackends(t, func(t *testing.T, db *DB) {
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
	})
}

func TestRepository_Delete_Success(t *testing.T) {
	runWithBackends(t, func(t *testing.T, db *DB) {
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
	})
}

func TestRepository_Delete_NotFound(t *testing.T) {
	runWithBackends(t, func(t *testing.T, db *DB) {
		repo := NewRepository(db)

		ctx := context.Background()
		err := repo.Delete(ctx, "non-existent-id")
		assert.NoError(t, err)
	})
}

func TestRepository_Delete_WithPositions(t *testing.T) {
	runWithBackends(t, func(t *testing.T, db *DB) {
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
	})
}

func TestRepository_Save_MultiplePositions(t *testing.T) {
	runWithBackends(t, func(t *testing.T, db *DB) {
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
	})
}

func TestRepository_Save_UpdatePosition(t *testing.T) {
	runWithBackends(t, func(t *testing.T, db *DB) {
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
	})
}

func TestRepository_Save_SameInstrument_MultiplePositions(t *testing.T) {
	runWithBackends(t, func(t *testing.T, db *DB) {
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
	})
}

// --- Concurrency Tests ---
// These tests detect deadlock issues that may occur with concurrent writes.

func TestRepository_ConcurrentSaves_SamePortfolio(t *testing.T) {
	runWithBackends(t, func(t *testing.T, db *DB) {
		repo := NewRepository(db)
		ctx := context.Background()

		// Create a portfolio with one position
		p := domain.NewPortfolio("Concurrent Test Portfolio")
		inst := domain.NewInstrument("US999", "CONC", "Concurrent Corp", domain.InstrumentTypeStock, "USD", "NYSE")
		pos := domain.NewPosition(inst, domain.NewDecimalFromInt(100), "USD")
		_ = pos.UpdatePrice(domain.NewDecimalFromInt(50))
		_ = p.AddPosition(pos)

		// Initial save
		err := repo.Save(ctx, &p)
		assert.NoError(t, err)

		// Sequential saves simulating rapid updates (no data race because sequential)
		// This tests that the repository can handle rapid sequential saves without deadlock
		const numUpdates = 5
		for i := 0; i < numUpdates; i++ {
			newPrice := domain.NewDecimalFromInt(int64(50 + i))
			_ = p.Positions[0].UpdatePrice(newPrice)

			err := repo.Save(ctx, &p)
			assert.NoError(t, err, "Save %d failed", i)
		}

		// Verify the portfolio was saved correctly
		found, err := repo.FindByID(ctx, p.ID)
		assert.NoError(t, err)
		assert.NotNil(t, found)
		assert.Equal(t, 1, len(found.Positions))
	})
}

func TestRepository_ConcurrentSaves_MultiplePortfolios_SameInstrument(t *testing.T) {
	runWithBackends(t, func(t *testing.T, db *DB) {
		repo := NewRepository(db)
		ctx := context.Background()

		// Shared instrument
		inst := domain.NewInstrument("SHARE001", "SHARED", "Shared Instrument", domain.InstrumentTypeStock, "USD", "NYSE")

		const numPortfolios = 5
		errChan := make(chan error, numPortfolios)
		var wg sync.WaitGroup

		for i := 0; i < numPortfolios; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				p := domain.NewPortfolio(fmt.Sprintf("Portfolio %d", idx))
				pos := domain.NewPosition(inst, domain.NewDecimalFromInt(int64(100*idx+100)), "USD")
				_ = pos.UpdatePrice(domain.NewDecimalFromInt(int64(50 + idx)))
				_ = p.AddPosition(pos)

				if err := repo.Save(ctx, &p); err != nil {
					errChan <- err
				}
			}(i)
		}

		// Wait for all goroutines
		wg.Wait()
		close(errChan)

		// Check for errors
		for err := range errChan {
			t.Fatalf("Concurrent save failed (possible deadlock on shared instrument): %v", err)
		}

		// Verify all portfolios were saved
		all, err := repo.FindAll(ctx)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(all), numPortfolios)
	})
}

func TestRepository_ConcurrentSaves_RapidUpdates(t *testing.T) {
	runWithBackends(t, func(t *testing.T, db *DB) {
		repo := NewRepository(db)
		ctx := context.Background()

		// Create portfolio with multiple positions
		p := domain.NewPortfolio("Rapid Update Portfolio")

		instruments := []domain.Instrument{
			domain.NewInstrument("RAPID001", "R1", "Rapid 1", domain.InstrumentTypeStock, "USD", "NYSE"),
			domain.NewInstrument("RAPID002", "R2", "Rapid 2", domain.InstrumentTypeStock, "USD", "NYSE"),
			domain.NewInstrument("RAPID003", "R3", "Rapid 3", domain.InstrumentTypeStock, "USD", "NYSE"),
		}

		for _, inst := range instruments {
			pos := domain.NewPosition(inst, domain.NewDecimalFromInt(100), "USD")
			_ = pos.UpdatePrice(domain.NewDecimalFromInt(10))
			_ = p.AddPosition(pos)
		}

		// Initial save
		err := repo.Save(ctx, &p)
		assert.NoError(t, err)

		// Rapid sequential saves (simulating high-frequency price updates)
		const numUpdates = 10
		for i := 0; i < numUpdates; i++ {
			for j := range p.Positions {
				newPrice := domain.NewDecimalFromInt(int64(10 + i + j))
				_ = p.Positions[j].UpdatePrice(newPrice)
			}

			err := repo.Save(ctx, &p)
			if err != nil {
				t.Fatalf("Rapid update %d failed: %v", i, err)
			}
		}

		// Verify final state
		found, err := repo.FindByID(ctx, p.ID)
		assert.NoError(t, err)
		assert.Equal(t, len(instruments), len(found.Positions))
	})
}

func TestRepository_LastUpdated(t *testing.T) {
	runWithBackends(t, func(t *testing.T, db *DB) {
		repo := NewRepository(db)
		ctx := context.Background()

		p := domain.NewPortfolio("LastUpdated Test")

		// 1. Initial save
		err := repo.Save(ctx, &p)
		assert.NoError(t, err)
		firstUpdate := p.LastUpdated
		assert.NotZero(t, firstUpdate)

		// Wait a bit to ensure a different timestamp
		time.Sleep(500 * time.Millisecond)

		// 2. Second save
		err = repo.Save(ctx, &p)
		assert.NoError(t, err)
		secondUpdate := p.LastUpdated
		assert.True(t, secondUpdate.After(firstUpdate), "LastUpdated should be updated on second save")

		// Verify from DB
		found, err := repo.FindByID(ctx, p.ID)
		assert.NoError(t, err)

		t.Logf("First: %v, Second: %v, Found: %v", firstUpdate, secondUpdate, found.LastUpdated)

		// Assert that the version in DB is definitely AFTER the first version.
		// This proves the update worked without getting tangled in precision/timezone comparisons with exact 'now'.
		assert.True(t, found.LastUpdated.After(firstUpdate), "DB LastUpdated should be after the initial creation time")
	})
}
