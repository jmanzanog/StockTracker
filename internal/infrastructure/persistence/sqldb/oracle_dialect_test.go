package sqldb

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmanzanog/stock-tracker/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestOracleDialect_UpsertPortfolio_Insert(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	dialect := &OracleDialect{}

	p := domain.NewPortfolio("Test Portfolio")
	p.CreatedAt = time.Now()
	p.LastUpdated = time.Now()

	mock.ExpectBegin()
	tx, err := db.Begin()
	assert.NoError(t, err)

	// 1. SELECT COUNT(*) - returns 0 (not exists)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM portfolios WHERE id = :1`).
		WithArgs(p.ID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	// 2. INSERT
	mock.ExpectExec(`INSERT INTO portfolios`).
		WithArgs(p.ID, p.Name, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	ctx := context.Background()
	err = dialect.UpsertPortfolio(ctx, tx, &p)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOracleDialect_UpsertPortfolio_Update(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	dialect := &OracleDialect{}

	p := domain.NewPortfolio("Test Portfolio")
	p.CreatedAt = time.Now()
	p.LastUpdated = time.Now()

	mock.ExpectBegin()
	tx, err := db.Begin()
	assert.NoError(t, err)

	// 1. SELECT COUNT(*) - returns 1 (exists)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM portfolios WHERE id = :1`).
		WithArgs(p.ID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	// 2. UPDATE
	mock.ExpectExec(`UPDATE portfolios SET`).
		WithArgs(p.Name, sqlmock.AnyArg(), p.ID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	ctx := context.Background()
	err = dialect.UpsertPortfolio(ctx, tx, &p)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOracleDialect_UpsertInstrument_Insert(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	dialect := &OracleDialect{}
	inst := domain.NewInstrument("US123", "AAPL", "Apple", "stock", "USD", "NASDAQ")

	mock.ExpectBegin()
	tx, err := db.Begin()
	assert.NoError(t, err)

	// 1. SELECT COUNT(*) - returns 0 (not exists)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM instruments WHERE isin = :1`).
		WithArgs(inst.ISIN).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	// 2. INSERT
	mock.ExpectExec(`INSERT INTO instruments`).
		WithArgs(inst.ISIN, inst.Symbol, inst.Name, string(inst.Type), inst.Currency, inst.Exchange).
		WillReturnResult(sqlmock.NewResult(1, 1))

	ctx := context.Background()
	err = dialect.UpsertInstrument(ctx, tx, &inst)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOracleDialect_UpsertInstrument_Skip(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	dialect := &OracleDialect{}
	inst := domain.NewInstrument("US123", "AAPL", "Apple", "stock", "USD", "NASDAQ")

	mock.ExpectBegin()
	tx, err := db.Begin()
	assert.NoError(t, err)

	// 1. SELECT COUNT(*) - returns 1 (already exists, skip insert)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM instruments WHERE isin = :1`).
		WithArgs(inst.ISIN).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	// No INSERT expected

	ctx := context.Background()
	err = dialect.UpsertInstrument(ctx, tx, &inst)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOracleDialect_UpsertPosition_Insert(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	dialect := &OracleDialect{}

	inst := domain.NewInstrument("US123", "AAPL", "Apple", "stock", "USD", "NASDAQ")
	pos := domain.NewPosition(inst, domain.NewDecimalFromInt(100), "USD")
	pos.PortfolioID = "port-1"

	mock.ExpectBegin()
	tx, err := db.Begin()
	assert.NoError(t, err)

	// 1. SELECT COUNT(*) - returns 0 (not exists)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM positions WHERE id = :1`).
		WithArgs(pos.ID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	// 2. INSERT
	mock.ExpectExec(`INSERT INTO positions`).
		WithArgs(
			pos.ID, pos.PortfolioID, pos.Instrument.ISIN,
			pos.InvestedAmount, pos.InvestedCurrency, pos.Quantity, pos.CurrentPrice, sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	ctx := context.Background()
	err = dialect.UpsertPosition(ctx, tx, &pos)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOracleDialect_UpsertPosition_Update(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	dialect := &OracleDialect{}

	inst := domain.NewInstrument("US123", "AAPL", "Apple", "stock", "USD", "NASDAQ")
	pos := domain.NewPosition(inst, domain.NewDecimalFromInt(100), "USD")
	pos.PortfolioID = "port-1"

	mock.ExpectBegin()
	tx, err := db.Begin()
	assert.NoError(t, err)

	// 1. SELECT COUNT(*) - returns 1 (exists)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM positions WHERE id = :1`).
		WithArgs(pos.ID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	// 2. UPDATE
	mock.ExpectExec(`UPDATE positions SET`).
		WithArgs(
			pos.InvestedAmount, pos.Quantity, pos.CurrentPrice,
			sqlmock.AnyArg(), pos.PortfolioID, pos.ID,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	ctx := context.Background()
	err = dialect.UpsertPosition(ctx, tx, &pos)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
