package sqldb

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmanzanog/stock-tracker/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestOracleDialect_UpsertPortfolio_QueryGeneration(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	dialect := &OracleDialect{}

	p := domain.NewPortfolio("Test Portfolio")
	p.CreatedAt = time.Now()
	p.LastUpdated = time.Now()

	// ORDER MATTERS:
	// 1. Begin Transaction
	mock.ExpectBegin()
	tx, err := db.Begin()
	assert.NoError(t, err)

	// 2. Execute Query
	mock.ExpectExec(`MERGE INTO portfolios p`).
		WithArgs(
			p.ID,             // 1
			p.Name,           // 2
			sqlmock.AnyArg(), // 3 (LastUpdated)
			p.ID,             // 4
			p.Name,           // 5
			sqlmock.AnyArg(), // 6 (LastUpdated)
			sqlmock.AnyArg(), // 7 (CreatedAt)
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	ctx := context.Background()
	err = dialect.UpsertPortfolio(ctx, tx, &p)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOracleDialect_UpsertInstrument_QueryGeneration(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	dialect := &OracleDialect{}
	inst := domain.NewInstrument("US123", "AAPL", "Apple", "stock", "USD", "NASDAQ")

	// 1. Begin
	mock.ExpectBegin()
	tx, err := db.Begin()
	assert.NoError(t, err)

	// 2. Exec
	mock.ExpectExec("MERGE INTO instruments i").
		WithArgs(
			inst.ISIN,         // 1
			inst.ISIN,         // 2
			inst.Symbol,       // 3
			inst.Name,         // 4
			string(inst.Type), // 5
			inst.Currency,     // 6
			inst.Exchange,     // 7
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	ctx := context.Background()
	err = dialect.UpsertInstrument(ctx, tx, &inst)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOracleDialect_UpsertPosition_QueryGeneration(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	dialect := &OracleDialect{}

	inst := domain.NewInstrument("US123", "AAPL", "Apple", "stock", "USD", "NASDAQ")
	pos := domain.NewPosition(inst, domain.NewDecimalFromInt(100), "USD")
	pos.PortfolioID = "port-1"

	// 1. Begin
	mock.ExpectBegin()
	tx, err := db.Begin()
	assert.NoError(t, err)

	// 2. Exec
	mock.ExpectExec("MERGE INTO positions t").
		WithArgs(
			pos.ID,               // 1
			pos.InvestedAmount,   // 2
			pos.Quantity,         // 3
			pos.CurrentPrice,     // 4
			sqlmock.AnyArg(),     // 5  (LastUpdated)
			pos.PortfolioID,      // 6
			pos.ID,               // 7
			pos.PortfolioID,      // 8
			pos.Instrument.ISIN,  // 9
			pos.InvestedAmount,   // 10
			pos.InvestedCurrency, // 11
			pos.Quantity,         // 12
			pos.CurrentPrice,     // 13
			sqlmock.AnyArg(),     // 14 (LastUpdated)
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	ctx := context.Background()
	err = dialect.UpsertPosition(ctx, tx, &pos)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
