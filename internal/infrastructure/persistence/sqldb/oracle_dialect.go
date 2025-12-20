package sqldb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/jmanzanog/stock-tracker/internal/domain"
	"github.com/jmanzanog/stock-tracker/internal/infrastructure/persistence/sqldb/migrations"
)

type OracleDialect struct{}

func (d *OracleDialect) Name() string { return "oracle" }

func (d *OracleDialect) Migrate(ctx context.Context, db *sql.DB) error {
	// Goose does not support Oracle natively in a way that is easy to cross-compile with go-ora.
	// We use the same pattern: read the SQL file and execute it.
	content, err := migrations.OracleFS.ReadFile("oracle/20240101000000_init.sql")
	if err != nil {
		return fmt.Errorf("reading migration file: %w", err)
	}

	// Split statements by '/' which is standard in Oracle scripts
	statements := strings.Split(string(content), "/")

	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}

		if _, err := db.ExecContext(ctx, stmt); err != nil {
			// ORA-00955: name is already used by an existing object
			if !strings.Contains(err.Error(), "ORA-00955") {
				return fmt.Errorf("migrating: %s: %w", stmt, err)
			}
		}
	}
	return nil
}

func (d *OracleDialect) UpsertPortfolio(ctx context.Context, tx *sql.Tx, p *domain.Portfolio) error {
	query := `MERGE INTO portfolios p
             USING (SELECT :1 as id_val FROM dual) s
             ON (p.id = s.id_val)
             WHEN MATCHED THEN
               UPDATE SET name = :2, last_updated = :3
             WHEN NOT MATCHED THEN
               INSERT (id, name, last_updated, created_at)
               VALUES (:4, :5, :6, :7)`

	_, err := tx.ExecContext(ctx, query,
		p.ID,          // 1 (s.id_val)
		p.Name,        // 2 (UPDATE)
		p.LastUpdated, // 3 (UPDATE)
		p.ID,          // 4 (INSERT)
		p.Name,        // 5 (INSERT)
		p.LastUpdated, // 6 (INSERT)
		p.CreatedAt,   // 7 (INSERT)
	)
	return err
}

func (d *OracleDialect) UpsertInstrument(ctx context.Context, tx *sql.Tx, i *domain.Instrument) error {
	// Oracle MERGE requires UPDATE clause usually or since 10g can be omitted?
	// It supports INSERT ONLY (MERGE ... WHEN NOT MATCHED THEN INSERT ...).
	// So we can omit WHEN MATCHED.

	query := `MERGE INTO instruments i
             USING (SELECT :1 as isin_val FROM dual) s
             ON (i.isin = s.isin_val)
             WHEN NOT MATCHED THEN
               INSERT (isin, symbol, name, type, currency, exchange)
               VALUES (:2, :3, :4, :5, :6, :7)`

	_, err := tx.ExecContext(ctx, query,
		i.ISIN,         // 1
		i.ISIN,         // 2 (INSERT)
		i.Symbol,       // 3
		i.Name,         // 4
		string(i.Type), // 5
		i.Currency,     // 6
		i.Exchange,     // 7
	)
	return err
}

func (d *OracleDialect) UpsertPosition(ctx context.Context, tx *sql.Tx, p *domain.Position) error {
	query := `MERGE INTO positions t
             USING (SELECT :1 as id_val FROM dual) s
             ON (t.id = s.id_val)
             WHEN MATCHED THEN
               UPDATE SET 
                 invested_amount = :2,
                 quantity = :3,
                 current_price = :4,
                 last_updated = :5,
                 portfolio_id = :6
             WHEN NOT MATCHED THEN
               INSERT (id, portfolio_id, instrument_isin, invested_amount, invested_currency, quantity, current_price, last_updated)
               VALUES (:7, :8, :9, :10, :11, :12, :13, :14)`

	_, err := tx.ExecContext(ctx, query,
		p.ID,               // 1
		p.InvestedAmount,   // 2 (UPDATE)
		p.Quantity,         // 3
		p.CurrentPrice,     // 4
		p.LastUpdated,      // 5
		p.PortfolioID,      // 6
		p.ID,               // 7 (INSERT)
		p.PortfolioID,      // 8
		p.Instrument.ISIN,  // 9
		p.InvestedAmount,   // 10
		p.InvestedCurrency, // 11
		p.Quantity,         // 12
		p.CurrentPrice,     // 13
		p.LastUpdated,      // 14
	)
	return err
}
