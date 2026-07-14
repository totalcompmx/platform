package main

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/jcroyoaun/totalcompmx/assets"
	"github.com/jcroyoaun/totalcompmx/internal/database"
	"github.com/jcroyoaun/totalcompmx/internal/request"
)

const (
	apiMaxComparePackages = 10
	apiMaxOtherBenefits   = 20
	apiMaxSalary          = 1_000_000_000
)

type apiOtherBenefit struct {
	Name         string  `json:"name"`
	Amount       float64 `json:"amount"`
	TaxFree      bool    `json:"tax_free"`
	Currency     string  `json:"currency"`
	Cadence      string  `json:"cadence"`
	IsPercentage bool    `json:"is_percentage"`
}

// The four defaultable numeric fields are pointers so an explicit 0 (honored,
// like the web form) is distinguishable from an omitted field (defaulted).
type apiPackageRequest struct {
	Name                   string            `json:"name"`
	Salary                 float64           `json:"salary"`
	Regime                 string            `json:"regime"`
	Currency               string            `json:"currency"`
	ExchangeRate           float64           `json:"exchange_rate"`
	PaymentFrequency       string            `json:"payment_frequency"`
	HoursPerWeek           float64           `json:"hours_per_week"`
	HasAguinaldo           bool              `json:"has_aguinaldo"`
	AguinaldoDays          *int              `json:"aguinaldo_days"`
	HasValesDespensa       bool              `json:"has_vales_despensa"`
	ValesDespensaAmount    float64           `json:"vales_despensa_amount"`
	HasPrimaVacacional     bool              `json:"has_prima_vacacional"`
	VacationDays           *int              `json:"vacation_days"`
	PrimaVacacionalPercent *float64          `json:"prima_vacacional_percent"`
	HasFondoAhorro         bool              `json:"has_fondo_ahorro"`
	FondoAhorroPercent     *float64          `json:"fondo_ahorro_percent"`
	HasInfonavitCredit     bool              `json:"has_infonavit_credit"`
	UnpaidVacationDays     int               `json:"unpaid_vacation_days"`
	OtherBenefits          []apiOtherBenefit `json:"other_benefits"`
	HasEquity              bool              `json:"has_equity"`
	InitialEquityUSD       float64           `json:"initial_equity_usd"`
	HasRefreshers          bool              `json:"has_refreshers"`
	RefresherMinUSD        float64           `json:"refresher_min_usd"`
	RefresherMaxUSD        float64           `json:"refresher_max_usd"`
}

type apiCompareRequest struct {
	Packages []apiPackageRequest `json:"packages"`
}

type apiFieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// apiCalculate handles POST /api/v1/calculate: one package in, full calculation out.
func (app *application) apiCalculate(w http.ResponseWriter, r *http.Request) {
	var req apiPackageRequest
	if err := request.DecodeJSONStrict(w, r, &req); err != nil {
		app.writeJSONError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	fiscalYear, ok := app.activeFiscalYearJSON(w, r)
	if !ok {
		return
	}

	spec, fieldErrors := req.spec(fiscalYear, "", 1)
	if len(fieldErrors) > 0 {
		app.writeAPIValidationError(w, r, fieldErrors)
		return
	}

	result, ok := app.apiPackageResult(w, r, spec, fiscalYear)
	if !ok {
		return
	}

	app.writeAPIResponse(w, r, map[string]any{
		"success": true,
		"data":    apiPackageData(spec, result),
		"meta":    apiMeta(fiscalYear),
	})
}

// apiCompare handles POST /api/v1/compare: multiple packages in, per-package
// calculations plus the best package out (same semantics as the web comparison).
func (app *application) apiCompare(w http.ResponseWriter, r *http.Request) {
	var req apiCompareRequest
	if err := request.DecodeJSONStrict(w, r, &req); err != nil {
		app.writeJSONError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	if len(req.Packages) < 2 || len(req.Packages) > apiMaxComparePackages {
		app.writeAPIValidationError(w, r, []apiFieldError{{
			Field:   "packages",
			Message: fmt.Sprintf("must contain between 2 and %d packages", apiMaxComparePackages),
		}})
		return
	}

	fiscalYear, ok := app.activeFiscalYearJSON(w, r)
	if !ok {
		return
	}

	specs, fieldErrors := compareSpecs(req, fiscalYear)
	if len(fieldErrors) > 0 {
		app.writeAPIValidationError(w, r, fieldErrors)
		return
	}

	results, best, ok := app.apiCompareResults(w, r, specs, fiscalYear)
	if !ok {
		return
	}

	app.writeAPIResponse(w, r, map[string]any{
		"success": true,
		"data": map[string]any{
			"results":      results,
			"best_package": best,
		},
		"meta": apiMeta(fiscalYear),
	})
}

func compareSpecs(req apiCompareRequest, fiscalYear database.FiscalYear) ([]packageSpec, []apiFieldError) {
	specs := make([]packageSpec, 0, len(req.Packages))
	var fieldErrors []apiFieldError
	for i, pkg := range req.Packages {
		spec, pkgErrors := pkg.spec(fiscalYear, fmt.Sprintf("packages[%d].", i), i+1)
		specs = append(specs, spec)
		fieldErrors = append(fieldErrors, pkgErrors...)
	}
	return specs, fieldErrors
}

func (app *application) apiCompareResults(w http.ResponseWriter, r *http.Request, specs []packageSpec, fiscalYear database.FiscalYear) ([]map[string]any, map[string]any, bool) {
	results := make([]map[string]any, 0, len(specs))
	var best map[string]any
	var bestYearlyNet float64

	for i, spec := range specs {
		result, ok := app.apiPackageResult(w, r, spec, fiscalYear)
		if !ok {
			return nil, nil, false
		}
		results = append(results, apiPackageData(spec, result))
		if best == nil || result.YearlyNet > bestYearlyNet {
			bestYearlyNet = result.YearlyNet
			best = map[string]any{
				"index":      i,
				"name":       result.PackageName,
				"yearly_net": result.YearlyNet,
			}
		}
	}

	return results, best, true
}

func (app *application) apiPackageResult(w http.ResponseWriter, r *http.Request, spec packageSpec, fiscalYear database.FiscalYear) (PackageResult, bool) {
	result, err := app.calculatePackage(spec, fiscalYear)
	if err != nil {
		if errors.Is(err, errNoRESICOBracket) {
			app.writeJSONError(w, r, http.StatusBadRequest, "salary is outside the supported RESICO brackets for the active fiscal year")
			return PackageResult{}, false
		}
		app.serverError(w, r, err)
		return PackageResult{}, false
	}
	return result, true
}

// spec validates and normalizes an API package request into the canonical
// packageSpec, applying the same defaults the web form uses. All problems are
// reported at once, with fields prefixed (e.g. "packages[1].") for compare.
func (req apiPackageRequest) spec(fiscalYear database.FiscalYear, fieldPrefix string, packageNumber int) (packageSpec, []apiFieldError) {
	v := &apiValidator{prefix: fieldPrefix}

	spec := packageSpec{
		Name:               req.Name,
		Regime:             v.regime(req.Regime),
		Salary:             v.salary(req.Salary),
		Currency:           v.currency("currency", req.Currency),
		ExchangeRate:       v.exchangeRate(req.ExchangeRate, fiscalYear),
		PaymentFrequency:   v.paymentFrequency(req.PaymentFrequency),
		HoursPerWeek:       v.hoursPerWeek(req.PaymentFrequency, req.HoursPerWeek),
		HasInfonavitCredit: req.HasInfonavitCredit,
		UnpaidVacationDays: v.unpaidVacationDays(req.Regime, req.UnpaidVacationDays),
		OtherBenefits:      v.otherBenefits(req.OtherBenefits),
	}
	if spec.Name == "" {
		spec.Name = fmt.Sprintf("Paquete %d", packageNumber)
	}

	spec.HasAguinaldo = req.HasAguinaldo
	spec.AguinaldoDays = v.optionalDays("aguinaldo_days", req.HasAguinaldo, req.AguinaldoDays, 15)
	spec.HasValesDespensa = req.HasValesDespensa
	spec.ValesDespensaAmount = v.valesAmount(req.HasValesDespensa, req.ValesDespensaAmount)
	spec.HasPrimaVacacional = req.HasPrimaVacacional
	spec.VacationDays = v.optionalDays("vacation_days", req.HasPrimaVacacional, req.VacationDays, 12)
	spec.PrimaVacacionalPercent = v.optionalPercent("prima_vacacional_percent", req.HasPrimaVacacional, req.PrimaVacacionalPercent, 25)
	spec.HasFondoAhorro = req.HasFondoAhorro
	spec.FondoAhorroPercent = v.optionalPercent("fondo_ahorro_percent", req.HasFondoAhorro, req.FondoAhorroPercent, 13)

	v.equity(&spec, req)

	return spec, v.errors
}

type apiValidator struct {
	prefix string
	errors []apiFieldError
}

func (v *apiValidator) add(field, message string) {
	v.errors = append(v.errors, apiFieldError{Field: v.prefix + field, Message: message})
}

func (v *apiValidator) regime(regime string) string {
	switch regime {
	case "sueldos", "sueldos_salarios":
		return "sueldos_salarios"
	case "resico":
		return "resico"
	case "":
		v.add("regime", "is required (one of: sueldos_salarios, resico)")
	default:
		v.add("regime", fmt.Sprintf("%q is not a valid regime (one of: sueldos_salarios, resico)", regime))
	}
	return regime
}

func (v *apiValidator) salary(salary float64) float64 {
	if salary <= 0 {
		v.add("salary", "must be greater than 0")
	}
	if salary > apiMaxSalary {
		v.add("salary", fmt.Sprintf("must not exceed %d", apiMaxSalary))
	}
	return salary
}

func (v *apiValidator) currency(field, currency string) string {
	switch currency {
	case "":
		return "MXN"
	case "MXN", "USD":
		return currency
	}
	v.add(field, fmt.Sprintf("%q is not a valid currency (one of: MXN, USD)", currency))
	return currency
}

func (v *apiValidator) exchangeRate(rate float64, fiscalYear database.FiscalYear) float64 {
	if rate < 0 {
		v.add("exchange_rate", "must not be negative")
		return rate
	}
	if rate > 10000 {
		v.add("exchange_rate", "must not exceed 10000")
		return rate
	}
	if rate == 0 {
		return fiscalYear.USDMXNRate
	}
	return rate
}

func (v *apiValidator) paymentFrequency(frequency string) string {
	switch frequency {
	case "":
		return "monthly"
	case "monthly", "biweekly", "weekly", "daily", "hourly":
		return frequency
	}
	v.add("payment_frequency", fmt.Sprintf("%q is not a valid payment frequency (one of: monthly, biweekly, weekly, daily, hourly)", frequency))
	return frequency
}

func (v *apiValidator) hoursPerWeek(frequency string, hours float64) float64 {
	if frequency != "hourly" {
		return hours
	}
	if hours == 0 {
		return 40
	}
	if hours < 0 || hours > 168 {
		v.add("hours_per_week", "must be between 1 and 168")
	}
	return hours
}

func (v *apiValidator) unpaidVacationDays(regime string, days int) int {
	if days < 0 || days > 365 {
		v.add("unpaid_vacation_days", "must be between 0 and 365")
		return 0
	}
	if days > 0 && regime != "resico" {
		v.add("unpaid_vacation_days", "only applies to the resico regime")
		return 0
	}
	return days
}

func (v *apiValidator) optionalDays(field string, enabled bool, days *int, fallback int) int {
	if !enabled || days == nil {
		return fallback
	}
	if *days < 0 || *days > 365 {
		v.add(field, "must be between 0 and 365")
	}
	return *days
}

func (v *apiValidator) optionalPercent(field string, enabled bool, percent *float64, fallback float64) float64 {
	if !enabled || percent == nil {
		return fallback
	}
	if *percent < 0 || *percent > 100 {
		v.add(field, "must be between 0 and 100")
	}
	return *percent
}

func (v *apiValidator) valesAmount(enabled bool, amount float64) float64 {
	if !enabled {
		return 0
	}
	if amount <= 0 {
		v.add("vales_despensa_amount", "must be greater than 0 when has_vales_despensa is true")
	}
	return amount
}

func (v *apiValidator) otherBenefits(benefits []apiOtherBenefit) []OtherBenefit {
	if len(benefits) > apiMaxOtherBenefits {
		v.add("other_benefits", fmt.Sprintf("must not contain more than %d items", apiMaxOtherBenefits))
		return nil
	}

	converted := make([]OtherBenefit, 0, len(benefits))
	for i, benefit := range benefits {
		converted = append(converted, v.otherBenefit(i, benefit))
	}
	return converted
}

func (v *apiValidator) otherBenefit(index int, benefit apiOtherBenefit) OtherBenefit {
	field := func(name string) string { return fmt.Sprintf("other_benefits[%d].%s", index, name) }

	if benefit.Name == "" {
		v.add(field("name"), "is required")
	}
	if benefit.IsPercentage {
		if benefit.Amount <= 0 || benefit.Amount > 100 {
			v.add(field("amount"), "must be between 0 and 100 when is_percentage is true")
		}
	} else if benefit.Amount <= 0 || benefit.Amount > apiMaxSalary {
		v.add(field("amount"), fmt.Sprintf("must be greater than 0 and must not exceed %d", apiMaxSalary))
	}

	cadence := benefit.Cadence
	switch cadence {
	case "":
		cadence = "monthly"
	case "monthly", "annual":
	default:
		v.add(field("cadence"), fmt.Sprintf("%q is not a valid cadence (one of: monthly, annual)", benefit.Cadence))
	}
	// Percentage benefits are a share of gross annual salary, so they are
	// always annual (same rule as the web form).
	if benefit.IsPercentage {
		cadence = "annual"
	}

	return OtherBenefit{
		Name:         benefit.Name,
		Amount:       benefit.Amount,
		TaxFree:      benefit.TaxFree,
		Currency:     v.currency(field("currency"), benefit.Currency),
		Cadence:      cadence,
		IsPercentage: benefit.IsPercentage,
	}
}

func (v *apiValidator) equity(spec *packageSpec, req apiPackageRequest) {
	if !req.HasEquity {
		return
	}

	spec.HasEquity = true
	spec.InitialEquityUSD = req.InitialEquityUSD
	if req.InitialEquityUSD <= 0 || req.InitialEquityUSD > apiMaxSalary {
		v.add("initial_equity_usd", fmt.Sprintf("must be greater than 0 and must not exceed %d when has_equity is true", apiMaxSalary))
	}

	if !req.HasRefreshers {
		return
	}

	spec.HasRefreshers = true
	spec.RefresherMinUSD = req.RefresherMinUSD
	spec.RefresherMaxUSD = req.RefresherMaxUSD
	if req.RefresherMinUSD <= 0 || req.RefresherMaxUSD <= 0 || req.RefresherMinUSD > apiMaxSalary || req.RefresherMaxUSD > apiMaxSalary {
		v.add("refresher_min_usd", fmt.Sprintf("refresher_min_usd and refresher_max_usd must be greater than 0 and must not exceed %d when has_refreshers is true", apiMaxSalary))
	}
}

func apiPackageData(spec packageSpec, result PackageResult) map[string]any {
	calc := result.SalaryCalculation

	return map[string]any{
		"name":              result.PackageName,
		"regime":            spec.Regime,
		"currency":          spec.Currency,
		"exchange_rate":     spec.ExchangeRate,
		"payment_frequency": spec.PaymentFrequency,
		"gross_salary":      calc.GrossSalary,
		"net_salary":        calc.NetSalary,
		"isr_tax":           calc.ISRTax,
		"subsidio_empleo":   calc.SubsidioEmpleo,
		"imss_worker":       calc.IMSSWorker,
		"sbc":               calc.SBC,
		"yearly_gross_base": calc.YearlyGrossBase,
		"yearly_gross":      calc.YearlyGross,
		"yearly_net":        calc.YearlyNet,
		"monthly_adjusted":  calc.MonthlyAdjusted,
		"breakdown":         apiBreakdown(calc),
		"other_benefits":    apiOtherBenefitResults(calc.OtherBenefits),
		"equity":            apiEquity(result),
	}
}

func apiBreakdown(calc *database.SalaryCalculation) map[string]any {
	return map[string]any{
		"aguinaldo_gross":               calc.AguinaldoGross,
		"aguinaldo_isr":                 calc.AguinaldoISR,
		"aguinaldo_net":                 calc.AguinaldoNet,
		"prima_vacacional_gross":        calc.PrimaVacacionalGross,
		"prima_vacacional_isr":          calc.PrimaVacacionalISR,
		"prima_vacacional_net":          calc.PrimaVacacionalNet,
		"vales_despensa_monthly":        calc.ValesDespensaMonthly,
		"fondo_ahorro_employee_monthly": calc.FondoAhorroEmployee,
		"fondo_ahorro_yearly":           calc.FondoAhorroYearly,
		"other_benefits_monthly_net":    calc.OtherBenefitsMonthlyNet,
		"infonavit_employer_monthly":    calc.InfonavitEmployerMonthly,
		"infonavit_employer_annual":     calc.InfonavitEmployerAnnual,
		"imss_employer_monthly":         calc.IMSSEmployerMonthly,
		"imss_employer_annual":          calc.IMSSEmployerAnnual,
		"has_infonavit_credit":          calc.HasInfonavitCredit,
		"unpaid_vacation_days":          calc.UnpaidVacationDays,
		"unpaid_vacation_loss":          calc.UnpaidVacationLoss,
	}
}

func apiOtherBenefitResults(benefits []database.OtherBenefitResult) []map[string]any {
	results := make([]map[string]any, 0, len(benefits))
	for _, benefit := range benefits {
		results = append(results, map[string]any{
			"name":     benefit.Name,
			"amount":   benefit.Amount,
			"tax_free": benefit.TaxFree,
			"isr":      benefit.ISR,
			"net":      benefit.Net,
			"cadence":  benefit.Cadence,
		})
	}
	return results
}

func apiEquity(result PackageResult) map[string]any {
	if result.EquityConfig == nil {
		return nil
	}

	schedule := make([]map[string]any, 0, len(result.EquitySchedule))
	for _, year := range result.EquitySchedule {
		schedule = append(schedule, map[string]any{
			"year":                      year.Year,
			"initial_grant_vested_usd":  year.InitialGrantVested,
			"refresher_total_usd":       year.RefresherTotal,
			"new_refresher_granted_usd": year.NewRefresherGranted,
			"total_vested_usd":          year.TotalVested,
			"total_vested_mxn":          year.TotalVestedMXN,
		})
	}

	return map[string]any{
		"config": map[string]any{
			"initial_grant_usd": result.EquityConfig.InitialGrantUSD,
			"has_refreshers":    result.EquityConfig.HasRefreshers,
			"refresher_min_usd": result.EquityConfig.RefresherMinUSD,
			"refresher_max_usd": result.EquityConfig.RefresherMaxUSD,
			"vesting_years":     result.EquityConfig.VestingYears,
			"exchange_rate":     result.EquityConfig.ExchangeRate,
		},
		"schedule": schedule,
	}
}

func apiMeta(fiscalYear database.FiscalYear) map[string]any {
	return map[string]any{
		"fiscal_year":  fiscalYear.Year,
		"uma_daily":    fiscalYear.UMADaily,
		"uma_monthly":  fiscalYear.UMAMonthly,
		"usd_mxn_rate": fiscalYear.USDMXNRate,
	}
}

func (app *application) activeFiscalYearJSON(w http.ResponseWriter, r *http.Request) (database.FiscalYear, bool) {
	fiscalYear, err := app.activeFiscalYear()
	if err != nil {
		if errors.Is(err, errNoActiveFiscalYear) {
			app.writeJSONError(w, r, http.StatusInternalServerError, "No active fiscal year configuration found")
			return database.FiscalYear{}, false
		}
		app.serverError(w, r, err)
		return database.FiscalYear{}, false
	}

	return fiscalYear, true
}

func (app *application) writeAPIResponse(w http.ResponseWriter, r *http.Request, body map[string]any) {
	if err := responseJSON(w, http.StatusOK, body); err != nil {
		app.serverError(w, r, err)
	}
}

func (app *application) writeAPIValidationError(w http.ResponseWriter, r *http.Request, fieldErrors []apiFieldError) {
	err := responseJSON(w, http.StatusBadRequest, map[string]any{
		"success": false,
		"error":   "Validation failed",
		"details": fieldErrors,
	})
	if err != nil {
		app.serverError(w, r, err)
	}
}

var openAPISpecFile = "static/api/openapi.json"

// apiOpenAPISpec serves the machine-readable API contract.
func (app *application) apiOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	spec, err := assets.EmbeddedFiles.ReadFile(openAPISpecFile)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(spec); err != nil {
		app.logger.Error("failed to write OpenAPI spec", "error", err)
	}
}
