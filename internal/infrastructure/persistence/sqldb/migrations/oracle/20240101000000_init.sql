CREATE TABLE instruments (
    isin VARCHAR2(50) PRIMARY KEY,
    symbol VARCHAR2(50) NOT NULL,
    name VARCHAR2(255) NOT NULL,
    type VARCHAR2(50) NOT NULL,
    currency VARCHAR2(10) NOT NULL,
    exchange VARCHAR2(50) NOT NULL
);
/
CREATE TABLE portfolios (
    id VARCHAR2(36) PRIMARY KEY,
    name VARCHAR2(255) NOT NULL,
    last_updated TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE
);
/
CREATE TABLE positions (
    id VARCHAR2(36) PRIMARY KEY,
    portfolio_id VARCHAR2(36) NOT NULL,
    instrument_isin VARCHAR2(50) NOT NULL,
    invested_amount NUMBER NOT NULL,
    invested_currency VARCHAR2(10) NOT NULL,
    quantity NUMBER NOT NULL,
    current_price NUMBER NOT NULL,
    last_updated TIMESTAMP WITH TIME ZONE,
    CONSTRAINT fk_pos_port FOREIGN KEY (portfolio_id) REFERENCES portfolios(id) ON DELETE CASCADE,
    CONSTRAINT fk_pos_inst FOREIGN KEY (instrument_isin) REFERENCES instruments(isin)
);
/
