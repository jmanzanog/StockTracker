package sqldb

import (
	"context"
	"database/sql"
	"fmt"
)

type DB struct {
	*sql.DB
	Dialect Dialect
}

func New(db *sql.DB, dialect Dialect) *DB {
	return &DB{
		DB:      db,
		Dialect: dialect,
	}
}

func (db *DB) WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
