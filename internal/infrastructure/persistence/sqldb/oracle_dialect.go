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

func (d *OracleDialect) Migrate(_ context.Context, db *sql.DB) error {
	migrator := migrations.NewMigrationSource("oracle")

	n, err := migrator.Run(db)
	if err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	if n > 0 {
		fmt.Printf("Applied %d migration(s)\n", n)
	}

	return nil
}

func (d *OracleDialect) UpsertPortfolio(ctx context.Context, tx *sql.Tx, p *domain.Portfolio) error {
	// Check if portfolio exists
	var count int
	err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM portfolios WHERE id = :1", p.ID).Scan(&count)
	if err != nil {
		return fmt.Errorf("checking portfolio existence: %w", err)
	}

	if count > 0 {
		// UPDATE existing
		_, err = tx.ExecContext(ctx,
			"UPDATE portfolios SET name = :1, last_updated = :2 WHERE id = :3",
			p.Name, p.LastUpdated, p.ID,
		)
		if err != nil {
			return fmt.Errorf("updating portfolio: %w", err)
		}
	} else {
		// INSERT new
		_, err = tx.ExecContext(ctx,
			"INSERT INTO portfolios (id, name, last_updated, created_at) VALUES (:1, :2, :3, :4)",
			p.ID, p.Name, p.LastUpdated, p.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("inserting portfolio: %w", err)
		}
	}
	return nil
}

func (d *OracleDialect) UpsertInstrument(ctx context.Context, tx *sql.Tx, i *domain.Instrument) error {
	// Check if instrument exists
	var count int
	err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM instruments WHERE isin = :1", i.ISIN).Scan(&count)
	if err != nil {
		return fmt.Errorf("checking instrument existence: %w", err)
	}

	// Only insert if not exists (instruments are immutable by ISIN)
	if count == 0 {
		_, err = tx.ExecContext(ctx,
			"INSERT INTO instruments (isin, symbol, name, type, currency, exchange) VALUES (:1, :2, :3, :4, :5, :6)",
			i.ISIN, i.Symbol, i.Name, string(i.Type), i.Currency, i.Exchange,
		)
		if err != nil {
			// ORA-00001: unique constraint violation - another transaction inserted the same row
			// This is expected in concurrent scenarios, ignore it
			if strings.Contains(err.Error(), "ORA-00001") {
				return nil
			}
			return fmt.Errorf("inserting instrument: %w", err)
		}
	}
	return nil
}

func (d *OracleDialect) UpsertPosition(ctx context.Context, tx *sql.Tx, p *domain.Position) error {
	// Check if position exists
	var count int
	err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM positions WHERE id = :1", p.ID).Scan(&count)
	if err != nil {
		return fmt.Errorf("checking position existence: %w", err)
	}

	if count > 0 {
		// UPDATE existing
		_, err = tx.ExecContext(ctx,
			`UPDATE positions SET 
				invested_amount = :1, quantity = :2, current_price = :3, 
				last_updated = :4, portfolio_id = :5 
			WHERE id = :6`,
			p.InvestedAmount, p.Quantity, p.CurrentPrice,
			p.LastUpdated, p.PortfolioID, p.ID,
		)
		if err != nil {
			return fmt.Errorf("updating position: %w", err)
		}
	} else {
		// INSERT new
		_, err = tx.ExecContext(ctx,
			`INSERT INTO positions 
				(id, portfolio_id, instrument_isin, invested_amount, invested_currency, quantity, current_price, last_updated) 
			VALUES (:1, :2, :3, :4, :5, :6, :7, :8)`,
			p.ID, p.PortfolioID, p.Instrument.ISIN,
			p.InvestedAmount, p.InvestedCurrency, p.Quantity, p.CurrentPrice, p.LastUpdated,
		)
		if err != nil {
			return fmt.Errorf("inserting position: %w", err)
		}
	}
	return nil
}
