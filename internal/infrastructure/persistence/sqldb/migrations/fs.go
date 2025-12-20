package migrations

import "embed"

//go:embed postgres/*.sql
var PostgresFS embed.FS

//go:embed oracle/*.sql
var OracleFS embed.FS
