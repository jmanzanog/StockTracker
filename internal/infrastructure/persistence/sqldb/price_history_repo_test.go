package sqldb

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmanzanog/stock-tracker/internal/domain"
)

func TestPriceHistoryRepository_SaveBatch(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	wrapper := New(db, &stubDialect{name: "postgres"})
	repo := NewPriceHistoryRepository(wrapper)

	t.Run("success", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectPrepare("INSERT INTO price_history")
		mock.ExpectExec("INSERT INTO price_history").WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		history := []domain.PriceHistory{
			{
				ID:             "ph1",
				InstrumentISIN: "US0378331005",
				Price:          domain.NewDecimalFromInt(150),
				Currency:       "USD",
				RecordedAt:     time.Now(),
				CreatedAt:      time.Now(),
			},
		}

		err := repo.SaveBatch(context.Background(), history)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unmet expectations: %v", err)
		}
	})

	t.Run("empty history", func(t *testing.T) {
		err := repo.SaveBatch(context.Background(), []domain.PriceHistory{})
		if err != nil {
			t.Errorf("expected no error for empty batch, got %v", err)
		}
	})

	t.Run("prepare error", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectPrepare("INSERT INTO price_history").WillReturnError(context.DeadlineExceeded)
		mock.ExpectRollback()

		history := []domain.PriceHistory{
			{
				ID:             "ph1",
				InstrumentISIN: "US0378331005",
				Price:          domain.NewDecimalFromInt(150),
				Currency:       "USD",
				RecordedAt:     time.Now(),
				CreatedAt:      time.Now(),
			},
		}

		err := repo.SaveBatch(context.Background(), history)
		if err == nil {
			t.Error("expected error when prepare fails")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unmet expectations: %v", err)
		}
	})
}

func TestPriceHistoryRepository_GetByISIN(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	wrapper := New(db, &stubDialect{name: "postgres"})
	repo := NewPriceHistoryRepository(wrapper)

	t.Run("success", func(t *testing.T) {
		now := time.Now()
		rows := sqlmock.NewRows([]string{"id", "instrument_isin", "price", "currency", "recorded_at", "created_at"}).
			AddRow("ph1", "US0378331005", "150", "USD", now, now).
			AddRow("ph2", "US0378331005", "151", "USD", now, now)

		mock.ExpectQuery("SELECT id, instrument_isin, price, currency, recorded_at, created_at FROM price_history").
			WithArgs("US0378331005", sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnRows(rows)

		history, err := repo.GetByISIN(context.Background(), "US0378331005", now.AddDate(0, 0, -7), now)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(history) != 2 {
			t.Errorf("expected 2 records, got %d", len(history))
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unmet expectations: %v", err)
		}
	})

	t.Run("query error", func(t *testing.T) {
		mock.ExpectQuery("SELECT id, instrument_isin, price, currency, recorded_at, created_at FROM price_history").
			WithArgs("US0378331005", sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnError(context.DeadlineExceeded)

		_, err := repo.GetByISIN(context.Background(), "US0378331005", time.Now().AddDate(0, 0, -7), time.Now())
		if err == nil {
			t.Error("expected error when query fails")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unmet expectations: %v", err)
		}
	})

	t.Run("empty result", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"id", "instrument_isin", "price", "currency", "recorded_at", "created_at"})

		mock.ExpectQuery("SELECT id, instrument_isin, price, currency, recorded_at, created_at FROM price_history").
			WithArgs("US0378331005", sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnRows(rows)

		history, err := repo.GetByISIN(context.Background(), "US0378331005", time.Now().AddDate(0, 0, -7), time.Now())
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(history) != 0 {
			t.Errorf("expected 0 records, got %d", len(history))
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unmet expectations: %v", err)
		}
	})
}

func TestPriceHistoryRepository_GetSparkline(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	wrapper := New(db, &stubDialect{name: "postgres"})
	repo := NewPriceHistoryRepository(wrapper)

	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "instrument_isin", "price", "currency", "recorded_at", "created_at"}).
		AddRow("ph1", "US0378331005", "150", "USD", now, now)

	mock.ExpectQuery("SELECT id, instrument_isin, price, currency, recorded_at, created_at FROM price_history").
		WithArgs("US0378331005", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(rows)

	history, err := repo.GetSparkline(context.Background(), "US0378331005", 7)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(history) != 1 {
		t.Errorf("expected 1 record, got %d", len(history))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestPriceHistoryRepository_GetSparklinesBatch(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	wrapper := New(db, &stubDialect{name: "postgres"})
	repo := NewPriceHistoryRepository(wrapper)

	t.Run("success single request", func(t *testing.T) {
		now := time.Now()
		rows := sqlmock.NewRows([]string{"id", "instrument_isin", "price", "currency", "recorded_at", "created_at"}).
			AddRow("ph1", "US0378331005", "150", "USD", now, now)

		mock.ExpectQuery("SELECT id, instrument_isin, price, currency, recorded_at, created_at FROM price_history").
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "US0378331005").
			WillReturnRows(rows)

		requests := []domain.SparklineRequest{
			{ISIN: "US0378331005", Days: 7},
		}

		results, err := repo.GetSparklinesBatch(context.Background(), requests)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(results) != 1 {
			t.Errorf("expected 1 result, got %d", len(results))
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unmet expectations: %v", err)
		}
	})

	t.Run("empty requests", func(t *testing.T) {
		results, err := repo.GetSparklinesBatch(context.Background(), []domain.SparklineRequest{})
		if err != nil {
			t.Fatalf("expected no error for empty requests, got %v", err)
		}
		if results != nil {
			t.Errorf("expected nil result for empty requests, got %v", results)
		}
	})

	t.Run("multiple isins", func(t *testing.T) {
		now := time.Now()
		rows := sqlmock.NewRows([]string{"id", "instrument_isin", "price", "currency", "recorded_at", "created_at"}).
			AddRow("ph1", "US0378331005", "150", "USD", now, now).
			AddRow("ph2", "US5949181045", "300", "USD", now, now)

		mock.ExpectQuery("SELECT id, instrument_isin, price, currency, recorded_at, created_at FROM price_history").
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnRows(rows)

		requests := []domain.SparklineRequest{
			{ISIN: "US0378331005", Days: 7},
			{ISIN: "US5949181045", Days: 30},
		}

		results, err := repo.GetSparklinesBatch(context.Background(), requests)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(results) != 2 {
			t.Errorf("expected 2 results, got %d", len(results))
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unmet expectations: %v", err)
		}
	})
}

func TestPriceHistoryRepository_CleanupOlderThan(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	wrapper := New(db, &stubDialect{name: "postgres"})
	repo := NewPriceHistoryRepository(wrapper)

	t.Run("success", func(t *testing.T) {
		mock.ExpectExec("DELETE FROM price_history").
			WithArgs(sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(0, 5))

		cutoff := time.Now()
		deleted, err := repo.CleanupOlderThan(context.Background(), cutoff)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if deleted != 5 {
			t.Errorf("expected 5 deleted, got %d", deleted)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unmet expectations: %v", err)
		}
	})

	t.Run("exec error", func(t *testing.T) {
		mock.ExpectExec("DELETE FROM price_history").
			WithArgs(sqlmock.AnyArg()).
			WillReturnError(context.DeadlineExceeded)

		cutoff := time.Now()
		_, err := repo.CleanupOlderThan(context.Background(), cutoff)
		if err == nil {
			t.Error("expected error when delete fails")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unmet expectations: %v", err)
		}
	})
}
