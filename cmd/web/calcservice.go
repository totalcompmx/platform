package main

import (
	"github.com/jcroyoaun/totalcompmx/internal/database"
	"github.com/jcroyoaun/totalcompmx/internal/equity"
)

// packageSpec is the canonical, fully-typed input for one compensation package.
// Both the web form and the JSON API decode into this before calculating, so
// the two surfaces can never drift apart.
type packageSpec struct {
	Name                   string
	Regime                 string  // "sueldos_salarios" or "resico"
	Salary                 float64 // as entered, in Currency at PaymentFrequency
	Currency               string  // "MXN" or "USD"
	ExchangeRate           float64 // MXN per USD, used for the salary and USD other-benefits
	PaymentFrequency       string  // monthly, biweekly, weekly, daily, hourly
	HoursPerWeek           float64 // used only when PaymentFrequency is "hourly"
	HasAguinaldo           bool
	AguinaldoDays          int
	HasValesDespensa       bool
	ValesDespensaAmount    float64
	HasPrimaVacacional     bool
	VacationDays           int
	PrimaVacacionalPercent float64
	HasFondoAhorro         bool
	FondoAhorroPercent     float64
	HasInfonavitCredit     bool
	UnpaidVacationDays     int // RESICO only: days off without pay
	OtherBenefits          []OtherBenefit
	HasEquity              bool
	InitialEquityUSD       float64
	HasRefreshers          bool
	RefresherMinUSD        float64
	RefresherMaxUSD        float64
}

// monthlyMXN normalizes the entered salary to a monthly amount in MXN.
func (spec packageSpec) monthlyMXN() float64 {
	amount := spec.Salary
	if spec.Currency == "USD" {
		amount *= spec.ExchangeRate
	}
	return monthlySalary(amount, spec.PaymentFrequency, spec.HoursPerWeek)
}

// calculatePackage runs the full calculation (payroll + equity) for one package.
func (app *application) calculatePackage(spec packageSpec, fiscalYear database.FiscalYear) (PackageResult, error) {
	calculation, err := app.calculateSpec(spec, fiscalYear)
	if err != nil {
		return PackageResult{}, err
	}

	config, schedule := spec.equitySchedule(fiscalYear)
	return PackageResult{spec.Name, &calculation, config, schedule}, nil
}

func (app *application) calculateSpec(spec packageSpec, fiscalYear database.FiscalYear) (database.SalaryCalculation, error) {
	monthly := spec.monthlyMXN()
	if spec.Regime == "resico" {
		return app.calculateRESICO(monthly, spec.UnpaidVacationDays, spec.OtherBenefits, spec.ExchangeRate, fiscalYear)
	}

	return app.calculateSalaryWithBenefits(
		monthly,
		spec.HasAguinaldo, spec.AguinaldoDays,
		spec.HasValesDespensa, spec.ValesDespensaAmount,
		spec.HasPrimaVacacional, spec.VacationDays, spec.PrimaVacacionalPercent,
		spec.HasFondoAhorro, spec.FondoAhorroPercent,
		spec.HasInfonavitCredit,
		spec.OtherBenefits,
		spec.ExchangeRate,
		fiscalYear,
	)
}

func (spec packageSpec) equitySchedule(fiscalYear database.FiscalYear) (*equity.EquityConfig, []equity.YearlyEquity) {
	if spec.InitialEquityUSD <= 0 {
		return nil, nil
	}

	refresherMin, refresherMax := spec.RefresherMinUSD, spec.RefresherMaxUSD
	if refresherMin > refresherMax {
		refresherMin, refresherMax = refresherMax, refresherMin
	}

	config := equity.EquityConfig{
		InitialGrantUSD: spec.InitialEquityUSD,
		HasRefreshers:   spec.HasRefreshers && refresherMin > 0 && refresherMax > 0,
		RefresherMinUSD: refresherMin,
		RefresherMaxUSD: refresherMax,
		VestingYears:    4,
		ExchangeRate:    fiscalYear.USDMXNRate,
	}
	schedule := equity.CalculateEquitySchedule(config, 4)
	return &config, schedule
}
