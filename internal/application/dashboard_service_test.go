package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jmanzanog/stock-tracker/internal/domain"
)

type mockDashboardPortfolioRepo struct {
	portfolio *domain.Portfolio
	findError error
}

func (m *mockDashboardPortfolioRepo) FindByID(_ context.Context, _ string) (*domain.Portfolio, error) {
	if m.findError != nil {
		return nil, m.findError
	}
	return m.portfolio, nil
}

func (m *mockDashboardPortfolioRepo) Save(_ context.Context, _ *domain.Portfolio) error {
	return nil
}

func (m *mockDashboardPortfolioRepo) FindAll(_ context.Context) ([]*domain.Portfolio, error) {
	if m.portfolio == nil {
		return []*domain.Portfolio{}, nil
	}
	return []*domain.Portfolio{m.portfolio}, nil
}

func (m *mockDashboardPortfolioRepo) Delete(_ context.Context, _ string) error {
	return nil
}

type mockDashboardPriceHistoryRepo struct {
	sparklines map[string][]domain.PriceHistory
	batchError error
}

func (m *mockDashboardPriceHistoryRepo) SaveBatch(_ context.Context, _ []domain.PriceHistory) error {
	return nil
}

func (m *mockDashboardPriceHistoryRepo) GetByISIN(_ context.Context, _ string, _, _ time.Time) ([]domain.PriceHistory, error) {
	return nil, nil
}

func (m *mockDashboardPriceHistoryRepo) GetSparkline(_ context.Context, isin string, days int) ([]domain.PriceHistory, error) {
	key := isin
	if m.sparklines == nil {
		return []domain.PriceHistory{}, nil
	}
	if history, ok := m.sparklines[key]; ok {
		return history, nil
	}
	return []domain.PriceHistory{}, nil
}

func (m *mockDashboardPriceHistoryRepo) GetSparklinesBatch(_ context.Context, requests []domain.SparklineRequest) ([]domain.SparklineResult, error) {
	if m.batchError != nil {
		return nil, m.batchError
	}
	results := make([]domain.SparklineResult, 0, len(requests))
	for _, req := range requests {
		key := req.ISIN
		var points []domain.PriceHistory
		if m.sparklines != nil {
			if history, ok := m.sparklines[key]; ok {
				points = history
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

func (m *mockDashboardPriceHistoryRepo) CleanupOlderThan(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}

func TestGetDashboard_Success(t *testing.T) {
	instrument := domain.NewInstrument("US0378331005", "AAPL", "Apple Inc.", domain.InstrumentTypeStock, "USD", "NASDAQ", "Technology")
	position := domain.NewPosition(instrument, domain.NewDecimalFromInt(10000), "USD")
	position.CurrentPrice = domain.NewDecimalFromInt(150)

	portfolio := &domain.Portfolio{
		ID:        "default",
		Name:      "Default Portfolio",
		Positions: []domain.Position{position},
	}

	priceHistory := []domain.PriceHistory{
		{
			ID:             "ph1",
			InstrumentISIN: "US0378331005",
			Price:          domain.NewDecimalFromInt(145),
			Currency:       "USD",
			RecordedAt:     time.Now().AddDate(0, 0, -1),
			CreatedAt:      time.Now(),
		},
		{
			ID:             "ph2",
			InstrumentISIN: "US0378331005",
			Price:          domain.NewDecimalFromInt(150),
			Currency:       "USD",
			RecordedAt:     time.Now(),
			CreatedAt:      time.Now(),
		},
	}

	repo := &mockDashboardPortfolioRepo{portfolio: portfolio}
	priceRepo := &mockDashboardPriceHistoryRepo{
		sparklines: map[string][]domain.PriceHistory{
			"US0378331005": priceHistory,
		},
	}

	service := NewDashboardService(repo, priceRepo)

	snapshot, err := service.GetDashboard(context.Background(), GetDashboardRequest{SparklineDays: []int{7, 30}})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if snapshot.PortfolioID != "default" {
		t.Errorf("expected portfolio ID 'default', got %s", snapshot.PortfolioID)
	}
	if len(snapshot.Positions) != 1 {
		t.Errorf("expected 1 position, got %d", len(snapshot.Positions))
	}
	if len(snapshot.ByCurrency) != 1 {
		t.Errorf("expected 1 currency allocation, got %d", len(snapshot.ByCurrency))
	}
	if len(snapshot.ByType) != 1 {
		t.Errorf("expected 1 type allocation, got %d", len(snapshot.ByType))
	}
	if len(snapshot.BySector) != 1 {
		t.Errorf("expected 1 sector allocation, got %d", len(snapshot.BySector))
	}

	pos := snapshot.Positions[0]
	if pos.Sparklines == nil {
		t.Error("expected sparklines map, got nil")
	}
	if _, ok := pos.Sparklines["7d"]; !ok {
		t.Error("expected sparkline for 7d")
	}
	if _, ok := pos.Sparklines["30d"]; !ok {
		t.Error("expected sparkline for 30d")
	}
}

func TestGetDashboard_PortfolioNotFound(t *testing.T) {
	repo := &mockDashboardPortfolioRepo{findError: errors.New("portfolio not found")}
	priceRepo := &mockDashboardPriceHistoryRepo{}

	service := NewDashboardService(repo, priceRepo)

	_, err := service.GetDashboard(context.Background(), GetDashboardRequest{SparklineDays: []int{7}})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetDashboard_EmptyPortfolio(t *testing.T) {
	portfolio := &domain.Portfolio{
		ID:        "default",
		Name:      "Default Portfolio",
		Positions: []domain.Position{},
	}

	repo := &mockDashboardPortfolioRepo{portfolio: portfolio}
	priceRepo := &mockDashboardPriceHistoryRepo{}

	service := NewDashboardService(repo, priceRepo)

	snapshot, err := service.GetDashboard(context.Background(), GetDashboardRequest{SparklineDays: []int{7}})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(snapshot.Positions) != 0 {
		t.Errorf("expected 0 positions, got %d", len(snapshot.Positions))
	}
}

func TestGetDashboard_SparklineBatchError(t *testing.T) {
	instrument := domain.NewInstrument("US0378331005", "AAPL", "Apple Inc.", domain.InstrumentTypeStock, "USD", "NASDAQ", "Technology")
	position := domain.NewPosition(instrument, domain.NewDecimalFromInt(10000), "USD")
	position.CurrentPrice = domain.NewDecimalFromInt(150)

	portfolio := &domain.Portfolio{
		ID:        "default",
		Name:      "Default Portfolio",
		Positions: []domain.Position{position},
	}

	repo := &mockDashboardPortfolioRepo{portfolio: portfolio}
	priceRepo := &mockDashboardPriceHistoryRepo{
		batchError: errors.New("batch query failed"),
	}

	service := NewDashboardService(repo, priceRepo)

	snapshot, err := service.GetDashboard(context.Background(), GetDashboardRequest{SparklineDays: []int{7}})

	if err != nil {
		t.Fatalf("expected no error (should continue despite sparkline error), got %v", err)
	}
	pos := snapshot.Positions[0]
	if len(pos.Sparklines["7d"]) != 0 {
		t.Error("expected empty sparkline when batch fails")
	}
}

func TestGetDashboard_MultiplePositions(t *testing.T) {
	instrument1 := domain.NewInstrument("US0378331005", "AAPL", "Apple Inc.", domain.InstrumentTypeStock, "USD", "NASDAQ", "Technology")
	instrument2 := domain.NewInstrument("US5949181045", "MSFT", "Microsoft Corp.", domain.InstrumentTypeStock, "USD", "NASDAQ", "Technology")

	position1 := domain.NewPosition(instrument1, domain.NewDecimalFromInt(10000), "USD")
	position1.CurrentPrice = domain.NewDecimalFromInt(150)

	position2 := domain.NewPosition(instrument2, domain.NewDecimalFromInt(5000), "USD")
	position2.CurrentPrice = domain.NewDecimalFromInt(300)

	portfolio := &domain.Portfolio{
		ID:        "default",
		Name:      "Default Portfolio",
		Positions: []domain.Position{position1, position2},
	}

	priceHistory := []domain.PriceHistory{
		{ID: "ph1", InstrumentISIN: "US0378331005", Price: domain.NewDecimalFromInt(150), RecordedAt: time.Now()},
	}

	repo := &mockDashboardPortfolioRepo{portfolio: portfolio}
	priceRepo := &mockDashboardPriceHistoryRepo{
		sparklines: map[string][]domain.PriceHistory{
			"US0378331005": priceHistory,
			"US5949181045": {},
		},
	}

	service := NewDashboardService(repo, priceRepo)

	snapshot, err := service.GetDashboard(context.Background(), GetDashboardRequest{SparklineDays: []int{7}})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(snapshot.Positions) != 2 {
		t.Errorf("expected 2 positions, got %d", len(snapshot.Positions))
	}
	if len(snapshot.ByCurrency) != 1 {
		t.Errorf("expected 1 currency allocation (all USD), got %d", len(snapshot.ByCurrency))
	}
	if len(snapshot.ByType) != 1 {
		t.Errorf("expected 1 type allocation (all Stock), got %d", len(snapshot.ByType))
	}
	if len(snapshot.BySector) != 1 {
		t.Errorf("expected 1 sector allocation (all Technology), got %d", len(snapshot.BySector))
	}
}

func TestGetDashboard_SectorAllocation_UnknownSector(t *testing.T) {
	instrument := domain.NewInstrument("US0378331005", "AAPL", "Apple Inc.", domain.InstrumentTypeStock, "USD", "NASDAQ", "")
	position := domain.NewPosition(instrument, domain.NewDecimalFromInt(10000), "USD")
	position.CurrentPrice = domain.NewDecimalFromInt(150)

	portfolio := &domain.Portfolio{
		ID:        "default",
		Name:      "Default Portfolio",
		Positions: []domain.Position{position},
	}

	repo := &mockDashboardPortfolioRepo{portfolio: portfolio}
	priceRepo := &mockDashboardPriceHistoryRepo{}

	service := NewDashboardService(repo, priceRepo)

	snapshot, err := service.GetDashboard(context.Background(), GetDashboardRequest{SparklineDays: []int{7}})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	foundNA := false
	for _, alloc := range snapshot.BySector {
		if alloc.Sector == "N/A" {
			foundNA = true
			break
		}
	}
	if !foundNA {
		t.Error("expected 'N/A' sector for empty sector field")
	}
}

func TestGetDashboard_MultipleCurrencies(t *testing.T) {
	instrument1 := domain.NewInstrument("US0378331005", "AAPL", "Apple Inc.", domain.InstrumentTypeStock, "USD", "NASDAQ", "Technology")
	instrument2 := domain.NewInstrument("EU0000000005", "SAP.DE", "SAP SE", domain.InstrumentTypeStock, "EUR", "XETRA", "Technology")

	position1 := domain.NewPosition(instrument1, domain.NewDecimalFromInt(10000), "USD")
	position1.CurrentPrice = domain.NewDecimalFromInt(150)

	position2 := domain.NewPosition(instrument2, domain.NewDecimalFromInt(8000), "EUR")
	position2.CurrentPrice = domain.NewDecimalFromInt(100)

	portfolio := &domain.Portfolio{
		ID:        "default",
		Name:      "Default Portfolio",
		Positions: []domain.Position{position1, position2},
	}

	repo := &mockDashboardPortfolioRepo{portfolio: portfolio}
	priceRepo := &mockDashboardPriceHistoryRepo{}

	service := NewDashboardService(repo, priceRepo)

	snapshot, err := service.GetDashboard(context.Background(), GetDashboardRequest{SparklineDays: []int{30}})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(snapshot.ByCurrency) != 2 {
		t.Errorf("expected 2 currencies, got %d", len(snapshot.ByCurrency))
	}

	currencies := make(map[string]bool)
	for _, c := range snapshot.ByCurrency {
		currencies[c.Currency] = true
	}
	if !currencies["USD"] || !currencies["EUR"] {
		t.Error("expected USD and EUR allocations")
	}
}

func TestGetDashboard_MultipleTypes(t *testing.T) {
	instrument1 := domain.NewInstrument("US0378331005", "AAPL", "Apple Inc.", domain.InstrumentTypeStock, "USD", "NASDAQ", "Technology")
	instrument2 := domain.NewInstrument("US0378331006", "SPY", "SPDR S&P 500 ETF", domain.InstrumentTypeETF, "USD", "NYSE", "Diversified")

	position1 := domain.NewPosition(instrument1, domain.NewDecimalFromInt(10000), "USD")
	position1.CurrentPrice = domain.NewDecimalFromInt(150)

	position2 := domain.NewPosition(instrument2, domain.NewDecimalFromInt(20000), "USD")
	position2.CurrentPrice = domain.NewDecimalFromInt(450)

	portfolio := &domain.Portfolio{
		ID:        "default",
		Name:      "Default Portfolio",
		Positions: []domain.Position{position1, position2},
	}

	repo := &mockDashboardPortfolioRepo{portfolio: portfolio}
	priceRepo := &mockDashboardPriceHistoryRepo{}

	service := NewDashboardService(repo, priceRepo)

	snapshot, err := service.GetDashboard(context.Background(), GetDashboardRequest{SparklineDays: []int{7}})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(snapshot.ByType) != 2 {
		t.Errorf("expected 2 types, got %d", len(snapshot.ByType))
	}

	types := make(map[domain.InstrumentType]bool)
	for _, at := range snapshot.ByType {
		types[at.Type] = true
	}
	if !types[domain.InstrumentTypeStock] || !types[domain.InstrumentTypeETF] {
		t.Error("expected Stock and ETF allocations")
	}
}

func TestGetDashboard_MultipleSectors(t *testing.T) {
	instrument1 := domain.NewInstrument("US0378331005", "AAPL", "Apple Inc.", domain.InstrumentTypeStock, "USD", "NASDAQ", "Technology")
	instrument2 := domain.NewInstrument("US0000000007", "JPM", "JPMorgan Chase", domain.InstrumentTypeStock, "USD", "NYSE", "Financial")

	position1 := domain.NewPosition(instrument1, domain.NewDecimalFromInt(10000), "USD")
	position1.CurrentPrice = domain.NewDecimalFromInt(150)

	position2 := domain.NewPosition(instrument2, domain.NewDecimalFromInt(15000), "USD")
	position2.CurrentPrice = domain.NewDecimalFromInt(180)

	portfolio := &domain.Portfolio{
		ID:        "default",
		Name:      "Default Portfolio",
		Positions: []domain.Position{position1, position2},
	}

	repo := &mockDashboardPortfolioRepo{portfolio: portfolio}
	priceRepo := &mockDashboardPriceHistoryRepo{}

	service := NewDashboardService(repo, priceRepo)

	snapshot, err := service.GetDashboard(context.Background(), GetDashboardRequest{SparklineDays: []int{7}})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(snapshot.BySector) != 2 {
		t.Errorf("expected 2 sectors, got %d", len(snapshot.BySector))
	}

	sectors := make(map[string]bool)
	for _, sa := range snapshot.BySector {
		sectors[sa.Sector] = true
	}
	if !sectors["Technology"] || !sectors["Financial"] {
		t.Error("expected Technology and Financial allocations")
	}
}

func TestGetDashboard_ZeroInvestedAmount(t *testing.T) {
	instrument := domain.NewInstrument("US0378331005", "AAPL", "Apple Inc.", domain.InstrumentTypeStock, "USD", "NASDAQ", "Technology")
	position := domain.NewPosition(instrument, domain.NewDecimalFromInt(0), "USD")
	position.CurrentPrice = domain.NewDecimalFromInt(150)

	portfolio := &domain.Portfolio{
		ID:        "default",
		Name:      "Default Portfolio",
		Positions: []domain.Position{position},
	}

	repo := &mockDashboardPortfolioRepo{portfolio: portfolio}
	priceRepo := &mockDashboardPriceHistoryRepo{}

	service := NewDashboardService(repo, priceRepo)

	snapshot, err := service.GetDashboard(context.Background(), GetDashboardRequest{SparklineDays: []int{7}})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(snapshot.ByCurrency) != 1 {
		t.Errorf("expected 1 currency allocation, got %d", len(snapshot.ByCurrency))
	}
}

func TestGetDashboard_NoSparklinesRequested(t *testing.T) {
	instrument := domain.NewInstrument("US0378331005", "AAPL", "Apple Inc.", domain.InstrumentTypeStock, "USD", "NASDAQ", "Technology")
	position := domain.NewPosition(instrument, domain.NewDecimalFromInt(10000), "USD")
	position.CurrentPrice = domain.NewDecimalFromInt(150)

	portfolio := &domain.Portfolio{
		ID:        "default",
		Name:      "Default Portfolio",
		Positions: []domain.Position{position},
	}

	repo := &mockDashboardPortfolioRepo{portfolio: portfolio}
	priceRepo := &mockDashboardPriceHistoryRepo{}

	service := NewDashboardService(repo, priceRepo)

	snapshot, err := service.GetDashboard(context.Background(), GetDashboardRequest{SparklineDays: []int{}})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(snapshot.Positions) != 1 {
		t.Errorf("expected 1 position, got %d", len(snapshot.Positions))
	}

	pos := snapshot.Positions[0]
	if len(pos.Sparklines) != 0 {
		t.Errorf("expected 0 sparklines, got %d", len(pos.Sparklines))
	}
}

func TestGetDashboard_GeneratedAtIsSet(t *testing.T) {
	instrument := domain.NewInstrument("US0378331005", "AAPL", "Apple Inc.", domain.InstrumentTypeStock, "USD", "NASDAQ", "Technology")
	position := domain.NewPosition(instrument, domain.NewDecimalFromInt(10000), "USD")
	position.CurrentPrice = domain.NewDecimalFromInt(150)

	portfolio := &domain.Portfolio{
		ID:        "default",
		Name:      "Default Portfolio",
		Positions: []domain.Position{position},
	}

	repo := &mockDashboardPortfolioRepo{portfolio: portfolio}
	priceRepo := &mockDashboardPriceHistoryRepo{}

	service := NewDashboardService(repo, priceRepo)

	before := time.Now().Add(-time.Second)
	snapshot, err := service.GetDashboard(context.Background(), GetDashboardRequest{SparklineDays: []int{7}})
	after := time.Now().Add(time.Second)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if snapshot.GeneratedAt.Before(before) || snapshot.GeneratedAt.After(after) {
		t.Error("GeneratedAt should be within a reasonable window around the call time")
	}
}

func TestGetDashboard_CalculatesCorrectTotals(t *testing.T) {
	instrument1 := domain.NewInstrument("US0378331005", "AAPL", "Apple Inc.", domain.InstrumentTypeStock, "USD", "NASDAQ", "Technology")
	instrument2 := domain.NewInstrument("US5949181045", "MSFT", "Microsoft Corp.", domain.InstrumentTypeStock, "USD", "NASDAQ", "Technology")

	position1 := domain.NewPosition(instrument1, domain.NewDecimalFromInt(10000), "USD")
	position1.CurrentPrice = domain.NewDecimalFromInt(150)

	position2 := domain.NewPosition(instrument2, domain.NewDecimalFromInt(20000), "USD")
	position2.CurrentPrice = domain.NewDecimalFromInt(300)

	portfolio := &domain.Portfolio{
		ID:        "default",
		Name:      "Default Portfolio",
		Positions: []domain.Position{position1, position2},
	}

	repo := &mockDashboardPortfolioRepo{portfolio: portfolio}
	priceRepo := &mockDashboardPriceHistoryRepo{}

	service := NewDashboardService(repo, priceRepo)

	snapshot, err := service.GetDashboard(context.Background(), GetDashboardRequest{SparklineDays: []int{7}})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if snapshot.TotalInvested.String() != "30000" {
		t.Errorf("expected TotalInvested=30000, got %s", snapshot.TotalInvested.String())
	}
}

func TestGetDashboard_SparklinesWithData(t *testing.T) {
	instrument := domain.NewInstrument("US0378331005", "AAPL", "Apple Inc.", domain.InstrumentTypeStock, "USD", "NASDAQ", "Technology")
	position := domain.NewPosition(instrument, domain.NewDecimalFromInt(10000), "USD")
	position.CurrentPrice = domain.NewDecimalFromInt(150)

	portfolio := &domain.Portfolio{
		ID:        "default",
		Name:      "Default Portfolio",
		Positions: []domain.Position{position},
	}

	now := time.Now()
	priceHistory := []domain.PriceHistory{
		{ID: "ph1", InstrumentISIN: "US0378331005", Price: domain.NewDecimalFromInt(145), RecordedAt: now.AddDate(0, 0, -1)},
		{ID: "ph2", InstrumentISIN: "US0378331005", Price: domain.NewDecimalFromInt(148), RecordedAt: now.AddDate(0, 0, -2)},
		{ID: "ph3", InstrumentISIN: "US0378331005", Price: domain.NewDecimalFromInt(150), RecordedAt: now},
	}

	repo := &mockDashboardPortfolioRepo{portfolio: portfolio}
	priceRepo := &mockDashboardPriceHistoryRepo{
		sparklines: map[string][]domain.PriceHistory{
			"US0378331005": priceHistory,
		},
	}

	service := NewDashboardService(repo, priceRepo)

	snapshot, err := service.GetDashboard(context.Background(), GetDashboardRequest{SparklineDays: []int{7}})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	pos := snapshot.Positions[0]
	if len(pos.Sparklines["7d"]) != 3 {
		t.Errorf("expected 3 sparkline points, got %d", len(pos.Sparklines["7d"]))
	}
}

func TestGetDashboard_EmptySparklineForUnknownISIN(t *testing.T) {
	instrument := domain.NewInstrument("US0378331005", "AAPL", "Apple Inc.", domain.InstrumentTypeStock, "USD", "NASDAQ", "Technology")
	position := domain.NewPosition(instrument, domain.NewDecimalFromInt(10000), "USD")
	position.CurrentPrice = domain.NewDecimalFromInt(150)

	portfolio := &domain.Portfolio{
		ID:        "default",
		Name:      "Default Portfolio",
		Positions: []domain.Position{position},
	}

	repo := &mockDashboardPortfolioRepo{portfolio: portfolio}
	priceRepo := &mockDashboardPriceHistoryRepo{
		sparklines: map[string][]domain.PriceHistory{},
	}

	service := NewDashboardService(repo, priceRepo)

	snapshot, err := service.GetDashboard(context.Background(), GetDashboardRequest{SparklineDays: []int{7}})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	pos := snapshot.Positions[0]
	if len(pos.Sparklines["7d"]) != 0 {
		t.Errorf("expected empty sparkline, got %d points", len(pos.Sparklines["7d"]))
	}
}

func TestGetDashboard_ThreeSparklineRanges(t *testing.T) {
	instrument := domain.NewInstrument("US0378331005", "AAPL", "Apple Inc.", domain.InstrumentTypeStock, "USD", "NASDAQ", "Technology")
	position := domain.NewPosition(instrument, domain.NewDecimalFromInt(10000), "USD")
	position.CurrentPrice = domain.NewDecimalFromInt(150)

	portfolio := &domain.Portfolio{
		ID:        "default",
		Name:      "Default Portfolio",
		Positions: []domain.Position{position},
	}

	repo := &mockDashboardPortfolioRepo{portfolio: portfolio}
	priceRepo := &mockDashboardPriceHistoryRepo{}

	service := NewDashboardService(repo, priceRepo)

	snapshot, err := service.GetDashboard(context.Background(), GetDashboardRequest{SparklineDays: []int{7, 30, 90}})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	pos := snapshot.Positions[0]
	if _, ok := pos.Sparklines["7d"]; !ok {
		t.Error("expected 7d sparkline")
	}
	if _, ok := pos.Sparklines["30d"]; !ok {
		t.Error("expected 30d sparkline")
	}
	if _, ok := pos.Sparklines["90d"]; !ok {
		t.Error("expected 90d sparkline")
	}
}

func TestGetDashboard_VerifyAllocationsAfterCalculation(t *testing.T) {
	instrument1 := domain.NewInstrument("US0378331005", "AAPL", "Apple Inc.", domain.InstrumentTypeStock, "USD", "NASDAQ", "Technology")
	instrument2 := domain.NewInstrument("US5949181045", "MSFT", "Microsoft Corp.", domain.InstrumentTypeStock, "USD", "NASDAQ", "Technology")

	position1 := domain.NewPosition(instrument1, domain.NewDecimalFromInt(10000), "USD")
	position1.CurrentPrice = domain.NewDecimalFromInt(150)

	position2 := domain.NewPosition(instrument2, domain.NewDecimalFromInt(5000), "USD")
	position2.CurrentPrice = domain.NewDecimalFromInt(300)

	portfolio := &domain.Portfolio{
		ID:        "default",
		Name:      "Default Portfolio",
		Positions: []domain.Position{position1, position2},
	}

	repo := &mockDashboardPortfolioRepo{portfolio: portfolio}
	priceRepo := &mockDashboardPriceHistoryRepo{}

	service := NewDashboardService(repo, priceRepo)

	snapshot, err := service.GetDashboard(context.Background(), GetDashboardRequest{SparklineDays: []int{7}})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if snapshot.TotalInvested.String() != "15000" {
		t.Errorf("expected TotalInvested=15000, got %s", snapshot.TotalInvested.String())
	}

	if len(snapshot.ByCurrency) != 1 {
		t.Errorf("expected 1 currency allocation, got %d", len(snapshot.ByCurrency))
	}
	if len(snapshot.ByType) != 1 {
		t.Errorf("expected 1 type allocation, got %d", len(snapshot.ByType))
	}
	if len(snapshot.BySector) != 1 {
		t.Errorf("expected 1 sector allocation, got %d", len(snapshot.BySector))
	}
}

func TestGetDashboard_PortfolioFindError(t *testing.T) {
	repo := &mockDashboardPortfolioRepo{findError: errors.New("database connection lost")}
	priceRepo := &mockDashboardPriceHistoryRepo{}

	service := NewDashboardService(repo, priceRepo)

	_, err := service.GetDashboard(context.Background(), GetDashboardRequest{SparklineDays: []int{7}})

	if err == nil {
		t.Fatal("expected error when portfolio repo fails")
	}
}
