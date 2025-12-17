FROM golang:1.23-alpine

WORKDIR /app

# Copy dependencies first for caching layers
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

ENV CGO_ENABLED=0
RUN go build -o stock-tracker ./cmd/tracker/main.go

EXPOSE 8080

CMD ["./stock-tracker"]
