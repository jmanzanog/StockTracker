package sqldb

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jmanzanog/stock-tracker/internal/domain"
)

type PriceHistoryRepository struct {
	db *DB
}

func NewPriceHistoryRepository(db *DB) *PriceHistoryRepository {
	return &PriceHistoryRepository{db: db}
}

func (r *PriceHistoryRepository) SaveBatch(ctx context.Context, history []domain.PriceHistory) error {
	if len(history) == 0 {
		return nil
	}

	isOracle := r.db.Dialect.Name() == "oracle"
	placeholder := "$"
	if isOracle {
		placeholder = ":"
	}

	valueStrings := make([]string, 0, len(history))
	valueArgs := make([]interface{}, 0, len(history)*6)

	for i, h := range history {
		offset := i * 6
		valueStrings = append(valueStrings, fmt.Sprintf("(%s%d, %s%d, %s%d, %s%d, %s%d, %s%d)",
			placeholder, offset+1,
			placeholder, offset+2,
			placeholder, offset+3,
			placeholder, offset+4,
			placeholder, offset+5,
			placeholder, offset+6))
		valueArgs = append(valueArgs, h.ID, h.InstrumentISIN, h.Price, h.Currency, h.RecordedAt, h.CreatedAt)
	}

	query := fmt.Sprintf(`
		INSERT INTO price_history (id, instrument_isin, price, currency, recorded_at, created_at)
		VALUES %s
	`, strings.Join(valueStrings, ", "))

	_, err := r.db.ExecContext(ctx, query, valueArgs...)
	if err != nil {
		return fmt.Errorf("bulk inserting price history: %w", err)
	}

	return nil
}

func (r *PriceHistoryRepository) GetByISIN(ctx context.Context, isin string, from, to time.Time) ([]domain.PriceHistory, error) {
	query := r.rebind(`
		SELECT id, instrument_isin, price, currency, recorded_at, created_at
		FROM price_history
		WHERE instrument_isin = $1 AND recorded_at >= $2 AND recorded_at <= $3
		ORDER BY recorded_at ASC
	`)

	rows, err := r.db.QueryContext(ctx, query, isin, from, to)
	if err != nil {
		return nil, fmt.Errorf("querying price history: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var history []domain.PriceHistory
	for rows.Next() {
		var h domain.PriceHistory
		if err := rows.Scan(&h.ID, &h.InstrumentISIN, &h.Price, &h.Currency, &h.RecordedAt, &h.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning price history row: %w", err)
		}
		history = append(history, h)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating price history rows: %w", err)
	}

	return history, nil
}

func (r *PriceHistoryRepository) GetSparkline(ctx context.Context, isin string, days int) ([]domain.PriceHistory, error) {
	from := time.Now().AddDate(0, 0, -days)
	to := time.Now()

	return r.GetByISIN(ctx, isin, from, to)
}

func (r *PriceHistoryRepository) GetSparklinesBatch(ctx context.Context, requests []domain.SparklineRequest) ([]domain.SparklineResult, error) {
	if len(requests) == 0 {
		return nil, nil
	}

	oldestFrom := time.Now()
	newestTo := time.Now()

	isinSet := make(map[string]struct{})
	for _, req := range requests {
		isinSet[req.ISIN] = struct{}{}
		from := time.Now().AddDate(0, 0, -req.Days)
		if from.Before(oldestFrom) {
			oldestFrom = from
		}
	}

	isins := make([]string, 0, len(isinSet))
	for isin := range isinSet {
		isins = append(isins, isin)
	}
	sort.Strings(isins)

	query := r.rebind(`
		SELECT id, instrument_isin, price, currency, recorded_at, created_at
		FROM price_history
		WHERE instrument_isin IN (` + r.inPlaceholders(isins) + `) AND recorded_at >= $1 AND recorded_at <= $2
		ORDER BY instrument_isin, recorded_at ASC
	`)

	args := make([]interface{}, 0, len(isins)+2)
	args = append(args, oldestFrom, newestTo)
	for _, isin := range isins {
		args = append(args, isin)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("batch querying price history: %w", err)
	}
	defer func() { _ = rows.Close() }()

	resultsMap := make(map[string][]domain.PriceHistory)
	for rows.Next() {
		var h domain.PriceHistory
		if err := rows.Scan(&h.ID, &h.InstrumentISIN, &h.Price, &h.Currency, &h.RecordedAt, &h.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning price history row: %w", err)
		}
		resultsMap[h.InstrumentISIN] = append(resultsMap[h.InstrumentISIN], h)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating price history rows: %w", err)
	}

	results := make([]domain.SparklineResult, 0, len(requests))
	for _, req := range requests {
		var points []domain.PriceHistory
		if history, ok := resultsMap[req.ISIN]; ok {
			cutoff := newestTo.AddDate(0, 0, -req.Days)
			for _, h := range history {
				if !h.RecordedAt.Before(cutoff) {
					points = append(points, h)
				}
			}
		}
		if points == nil {
			points = []domain.PriceHistory{}
		}
		results = append(results, domain.SparklineResult{
			ISIN:   req.ISIN,
			Days:   req.Days,
			Points: points,
		})
	}

	return results, nil
}

func (r *PriceHistoryRepository) inPlaceholders(values []string) string {
	var sb strings.Builder
	for i := range values {
		if i > 0 {
			sb.WriteString(", ")
		}
		fmt.Fprintf(&sb, "$%d", i+3)
	}
	return sb.String()
}

func (r *PriceHistoryRepository) CleanupOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	query := r.rebind("DELETE FROM price_history WHERE recorded_at < $1")
	result, err := r.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("deleting old price history: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("getting rows affected: %w", err)
	}

	if rowsAffected > 0 {
		slog.Info("Cleaned up old price history", "rows_deleted", rowsAffected, "cutoff", cutoff)
	}

	return rowsAffected, nil
}

func (r *PriceHistoryRepository) rebind(query string) string {
	if r.db.Dialect.Name() == "oracle" {
		re := regexp.MustCompile(`\$(\d+)`)
		return re.ReplaceAllString(query, `:$1`)
	}
	return query
}
