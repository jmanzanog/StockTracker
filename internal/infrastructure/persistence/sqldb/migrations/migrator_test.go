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

	// 3. Test Rollback
	rollbackCount, err := m.Rollback(db)
	assert.NoError(t, err)
	assert.Equal(t, 1, rollbackCount, "Should have rolled back exactly one migration")

	// Verify rollback took effect in records
	appliedAfter, err := m.GetApplied(db)
	assert.NoError(t, err)
	assert.Equal(t, len(applied)-1, len(appliedAfter), "Should have one less applied record after rollback")
}
