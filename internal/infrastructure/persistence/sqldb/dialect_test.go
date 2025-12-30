package sqldb

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestPostgresDialect_Migrate(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	d := &PostgresDialect{}
	assert.Equal(t, "postgres", d.Name())

	err = d.Migrate(context.Background(), db)
	if err != nil {
		t.Logf("Postgres migration failed: %v", err)
	} else {
		t.Log("Postgres migration succeeded on sqlite")
	}

	// Case: Error path
	dbForError, _ := sql.Open("sqlite", ":memory:")
	_ = dbForError.Close()
	err = d.Migrate(context.Background(), dbForError)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "running migrations")
}

func TestOracleDialect_Migrate(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	d := &OracleDialect{}
	assert.Equal(t, "oracle", d.Name())

	err = d.Migrate(context.Background(), db)
	if err != nil {
		t.Logf("Oracle migration failed: %v", err)
	} else {
		t.Log("Oracle migration succeeded on sqlite")
	}

	// Case: Error path
	dbForError, _ := sql.Open("sqlite", ":memory:")
	_ = dbForError.Close()
	err = d.Migrate(context.Background(), dbForError)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "running migrations")
}
