package sqldb

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestDB_WithTx_Commit(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	wrapper := New(db, &PostgresDialect{})

	mock.ExpectBegin()
	mock.ExpectCommit()

	ctx := context.Background()
	err = wrapper.WithTx(ctx, func(tx *sql.Tx) error {
		return nil
	})

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDB_WithTx_RollbackOnError(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	wrapper := New(db, &PostgresDialect{})

	mock.ExpectBegin()
	mock.ExpectRollback()

	ctx := context.Background()
	expectedErr := errors.New("business error")
	err = wrapper.WithTx(ctx, func(tx *sql.Tx) error {
		return expectedErr
	})

	assert.Equal(t, expectedErr, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDB_WithTx_RollbackOnPanic(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	wrapper := New(db, &PostgresDialect{})

	mock.ExpectBegin()
	mock.ExpectRollback()

	ctx := context.Background()

	// Recover from the panic in the test to assert
	defer func() {
		if r := recover(); r != nil {
			assert.Equal(t, "unexpected panic", r)
			assert.NoError(t, mock.ExpectationsWereMet())
		}
	}()

	_ = wrapper.WithTx(ctx, func(tx *sql.Tx) error {
		panic("unexpected panic")
	})
}
