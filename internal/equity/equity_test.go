package equity

import "testing"

func TestCalculateEquityScheduleWithoutRefreshers(t *testing.T) {
	config := EquityConfig{InitialGrantUSD: 40000, VestingYears: 4, ExchangeRate: 20}

	schedule := CalculateEquitySchedule(config, 5)

	assertEquityYear(t, schedule[0], 0, 0, 0, 0, 0)
	assertEquityYear(t, schedule[1], 1, 10000, 0, 10000, 200000)
	assertEquityYear(t, schedule[4], 4, 10000, 0, 10000, 200000)
	assertEquityYear(t, schedule[5], 5, 0, 0, 0, 0)
}

func TestCalculateEquityScheduleWithRefreshers(t *testing.T) {
	config := EquityConfig{
		InitialGrantUSD: 40000,
		HasRefreshers:   true,
		RefresherMinUSD: 8000,
		RefresherMaxUSD: 12000,
		VestingYears:    4,
		ExchangeRate:    18.5,
	}

	schedule := CalculateEquitySchedule(config, 3)

	assertEquityYear(t, schedule[1], 1, 10000, 0, 10000, 185000)
	assertEquityYear(t, schedule[2], 2, 10000, 2500, 12500, 231250)
	assertEquityYear(t, schedule[3], 3, 10000, 5000, 15000, 277500)
	assertRefresher(t, schedule[2], 1, 2500)
	assertRefresher(t, schedule[3], 1, 2500)
	assertRefresher(t, schedule[3], 2, 2500)
}

func TestGrantVestsInYear(t *testing.T) {
	tests := []struct {
		name string
		year int
		want bool
	}{
		{"before vesting starts", 1, false},
		{"vesting starts next year", 2, true},
		{"vesting ends at vesting length", 5, true},
		{"after vesting ends", 6, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := grantVestsInYear(1, tt.year, 4)
			assertBool(t, got, tt.want)
		})
	}
}

func TestGetTotalEquityOver4Years(t *testing.T) {
	config := EquityConfig{InitialGrantUSD: 40000, VestingYears: 4, ExchangeRate: 20}

	totalUSD, totalMXN := GetTotalEquityOver4Years(config)

	assertFloat(t, totalUSD, 40000)
	assertFloat(t, totalMXN, 800000)
}

func TestRoundMoney(t *testing.T) {
	assertFloat(t, roundMoney(10.235), 10.24)
}

func assertEquityYear(t *testing.T, got YearlyEquity, year int, initial float64, refreshers float64, total float64, totalMXN float64) {
	t.Helper()
	assertInt(t, got.Year, year)
	assertFloat(t, got.InitialGrantVested, initial)
	assertFloat(t, got.RefresherTotal, refreshers)
	assertFloat(t, got.TotalVested, total)
	assertFloat(t, got.TotalVestedMXN, totalMXN)
}

func assertRefresher(t *testing.T, got YearlyEquity, grantYear int, want float64) {
	t.Helper()
	assertFloat(t, got.RefresherVested[grantYear], want)
}

func assertInt(t *testing.T, got int, want int) {
	t.Helper()
	if got != want {
		t.Fatalf("got %d; want %d", got, want)
	}
}

func assertFloat(t *testing.T, got float64, want float64) {
	t.Helper()
	if got != want {
		t.Fatalf("got %f; want %f", got, want)
	}
}

func assertBool(t *testing.T, got bool, want bool) {
	t.Helper()
	if got != want {
		t.Fatalf("got %t; want %t", got, want)
	}
}
