package domain

type InstrumentType string

const (
	InstrumentTypeStock InstrumentType = "stock"
	InstrumentTypeETF   InstrumentType = "etf"
)

type Instrument struct {
	ISIN     string         `json:"isin" gorm:"primaryKey"`
	Symbol   string         `json:"symbol"`
	Name     string         `json:"name"`
	Type     InstrumentType `json:"type"`
	Currency string         `json:"currency"`
	Exchange string         `json:"exchange"`
}

func NewInstrument(isin, symbol, name string, instrumentType InstrumentType, currency, exchange string) Instrument {
	return Instrument{
		ISIN:     isin,
		Symbol:   symbol,
		Name:     name,
		Type:     instrumentType,
		Currency: currency,
		Exchange: exchange,
	}
}

func (i Instrument) IsValid() bool {
	return i.ISIN != "" && i.Symbol != ""
}
