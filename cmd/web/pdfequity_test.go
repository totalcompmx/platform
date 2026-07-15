package main

import (
	"testing"

	"github.com/jcroyoaun/totalcompmx/internal/assert"
	"github.com/jcroyoaun/totalcompmx/internal/equity"
)

func TestPDFPackageEquity(t *testing.T) {
	t.Run("returns nil without an equity config", func(t *testing.T) {
		assert.True(t, pdfPackageEquity(PackageResult{}) == nil)
	})

	t.Run("maps the schedule and skips year zero", func(t *testing.T) {
		config := &equity.EquityConfig{
			InitialGrantUSD: 60000,
			HasRefreshers:   true,
			RefresherMinUSD: 10000,
			RefresherMaxUSD: 20000,
			VestingYears:    4,
			ExchangeRate:    20,
		}
		schedule := equity.CalculateEquitySchedule(*config, 4)

		result := pdfPackageEquity(PackageResult{EquityConfig: config, EquitySchedule: schedule})

		assert.Equal(t, result.InitialGrantUSD, 60000.0)
		assert.Equal(t, len(result.Schedule), 4)
		assert.Equal(t, result.Schedule[0].Year, 1)
		assert.Equal(t, result.TotalUSD, 82500.0)
		assert.Equal(t, result.TotalMXN, 1650000.0)
	})
}
