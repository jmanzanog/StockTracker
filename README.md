# Stock Tracker

A Go-based application for tracking and analyzing financial instruments (ETFs and stocks) using ISIN codes. The application provides real-time price updates and portfolio management through a REST API.

## Features

- 🔍 **ISIN Lookup**: Search for financial instruments by ISIN code
- 💰 **Portfolio Management**: Add, remove, and track multiple positions
- 📊 **Real-time Updates**: Automatic price refresh at configurable intervals
- 📈 **P/L Tracking**: Calculate profit/loss for individual positions and entire portfolio
- 🌐 **REST API**: HTTP endpoints for easy integration
- 🏗️ **Clean Architecture**: Domain-driven design with clear separation of concerns

## Architecture

The project follows **Clean Architecture** principles with the following layers:

```
stock-tracker/
├── cmd/tracker/              # Application entry point
├── internal/
│   ├── domain/              # Business entities and logic
│   ├── application/         # Use cases and services
│   ├── infrastructure/      # External integrations
│   │   ├── marketdata/     # Market data providers
│   │   ├── persistence/    # Data repositories
│   │   └── config/         # Configuration
│   └── interfaces/         # HTTP handlers and routes
```

## Prerequisites

- Go 1.21 or higher
- Twelve Data API key (free tier available at [twelvedata.com](https://twelvedata.com))

## Installation

1. Clone the repository:
```bash
git clone https://github.com/josemanzano/stock-tracker.git
cd stock-tracker
```

2. Install dependencies:
```bash
go mod download
```

3. Create a `.env` file from the example:
```bash
cp .env.example .env
```

4. Edit `.env` and add your Twelve Data API key:
```
TWELVE_DATA_API_KEY=your_api_key_here
```

## Running the Application

### Local Development

```bash
go run cmd/tracker/main.go
```

The server will start on `http://localhost:8080`

### Using Docker

Build the image:
```bash
docker build -t stock-tracker .
```

Run the container:
```bash
docker run -p 8080:8080 --env-file .env stock-tracker
```

## API Endpoints

### Health Check
```http
GET /health
```

### Add Position
```http
POST /api/v1/positions
Content-Type: application/json

{
  "isin": "US0378331005",
  "invested_amount": "10000",
  "currency": "USD"
}
```

### List All Positions
```http
GET /api/v1/positions
```

### Get Position Details
```http
GET /api/v1/positions/:id
```

### Delete Position
```http
DELETE /api/v1/positions/:id
```

### Get Portfolio Summary
```http
GET /api/v1/portfolio
```

Response includes:
- Total value
- Total invested
- Total profit/loss
- Total profit/loss percentage
- All positions

### Manually Refresh Prices
```http
POST /api/v1/portfolio/refresh
```

## Configuration

Environment variables (see `.env.example`):

| Variable | Description | Default |
|----------|-------------|---------|
| `TWELVE_DATA_API_KEY` | API key for Twelve Data | *required* |
| `SERVER_PORT` | HTTP server port | `8080` |
| `SERVER_HOST` | HTTP server host | `localhost` |
| `PRICE_REFRESH_INTERVAL` | Auto-refresh interval | `60s` |
| `LOG_LEVEL` | Logging level | `info` |

## Testing

Run all tests:
```bash
go test ./...
```

Run tests with coverage:
```bash
go test -cover ./...
```

Run tests for a specific package:
```bash
go test ./internal/domain
```

## Example Usage

### Adding a Position

```bash
curl -X POST http://localhost:8080/api/v1/positions \
  -H "Content-Type: application/json" \
  -d '{
    "isin": "US0378331005",
    "invested_amount": "10000",
    "currency": "USD"
  }'
```

### Viewing Portfolio

```bash
curl http://localhost:8080/api/v1/portfolio
```

Example response:
```json
{
  "id": "abc-123",
  "name": "default",
  "positions": [...],
  "total_value": "12500.00",
  "total_invested": "10000.00",
  "total_profit_loss": "2500.00",
  "total_profit_loss_percent": "25.00",
  "created_at": "2025-12-11T20:00:00Z"
}
```

## Market Data Provider

The application uses **Twelve Data** API for market data:

- ✅ Native ISIN support
- ✅ Global coverage (stocks, ETFs, forex, crypto)
- ✅ Free tier: 800 requests/day, 8 requests/minute
- ✅ 15-minute delayed data on free plan

The provider is abstracted behind the `MarketDataProvider` interface, making it easy to swap providers in the future.

## Development

### Project Structure

- **Domain Layer**: Pure business logic, no external dependencies
- **Application Layer**: Use case orchestration
- **Infrastructure Layer**: External integrations (API clients, databases)
- **Interfaces Layer**: HTTP handlers and routing

### Adding a New Market Data Provider

1. Implement the `MarketDataProvider` interface in `internal/infrastructure/marketdata/`
2. Update dependency injection in `cmd/tracker/main.go`

## License

MIT

## Contributing

Contributions are welcome! Please open an issue or submit a pull request.
