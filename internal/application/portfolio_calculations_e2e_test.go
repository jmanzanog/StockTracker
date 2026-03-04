package application_test

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmanzanog/stock-tracker/internal/application"
	"github.com/jmanzanog/stock-tracker/internal/domain"
	"github.com/jmanzanog/stock-tracker/internal/infrastructure/persistence/sqldb"
	_ "github.com/sijms/go-ora/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// =============================================================================
// Shared Container Infrastructure
// =============================================================================
// Containers are started once per test run and reused across all tests.
// This reduces test time from ~4s per test to ~0.3s per test after startup.

var (
	// Postgres shared container
	sharedPostgresDB        *sqldb.DB
	sharedPostgresContainer *postgres.PostgresContainer
	postgresSetupOnce       sync.Once
	postgresSetupErr        error

	// Oracle shared container
	sharedOracleDB        *sqldb.DB
	sharedOracleContainer testcontainers.Container
	oracleSetupOnce       sync.Once
	oracleSetupErr        error
)

// TestMain sets up shared containers before running tests.
func TestMain(m *testing.M) {
	ctx := context.Background()

	// Initialize Postgres container
	postgresSetupOnce.Do(func() {
		sharedPostgresDB, sharedPostgresContainer, postgresSetupErr = startE2EPostgresContainer(ctx)
	})
	if postgresSetupErr != nil {
		log.Fatalf("Failed to start shared Postgres container: %v", postgresSetupErr)
	}

	// Initialize Oracle container
	oracleSetupOnce.Do(func() {
		sharedOracleDB, sharedOracleContainer, oracleSetupErr = startE2EOracleContainer(ctx)
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

// startE2EPostgresContainer initializes the Postgres container.
func startE2EPostgresContainer(ctx context.Context) (*sqldb.DB, *postgres.PostgresContainer, error) {
	pgContainer, err := postgres.Run(ctx,
		"postgres:18-alpine",
		postgres.WithDatabase("e2e_testdb"),
		postgres.WithUsername("e2euser"),
		postgres.WithPassword("e2epassword"),
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

	db := sqldb.New(rawDB, &sqldb.PostgresDialect{})
	if err := db.Dialect.Migrate(ctx, rawDB); err != nil {
		return nil, pgContainer, fmt.Errorf("failed to migrate: %w", err)
	}

	log.Println("E2E Postgres container started and migrated successfully")
	return db, pgContainer, nil
}

// startE2EOracleContainer initializes the Oracle container.
func startE2EOracleContainer(ctx context.Context) (*sqldb.DB, testcontainers.Container, error) {
	req := testcontainers.ContainerRequest{
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

	dsn := fmt.Sprintf("oracle://system:password@%s:%s/FREE", host, port.Port())

	rawDB, err := sql.Open("oracle", dsn)
	if err != nil {
		return nil, c, fmt.Errorf("failed to open db: %w", err)
	}

	db := sqldb.New(rawDB, &sqldb.OracleDialect{})
	if err := db.Dialect.Migrate(ctx, rawDB); err != nil {
		return nil, c, fmt.Errorf("failed to migrate: %w", err)
	}

	log.Println("E2E Oracle container started and migrated successfully")
	return db, c, nil
}

// cleanupPostgres truncates all tables for test isolation.
func cleanupPostgres(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	cleanupQueries := []string{
		"TRUNCATE TABLE positions, portfolios, instruments CASCADE",
	}
	for _, query := range cleanupQueries {
		if _, err := sharedPostgresDB.ExecContext(ctx, query); err != nil {
			t.Logf("Warning: cleanup query failed: %s - %v", query, err)
		}
	}
}

// cleanupOracle deletes all data for test isolation.
func cleanupOracle(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	cleanupQueries := []string{
		"DELETE FROM positions",
		"DELETE FROM portfolios",
		"DELETE FROM instruments",
	}
	for _, query := range cleanupQueries {
		if _, err := sharedOracleDB.ExecContext(ctx, query); err != nil {
			t.Logf("Warning: cleanup query failed: %s - %v", query, err)
		}
	}
}

// runE2EWithBackends runs the test function against both Postgres and Oracle.
func runE2EWithBackends(t *testing.T, testFunc func(t *testing.T, db *sqldb.DB)) {
	t.Run("Postgres", func(t *testing.T) {
		if sharedPostgresDB == nil {
			t.Fatal("Postgres container not initialized")
		}
		cleanupPostgres(t)
		testFunc(t, sharedPostgresDB)
	})

	t.Run("Oracle", func(t *testing.T) {
		if sharedOracleDB == nil {
			t.Fatal("Oracle container not initialized")
		}
		cleanupOracle(t)
		testFunc(t, sharedOracleDB)
	})
}

// =============================================================================
// Configurable Mock Market Data Provider
// =============================================================================

type instrumentConfig struct {
	isin     string
	symbol   string
	name     string
	currency string
}

type MockMarketDataProvider struct {
	mu          sync.RWMutex
	instruments map[string]instrumentConfig
	prices      map[string]string
}

func NewMockMarketDataProvider() *MockMarketDataProvider {
	return &MockMarketDataProvider{
		instruments: make(map[string]instrumentConfig),
		prices:      make(map[string]string),
	}
}

func (m *MockMarketDataProvider) RegisterInstrument(isin, symbol, name, currency string, initialPrice string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.instruments[isin] = instrumentConfig{isin: isin, symbol: symbol, name: name, currency: currency}
	m.prices[symbol] = initialPrice
}

func (m *MockMarketDataProvider) UpdatePrice(symbol, newPrice string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.prices[symbol] = newPrice
}

func (m *MockMarketDataProvider) SearchByISIN(_ context.Context, isin string) (*domain.Instrument, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg, ok := m.instruments[isin]
	if !ok {
		return nil, domain.ErrPositionNotFound
	}
	inst := domain.NewInstrument(cfg.isin, cfg.symbol, cfg.name, domain.InstrumentTypeETF, cfg.currency, "XETRA")
	return &inst, nil
}

func (m *MockMarketDataProvider) GetQuote(_ context.Context, symbol string) (*domain.QuoteResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	priceStr, ok := m.prices[symbol]
	if !ok {
		return nil, domain.ErrPositionNotFound
	}
	price, err := domain.NewDecimalFromString(priceStr)
	if err != nil {
		return nil, err
	}
	return &domain.QuoteResult{Symbol: symbol, Price: price, Currency: "EUR", Time: time.Now().Format(time.RFC3339)}, nil
}

// MockBatchMarketDataProvider extends the mock to support batch operations.
type MockBatchMarketDataProvider struct {
	*MockMarketDataProvider
}

func NewMockBatchMarketDataProvider() *MockBatchMarketDataProvider {
	return &MockBatchMarketDataProvider{MockMarketDataProvider: NewMockMarketDataProvider()}
}

func (m *MockBatchMarketDataProvider) SearchByISINBatch(ctx context.Context, isins []string) []domain.SearchResult {
	results := make([]domain.SearchResult, 0, len(isins))
	for _, isin := range isins {
		inst, err := m.SearchByISIN(ctx, isin)
		results = append(results, domain.SearchResult{ISIN: isin, Instrument: inst, Error: err})
	}
	return results
}

func (m *MockBatchMarketDataProvider) GetQuoteBatch(ctx context.Context, symbols []string) []domain.QuoteBatchResult {
	results := make([]domain.QuoteBatchResult, 0, len(symbols))
	for _, symbol := range symbols {
		quote, err := m.GetQuote(ctx, symbol)
		results = append(results, domain.QuoteBatchResult{Symbol: symbol, Quote: quote, Error: err})
	}
	return results
}

// =============================================================================
// Helper Functions
// =============================================================================

func decimalString(t *testing.T, value string) domain.Decimal {
	t.Helper()
	d, err := domain.NewDecimalFromString(value)
	require.NoError(t, err, "failed to parse decimal: %s", value)
	return d
}

func assertDecimalEqual(t *testing.T, expected, actual domain.Decimal, msg string) {
	t.Helper()
	expectedRounded, err := expected.Round(10)
	require.NoError(t, err, "failed to round expected")
	actualRounded, err := actual.Round(10)
	require.NoError(t, err, "failed to round actual")
	if !expectedRounded.Equal(actualRounded) {
		t.Errorf("%s: expected %s, got %s", msg, expectedRounded.String(), actualRounded.String())
	}
}

// =============================================================================
// E2E Test: Portfolio Calculations with Simulated Market Movements
// =============================================================================

func TestPortfolioCalculations_E2E_WithSimulatedMarketMovements(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	runE2EWithBackends(t, func(t *testing.T, db *sqldb.DB) {
		ctx := context.Background()
		repo := sqldb.NewRepository(db)
		mockMarket := NewMockMarketDataProvider()

		mockMarket.RegisterInstrument("IE00B4L5Y983", "IWDA", "iShares Core MSCI World", "EUR", "85.50")
		mockMarket.RegisterInstrument("IE00B5BMR087", "CSP5", "iShares Core S&P 500", "EUR", "520.75")
		mockMarket.RegisterInstrument("IE00BK5BQT80", "VWCE", "Vanguard FTSE All-World", "EUR", "112.25")

		service, err := application.NewPortfolioService(repo, mockMarket)
		require.NoError(t, err)

		// Add positions
		pos1, err := service.AddPosition(ctx, "IE00B4L5Y983", decimalString(t, "5000"), "EUR")
		require.NoError(t, err)
		pos2, err := service.AddPosition(ctx, "IE00B5BMR087", decimalString(t, "3000"), "EUR")
		require.NoError(t, err)
		pos3, err := service.AddPosition(ctx, "IE00BK5BQT80", decimalString(t, "2000"), "EUR")
		require.NoError(t, err)

		// Validate initial state
		portfolio, err := service.GetPortfolioSummary(ctx)
		require.NoError(t, err)
		require.Len(t, portfolio.Positions, 3)

		totalInvested, _ := portfolio.TotalInvested()
		assertDecimalEqual(t, decimalString(t, "10000"), totalInvested, "Initial TotalInvested")
		totalValue, _ := portfolio.TotalValue()
		assertDecimalEqual(t, decimalString(t, "10000"), totalValue, "Initial TotalValue")

		// Simulate BULLISH Market (+10%)
		mockMarket.UpdatePrice("IWDA", "94.05")
		mockMarket.UpdatePrice("CSP5", "572.825")
		mockMarket.UpdatePrice("VWCE", "123.475")
		require.NoError(t, service.RefreshPrices(ctx))

		portfolio, _ = service.GetPortfolioSummary(ctx)
		totalValue, _ = portfolio.TotalValue()
		assertDecimalEqual(t, decimalString(t, "11000"), totalValue, "Value after +10%")

		iwdaPos, _ := portfolio.GetPosition(pos1.ID)
		iwdaValue, _ := iwdaPos.CurrentValue()
		assertDecimalEqual(t, decimalString(t, "5500"), iwdaValue, "IWDA CurrentValue")

		// Simulate BEARISH Market (-12% net from initial)
		mockMarket.UpdatePrice("IWDA", "75.24")
		mockMarket.UpdatePrice("CSP5", "458.26")
		mockMarket.UpdatePrice("VWCE", "98.78")
		require.NoError(t, service.RefreshPrices(ctx))

		portfolio, _ = service.GetPortfolioSummary(ctx)
		totalValue, _ = portfolio.TotalValue()
		assertDecimalEqual(t, decimalString(t, "8800"), totalValue, "Value after -12% net")

		profitLoss, _ := portfolio.TotalProfitLoss()
		assertDecimalEqual(t, decimalString(t, "-1200"), profitLoss, "P/L after -12%")

		// Simulate MIXED Market
		mockMarket.UpdatePrice("IWDA", "128.25")   // +50%
		mockMarket.UpdatePrice("CSP5", "364.525")  // -30%
		mockMarket.UpdatePrice("VWCE", "117.8625") // +5%
		require.NoError(t, service.RefreshPrices(ctx))

		portfolio, _ = service.GetPortfolioSummary(ctx)
		totalValue, _ = portfolio.TotalValue()
		assertDecimalEqual(t, decimalString(t, "11700"), totalValue, "Value after mixed")

		csp5Pos, _ := portfolio.GetPosition(pos2.ID)
		csp5Value, _ := csp5Pos.CurrentValue()
		assertDecimalEqual(t, decimalString(t, "2100"), csp5Value, "CSP5 CurrentValue (-30%)")

		vwcePos, _ := portfolio.GetPosition(pos3.ID)
		vwceValue, _ := vwcePos.CurrentValue()
		assertDecimalEqual(t, decimalString(t, "2100"), vwceValue, "VWCE CurrentValue (+5%)")

		t.Log("=== Market Movements Test: Passed! ===")
	})
}

// =============================================================================
// E2E Test: High Precision Decimal Calculations
// =============================================================================

func TestPortfolioCalculations_E2E_HighPrecisionDecimals(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	runE2EWithBackends(t, func(t *testing.T, db *sqldb.DB) {
		ctx := context.Background()
		repo := sqldb.NewRepository(db)
		mockMarket := NewMockMarketDataProvider()
		mockMarket.RegisterInstrument("PRECISION01", "PREC", "Precision Test ETF", "EUR", "77.123456")

		service, err := application.NewPortfolioService(repo, mockMarket)
		require.NoError(t, err)

		pos, err := service.AddPosition(ctx, "PRECISION01", decimalString(t, "1234.56789"), "EUR")
		require.NoError(t, err)
		t.Logf("Precision test: quantity=%s", pos.Quantity.String())

		// Simulate 7.531% increase
		mockMarket.UpdatePrice("PREC", "82.935678936")
		require.NoError(t, service.RefreshPrices(ctx))

		portfolio, _ := service.GetPortfolioSummary(ctx)
		actualPos, _ := portfolio.GetPosition(pos.ID)
		plPct, _ := actualPos.ProfitLossPercent()

		expectedPct := decimalString(t, "7.531")
		tolerance := decimalString(t, "0.01")
		diff, _ := plPct.Sub(expectedPct)
		if diff.Cmp(domain.Zero) < 0 {
			diff, _ = domain.Zero.Sub(diff)
		}
		assert.True(t, diff.Cmp(tolerance) <= 0, "Expected ~7.531%%, got %s", plPct.String())

		t.Log("=== High Precision Test: Passed! ===")
	})
}

// =============================================================================
// E2E Test: Position Averaging (Accumulation)
// =============================================================================

func TestPortfolioCalculations_E2E_PositionAveraging(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	runE2EWithBackends(t, func(t *testing.T, db *sqldb.DB) {
		ctx := context.Background()
		repo := sqldb.NewRepository(db)
		mockMarket := NewMockMarketDataProvider()
		mockMarket.RegisterInstrument("ACCUM01", "ACCUM", "Accumulation Test ETF", "EUR", "100")

		service, err := application.NewPortfolioService(repo, mockMarket)
		require.NoError(t, err)

		// First purchase: 1000 EUR at 100 EUR/unit = 10 units
		_, err = service.AddPosition(ctx, "ACCUM01", decimalString(t, "1000"), "EUR")
		require.NoError(t, err)

		portfolio, _ := service.GetPortfolioSummary(ctx)
		assert.Len(t, portfolio.Positions, 1)

		// Price rises to 110
		mockMarket.UpdatePrice("ACCUM", "110")
		require.NoError(t, service.RefreshPrices(ctx))

		// Second purchase: 550 EUR at 110 EUR/unit = 5 more units
		_, err = service.AddPosition(ctx, "ACCUM01", decimalString(t, "550"), "EUR")
		require.NoError(t, err)

		portfolio, _ = service.GetPortfolioSummary(ctx)
		assert.Len(t, portfolio.Positions, 1, "Positions should merge by ISIN")

		totalInvested, _ := portfolio.TotalInvested()
		assertDecimalEqual(t, decimalString(t, "1550"), totalInvested, "Total invested")

		// 15 units * 110 = 1650
		totalValue, _ := portfolio.TotalValue()
		assertDecimalEqual(t, decimalString(t, "1650"), totalValue, "Total value")

		// Price drops to 90
		mockMarket.UpdatePrice("ACCUM", "90")
		require.NoError(t, service.RefreshPrices(ctx))

		portfolio, _ = service.GetPortfolioSummary(ctx)
		totalValue, _ = portfolio.TotalValue()
		assertDecimalEqual(t, decimalString(t, "1350"), totalValue, "Value after drop")

		profitLoss, _ := portfolio.TotalProfitLoss()
		assertDecimalEqual(t, decimalString(t, "-200"), profitLoss, "Loss after drop")

		t.Log("=== Position Averaging Test: Passed! ===")
	})
}

// =============================================================================
// E2E Test: Edge Cases (Small Values)
// =============================================================================

func TestPortfolioCalculations_E2E_EdgeCases(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	runE2EWithBackends(t, func(t *testing.T, db *sqldb.DB) {
		ctx := context.Background()
		repo := sqldb.NewRepository(db)
		mockMarket := NewMockMarketDataProvider()
		mockMarket.RegisterInstrument("EDGE001", "SMAL", "Small Values ETF", "EUR", "0.0123")

		service, err := application.NewPortfolioService(repo, mockMarket)
		require.NoError(t, err)

		pos, err := service.AddPosition(ctx, "EDGE001", decimalString(t, "0.50"), "EUR")
		require.NoError(t, err)
		t.Logf("Small investment: quantity=%s", pos.Quantity.String())

		// Price doubles
		mockMarket.UpdatePrice("SMAL", "0.0246")
		require.NoError(t, service.RefreshPrices(ctx))

		portfolio, _ := service.GetPortfolioSummary(ctx)
		totalValue, _ := portfolio.TotalValue()
		assertDecimalEqual(t, decimalString(t, "1.00"), totalValue, "Value after 2x")

		profitLossPct, _ := portfolio.TotalProfitLossPercent()
		assertDecimalEqual(t, decimalString(t, "100"), profitLossPct, "Profit% = 100%")

		t.Log("=== Edge Cases Test: Passed! ===")
	})
}

// =============================================================================
// E2E Test: Price Updater (Cron) - Automatic Price Refresh
// =============================================================================

func TestPortfolioCalculations_E2E_PriceUpdaterCron(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	runE2EWithBackends(t, func(t *testing.T, db *sqldb.DB) {
		ctx := context.Background()
		repo := sqldb.NewRepository(db)
		mockMarket := NewMockMarketDataProvider()
		mockMarket.RegisterInstrument("CRON001", "CRON", "Cron Test ETF", "EUR", "100")

		service, err := application.NewPortfolioService(repo, mockMarket)
		require.NoError(t, err)

		_, err = service.AddPosition(ctx, "CRON001", decimalString(t, "1000"), "EUR")
		require.NoError(t, err)

		// Start PriceUpdater
		updater := application.NewPriceUpdater(service, 50*time.Millisecond)
		updaterCtx, cancel := context.WithCancel(ctx)
		go updater.Start(updaterCtx)

		// Simulate price change
		mockMarket.UpdatePrice("CRON", "120")
		time.Sleep(150 * time.Millisecond)

		portfolio, _ := service.GetPortfolioSummary(ctx)
		totalValue, _ := portfolio.TotalValue()
		assertDecimalEqual(t, decimalString(t, "1200"), totalValue, "Value after cron (+20%)")

		// Another price change
		mockMarket.UpdatePrice("CRON", "80")
		time.Sleep(100 * time.Millisecond)

		portfolio, _ = service.GetPortfolioSummary(ctx)
		totalValue, _ = portfolio.TotalValue()
		assertDecimalEqual(t, decimalString(t, "800"), totalValue, "Value after cron (-20%)")

		updater.Stop()
		cancel()
		time.Sleep(100 * time.Millisecond) // Ensure goroutine fully stops

		t.Log("=== Price Updater Test: Passed! ===")
	})
}

// =============================================================================
// E2E Test: Batch Operations
// =============================================================================

func TestPortfolioCalculations_E2E_BatchOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	runE2EWithBackends(t, func(t *testing.T, db *sqldb.DB) {
		ctx := context.Background()
		repo := sqldb.NewRepository(db)
		mockMarket := NewMockBatchMarketDataProvider()

		mockMarket.RegisterInstrument("BATCH001", "BAT1", "Batch Test 1", "EUR", "50")
		mockMarket.RegisterInstrument("BATCH002", "BAT2", "Batch Test 2", "EUR", "100")
		mockMarket.RegisterInstrument("BATCH003", "BAT3", "Batch Test 3", "EUR", "200")

		service, err := application.NewPortfolioService(repo, mockMarket)
		require.NoError(t, err)

		requests := []application.AddPositionBatchRequest{
			{ISIN: "BATCH001", InvestedAmount: decimalString(t, "500"), Currency: "EUR"},
			{ISIN: "BATCH002", InvestedAmount: decimalString(t, "1000"), Currency: "EUR"},
			{ISIN: "BATCH003", InvestedAmount: decimalString(t, "2000"), Currency: "EUR"},
			{ISIN: "BATCH004", InvestedAmount: decimalString(t, "500"), Currency: "EUR"}, // Will fail
		}

		result := service.AddPositionsBatch(ctx, requests)
		assert.Len(t, result.Successful, 3)
		assert.Len(t, result.Failed, 1)

		portfolio, _ := service.GetPortfolioSummary(ctx)
		assert.Len(t, portfolio.Positions, 3)

		totalInvested, _ := portfolio.TotalInvested()
		assertDecimalEqual(t, decimalString(t, "3500"), totalInvested, "Total invested")

		// Mixed market movement
		mockMarket.UpdatePrice("BAT1", "60")  // +20%
		mockMarket.UpdatePrice("BAT2", "110") // +10%
		mockMarket.UpdatePrice("BAT3", "180") // -10%
		require.NoError(t, service.RefreshPrices(ctx))

		portfolio, _ = service.GetPortfolioSummary(ctx)
		totalValue, _ := portfolio.TotalValue()
		assertDecimalEqual(t, decimalString(t, "3500"), totalValue, "Net zero P/L")

		t.Log("=== Batch Operations Test: Passed! ===")
	})
}

// =============================================================================
// E2E Test: Persistence and Recovery After Restart
// =============================================================================

func TestPortfolioCalculations_E2E_PersistenceRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	runE2EWithBackends(t, func(t *testing.T, db *sqldb.DB) {
		ctx := context.Background()
		repo := sqldb.NewRepository(db)
		mockMarket := NewMockMarketDataProvider()
		mockMarket.RegisterInstrument("PERSIST001", "PERS", "Persistence Test ETF", "EUR", "100")

		// First "application run"
		service1, err := application.NewPortfolioService(repo, mockMarket)
		require.NoError(t, err)

		pos, err := service1.AddPosition(ctx, "PERSIST001", decimalString(t, "1000"), "EUR")
		require.NoError(t, err)
		positionID := pos.ID

		mockMarket.UpdatePrice("PERS", "150")
		require.NoError(t, service1.RefreshPrices(ctx))

		portfolio1, _ := service1.GetPortfolioSummary(ctx)
		portfolioID := portfolio1.ID

		// Simulate restart
		service2, err := application.NewPortfolioService(repo, mockMarket)
		require.NoError(t, err)

		portfolio2, _ := service2.GetPortfolioSummary(ctx)
		assert.Equal(t, portfolioID, portfolio2.ID, "Same portfolio ID")
		assert.Len(t, portfolio2.Positions, 1)

		recoveredPos, _ := portfolio2.GetPosition(positionID)
		assertDecimalEqual(t, decimalString(t, "1000"), recoveredPos.InvestedAmount, "InvestedAmount persisted")
		assertDecimalEqual(t, decimalString(t, "10"), recoveredPos.Quantity, "Quantity persisted")

		// Continue with new price
		mockMarket.UpdatePrice("PERS", "200")
		require.NoError(t, service2.RefreshPrices(ctx))

		portfolio2, _ = service2.GetPortfolioSummary(ctx)
		totalValue, _ := portfolio2.TotalValue()
		assertDecimalEqual(t, decimalString(t, "2000"), totalValue, "Value after restart")

		t.Log("=== Persistence Recovery Test: Passed! ===")
	})
}

// =============================================================================
// E2E Test: Position Removal and Recalculation
// =============================================================================

func TestPortfolioCalculations_E2E_PositionRemoval(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	runE2EWithBackends(t, func(t *testing.T, db *sqldb.DB) {
		ctx := context.Background()
		repo := sqldb.NewRepository(db)
		mockMarket := NewMockMarketDataProvider()

		mockMarket.RegisterInstrument("REM001", "RM1", "Remove Test 1", "EUR", "100")
		mockMarket.RegisterInstrument("REM002", "RM2", "Remove Test 2", "EUR", "50")
		mockMarket.RegisterInstrument("REM003", "RM3", "Remove Test 3", "EUR", "200")

		service, err := application.NewPortfolioService(repo, mockMarket)
		require.NoError(t, err)

		pos1, err := service.AddPosition(ctx, "REM001", decimalString(t, "1000"), "EUR")
		require.NoError(t, err)
		pos2, err := service.AddPosition(ctx, "REM002", decimalString(t, "500"), "EUR")
		require.NoError(t, err)
		pos3, err := service.AddPosition(ctx, "REM003", decimalString(t, "2000"), "EUR")
		require.NoError(t, err)

		// Market changes
		mockMarket.UpdatePrice("RM1", "120")
		mockMarket.UpdatePrice("RM2", "40")
		mockMarket.UpdatePrice("RM3", "240")
		require.NoError(t, service.RefreshPrices(ctx))

		portfolio, _ := service.GetPortfolioSummary(ctx)
		totalValue, _ := portfolio.TotalValue()
		assertDecimalEqual(t, decimalString(t, "4000"), totalValue, "Before removal")

		// Remove losing position
		require.NoError(t, service.RemovePosition(ctx, pos2.ID))

		portfolio, _ = service.GetPortfolioSummary(ctx)
		assert.Len(t, portfolio.Positions, 2)
		totalValue, _ = portfolio.TotalValue()
		assertDecimalEqual(t, decimalString(t, "3600"), totalValue, "After removing loser")

		// Remove best performer
		require.NoError(t, service.RemovePosition(ctx, pos3.ID))
		portfolio, _ = service.GetPortfolioSummary(ctx)
		assert.Len(t, portfolio.Positions, 1)

		// Remove last
		require.NoError(t, service.RemovePosition(ctx, pos1.ID))
		portfolio, _ = service.GetPortfolioSummary(ctx)
		assert.Len(t, portfolio.Positions, 0)

		totalValue, _ = portfolio.TotalValue()
		assertDecimalEqual(t, decimalString(t, "0"), totalValue, "Empty portfolio")

		t.Log("=== Position Removal Test: Passed! ===")
	})
}

// =============================================================================
// E2E Test: Concurrent Market Updates
// =============================================================================

func TestPortfolioCalculations_E2E_ConcurrentUpdates(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	runE2EWithBackends(t, func(t *testing.T, db *sqldb.DB) {
		ctx := context.Background()
		repo := sqldb.NewRepository(db)
		mockMarket := NewMockMarketDataProvider()

		mockMarket.RegisterInstrument("CONC001", "CON1", "Concurrent Test 1", "EUR", "100")
		mockMarket.RegisterInstrument("CONC002", "CON2", "Concurrent Test 2", "EUR", "100")
		mockMarket.RegisterInstrument("CONC003", "CON3", "Concurrent Test 3", "EUR", "100")

		service, err := application.NewPortfolioService(repo, mockMarket)
		require.NoError(t, err)

		_, _ = service.AddPosition(ctx, "CONC001", decimalString(t, "1000"), "EUR")
		_, _ = service.AddPosition(ctx, "CONC002", decimalString(t, "1000"), "EUR")
		_, _ = service.AddPosition(ctx, "CONC003", decimalString(t, "1000"), "EUR")

		var wg sync.WaitGroup
		numGoroutines := 5
		numIterations := 10
		errorsChan := make(chan error, numGoroutines*numIterations*2)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numIterations; j++ {
					price := 100 + (id * 10) + j
					mockMarket.UpdatePrice("CON1", fmt.Sprintf("%d", price))
					mockMarket.UpdatePrice("CON2", fmt.Sprintf("%d", price+10))
					mockMarket.UpdatePrice("CON3", fmt.Sprintf("%d", price+20))

					if err := service.RefreshPrices(ctx); err != nil {
						errorsChan <- err
					}
					if _, err := service.GetPortfolioSummary(ctx); err != nil {
						errorsChan <- err
					}
				}
			}(i)
		}

		wg.Wait()
		close(errorsChan)

		var errors []error
		for err := range errorsChan {
			errors = append(errors, err)
		}
		assert.Empty(t, errors, "No errors during concurrent updates")

		portfolio, _ := service.GetPortfolioSummary(ctx)
		assert.Len(t, portfolio.Positions, 3)

		totalInvested, _ := portfolio.TotalInvested()
		assertDecimalEqual(t, decimalString(t, "3000"), totalInvested, "Invested unchanged")

		for _, pos := range portfolio.Positions {
			assertDecimalEqual(t, decimalString(t, "10"), pos.Quantity,
				fmt.Sprintf("Quantity for %s preserved", pos.Instrument.Symbol))
		}

		t.Log("=== Concurrent Updates Test: Passed! ===")
	})
}

// =============================================================================
// E2E Test: Full Application Flow
// =============================================================================

func TestPortfolioCalculations_E2E_FullApplicationFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	runE2EWithBackends(t, func(t *testing.T, db *sqldb.DB) {
		ctx := context.Background()
		repo := sqldb.NewRepository(db)
		mockMarket := NewMockBatchMarketDataProvider()

		mockMarket.RegisterInstrument("IE00B4L5Y983", "IWDA", "iShares Core MSCI World", "EUR", "85.50")
		mockMarket.RegisterInstrument("IE00B5BMR087", "CSPX", "iShares Core S&P 500", "EUR", "520.75")
		mockMarket.RegisterInstrument("IE00BK5BQT80", "VWCE", "Vanguard FTSE All-World", "EUR", "112.25")
		mockMarket.RegisterInstrument("IE00B4ND3602", "IEMA", "iShares MSCI EM", "EUR", "34.80")

		service, err := application.NewPortfolioService(repo, mockMarket)
		require.NoError(t, err)

		// Step 1: Batch create portfolio
		t.Log("Step 1: Batch create portfolio")
		batchResult := service.AddPositionsBatch(ctx, []application.AddPositionBatchRequest{
			{ISIN: "IE00B4L5Y983", InvestedAmount: decimalString(t, "5000"), Currency: "EUR"},
			{ISIN: "IE00B5BMR087", InvestedAmount: decimalString(t, "3000"), Currency: "EUR"},
			{ISIN: "IE00BK5BQT80", InvestedAmount: decimalString(t, "2000"), Currency: "EUR"},
		})
		assert.Len(t, batchResult.Successful, 3)

		// Step 2: Add more positions
		t.Log("Step 2: Add more positions")
		_, _ = service.AddPosition(ctx, "IE00B4ND3602", decimalString(t, "1000"), "EUR")

		portfolio, _ := service.GetPortfolioSummary(ctx)
		assert.Len(t, portfolio.Positions, 4)

		totalInvested, _ := portfolio.TotalInvested()
		assertDecimalEqual(t, decimalString(t, "11000"), totalInvested, "Total invested")

		// Step 3: Market crash simulation
		t.Log("Step 3: Market crash simulation")
		mockMarket.UpdatePrice("IWDA", "70.00")
		mockMarket.UpdatePrice("CSPX", "420.00")
		mockMarket.UpdatePrice("VWCE", "90.00")
		mockMarket.UpdatePrice("IEMA", "28.00")
		require.NoError(t, service.RefreshPrices(ctx))

		portfolio, _ = service.GetPortfolioSummary(ctx)
		profitLoss, _ := portfolio.TotalProfitLoss()
		assert.True(t, profitLoss.Cmp(domain.Zero) < 0, "Should be in loss after crash")
		t.Logf("After crash: P/L = %s", profitLoss.String())

		// Step 4: Sell worst performer
		t.Log("Step 4: Sell worst performer")
		var worstPosID string
		worstPL := domain.Zero
		for _, pos := range portfolio.Positions {
			pl, _ := pos.ProfitLoss()
			if pl.Cmp(worstPL) < 0 {
				worstPL = pl
				worstPosID = pos.ID
			}
		}
		if worstPosID != "" {
			t.Logf("Selling position with P/L: %s", worstPL.String())
			_ = service.RemovePosition(ctx, worstPosID)
		}

		portfolio, _ = service.GetPortfolioSummary(ctx)
		assert.Len(t, portfolio.Positions, 3, "Should have 3 positions after selling")

		// Step 5: Market recovery
		t.Log("Step 5: Market recovery simulation")
		mockMarket.UpdatePrice("IWDA", "100.00")
		mockMarket.UpdatePrice("CSPX", "600.00")
		mockMarket.UpdatePrice("VWCE", "130.00")
		mockMarket.UpdatePrice("IEMA", "40.00")
		require.NoError(t, service.RefreshPrices(ctx))

		portfolio, _ = service.GetPortfolioSummary(ctx)
		totalValue, _ := portfolio.TotalValue()
		totalInvested, _ = portfolio.TotalInvested()
		profitLoss, _ = portfolio.TotalProfitLoss()

		// Consistency check
		calculatedPL, _ := totalValue.Sub(totalInvested)
		assertDecimalEqual(t, calculatedPL, profitLoss, "P/L consistency")

		t.Logf("Final: Invested=%s, Value=%s, P/L=%s", totalInvested.String(), totalValue.String(), profitLoss.String())
		t.Log("=== Full Application Flow Test: Passed! ===")
	})
}
