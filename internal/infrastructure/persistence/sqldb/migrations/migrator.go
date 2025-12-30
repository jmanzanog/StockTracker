// Package migrations provides database migration utilities using sql-migrate.
// It supports both PostgreSQL and Oracle databases with a unified migration source.
package migrations

import (
	"database/sql"
	"embed"
	"fmt"

	migrate "github.com/rubenv/sql-migrate"
)

//go:embed common/*.sql
var commonFS embed.FS

// MigrationSource provides migration capabilities for different database dialects.
type MigrationSource struct {
	dialect string
}

// NewMigrationSource creates a new migration source for the specified dialect.
// Supported dialects: "postgres", "oracle"
func NewMigrationSource(dialect string) *MigrationSource {
	return &MigrationSource{dialect: dialect}
}

// mapDialect maps internal dialect names to sql-migrate supported dialect names.
// sql-migrate supports: sqlite3, postgres, mysql, mssql, oci8, godror, snowflake
func (m *MigrationSource) mapDialect() string {
	switch m.dialect {
	case "oracle":
		return "oci8" // sql-migrate uses oci8 for Oracle
	default:
		return m.dialect
	}
}

// Run executes all pending migrations against the provided database connection.
// It returns the number of applied migrations and any error encountered.
func (m *MigrationSource) Run(db *sql.DB) (int, error) {
	source := &migrate.EmbedFileSystemMigrationSource{
		FileSystem: commonFS,
		Root:       "common",
	}

	n, err := migrate.Exec(db, m.mapDialect(), source, migrate.Up)
	if err != nil {
		return 0, fmt.Errorf("executing migrations: %w", err)
	}

	return n, nil
}

// Rollback rolls back the last migration.
// It returns the number of rolled back migrations and any error encountered.
func (m *MigrationSource) Rollback(db *sql.DB) (int, error) {
	source := &migrate.EmbedFileSystemMigrationSource{
		FileSystem: commonFS,
		Root:       "common",
	}

	n, err := migrate.Exec(db, m.mapDialect(), source, migrate.Down)
	if err != nil {
		return 0, fmt.Errorf("rolling back migration: %w", err)
	}

	return n, nil
}

// GetApplied returns a list of already applied migrations.
func (m *MigrationSource) GetApplied(db *sql.DB) ([]*migrate.MigrationRecord, error) {
	records, err := migrate.GetMigrationRecords(db, m.mapDialect())
	if err != nil {
		return nil, fmt.Errorf("getting migration records: %w", err)
	}

	return records, nil
}
