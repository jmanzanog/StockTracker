# Stock Tracker

A Go-based application for tracking and analyzing financial instruments (ETFs and stocks) using ISIN codes. The application provides real-time price updates and portfolio management through a REST API.

## Features

- 🔍 **ISIN Lookup**: Search for financial instruments by ISIN code
- 💰 **Portfolio Management**: Add, remove, and track multiple positions
- 📦 **Batch Operations**: Add multiple positions in a single request with partial failure handling
- 📊 **Real-time Updates**: Automatic price refresh at configurable intervals
- 📈 **P/L Tracking**: Calculate profit/loss for individual positions and entire portfolio
- 📉 **Visual Dashboard**: Portfolio overview with sparkline charts and allocation breakdowns
- 🌐 **REST API**: HTTP endpoints for easy integration
- 🏗️ **Clean Architecture**: Domain-driven design with clear separation of concerns
- 🐳 **Docker Ready**: Full stack containerization with PostgreSQL


## Architecture

The project follows **Clean Architecture (DDD)** principles:

```
stock-tracker/
├── cmd/tracker/              # Application entry point
├── internal/
│   ├── domain/              # Pure Business entities and logic
│   ├── application/         # Use cases and orchestration
│   ├── infrastructure/      # Adapter Implementations (PostgreSQL, Market Data)
│   │   ├── marketdata/      # Market data providers (TwelveData, Finnhub, YFinance)
│   │   ├── persistence/     # SQL Repositories (PostgreSQL, Oracle)
│   │   └── config/          # Configuration loading
│   └── interfaces/          # HTTP Ports (Gin Handlers)
└── docker-compose.yml       # Infrastructure orchestration
```

## Prerequisites

- **Go 1.22+**
- **Docker & Docker Compose** (Recommended for full stack)
- **PostgreSQL 15+** (Or use the Docker container provided)
- Market Data API Key (one of the following):
  - [Twelve Data API Key](https://twelvedata.com/) - Default provider (8 credits/min free tier)
  - [Finnhub API Key](https://finnhub.io/) - Alternative provider (60 req/min free tier)
  - **YFinance Market Data Service** - Self-hosted Python microservice (no API key required, supports batch)

### Market Data Provider Comparison

| Provider | Batch API | Rate Limits (Free) | Notes |
|----------|-----------|-------------------|-------|
| **TwelveData** | ✅ Yes | 8 credits/min, 800/day | Each symbol = 1 credit |
| **Finnhub** | ❌ No | 60 req/min, 30 req/s | Uses concurrent fallback |
| **YFinance** | ✅ Yes | Self-hosted (no limit) | Best for batch operations |


## Domain Logic

### Portfolio Management
- **Automatic Position Merging**: If you add a position for an Instrument (ISIN) that is already in your portfolio, the system will automatically merge it:
  - `Invested Amount`: Summed with existing amount.
  - `Quantity`: Summed with existing quantity.
  - `Current Price`: Updated to the latest market price.
  - **No Duplicates**: A portfolio cannot have two separate entries for the same ISIN.

### Price History & Sparklines
- **Automatic Capture**: Price data is captured every 60 seconds into the `price_history` table
- **Sparkline Data**: Historical prices are used to generate sparkline charts for the dashboard endpoint
- **Data Retention**: Old price history is automatically cleaned up after 90 days
- **Sector Information**: Instruments include a sector field (may be empty/"N/A" if not provided by market data API)

## Installation

1. Clone the repository:
```bash
git clone https://github.com/jmanzanog/stock-tracker.git
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

4. Edit `.env` and add your keys:
```env
# Market Data Provider: "twelvedata" (default), "finnhub", or "yfinance"
MARKET_DATA_PROVIDER=twelvedata

# TwelveData API Key (required if MARKET_DATA_PROVIDER=twelvedata)
TWELVE_DATA_API_KEY=your_key

# Finnhub API Key (required if MARKET_DATA_PROVIDER=finnhub)
# FINNHUB_API_KEY=your_key

# YFinance Service URL (required if MARKET_DATA_PROVIDER=yfinance)
# YFINANCE_BASE_URL=http://localhost:8000

# Database config is pre-set for local docker dev
```

## Running the Application

### Option A: Docker Compose (Recommended)

This starts both the PostgreSQL database and the Application in containers.

```bash
docker compose --profile deployment up --build
```
- App URL: `http://localhost:8080`
- Database: Persisted in `./postgres_data` volume.

### Option B: Hybrid Mode (Local App + Docker DB)

Ideal for development and debugging requiring database.

1. Start only the database:
   ```bash
   docker compose up -d
   ```
2. Run the application locally:
   ```bash
   go run cmd/tracker/main.go
   ```

**Note**: Ensure your local `.env` has `DB_HOST=localhost` for this mode.

### Option C: Pure Local

Requires a local PostgreSQL instance running.

```bash
export DB_DSN="host=localhost user=postgres password=... dbname=stocktracker"
go run cmd/tracker/main.go
```

## API Endpoints

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

### Add Positions (Batch)
Add multiple positions in a single request. The API uses batch operations when supported by the market data provider (YFinance), or falls back to concurrent processing (Finnhub/TwelveData).

```http
POST /api/v1/positions/batch
Content-Type: application/json

[
  {"isin": "US0378331005", "invested_amount": "10000", "currency": "USD"},
  {"isin": "IE00B4L5Y983", "invested_amount": "5000", "currency": "EUR"},
  {"isin": "US5949181045", "invested_amount": "8000", "currency": "USD"}
]
```

**Response** (HTTP 201 for success, 207 for partial success):
```json
{
  "successful": [
    {"isin": "US0378331005", "position": {...}},
    {"isin": "US5949181045", "position": {...}}
  ],
  "failed": [
    {"isin": "IE00B4L5Y983", "error": "instrument not found"}
  ]
}
```

### List All Positions
```http
GET /api/v1/positions
```

### Get Portfolio Summary
```http
GET /api/v1/portfolio
```

### Visual Dashboard (Beta)
Returns portfolio data formatted for visualization with sparkline charts and allocation breakdowns.

```http
GET /api/v1/dashboard?sparklines=7,30,90
```

**Query Parameters:**
- `sparklines`: Comma-separated list of day ranges for sparkline data (default: `7,30,90`, max: 365 days per range, max 5 ranges)

**Response:**
```json
{
  "portfolio_id": "default",
  "generated_at": "2024-01-15T10:30:00Z",
  "total_value": "45000.00",
  "total_invested": "30000.00",
  "total_pnl": "15000.00",
  "pnl_percent": "50.00",
  "by_currency": [
    {"currency": "USD", "total_value": "30000.00", "percent": "66.67"},
    {"currency": "EUR", "total_value": "15000.00", "percent": "33.33"}
  ],
  "by_type": [
    {"type": "stock", "total_value": "25000.00", "percent": "55.56"},
    {"type": "etf", "total_value": "20000.00", "percent": "44.44"}
  ],
  "by_sector": [
    {"sector": "Technology", "total_value": "20000.00", "percent": "44.44"},
    {"sector": "Financial", "total_value": "15000.00", "percent": "33.33"},
    {"sector": "N/A", "total_value": "10000.00", "percent": "22.22"}
  ],
  "positions": [
    {
      "id": "pos-1",
      "isin": "US0378331005",
      "symbol": "AAPL",
      "name": "Apple Inc.",
      "type": "stock",
      "sector": "Technology",
      "quantity": "100.00",
      "current_price": "150.00",
      "current_value": "15000.00",
      "invested_amount": "10000.00",
      "pnl": "5000.00",
      "pnl_percent": "50.00",
      "currency": "USD",
      "sparklines": {
        "7d": [{"date": "2024-01-15T10:00:00Z", "price": "148.50"}, ...],
        "30d": [...],
        "90d": [...]
      }
    }
  ]
}
```

**Notes:**
- Sparklines show price history captured every 60 seconds
- Price history is retained for 90 days and automatically cleaned up
- Sector field may be "N/A" if the instrument provider doesn't supply sector information

## Configuration

Environment variables (see `.env.example`):

| Variable | Description | Default |
|----------|-------------|---------|
| `MARKET_DATA_PROVIDER` | Market data provider (`twelvedata`, `finnhub`, or `yfinance`) | `twelvedata` |
| `TWELVE_DATA_API_KEY` | API key for Twelve Data (required if provider is twelvedata) | - |
| `FINNHUB_API_KEY` | API key for Finnhub (required if provider is finnhub) | - |
| `YFINANCE_BASE_URL` | URL for yfinance microservice (required if provider is yfinance) | `http://localhost:8000` |
| `SERVER_PORT` | HTTP server port | `8080` |
| `SERVER_HOST` | HTTP server host | `localhost` |
| `PRICE_REFRESH_INTERVAL` | Auto-refresh interval | `60s` |
| `LOG_LEVEL` | Logging level | `info` |
| `DB_DRIVER` | Database Driver | `postgres` |
| `DB_DSN` | Connection String | *required* |

## YFinance Market Data Service

The YFinance provider uses a self-hosted Python microservice that wraps the [yfinance](https://github.com/ranaroussi/yfinance) library. This is ideal for:

- **No API key required**: Unlike TwelveData or Finnhub, no registration needed
- **Global coverage**: Supports US, UK, EU, and Asian markets
- **Self-hosted**: Full control over the service and data

### Setup

1. Clone or deploy the Market Data Service:
```bash
# Clone the market-data-service repository
cd market-data-service
docker compose up --build
```

2. Configure StockTracker to use it:
```env
MARKET_DATA_PROVIDER=yfinance
YFINANCE_BASE_URL=http://localhost:8000
```

### Kubernetes Deployment

Example K8s deployment for the Market Data Service:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: market-data-service
spec:
  replicas: 2
  selector:
    matchLabels:
      app: market-data-service
  template:
    metadata:
      labels:
        app: market-data-service
    spec:
      containers:
      - name: market-data-service
        image: ghcr.io/your-username/market-data-service:latest
        ports:
        - containerPort: 8000
        livenessProbe:
          httpGet:
            path: /health
            port: 8000
          initialDelaySeconds: 10
          periodSeconds: 30
        resources:
          limits:
            memory: "256Mi"
            cpu: "500m"
---
apiVersion: v1
kind: Service
metadata:
  name: market-data-service
spec:
  selector:
    app: market-data-service
  ports:
  - port: 8000
    targetPort: 8000
  type: ClusterIP
```

Then configure StockTracker:
```env
MARKET_DATA_PROVIDER=yfinance
YFINANCE_BASE_URL=http://market-data-service:8000
```

## Testing

> **Note**: Integration tests utilize **Testcontainers**, so you must have **Docker** installed and running on your machine to execute them successfully.

The project includes comprehensive test coverage with **optimized reusable containers** for both PostgreSQL and Oracle backends.

### Performance Optimization

Tests use **shared containers** that start only once per test run:
- **PostgreSQL**: ~5 seconds startup, ~5ms per test
- **Oracle**: ~25 seconds startup, ~10ms per test
- **Total for all tests**: ~35 seconds (vs ~30+ minutes without optimization)

### Running Tests

```bash
go test ./...
```
- ✅ Runs all tests against **both PostgreSQL and Oracle**
- ⚡ Fast execution (~35 seconds total)
- 🔄 Containers are started once and reused across all tests

### Test Coverage
Generate coverage report:
```bash
go test -v ./... -race -coverprofile=coverage.txt -covermode=atomic
```

## CI Verification Scripts

To ensure your changes pass the CI checks before pushing, you can use the provided verification scripts. These scripts run `go mod tidy`, `go fmt`, `golangci-lint`, and all unit tests.

### Windows (PowerShell)
```powershell
.\scripts\verify.ps1
```

### Linux / macOS (Bash)
```bash
chmod +x scripts/verify.sh
./scripts/verify.sh
```

## License
MIT
