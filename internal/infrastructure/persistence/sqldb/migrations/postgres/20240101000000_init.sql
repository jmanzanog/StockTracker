-- +goose Up
CREATE TABLE IF NOT EXISTS instruments (
    isin TEXT PRIMARY KEY,
    symbol TEXT NOT NULL,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    currency TEXT NOT NULL,
    exchange TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS portfolios (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    last_updated TIMESTAMPTZ,
    created_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS positions (
    id TEXT PRIMARY KEY,
    portfolio_id TEXT NOT NULL REFERENCES portfolios(id) ON DELETE CASCADE,
    instrument_isin TEXT NOT NULL REFERENCES instruments(isin) ON DELETE RESTRICT,
    invested_amount NUMERIC NOT NULL,
    invested_currency TEXT NOT NULL,
    quantity NUMERIC NOT NULL,
    current_price NUMERIC NOT NULL,
    last_updated TIMESTAMPTZ
);

-- +goose Down
DROP TABLE IF EXISTS positions;
DROP TABLE IF EXISTS portfolios;
DROP TABLE IF EXISTS instruments;
