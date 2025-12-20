package application_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmanzanog/stock-tracker/internal/application"
	"github.com/jmanzanog/stock-tracker/internal/domain"
	"github.com/jmanzanog/stock-tracker/internal/infrastructure/marketdata"
	"github.com/jmanzanog/stock-tracker/internal/infrastructure/persistence/sqldb"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// setupIntegrationDB initiates a real Postgres container and returns a configured DB connection and a cleanup function.
// This duplicates setup logic from infrastructure tests to ensure application tests are self-contained.
func setupIntegrationDB(t *testing.T) (*sql.DB, func()) {
	ctx := context.Background()

	// Start Postgres Container
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
		t.Fatalf("failed to start postgres container: %s", err)
	}

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = pgContainer.Terminate(ctx)
		t.Fatalf("failed to get connection string: %s", err)
	}

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		_ = pgContainer.Terminate(ctx)
		t.Fatalf("failed to open db: %s", err)
	}

	// Initialize SQLDB Wrapper and Run Migrations
	sqlDB := sqldb.New(db, &sqldb.PostgresDialect{})
	if err := sqlDB.Dialect.Migrate(ctx, db); err != nil {
		_ = db.Close()
		_ = pgContainer.Terminate(ctx)
		t.Fatalf("failed to migrate: %s", err)
	}

	return db, func() {
		_ = db.Close()
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %s", err)
		}
	}
}

// Mock implementation of MarketData for integration test
type MockIntegrationMarketData struct{}

func (m *MockIntegrationMarketData) SearchByISIN(_ context.Context, _ string) (*domain.Instrument, error) {
	return nil, nil
}
func (m *MockIntegrationMarketData) GetQuote(_ context.Context, _ string) (*marketdata.QuoteResult, error) {
	return nil, nil
}

func TestPortfolioService_Persistence_ReusesDefaultPortfolio(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// 1. Setup Infra (Real DB)
	rawDB, cleanup := setupIntegrationDB(t)
	defer cleanup()

	dbWrapper := sqldb.New(rawDB, &sqldb.PostgresDialect{})
	repo := sqldb.NewRepository(dbWrapper)
	mockMD := &MockIntegrationMarketData{}

	// 2. First Service Initialization (Cold Start)
	service1, err := application.NewPortfolioService(repo, mockMD)
	assert.NoError(t, err)

	p1, err := service1.GetPortfolioSummary(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, p1)
	assert.Equal(t, "default", p1.Name)
	id1 := p1.ID
	assert.NotEmpty(t, id1)
	t.Logf("First Service Initialized. Portfolio ID: %s", id1)

	// 3. Second Service Initialization (Simulated Restart)
	// Using the same repo instance connected to the same DB container
	service2, err := application.NewPortfolioService(repo, mockMD)
	assert.NoError(t, err)

	p2, err := service2.GetPortfolioSummary(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, p2)
	id2 := p2.ID
	t.Logf("Second Service Initialized. Portfolio ID: %s", id2)

	// 4. Verify Consistency
	assert.Equal(t, id1, id2, "PortfolioService DID NOT reuse the existing 'default' portfolio. It created a new one!")
}
