package sqldb

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmanzanog/stock-tracker/internal/domain"
)

func beginTx(t *testing.T, db *sql.DB, mock sqlmock.Sqlmock) *sql.Tx {
	t.Helper()

	mock.ExpectBegin()
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("failed to begin tx: %v", err)
	}
	return tx
}

func TestOracleDialect_UpsertPortfolio_Update(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	tx := beginTx(t, db, mock)
	defer func() {
		_ = tx.Rollback()
	}()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM portfolios`).WithArgs("id-1").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec("UPDATE portfolios").WithArgs("name", sqlmock.AnyArg(), "id-1").WillReturnResult(sqlmock.NewResult(1, 1))

	portfolio := &domain.Portfolio{ID: "id-1", Name: "name", LastUpdated: time.Now()}
	if err := (&OracleDialect{}).UpsertPortfolio(context.Background(), tx, portfolio); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestOracleDialect_UpsertPortfolio_Insert(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	tx := beginTx(t, db, mock)
	defer func() {
		_ = tx.Rollback()
	}()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM portfolios`).WithArgs("id-1").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec("INSERT INTO portfolios").WithArgs("id-1", "name", sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))

	portfolio := &domain.Portfolio{ID: "id-1", Name: "name", LastUpdated: time.Now(), CreatedAt: time.Now()}
	if err := (&OracleDialect{}).UpsertPortfolio(context.Background(), tx, portfolio); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestOracleDialect_UpsertPortfolio_QueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	tx := beginTx(t, db, mock)
	defer func() {
		_ = tx.Rollback()
	}()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM portfolios`).WithArgs("id-1").WillReturnError(fmt.Errorf("query error"))

	portfolio := &domain.Portfolio{ID: "id-1", Name: "name"}
	if err := (&OracleDialect{}).UpsertPortfolio(context.Background(), tx, portfolio); err == nil {
		t.Fatal("expected query error")
	}
}

func TestOracleDialect_UpsertPortfolio_UpdateError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	tx := beginTx(t, db, mock)
	defer func() {
		_ = tx.Rollback()
	}()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM portfolios`).WithArgs("id-1").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec("UPDATE portfolios").WithArgs("name", sqlmock.AnyArg(), "id-1").WillReturnError(fmt.Errorf("update error"))

	portfolio := &domain.Portfolio{ID: "id-1", Name: "name", LastUpdated: time.Now()}
	if err := (&OracleDialect{}).UpsertPortfolio(context.Background(), tx, portfolio); err == nil {
		t.Fatal("expected update error")
	}
}

func TestOracleDialect_UpsertInstrument_Insert(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	tx := beginTx(t, db, mock)
	defer func() {
		_ = tx.Rollback()
	}()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM instruments`).WithArgs("US123").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec("INSERT INTO instruments").WithArgs("US123", "AAPL", "Apple", "stock", "USD", "NASDAQ").WillReturnResult(sqlmock.NewResult(1, 1))

	inst := &domain.Instrument{ISIN: "US123", Symbol: "AAPL", Name: "Apple", Type: domain.InstrumentTypeStock, Currency: "USD", Exchange: "NASDAQ"}
	if err := (&OracleDialect{}).UpsertInstrument(context.Background(), tx, inst); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOracleDialect_UpsertInstrument_UniqueViolationIgnored(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	tx := beginTx(t, db, mock)
	defer func() {
		_ = tx.Rollback()
	}()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM instruments`).WithArgs("US123").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec("INSERT INTO instruments").WillReturnError(fmt.Errorf("ORA-00001: unique constraint"))

	inst := &domain.Instrument{ISIN: "US123"}
	if err := (&OracleDialect{}).UpsertInstrument(context.Background(), tx, inst); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOracleDialect_UpsertInstrument_QueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	tx := beginTx(t, db, mock)
	defer func() {
		_ = tx.Rollback()
	}()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM instruments`).WithArgs("US123").WillReturnError(fmt.Errorf("query error"))

	inst := &domain.Instrument{ISIN: "US123"}
	if err := (&OracleDialect{}).UpsertInstrument(context.Background(), tx, inst); err == nil {
		t.Fatal("expected query error")
	}
}

func TestOracleDialect_UpsertInstrument_InsertError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	tx := beginTx(t, db, mock)
	defer func() {
		_ = tx.Rollback()
	}()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM instruments`).WithArgs("US123").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec("INSERT INTO instruments").WillReturnError(fmt.Errorf("insert error"))

	inst := &domain.Instrument{ISIN: "US123"}
	if err := (&OracleDialect{}).UpsertInstrument(context.Background(), tx, inst); err == nil {
		t.Fatal("expected insert error")
	}
}

func TestOracleDialect_UpsertPosition_InsertAndUpdateErrors(t *testing.T) {
	for _, tc := range []struct {
		name      string
		count     int
		execQuery string
		execError string
	}{
		{name: "update error", count: 1, execQuery: "UPDATE positions", execError: "update error"},
		{name: "insert error", count: 0, execQuery: "INSERT INTO positions", execError: "insert error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer func() {
				_ = db.Close()
			}()

			tx := beginTx(t, db, mock)
			defer func() {
				_ = tx.Rollback()
			}()

			mock.ExpectQuery(`SELECT COUNT\(\*\) FROM positions`).WithArgs("pos-1").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(tc.count))
			mock.ExpectExec(tc.execQuery).WillReturnError(fmt.Errorf("%s", tc.execError))

			pos := &domain.Position{ID: "pos-1", PortfolioID: "id-1", Instrument: domain.Instrument{ISIN: "US123"}, InvestedCurrency: "USD", LastUpdated: time.Now()}
			if err := (&OracleDialect{}).UpsertPosition(context.Background(), tx, pos); err == nil {
				t.Fatal("expected upsert position error")
			}
		})
	}
}

func TestOracleDialect_UpsertPosition_QueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	tx := beginTx(t, db, mock)
	defer func() {
		_ = tx.Rollback()
	}()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM positions`).WithArgs("pos-1").WillReturnError(fmt.Errorf("query error"))

	pos := &domain.Position{ID: "pos-1"}
	if err := (&OracleDialect{}).UpsertPosition(context.Background(), tx, pos); err == nil {
		t.Fatal("expected query error")
	}
}
