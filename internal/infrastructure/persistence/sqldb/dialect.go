package sqldb

import (
	"context"
	"database/sql"

	"github.com/jmanzanog/stock-tracker/internal/domain"
)

type Dialect interface {
	Name() string
	Migrate(ctx context.Context, db *sql.DB) error
	UpsertPortfolio(ctx context.Context, tx *sql.Tx, p *domain.Portfolio) error
	UpsertInstrument(ctx context.Context, tx *sql.Tx, i *domain.Instrument) error
	UpsertPosition(ctx context.Context, tx *sql.Tx, p *domain.Position) error
}
