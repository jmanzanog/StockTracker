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

type stubDialect struct {
	name                string
	migrateErr          error
	upsertPortfolioErr  error
	upsertInstrumentErr error
	upsertPositionErr   error
}

func (d *stubDialect) Name() string { return d.name }

func (d *stubDialect) Migrate(_ context.Context, _ *sql.DB) error {
	return d.migrateErr
}

func (d *stubDialect) UpsertPortfolio(_ context.Context, _ *sql.Tx, _ *domain.Portfolio) error {
	return d.upsertPortfolioErr
}

func (d *stubDialect) UpsertInstrument(_ context.Context, _ *sql.Tx, _ *domain.Instrument) error {
	return d.upsertInstrumentErr
}

func (d *stubDialect) UpsertPosition(_ context.Context, _ *sql.Tx, _ *domain.Position) error {
	return d.upsertPositionErr
}

func TestDB_WithTx_BeginError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	mock.ExpectBegin().WillReturnError(fmt.Errorf("begin fail"))

	wrapper := New(db, &stubDialect{name: "postgres"})
	if err := wrapper.WithTx(context.Background(), func(_ *sql.Tx) error { return nil }); err == nil {
		t.Fatal("expected begin error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestDB_WithTx_CommitError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	mock.ExpectBegin()
	mock.ExpectCommit().WillReturnError(fmt.Errorf("commit fail"))

	wrapper := New(db, &stubDialect{name: "postgres"})
	if err := wrapper.WithTx(context.Background(), func(_ *sql.Tx) error { return nil }); err == nil {
		t.Fatal("expected commit error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestDB_WithTx_RollbackOnError_Mock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	mock.ExpectBegin()
	mock.ExpectRollback()

	wrapper := New(db, &stubDialect{name: "postgres"})
	if err := wrapper.WithTx(context.Background(), func(_ *sql.Tx) error { return fmt.Errorf("fn error") }); err == nil {
		t.Fatal("expected error from tx fn")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestDB_WithTx_RollbackOnPanic_Mock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	mock.ExpectBegin()
	mock.ExpectRollback()

	wrapper := New(db, &stubDialect{name: "postgres"})

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic to propagate")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	}()

	_ = wrapper.WithTx(context.Background(), func(_ *sql.Tx) error {
		panic("boom")
	})
}

func TestRepository_AutoMigrate_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	repo := NewRepository(New(db, &stubDialect{name: "postgres", migrateErr: fmt.Errorf("migrate fail")}))

	if err := repo.AutoMigrate(); err == nil {
		t.Fatal("expected migrate error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRepository_Save_UpsertPortfolioError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	mock.ExpectBegin()
	mock.ExpectRollback()

	repo := NewRepository(New(db, &stubDialect{name: "postgres", upsertPortfolioErr: fmt.Errorf("portfolio err")}))
	portfolio := domain.NewPortfolio("test")

	if err := repo.Save(context.Background(), &portfolio); err == nil {
		t.Fatal("expected upsert portfolio error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRepository_Save_UpsertInstrumentError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	mock.ExpectBegin()
	mock.ExpectRollback()

	repo := NewRepository(New(db, &stubDialect{name: "postgres", upsertInstrumentErr: fmt.Errorf("instrument err")}))
	portfolio := domain.NewPortfolio("test")
	inst := domain.NewInstrument("US123", "AAPL", "Apple", domain.InstrumentTypeStock, "USD", "NASDAQ", "Technology")
	pos := domain.NewPosition(inst, domain.NewDecimalFromInt(1), "USD")
	portfolio.Positions = []domain.Position{pos}

	if err := repo.Save(context.Background(), &portfolio); err == nil {
		t.Fatal("expected upsert instrument error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRepository_Save_UpsertPositionError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	mock.ExpectBegin()
	mock.ExpectRollback()

	repo := NewRepository(New(db, &stubDialect{name: "postgres", upsertPositionErr: fmt.Errorf("position err")}))
	portfolio := domain.NewPortfolio("test")
	inst := domain.NewInstrument("US123", "AAPL", "Apple", domain.InstrumentTypeStock, "USD", "NASDAQ", "Technology")
	pos := domain.NewPosition(inst, domain.NewDecimalFromInt(1), "USD")
	portfolio.Positions = []domain.Position{pos}

	if err := repo.Save(context.Background(), &portfolio); err == nil {
		t.Fatal("expected upsert position error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRepository_FindByID_QueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	mock.ExpectQuery("SELECT").WithArgs("id-1").WillReturnError(fmt.Errorf("query error"))

	repo := NewRepository(New(db, &stubDialect{name: "postgres"}))
	if _, err := repo.FindByID(context.Background(), "id-1"); err == nil {
		t.Fatal("expected query error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRepository_FindByID_ScanError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	rows := sqlmock.NewRows([]string{"id"}).AddRow("id-1")
	mock.ExpectQuery("SELECT").WithArgs("id-1").WillReturnRows(rows)

	repo := NewRepository(New(db, &stubDialect{name: "postgres"}))
	if _, err := repo.FindByID(context.Background(), "id-1"); err == nil {
		t.Fatal("expected scan error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRepository_FindByID_RowsError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	rows := sqlmock.NewRows([]string{
		"p_id", "p_name", "p_last", "p_created",
		"pos_id", "pos_port", "pos_isin", "pos_invested", "pos_currency", "pos_qty", "pos_price", "pos_last",
		"i_isin", "i_symbol", "i_name", "i_type", "i_currency", "i_exchange",
	}).AddRow(
		"id-1", "name", time.Now(), time.Now(),
		"pos-1", "id-1", "US123", domain.NewDecimalFromInt(1), "USD", domain.NewDecimalFromInt(1), domain.NewDecimalFromInt(1), time.Now(),
		"US123", "AAPL", "Apple", "stock", "USD", "NASDAQ",
	).RowError(0, fmt.Errorf("row error"))

	mock.ExpectQuery("SELECT").WithArgs("id-1").WillReturnRows(rows)

	repo := NewRepository(New(db, &stubDialect{name: "postgres"}))
	if _, err := repo.FindByID(context.Background(), "id-1"); err == nil {
		t.Fatal("expected rows error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRepository_FindAll_QueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	mock.ExpectQuery("FROM portfolios").WillReturnError(fmt.Errorf("query error"))

	repo := NewRepository(New(db, &stubDialect{name: "postgres"}))
	if _, err := repo.FindAll(context.Background()); err == nil {
		t.Fatal("expected query error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRepository_FindAll_ScanError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	rows := sqlmock.NewRows([]string{"id"}).AddRow("id-1")
	mock.ExpectQuery("FROM portfolios").WillReturnRows(rows)

	repo := NewRepository(New(db, &stubDialect{name: "postgres"}))
	if _, err := repo.FindAll(context.Background()); err == nil {
		t.Fatal("expected scan error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRepository_FindAll_RowsError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	rows := sqlmock.NewRows([]string{
		"p_id", "p_name", "p_last", "p_created",
		"pos_id", "pos_port", "pos_isin", "pos_invested", "pos_currency", "pos_qty", "pos_price", "pos_last",
		"i_isin", "i_symbol", "i_name", "i_type", "i_currency", "i_exchange",
	}).AddRow(
		"id-1", "name", time.Now(), time.Now(),
		"pos-1", "id-1", "US123", domain.NewDecimalFromInt(1), "USD", domain.NewDecimalFromInt(1), domain.NewDecimalFromInt(1), time.Now(),
		"US123", "AAPL", "Apple", "stock", "USD", "NASDAQ",
	).RowError(0, fmt.Errorf("row error"))

	mock.ExpectQuery("FROM portfolios").WillReturnRows(rows)

	repo := NewRepository(New(db, &stubDialect{name: "postgres"}))
	if _, err := repo.FindAll(context.Background()); err == nil {
		t.Fatal("expected rows error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRepository_Delete_PositionsError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM positions").WithArgs("id-1").WillReturnError(fmt.Errorf("delete positions fail"))
	mock.ExpectRollback()

	repo := NewRepository(New(db, &stubDialect{name: "postgres"}))
	if err := repo.Delete(context.Background(), "id-1"); err == nil {
		t.Fatal("expected delete positions error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRepository_Delete_PortfolioError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM positions").WithArgs("id-1").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("DELETE FROM portfolios").WithArgs("id-1").WillReturnError(fmt.Errorf("delete portfolio fail"))
	mock.ExpectRollback()

	repo := NewRepository(New(db, &stubDialect{name: "postgres"}))
	if err := repo.Delete(context.Background(), "id-1"); err == nil {
		t.Fatal("expected delete portfolio error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRepository_Rebind_Oracle(t *testing.T) {
	repo := NewRepository(New(&sql.DB{}, &stubDialect{name: "oracle"}))
	query := "SELECT * FROM t WHERE a = $1 AND b = $2"
	if reb := repo.rebind(query); reb == query {
		t.Fatal("expected oracle rebinding")
	}
}
