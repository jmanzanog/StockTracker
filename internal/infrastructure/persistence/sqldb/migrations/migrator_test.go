package migrations

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestMigrationSource_mapDialect(t *testing.T) {
	tests := []struct {
		name     string
		dialect  string
		expected string
	}{
		{
			name:     "postgres stays postgres",
			dialect:  "postgres",
			expected: "postgres",
		},
		{
			name:     "oracle maps to oci8",
			dialect:  "oracle",
			expected: "oci8",
		},
		{
			name:     "sqlite3 stays sqlite3",
			dialect:  "sqlite3",
			expected: "sqlite3",
		},
		{
			name:     "unknown stays unknown",
			dialect:  "unknown",
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMigrationSource(tt.dialect)
			assert.Equal(t, tt.expected, m.mapDialect())
		})
	}
}

func TestMigrationSource_Integration(t *testing.T) {
	// Use sqlite in memory for testing
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	m := NewMigrationSource("sqlite3")

	// 1. Test Run
	n, err := m.Run(db)
	assert.NoError(t, err)
	assert.Greater(t, n, 0, "Should have applied at least one migration")

	// 2. Test GetApplied
	applied, err := m.GetApplied(db)
	assert.NoError(t, err)
	assert.Equal(t, n, len(applied), "Applied records should match count")

	// 3. Test Rollback (rolls back ALL migrations - this is the current behavior of migrate.Down)
	appliedBefore := len(applied)
	rollbackCount, err := m.Rollback(db)
	assert.NoError(t, err)
	assert.Equal(t, appliedBefore, rollbackCount, "Should have rolled back all applied migrations")

	// Verify rollback took effect in records - all migrations should be rolled back
	appliedAfter, err := m.GetApplied(db)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(appliedAfter), "Should have no applied records after full rollback")
}

func TestMigrationSource_Errors(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	// Do not defer close here, we want to close it manually to trigger errors

	m := NewMigrationSource("sqlite3")

	t.Run("Run error with closed db", func(t *testing.T) {
		_ = db.Close()
		_, err := m.Run(db)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "executing migrations")
	})

	t.Run("GetApplied error with closed db", func(t *testing.T) {
		_, err := m.GetApplied(db)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "getting migration records")
	})

	t.Run("Rollback error with closed db", func(t *testing.T) {
		_, err := m.Rollback(db)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "rolling back migration")
	})
}
