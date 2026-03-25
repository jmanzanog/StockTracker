-- +migrate Up
-- Add sector to instruments table
ALTER TABLE instruments ADD sector VARCHAR(100);

-- Create price_history table for sparkline data
CREATE TABLE IF NOT EXISTS price_history (
    id              VARCHAR(36) PRIMARY KEY,
    instrument_isin VARCHAR(50) NOT NULL,
    price           NUMERIC NOT NULL,
    currency        VARCHAR(10) NOT NULL,
    recorded_at     TIMESTAMP NOT NULL,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_ph_inst FOREIGN KEY (instrument_isin) REFERENCES instruments(isin) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_price_history_isin_recorded ON price_history(instrument_isin, recorded_at DESC);
CREATE INDEX IF NOT EXISTS idx_price_history_recorded_at ON price_history(recorded_at);

-- +migrate Down
DROP INDEX idx_price_history_recorded_at;
DROP INDEX idx_price_history_isin_recorded;
DROP TABLE price_history;
ALTER TABLE instruments DROP COLUMN sector;