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

func (r *Repository) AutoMigrate() error {
	return r.db.Dialect.Migrate(context.Background(), r.db.DB)
}

func (r *Repository) Save(ctx context.Context, p *domain.Portfolio) error {
	p.LastUpdated = time.Now()
	return r.db.WithTx(ctx, func(tx *sql.Tx) error {
		if err := r.db.Dialect.UpsertPortfolio(ctx, tx, p); err != nil {
			slog.Error("Failed to save portfolio", "portfolio_id", p.ID, "error", err)
			return fmt.Errorf("upsert portfolio: %w", err)
		}

		newPositionIDs := make(map[string]bool)
		for _, pos := range p.Positions {
			newPositionIDs[pos.ID] = true
		}

		for i := range p.Positions {
			if err := r.db.Dialect.UpsertInstrument(ctx, tx, &p.Positions[i].Instrument); err != nil {
				slog.Error("Failed to save instrument", "isin", p.Positions[i].Instrument.ISIN, "error", err)
				return fmt.Errorf("upsert instrument: %w", err)
			}

			p.Positions[i].PortfolioID = p.ID

			if err := r.db.Dialect.UpsertPosition(ctx, tx, &p.Positions[i]); err != nil {
				slog.Error("Failed to save position", "position_id", p.Positions[i].ID, "error", err)
				return fmt.Errorf("upsert position: %w", err)
			}
		}

		existingRows, err := tx.QueryContext(ctx, r.rebind("SELECT id FROM positions WHERE portfolio_id = $1"), p.ID)
		if err != nil {
			return fmt.Errorf("query existing positions for orphan cleanup: %w", err)
		}
		var orphans []string
		for existingRows.Next() {
			var id string
			if err := existingRows.Scan(&id); err != nil {
				_ = existingRows.Close()
				return fmt.Errorf("scanning position id: %w", err)
			}
			if !newPositionIDs[id] {
				orphans = append(orphans, id)
			}
		}
		if err := existingRows.Err(); err != nil {
			_ = existingRows.Close()
			return fmt.Errorf("iterating positions: %w", err)
		}
		_ = existingRows.Close()
		for _, id := range orphans {
			if _, err := tx.ExecContext(ctx, r.rebind("DELETE FROM positions WHERE id = $1"), id); err != nil {
				return fmt.Errorf("deleting orphaned position %s: %w", id, err)
			}
			slog.Debug("Deleted orphaned position", "position_id", id, "portfolio_id", p.ID)
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
	var portfolios []*domain.Portfolio
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
		q1 := r.rebind("DELETE FROM positions WHERE portfolio_id = $1")
		if _, err := tx.ExecContext(ctx, q1, id); err != nil {
			return fmt.Errorf("failed to delete positions: %w", err)
		}

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
