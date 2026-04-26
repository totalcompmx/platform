package equity

import "math"

// YearlyEquity represents equity vesting for a single year
type YearlyEquity struct {
	Year                int
	InitialGrantVested  float64         // Amount vested from initial grant (USD)
	RefresherVested     map[int]float64 // key = refresher year (1,2,3...), value = amount vested (USD)
	RefresherTotal      float64         // Total from all refreshers this year (USD)
	TotalVested         float64         // Total vested this year (USD)
	NewRefresherGranted float64         // Refresher granted THIS year (USD, starts vesting next year)
	TotalVestedMXN      float64         // Total vested in MXN using exchange rate
}

// EquityConfig holds the configuration for equity calculations
type EquityConfig struct {
	InitialGrantUSD float64
	HasRefreshers   bool
	RefresherMinUSD float64
	RefresherMaxUSD float64
	VestingYears    int     // Typically 4
	ExchangeRate    float64 // From FiscalYear.USDMXNRate
}

// CalculateEquitySchedule calculates year-by-year equity vesting with refresher stacking
// Returns a slice of YearlyEquity for the specified number of years (including Year 0)
func CalculateEquitySchedule(config EquityConfig, years int) []YearlyEquity {
	schedule := make([]YearlyEquity, years+1) // +1 for Year 0
	annualVestPercent := 1.0 / float64(config.VestingYears)
	avgRefresher := averageRefresher(config)
	refresherGrants := make(map[int]float64)

	schedule[0] = emptyEquityYear(0)
	for year := 1; year <= years; year++ {
		yearEquity := calculateEquityYear(config, year, annualVestPercent, avgRefresher, refresherGrants)
		schedule[year] = yearEquity
	}

	return schedule
}

func averageRefresher(config EquityConfig) float64 {
	if config.HasRefreshers {
		return (config.RefresherMinUSD + config.RefresherMaxUSD) / 2.0
	}
	return 0
}

func emptyEquityYear(year int) YearlyEquity {
	return YearlyEquity{Year: year, RefresherVested: make(map[int]float64)}
}

func calculateEquityYear(config EquityConfig, year int, annualVestPercent float64, avgRefresher float64, refresherGrants map[int]float64) YearlyEquity {
	yearEquity := emptyEquityYear(year)
	yearEquity.InitialGrantVested = initialGrantVested(config, year, annualVestPercent)
	yearEquity.RefresherVested = vestedRefreshers(config, year, annualVestPercent, refresherGrants)
	grantRefresher(config, year, avgRefresher, refresherGrants, &yearEquity)
	finalizeEquityYear(config, &yearEquity)
	return yearEquity
}

func initialGrantVested(config EquityConfig, year int, annualVestPercent float64) float64 {
	if year <= config.VestingYears {
		return config.InitialGrantUSD * annualVestPercent
	}
	return 0
}

func vestedRefreshers(config EquityConfig, year int, annualVestPercent float64, refresherGrants map[int]float64) map[int]float64 {
	vested := make(map[int]float64)
	if !config.HasRefreshers {
		return vested
	}
	for grantYear, grantAmount := range refresherGrants {
		if grantVestsInYear(grantYear, year, config.VestingYears) {
			vested[grantYear] = grantAmount * annualVestPercent
		}
	}
	return vested
}

func grantVestsInYear(grantYear int, year int, vestingYears int) bool {
	vestingStartYear := grantYear + 1
	vestingEndYear := grantYear + vestingYears
	return year >= vestingStartYear && year <= vestingEndYear
}

func grantRefresher(config EquityConfig, year int, avgRefresher float64, refresherGrants map[int]float64, yearEquity *YearlyEquity) {
	if !config.HasRefreshers {
		return
	}
	yearEquity.NewRefresherGranted = avgRefresher
	refresherGrants[year] = avgRefresher
}

func finalizeEquityYear(config EquityConfig, yearEquity *YearlyEquity) {
	yearEquity.RefresherTotal = totalRefresherVested(yearEquity.RefresherVested)
	yearEquity.TotalVested = yearEquity.InitialGrantVested + yearEquity.RefresherTotal
	yearEquity.TotalVestedMXN = yearEquity.TotalVested * config.ExchangeRate
	roundEquityYear(yearEquity)
}

func totalRefresherVested(refreshers map[int]float64) float64 {
	total := 0.0
	for _, amount := range refreshers {
		total += amount
	}
	return total
}

func roundEquityYear(yearEquity *YearlyEquity) {
	yearEquity.TotalVested = roundMoney(yearEquity.TotalVested)
	yearEquity.TotalVestedMXN = roundMoney(yearEquity.TotalVestedMXN)
	yearEquity.InitialGrantVested = roundMoney(yearEquity.InitialGrantVested)
	yearEquity.RefresherTotal = roundMoney(yearEquity.RefresherTotal)
	yearEquity.NewRefresherGranted = roundMoney(yearEquity.NewRefresherGranted)
	for k, v := range yearEquity.RefresherVested {
		yearEquity.RefresherVested[k] = roundMoney(v)
	}
}

func roundMoney(value float64) float64 {
	return math.Round(value*100) / 100
}

// GetTotalEquityOver4Years returns the total equity vested over 4 years
func GetTotalEquityOver4Years(config EquityConfig) (float64, float64) {
	schedule := CalculateEquitySchedule(config, 4)

	totalUSD := 0.0
	totalMXN := 0.0

	for _, year := range schedule {
		totalUSD += year.TotalVested
		totalMXN += year.TotalVestedMXN
	}

	return totalUSD, totalMXN
}
