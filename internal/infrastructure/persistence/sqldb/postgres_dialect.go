package sqldb

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmanzanog/stock-tracker/internal/domain"
	"github.com/jmanzanog/stock-tracker/internal/infrastructure/persistence/sqldb/migrations"
)

type PostgresDialect struct{}

func (d *PostgresDialect) Name() string { return "postgres" }

func (d *PostgresDialect) Migrate(_ context.Context, db *sql.DB) error {
	migrator := migrations.NewMigrationSource("postgres")

	n, err := migrator.Run(db)
	if err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	if n > 0 {
		fmt.Printf("Applied %d migration(s)\n", n)
	}

	return nil
}

func (d *PostgresDialect) UpsertPortfolio(ctx context.Context, tx *sql.Tx, p *domain.Portfolio) error {
	query := `
		INSERT INTO portfolios (id, name, last_updated, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			last_updated = EXCLUDED.last_updated
	`
	_, err := tx.ExecContext(ctx, query, p.ID, p.Name, p.LastUpdated, p.CreatedAt)
	return err
}

func (d *PostgresDialect) UpsertInstrument(ctx context.Context, tx *sql.Tx, i *domain.Instrument) error {
	query := `
		INSERT INTO instruments (isin, symbol, name, type, currency, exchange)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (isin) DO NOTHING
	`
	_, err := tx.ExecContext(ctx, query, i.ISIN, i.Symbol, i.Name, i.Type, i.Currency, i.Exchange)
	return err
}

func (d *PostgresDialect) UpsertPosition(ctx context.Context, tx *sql.Tx, p *domain.Position) error {
	query := `
		INSERT INTO positions (id, portfolio_id, instrument_isin, invested_amount, invested_currency, quantity, current_price, last_updated)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			invested_amount = EXCLUDED.invested_amount,
			quantity = EXCLUDED.quantity,
			current_price = EXCLUDED.current_price,
			last_updated = EXCLUDED.last_updated,
            portfolio_id = EXCLUDED.portfolio_id
	`
	_, err := tx.ExecContext(ctx, query, p.ID, p.PortfolioID, p.Instrument.ISIN, p.InvestedAmount, p.InvestedCurrency, p.Quantity, p.CurrentPrice, p.LastUpdated)
	return err
}
