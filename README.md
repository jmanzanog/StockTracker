# Stock Tracker

A Go-based application for tracking and analyzing financial instruments (ETFs and stocks) using ISIN codes. The application provides real-time price updates and portfolio management through a REST API.

## Features

- 🔍 **ISIN Lookup**: Search for financial instruments by ISIN code
- 💰 **Portfolio Management**: Add, remove, and track multiple positions
- 📊 **Real-time Updates**: Automatic price refresh at configurable intervals
- 📈 **P/L Tracking**: Calculate profit/loss for individual positions and entire portfolio
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
│   ├── infrastructure/      # Adapter Implementations (PostgreSQL, TwelveData)
│   │   ├── marketdata/      # Market data providers
│   │   ├── persistence/     # GORM Repositories
│   │   └── config/          # Configuration loading
│   └── interfaces/          # HTTP Ports (Gin Handlers)
└── docker-compose.yml       # Infrastructure orchestration
```

## Prerequisites

- **Go 1.22+**
- **Docker & Docker Compose** (Recommended for full stack)
- **PostgreSQL 15+** (Or use the Docker container provided)
- [Twelve Data API Key](https://twelvedata.com/)

## Domain Logic

### Portfolio Management
- **Automatic Position Merging**: If you add a position for an Instrument (ISIN) that is already in your portfolio, the system will automatically merge it:
  - `Invested Amount`: Summed with existing amount.
  - `Quantity`: Summed with existing quantity.
  - `Current Price`: Updated to the latest market price.
  - **No Duplicates**: A portfolio cannot have two separate entries for the same ISIN.

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
TWELVE_DATA_API_KEY=your_key
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

### List All Positions
```http
GET /api/v1/positions
```

### Get Portfolio Summary
```http
GET /api/v1/portfolio
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
| `DB_DRIVER` | Database Driver | `postgres` |
| `DB_DSN` | Connection String | *required* |

## Testing

> **Note**: Integration tests utilize **Testcontainers**, so you must have **Docker** installed and running on your machine to execute them successfully.

The project includes comprehensive test coverage with support for multiple database backends (PostgreSQL and Oracle).

### Test Execution Modes

#### 1. Default Mode (Fast) - PostgreSQL Only
Perfect for rapid local development and CI pipelines:
```bash
go test ./...
```
- ✅ Runs all tests including integration tests against **PostgreSQL only**
- ⚡ Fast execution (typically 30-60 seconds)
- 🔄 Default mode when no environment variable is set

#### 2. Oracle Only
For testing Oracle-specific dialect and compatibility:
```bash
TEST_DB=oracle go test ./internal/infrastructure/persistence/sqldb/...
```
- ✅ Runs integration tests against **Oracle only**
- 🐌 Slower execution (~2-3 minutes due to Oracle container startup)
- 🎯 Use when working on Oracle-specific features

#### 3. Full Multi-Database Suite
Complete validation against both databases:
```bash
TEST_DB=all go test ./internal/infrastructure/persistence/sqldb/...
```
- ✅ Runs integration tests against **both PostgreSQL and Oracle**
- 🐌 Slowest execution (~3-4 minutes)
- 🚀 Automatically executed in the GHCR release pipeline

### Test Coverage
Generate coverage report:
```bash
go test -v ./... -race -coverprofile=coverage.txt -covermode=atomic
```

### CI/CD Test Strategy
- **Pull Requests & Main CI**: PostgreSQL only (fast feedback)
- **Release Pipeline (GHCR)**: Full multi-database suite (comprehensive validation)

## License
MIT
