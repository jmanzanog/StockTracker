# Example API Usage

This file contains example curl commands to test the Stock Tracker API.

## Prerequisites

1. Start the server:
```bash
go run cmd/tracker/main.go
```

2. Make sure you have a valid Twelve Data API key in your `.env` file.

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

### 4. List All Positions
```bash
curl http://localhost:8080/api/v1/positions
```

### 5. Get Portfolio Summary
```bash
curl http://localhost:8080/api/v1/portfolio
```

Expected response includes:
- `total_value`: Current market value of all positions
- `total_invested`: Total amount invested
- `total_profit_loss`: Absolute profit/loss
- `total_profit_loss_percent`: Percentage profit/loss
- `positions`: Array of all positions with details

### 6. Get Specific Position
```bash
# Replace {position-id} with actual ID from previous responses
curl http://localhost:8080/api/v1/positions/{position-id}
```

### 7. Manually Refresh Prices
```bash
curl -X POST http://localhost:8080/api/v1/portfolio/refresh
```

### 8. Delete Position
```bash
# Replace {position-id} with actual ID
curl -X DELETE http://localhost:8080/api/v1/positions/{position-id}
```

## Notes

- Prices are automatically refreshed every 60 seconds (configurable via `PRICE_REFRESH_INTERVAL`)
- The free tier of Twelve Data has a limit of 8 requests/minute
- ISINs must be valid and recognized by Twelve Data
- All monetary values use `decimal.Decimal` for precision
