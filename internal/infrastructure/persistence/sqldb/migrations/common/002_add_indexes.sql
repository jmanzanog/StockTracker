-- +migrate Up
-- Add indexes for better query performance

-- Foreign key indexes on positions table (critical for JOINs and CASCADE deletes)
CREATE INDEX IF NOT EXISTS idx_positions_portfolio_id ON positions(portfolio_id);
CREATE INDEX IF NOT EXISTS idx_positions_instrument_isin ON positions(instrument_isin);

-- Index for symbol lookups on instruments table
CREATE INDEX IF NOT EXISTS idx_instruments_symbol ON instruments(symbol);

-- +migrate Down
DROP INDEX IF EXISTS idx_instruments_symbol;
DROP INDEX IF EXISTS idx_positions_instrument_isin;
DROP INDEX IF EXISTS idx_positions_portfolio_id;
