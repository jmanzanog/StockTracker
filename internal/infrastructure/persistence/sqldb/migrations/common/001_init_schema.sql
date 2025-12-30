-- +migrate Up
-- Common migration for PostgreSQL and Oracle
-- This file uses ANSI SQL compatible with both databases

CREATE TABLE IF NOT EXISTS instruments (
    isin VARCHAR(50) PRIMARY KEY,
    symbol VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL,
    currency VARCHAR(10) NOT NULL,
    exchange VARCHAR(50) NOT NULL
);

CREATE TABLE IF NOT EXISTS portfolios (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    last_updated TIMESTAMP,
    created_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS positions (
    id VARCHAR(36) PRIMARY KEY,
    portfolio_id VARCHAR(36) NOT NULL,
    instrument_isin VARCHAR(50) NOT NULL,
    invested_amount NUMERIC NOT NULL,
    invested_currency VARCHAR(10) NOT NULL,
    quantity NUMERIC NOT NULL,
    current_price NUMERIC NOT NULL,
    last_updated TIMESTAMP,
    CONSTRAINT fk_pos_port FOREIGN KEY (portfolio_id) REFERENCES portfolios(id) ON DELETE CASCADE,
    CONSTRAINT fk_pos_inst FOREIGN KEY (instrument_isin) REFERENCES instruments(isin)
);

-- +migrate Down
DROP TABLE IF EXISTS positions;
DROP TABLE IF EXISTS portfolios;
DROP TABLE IF EXISTS instruments;
