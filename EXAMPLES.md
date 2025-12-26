# Example API Usage

This file contains example curl commands to test the Stock Tracker API.

## Prerequisites

1. Start the server:
```bash
go run cmd/tracker/main.go
```

2. Make sure you have a valid API key in your `.env` file (TwelveData, Finnhub, or YFinance service running).

## Examples

### 1. Health Check
```bash
curl http://localhost:8080/health
```

Expected response:
```json
{"status":"ok"}
```

### 2. Add Position - Apple Stock (AAPL)
```bash
curl -X POST http://localhost:8080/api/v1/positions \
  -H "Content-Type: application/json" \
  -d '{
    "isin": "US0378331005",
    "invested_amount": "10000",
    "currency": "USD"
  }'
```

### 3. Add Position - iShares Core MSCI World ETF
```bash
curl -X POST http://localhost:8080/api/v1/positions \
  -H "Content-Type: application/json" \
  -d '{
    "isin": "IE00B4L5Y983",
    "invested_amount": "5000",
    "currency": "EUR"
  }'
```

### 4. Add Multiple Positions (Batch)
Add multiple positions in a single request. Ideal for bulk portfolio creation.

```bash
curl -X POST http://localhost:8080/api/v1/positions/batch \
  -H "Content-Type: application/json" \
  -d '[
    {"isin": "US0378331005", "invested_amount": "10000", "currency": "USD"},
    {"isin": "IE00B4L5Y983", "invested_amount": "5000", "currency": "EUR"},
    {"isin": "US5949181045", "invested_amount": "8000", "currency": "USD"}
  ]'
```

Expected response (HTTP 201 for full success, 207 for partial):
```json
{
  "successful": [
    {"isin": "US0378331005", "position": {"id": "...", "instrument": {...}, ...}},
    {"isin": "US5949181045", "position": {"id": "...", "instrument": {...}, ...}}
  ],
  "failed": [
    {"isin": "IE00B4L5Y983", "error": "no instrument found for ISIN: IE00B4L5Y983"}
  ]
}
```

### 5. List All Positions
```bash
curl http://localhost:8080/api/v1/positions
```

### 6. Get Portfolio Summary
```bash
curl http://localhost:8080/api/v1/portfolio
```

Expected response includes:
- `total_value`: Current market value of all positions
- `total_invested`: Total amount invested
- `total_profit_loss`: Absolute profit/loss
- `total_profit_loss_percent`: Percentage profit/loss
- `positions`: Array of all positions with details

### 7. Get Specific Position
```bash
# Replace {position-id} with actual ID from previous responses
curl http://localhost:8080/api/v1/positions/{position-id}
```

### 8. Manually Refresh Prices
```bash
curl -X POST http://localhost:8080/api/v1/portfolio/refresh
```

### 9. Delete Position
```bash
# Replace {position-id} with actual ID
curl -X DELETE http://localhost:8080/api/v1/positions/{position-id}
```

## Notes

- Prices are automatically refreshed every 60 seconds (configurable via `PRICE_REFRESH_INTERVAL`)
- **Rate limits by provider**:
  - TwelveData: 8 credits/min (each symbol = 1 credit)
  - Finnhub: 60 req/min, 30 req/s
  - YFinance: Self-hosted, no external limits
- **Batch operations**: Use `/positions/batch` for adding multiple positions efficiently
  - YFinance provider uses true batch API calls
  - Other providers use concurrent processing with goroutines
- ISINs must be valid and recognized by the configured market data provider
- All monetary values use `decimal.Decimal` for precision

