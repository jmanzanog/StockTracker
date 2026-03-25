package domain

import (
	"testing"
	"time"
)

type mockClock struct {
	now time.Time
}

func (m mockClock) Now() time.Time {
	return m.now
}

func TestRealClock_Now(t *testing.T) {
	clock := RealClock{}
	before := time.Now()
	now := clock.Now()
	after := time.Now()

	if now.Before(before) || now.After(after) {
		t.Errorf("Now() returned time outside expected range")
	}
}

func TestMockClock(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	clock := mockClock{now: fixedTime}

	result := clock.Now()
	if !result.Equal(fixedTime) {
		t.Errorf("expected %v, got %v", fixedTime, result)
	}
}

func TestDashboardSnapshot_CalculateAllocations_Idempotent(t *testing.T) {
	snapshot := &DashboardSnapshot{
		PortfolioID:   "test",
		GeneratedAt:   time.Now(),
		TotalValue:    NewDecimalFromInt(1000),
		TotalInvested: NewDecimalFromInt(800),
		TotalPnL:      NewDecimalFromInt(200),
		PnLPercent:    NewDecimalFromInt(25),
		Positions: []PositionDashboard{
			{
				ISIN:         "US1234567890",
				CurrentValue: NewDecimalFromInt(1000),
				Currency:     "USD",
			},
		},
	}

	snapshot.CalculateAllocations()
	firstByCurrency := snapshot.ByCurrency

	snapshot.CalculateAllocations()
	secondByCurrency := snapshot.ByCurrency

	if len(firstByCurrency) != len(secondByCurrency) {
		t.Errorf("expected same length, got %d and %d", len(firstByCurrency), len(secondByCurrency))
	}
}
