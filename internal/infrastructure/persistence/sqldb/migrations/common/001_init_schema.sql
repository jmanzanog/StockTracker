-- +migrate Up

-- Instruments table
CREATE TABLE IF NOT EXISTS instruments (
    isin VARCHAR(12) PRIMARY KEY,
    symbol VARCHAR(20) NOT NULL,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(20) NOT NULL,
    currency VARCHAR(3) NOT NULL,
    exchange VARCHAR(50)
);

-- Portfolios table
CREATE TABLE IF NOT EXISTS portfolios (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    last_updated TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Positions table
CREATE TABLE IF NOT EXISTS positions (
    id VARCHAR(36) PRIMARY KEY,
    portfolio_id VARCHAR(36) NOT NULL,
    instrument_isin VARCHAR(12) NOT NULL,
    invested_amount DECIMAL(19,4) NOT NULL,
    invested_currency VARCHAR(3) NOT NULL,
    quantity DECIMAL(19,4) NOT NULL,
    current_price DECIMAL(19,4),
    last_updated TIMESTAMP WITH TIME ZONE,
    CONSTRAINT fk_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios(id),
    CONSTRAINT fk_instrument FOREIGN KEY (instrument_isin) REFERENCES instruments(isin)
);

-- +migrate Down
DROP TABLE IF EXISTS positions;
DROP TABLE IF EXISTS portfolios;
DROP TABLE IF EXISTS instruments;
