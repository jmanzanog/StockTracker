package sqldb

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jmanzanog/stock-tracker/internal/domain"
)

type Repository struct {
	db *DB
}

func NewRepository(db *DB) *Repository {
	return &Repository{db: db}
}

// AutoMigrate is replaced by explicit migration call in main, but strictly speaking
// the repository interface didn't have AutoMigrate, the struct did.
// The main.go calls r.AutoMigrate().
// I will add it here for compatibility during refactoring, but it will use Dialect.Migrate.
func (r *Repository) AutoMigrate() error {
	return r.db.Dialect.Migrate(context.Background(), r.db.DB)
}

func (r *Repository) Save(ctx context.Context, p *domain.Portfolio) error {
	return r.db.WithTx(ctx, func(tx *sql.Tx) error {
		// 1. Upsert Portfolio
		if err := r.db.Dialect.UpsertPortfolio(ctx, tx, p); err != nil {
			slog.Error("Failed to save portfolio", "portfolio_id", p.ID, "error", err)
			return fmt.Errorf("upsert portfolio: %w", err)
		}

		// 2. Upsert Instruments and Positions
		for i := range p.Positions {
			// Ensure instrument exists
			if err := r.db.Dialect.UpsertInstrument(ctx, tx, &p.Positions[i].Instrument); err != nil {
				slog.Error("Failed to save instrument", "isin", p.Positions[i].Instrument.ISIN, "error", err)
				return fmt.Errorf("upsert instrument: %w", err)
			}

			// Ensure portfolio ID is set
			p.Positions[i].PortfolioID = p.ID

			// Upsert Position
			if err := r.db.Dialect.UpsertPosition(ctx, tx, &p.Positions[i]); err != nil {
				slog.Error("Failed to save position", "position_id", p.Positions[i].ID, "error", err)
				return fmt.Errorf("upsert position: %w", err)
			}
		}
		return nil
	})
}

func (r *Repository) FindByID(ctx context.Context, id string) (*domain.Portfolio, error) {
	query := `
        SELECT
            p.id, p.name, p.last_updated, p.created_at,
            pos.id, pos.portfolio_id, pos.instrument_isin, pos.invested_amount, pos.invested_currency, pos.quantity, pos.current_price, pos.last_updated,
            i.isin, i.symbol, i.name, i.type, i.currency, i.exchange
        FROM portfolios p
        LEFT JOIN positions pos ON p.id = pos.portfolio_id
        LEFT JOIN instruments i ON pos.instrument_isin = i.isin
        WHERE p.id = $1
    `
	query = r.rebind(query)

	rows, err := r.db.QueryContext(ctx, query, id)
	if err != nil {
		slog.Error("Failed to find portfolio", "id", id, "error", err)
		return nil, fmt.Errorf("querying portfolio: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			slog.Error("Failed to close rows", "error", err)
		}
	}(rows)

	var portfolio *domain.Portfolio

	for rows.Next() {
		var pID, pName string
		var pLastTime, pCreateTime time.Time
		var posID, posPortID, posInstISIN sql.NullString
		var posInvAmt, posQty, posPrice domain.Decimal
		var posInvCurr sql.NullString
		var posLast sql.NullTime
		var iISIN, iSym, iName, iType, iCurr, iExch sql.NullString

		err := rows.Scan(
			&pID, &pName, &pLastTime, &pCreateTime,
			&posID, &posPortID, &posInstISIN, &posInvAmt, &posInvCurr, &posQty, &posPrice, &posLast,
			&iISIN, &iSym, &iName, &iType, &iCurr, &iExch,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		if portfolio == nil {
			portfolio = &domain.Portfolio{
				ID:          pID,
				Name:        pName,
				LastUpdated: pLastTime,
				CreatedAt:   pCreateTime,
				Positions:   []domain.Position{},
			}
		}

		if posID.Valid {
			inst := domain.Instrument{
				ISIN:     iISIN.String,
				Symbol:   iSym.String,
				Name:     iName.String,
				Type:     domain.InstrumentType(iType.String),
				Currency: iCurr.String,
				Exchange: iExch.String,
			}

			pos := domain.Position{
				ID:               posID.String,
				PortfolioID:      posPortID.String,
				InstrumentISIN:   posInstISIN.String,
				Instrument:       inst,
				InvestedAmount:   posInvAmt,
				InvestedCurrency: posInvCurr.String,
				Quantity:         posQty,
				CurrentPrice:     posPrice,
				LastUpdated:      posLast.Time,
			}
			portfolio.Positions = append(portfolio.Positions, pos)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if portfolio == nil {
		slog.Debug("Portfolio not found", "id", id)
		return nil, fmt.Errorf("portfolio not found: %s", id)
	}

	return portfolio, nil
}

func (r *Repository) FindAll(ctx context.Context) ([]*domain.Portfolio, error) {
	query := `
        SELECT
            p.id, p.name, p.last_updated, p.created_at,
            pos.id, pos.portfolio_id, pos.instrument_isin, pos.invested_amount, pos.invested_currency, pos.quantity, pos.current_price, pos.last_updated,
            i.isin, i.symbol, i.name, i.type, i.currency, i.exchange
        FROM portfolios p
        LEFT JOIN positions pos ON p.id = pos.portfolio_id
        LEFT JOIN instruments i ON pos.instrument_isin = i.isin
        ORDER BY p.created_at DESC
    `
	// Note: added ORDER BY for stability
	query = r.rebind(query)

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying portfolios: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			slog.Error("Failed to close rows", "error", err)
		}
	}(rows)

	portfolioMap := make(map[string]*domain.Portfolio)
	var portfolios []*domain.Portfolio // To keep order if needed, but map invalidates order.
	// To keep order, we can track IDs.
	var ids []string

	for rows.Next() {
		var pID, pName string
		var pLastTime, pCreateTime time.Time
		var posID, posPortID, posInstISIN sql.NullString
		var posInvAmt, posQty, posPrice domain.Decimal
		var posInvCurr sql.NullString
		var posLast sql.NullTime
		var iISIN, iSym, iName, iType, iCurr, iExch sql.NullString

		err := rows.Scan(
			&pID, &pName, &pLastTime, &pCreateTime,
			&posID, &posPortID, &posInstISIN, &posInvAmt, &posInvCurr, &posQty, &posPrice, &posLast,
			&iISIN, &iSym, &iName, &iType, &iCurr, &iExch,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		p, exists := portfolioMap[pID]
		if !exists {
			p = &domain.Portfolio{
				ID:          pID,
				Name:        pName,
				LastUpdated: pLastTime,
				CreatedAt:   pCreateTime,
				Positions:   []domain.Position{},
			}
			portfolioMap[pID] = p
			ids = append(ids, pID)
		}

		if posID.Valid {
			inst := domain.Instrument{
				ISIN:     iISIN.String,
				Symbol:   iSym.String,
				Name:     iName.String,
				Type:     domain.InstrumentType(iType.String),
				Currency: iCurr.String,
				Exchange: iExch.String,
			}

			pos := domain.Position{
				ID:               posID.String,
				PortfolioID:      posPortID.String,
				InstrumentISIN:   posInstISIN.String,
				Instrument:       inst,
				InvestedAmount:   posInvAmt,
				InvestedCurrency: posInvCurr.String,
				Quantity:         posQty,
				CurrentPrice:     posPrice,
				LastUpdated:      posLast.Time,
			}
			p.Positions = append(p.Positions, pos)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, id := range ids {
		portfolios = append(portfolios, portfolioMap[id])
	}

	return portfolios, nil
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	return r.db.WithTx(ctx, func(tx *sql.Tx) error {
		// 1. Delete Positions
		q1 := r.rebind("DELETE FROM positions WHERE portfolio_id = $1")
		if _, err := tx.ExecContext(ctx, q1, id); err != nil {
			return fmt.Errorf("failed to delete positions: %w", err)
		}

		// 2. Delete Portfolio
		q2 := r.rebind("DELETE FROM portfolios WHERE id = $1")
		if _, err := tx.ExecContext(ctx, q2, id); err != nil {
			return fmt.Errorf("failed to delete portfolio: %w", err)
		}

		return nil
	})
}

func (r *Repository) rebind(query string) string {
	if r.db.Dialect.Name() == "oracle" {
		for i := 1; i <= 10; i++ {
			query = strings.ReplaceAll(query, fmt.Sprintf("$%d", i), fmt.Sprintf(":%d", i))
		}
	}
	return query
}
