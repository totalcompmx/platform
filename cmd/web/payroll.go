package main

import (
	"fmt"
	"math"

	"github.com/jcroyoaun/totalcompmx/internal/database"
)

// calculateRESICO performs RESICO regime calculation (flat rate, no IMSS, no subsidio)
func (app *application) calculateRESICO(
	monthlyIncome float64,
	unpaidVacationDays int,
	otherBenefits []OtherBenefit,
	exchangeRate float64,
	fiscalYear database.FiscalYear,
) (database.SalaryCalculation, error) {
	result := database.SalaryCalculation{
		GrossSalary:        monthlyIncome,
		UnpaidVacationDays: unpaidVacationDays,
	}

	resicoBracket, err := app.resicoBracket(fiscalYear.ID, monthlyIncome)
	if err != nil {
		return result, err
	}

	result.ISRTax = monthlyIncome * resicoBracket.ApplicableRate
	result.NetSalary = monthlyIncome - result.ISRTax

	totals := app.resicoOtherBenefits(&result, otherBenefits, monthlyIncome, exchangeRate, resicoBracket.ApplicableRate)
	applyRESICOTotals(&result, monthlyIncome, unpaidVacationDays, totals)

	return result, nil
}

type benefitTotals struct {
	MonthlyNet float64
	AnnualNet  float64
}

func (app *application) resicoBracket(fiscalYearID int, monthlyIncome float64) (database.RESICOBracket, error) {
	resicoBracket, found, err := app.db.GetRESICOBracket(fiscalYearID, monthlyIncome)
	if err != nil {
		return database.RESICOBracket{}, err
	}
	if !found {
		return database.RESICOBracket{}, fmt.Errorf("no RESICO bracket found for income %.2f", monthlyIncome)
	}
	return resicoBracket, nil
}

func (app *application) resicoOtherBenefits(result *database.SalaryCalculation, benefits []OtherBenefit, monthlyIncome float64, exchangeRate float64, rate float64) benefitTotals {
	totals := benefitTotals{}
	grossAnnualSalary := monthlyIncome * 12.0
	for _, benefit := range benefits {
		benefitResult := app.resicoOtherBenefit(benefit, grossAnnualSalary, exchangeRate, rate)
		result.OtherBenefits = append(result.OtherBenefits, benefitResult)
		totals.add(benefit.Cadence, benefitResult.Net)
	}
	result.OtherBenefitsMonthlyNet = totals.MonthlyNet
	result.NetSalary += totals.MonthlyNet
	return totals
}

func (app *application) resicoOtherBenefit(benefit OtherBenefit, grossAnnualSalary float64, exchangeRate float64, rate float64) database.OtherBenefitResult {
	amount := benefitAmount(benefit, grossAnnualSalary, exchangeRate)
	result := database.OtherBenefitResult{Name: benefit.Name, Amount: amount, TaxFree: benefit.TaxFree, Cadence: benefit.Cadence}
	if benefit.TaxFree {
		result.Net = amount
		app.logger.Info("RESICO other benefit (tax-free)", "name", benefit.Name, "gross", amount, "net", result.Net)
		return result
	}
	result.ISR = amount * rate
	result.Net = amount - result.ISR
	app.logger.Info("RESICO other benefit (taxable)", "name", benefit.Name, "gross", amount, "isr", result.ISR, "net", result.Net, "rate", rate)
	return result
}

func benefitAmount(benefit OtherBenefit, grossAnnualSalary float64, exchangeRate float64) float64 {
	if benefit.IsPercentage {
		return grossAnnualSalary * (benefit.Amount / 100.0)
	}
	if benefit.Currency == "USD" {
		return benefit.Amount * exchangeRate
	}
	return benefit.Amount
}

func (totals *benefitTotals) add(cadence string, net float64) {
	if cadence == "annual" {
		totals.AnnualNet += net
		return
	}
	totals.MonthlyNet += net
}

func applyRESICOTotals(result *database.SalaryCalculation, monthlyIncome float64, unpaidVacationDays int, totals benefitTotals) {
	result.YearlyGrossBase = monthlyIncome * 12
	result.YearlyGross = result.YearlyGrossBase
	if unpaidVacationDays > 0 {
		dailyRate := monthlyIncome / 30.4
		result.UnpaidVacationLoss = dailyRate * float64(unpaidVacationDays)
		result.YearlyGrossBase -= result.UnpaidVacationLoss
		result.YearlyGross -= result.UnpaidVacationLoss
	}
	result.YearlyNet = (result.NetSalary * 12) + totals.AnnualNet - result.UnpaidVacationLoss
	result.MonthlyAdjusted = result.YearlyNet / 12.0
}

// calculateSalaryWithBenefits performs the full Mexican payroll calculation with benefits
func (app *application) calculateSalaryWithBenefits(
	grossMonthlySalary float64,
	hasAguinaldo bool, aguinaldoDays int,
	hasValesDespensa bool, valesDespensaAmount float64,
	hasPrimaVacacional bool, vacationDays int, primaVacacionalPercent float64,
	hasFondoAhorro bool, fondoAhorroPercent float64,
	hasInfonavitCredit bool,
	otherBenefits []OtherBenefit,
	exchangeRate float64,
	fiscalYear database.FiscalYear,
) (database.SalaryCalculation, error) {
	input := salaryBenefitsInput{
		GrossMonthlySalary:  grossMonthlySalary,
		HasAguinaldo:        hasAguinaldo,
		AguinaldoDays:       aguinaldoDays,
		HasValesDespensa:    hasValesDespensa,
		ValesDespensaAmount: valesDespensaAmount,
		HasPrimaVacacional:  hasPrimaVacacional,
		VacationDays:        vacationDays,
		PrimaVacacionalPct:  primaVacacionalPercent,
		HasFondoAhorro:      hasFondoAhorro,
		FondoAhorroPct:      fondoAhorroPercent,
		HasInfonavitCredit:  hasInfonavitCredit,
		OtherBenefits:       otherBenefits,
		ExchangeRate:        exchangeRate,
	}

	result, err := app.calculateSalary(grossMonthlySalary, 1, fiscalYear)
	if err != nil {
		return result, err
	}
	if err := app.applySalaryBenefits(&result, input, fiscalYear); err != nil {
		return result, err
	}
	return result, nil
}

type salaryBenefitsInput struct {
	GrossMonthlySalary  float64
	HasAguinaldo        bool
	AguinaldoDays       int
	HasValesDespensa    bool
	ValesDespensaAmount float64
	HasPrimaVacacional  bool
	VacationDays        int
	PrimaVacacionalPct  float64
	HasFondoAhorro      bool
	FondoAhorroPct      float64
	HasInfonavitCredit  bool
	OtherBenefits       []OtherBenefit
	ExchangeRate        float64
}

func (app *application) applySalaryBenefits(result *database.SalaryCalculation, input salaryBenefitsInput, fiscalYear database.FiscalYear) error {
	app.applyMonthlyBenefits(result, input, fiscalYear)
	otherTotals, err := app.salaryOtherBenefits(result, input, fiscalYear)
	if err != nil {
		return err
	}
	if err := app.applyAnnualBenefits(result, input, fiscalYear); err != nil {
		return err
	}
	if err := app.applyEmployerBenefits(result, input, fiscalYear); err != nil {
		return err
	}
	applySalaryYearlyTotals(result, input, otherTotals)
	return nil
}

func (app *application) applyMonthlyBenefits(result *database.SalaryCalculation, input salaryBenefitsInput, fiscalYear database.FiscalYear) {
	if input.HasFondoAhorro {
		monthlyDeduction := monthlyFondoDeduction(input.GrossMonthlySalary, input.FondoAhorroPct, fiscalYear)
		result.FondoAhorroEmployee = monthlyDeduction
		result.NetSalary -= monthlyDeduction
	}
	if input.HasValesDespensa {
		monthlyVales := math.Min(input.ValesDespensaAmount, fiscalYear.UMAMonthly)
		result.ValesDespensaMonthly = monthlyVales
		result.NetSalary += monthlyVales
	}
}

func monthlyFondoDeduction(grossMonthlySalary float64, fondoAhorroPercent float64, fiscalYear database.FiscalYear) float64 {
	monthlyDeduction := grossMonthlySalary * (fondoAhorroPercent / 100.0)
	maxMonthlyDeduction := (fiscalYear.UMAAnnual * 1.3) / 12.0
	return math.Min(monthlyDeduction, maxMonthlyDeduction)
}

func (app *application) salaryOtherBenefits(result *database.SalaryCalculation, input salaryBenefitsInput, fiscalYear database.FiscalYear) (benefitTotals, error) {
	totals := benefitTotals{}
	grossAnnualSalary := input.GrossMonthlySalary * 12.0
	for _, benefit := range input.OtherBenefits {
		benefitResult, err := app.salaryOtherBenefit(benefit, grossAnnualSalary, input.ExchangeRate, input.GrossMonthlySalary, fiscalYear)
		if err != nil {
			return benefitTotals{}, err
		}
		result.OtherBenefits = append(result.OtherBenefits, benefitResult)
		totals.add(benefit.Cadence, benefitResult.Net)
	}
	result.OtherBenefitsMonthlyNet = totals.MonthlyNet
	result.NetSalary += totals.MonthlyNet
	return totals, nil
}

func (app *application) salaryOtherBenefit(benefit OtherBenefit, grossAnnualSalary float64, exchangeRate float64, grossMonthlySalary float64, fiscalYear database.FiscalYear) (database.OtherBenefitResult, error) {
	amount := benefitAmount(benefit, grossAnnualSalary, exchangeRate)
	result := database.OtherBenefitResult{Name: benefit.Name, Amount: amount, TaxFree: benefit.TaxFree, Cadence: benefit.Cadence}
	if benefit.TaxFree {
		result.Net = amount
		app.logger.Info("Other benefit (tax-free)", "name", benefit.Name, "gross", amount, "net", result.Net)
		return result, nil
	}
	isr, err := app.salaryOtherBenefitISR(benefit.Cadence, amount, grossMonthlySalary, fiscalYear.ID)
	if err != nil {
		return database.OtherBenefitResult{}, err
	}
	result.ISR = isr
	result.Net = amount - result.ISR
	app.logger.Info("Other benefit (taxable)", "name", benefit.Name, "gross", amount, "isr", result.ISR, "net", result.Net)
	return result, nil
}

func (app *application) salaryOtherBenefitISR(cadence string, amount float64, grossMonthlySalary float64, fiscalYearID int) (float64, error) {
	isrBrackets, err := app.db.GetISRBrackets(fiscalYearID)
	if err != nil {
		return 0, err
	}
	if cadence == "annual" {
		return calculateTaxArt174(grossMonthlySalary, amount, isrBrackets), nil
	}
	return calculateISR(amount, isrBrackets), nil
}

func (app *application) applyAnnualBenefits(result *database.SalaryCalculation, input salaryBenefitsInput, fiscalYear database.FiscalYear) error {
	if err := app.applyAguinaldo(result, input, fiscalYear); err != nil {
		return err
	}
	if err := app.applyPrimaVacacional(result, input, fiscalYear); err != nil {
		return err
	}
	if input.HasFondoAhorro {
		yearlyEmployeeContribution := result.FondoAhorroEmployee * 12
		result.FondoAhorroYearly = yearlyEmployeeContribution * 2
	}
	return nil
}

func (app *application) applyAguinaldo(result *database.SalaryCalculation, input salaryBenefitsInput, fiscalYear database.FiscalYear) error {
	if !input.HasAguinaldo {
		return nil
	}
	dailySalary := input.GrossMonthlySalary / 30.4
	result.AguinaldoGross = dailySalary * float64(input.AguinaldoDays)
	isr, err := app.exemptAnnualISR(input.GrossMonthlySalary, result.AguinaldoGross, 30.0*fiscalYear.UMADaily, fiscalYear.ID)
	if err != nil {
		return err
	}
	result.AguinaldoISR = isr
	result.AguinaldoNet = result.AguinaldoGross - result.AguinaldoISR
	return nil
}

func (app *application) applyPrimaVacacional(result *database.SalaryCalculation, input salaryBenefitsInput, fiscalYear database.FiscalYear) error {
	if !input.HasPrimaVacacional {
		return nil
	}
	dailySalary := input.GrossMonthlySalary / 30.4
	vacationSalary := dailySalary * float64(input.VacationDays)
	result.PrimaVacacionalGross = vacationSalary * (input.PrimaVacacionalPct / 100.0)
	isr, err := app.exemptAnnualISR(input.GrossMonthlySalary, result.PrimaVacacionalGross, 15.0*fiscalYear.UMADaily, fiscalYear.ID)
	if err != nil {
		return err
	}
	result.PrimaVacacionalISR = isr
	result.PrimaVacacionalNet = result.PrimaVacacionalGross - result.PrimaVacacionalISR
	return nil
}

func (app *application) exemptAnnualISR(grossMonthlySalary float64, grossAmount float64, exemptAmount float64, fiscalYearID int) (float64, error) {
	taxableBase := math.Max(0, grossAmount-exemptAmount)
	if taxableBase <= 0 {
		return 0, nil
	}
	isrBrackets, err := app.db.GetISRBrackets(fiscalYearID)
	if err != nil {
		return 0, err
	}
	return calculateTaxArt174(grossMonthlySalary, taxableBase, isrBrackets), nil
}

func (app *application) applyEmployerBenefits(result *database.SalaryCalculation, input salaryBenefitsInput, fiscalYear database.FiscalYear) error {
	monthlySBC := result.SBC * 30.4 // Daily SBC to Monthly
	result.InfonavitEmployerMonthly = monthlySBC * 0.05
	result.InfonavitEmployerAnnual = result.InfonavitEmployerMonthly * 12
	result.HasInfonavitCredit = input.HasInfonavitCredit

	imssEmployer, err := app.calculateIMSSEmployer(input.GrossMonthlySalary, fiscalYear)
	if err != nil {
		return err
	}
	result.IMSSEmployerMonthly = imssEmployer
	result.IMSSEmployerAnnual = imssEmployer * 12
	return nil
}

func applySalaryYearlyTotals(result *database.SalaryCalculation, input salaryBenefitsInput, totals benefitTotals) {
	result.YearlyGrossBase = input.GrossMonthlySalary * 12
	result.YearlyGross = result.YearlyGrossBase + result.AguinaldoGross + result.PrimaVacacionalGross +
		(result.InfonavitEmployerMonthly * 12) + (result.IMSSEmployerMonthly * 12)
	result.YearlyNet = (result.NetSalary * 12) + result.AguinaldoNet + result.PrimaVacacionalNet + result.FondoAhorroYearly + totals.AnnualNet
	result.MonthlyAdjusted = result.YearlyNet / 12.0
}

// calculateSalary performs the full Mexican payroll calculation
func (app *application) calculateSalary(grossMonthlySalary float64, yearsOfService int, fiscalYear database.FiscalYear) (database.SalaryCalculation, error) {
	result := database.SalaryCalculation{
		GrossSalary: grossMonthlySalary,
	}

	// Calculate ISR Tax
	isrBrackets, err := app.db.GetISRBrackets(fiscalYear.ID)
	if err != nil {
		return result, err
	}

	result.ISRTax = calculateISR(grossMonthlySalary, isrBrackets)

	// Calculate Subsidio al Empleo (if applicable)
	if grossMonthlySalary <= fiscalYear.SubsidyThresholdMonthly {
		result.SubsidioEmpleo = grossMonthlySalary * fiscalYear.SubsidyFactor
	}

	// Calculate IMSS Worker contributions
	imssWorker, err := app.calculateIMSSWorker(grossMonthlySalary, fiscalYear)
	if err != nil {
		return result, err
	}
	result.IMSSWorker = imssWorker

	// Calculate SBC (Salario Base de Cotización)
	// For simplicity in MVP, using gross salary as base
	// In production, you'd calculate with aguinaldo, prima vacacional, etc.
	result.SBC = calculateSBC(grossMonthlySalary, yearsOfService, fiscalYear)

	// Calculate Net Salary
	// Net = Gross - ISR + Subsidio - IMSS - Other Deductions
	result.NetSalary = grossMonthlySalary - result.ISRTax + result.SubsidioEmpleo - result.IMSSWorker

	return result, nil
}

// calculateISR calculates the ISR tax based on progressive brackets
func calculateISR(grossSalary float64, brackets []database.ISRBracket) float64 {
	for _, bracket := range brackets {
		if grossSalary >= bracket.LowerLimit && grossSalary <= bracket.UpperLimit {
			surplus := grossSalary - bracket.LowerLimit
			isr := bracket.FixedFee + (surplus * bracket.SurplusPercent)
			return math.Round(isr*100) / 100 // Round to 2 decimals
		}
	}
	return 0
}

// calculateTaxArt174 calculates ISR on annual bonuses using Article 174 methodology
// This prevents under-taxation by considering that the base salary has already consumed lower brackets
func calculateTaxArt174(grossMonthlySalary, annualBonusAmount float64, brackets []database.ISRBracket) float64 {
	if annualBonusAmount <= 0 {
		return 0
	}

	// Step A: Convert bonus to daily rate, then to monthly equivalent
	// This represents what the bonus would be if spread over the year
	remuneracionMensual := (annualBonusAmount / 365.0) * 30.4

	// Step B: Calculate partial tax
	// Tax on (Salary + Monthly Share of Bonus)
	taxOnTotal := calculateISR(grossMonthlySalary+remuneracionMensual, brackets)

	// Tax on Salary alone
	taxOnSalary := calculateISR(grossMonthlySalary, brackets)

	// Tax attributable to the monthly share of the bonus
	taxOnShare := taxOnTotal - taxOnSalary

	// Step C: Calculate effective rate
	// This is the marginal rate at which this bonus should be taxed
	var effectiveRate float64
	if remuneracionMensual > 0 {
		effectiveRate = taxOnShare / remuneracionMensual
	}

	// Step D: Apply rate to full bonus
	taxToWithhold := annualBonusAmount * effectiveRate

	return math.Round(taxToWithhold*100) / 100 // Round to 2 decimals
}

// calculateIMSSWorker calculates the worker's IMSS contributions
func (app *application) calculateIMSSWorker(grossSalary float64, fiscalYear database.FiscalYear) (float64, error) {
	return app.calculateIMSSContributions(grossSalary, fiscalYear, app.workerIMSSContribution)
}

// calculateIMSSEmployer calculates the employer's IMSS contributions
// This is NON-LIQUID compensation (doesn't go to employee's pocket)
func (app *application) calculateIMSSEmployer(grossSalary float64, fiscalYear database.FiscalYear) (float64, error) {
	return app.calculateIMSSContributions(grossSalary, fiscalYear, app.employerIMSSContribution)
}

type imssContributionFunc func(concept database.IMSSConcept, dailySalary float64, monthlyBase float64, fiscalYear database.FiscalYear) (float64, error)

func (app *application) calculateIMSSContributions(grossSalary float64, fiscalYear database.FiscalYear, contributionFor imssContributionFunc) (float64, error) {
	concepts, err := app.db.GetIMSSConcepts()
	if err != nil {
		return 0, err
	}
	return app.sumIMSSContributions(grossSalary, fiscalYear, concepts, contributionFor)
}

func (app *application) sumIMSSContributions(grossSalary float64, fiscalYear database.FiscalYear, concepts []database.IMSSConcept, contributionFor imssContributionFunc) (float64, error) {
	dailySalary := grossSalary / 30.4 // Average days in a month
	var total float64
	for _, concept := range concepts {
		monthlyBase := imssMonthlyBase(dailySalary, concept, fiscalYear)
		contribution, err := contributionFor(concept, dailySalary, monthlyBase, fiscalYear)
		if err != nil {
			return 0, err
		}
		total += contribution
	}
	return math.Round(total*100) / 100, nil
}

func imssMonthlyBase(dailySalary float64, concept database.IMSSConcept, fiscalYear database.FiscalYear) float64 {
	return imssBaseForCalculation(dailySalary, concept, fiscalYear) * 30.4
}

func imssBaseForCalculation(dailySalary float64, concept database.IMSSConcept, fiscalYear database.FiscalYear) float64 {
	if concept.BaseCapInUMAs <= 0 {
		return dailySalary
	}
	maxBase := float64(concept.BaseCapInUMAs) * fiscalYear.UMADaily
	return math.Min(dailySalary, maxBase)
}

func (app *application) workerIMSSContribution(concept database.IMSSConcept, dailySalary float64, monthlyBase float64, fiscalYear database.FiscalYear) (float64, error) {
	contribution := monthlyBase * concept.WorkerPercent
	if isProgressiveCesantia(concept) {
		_, _, err := app.db.GetCesantiaBracket(fiscalYear.ID, dailySalary/fiscalYear.UMADaily)
		if err != nil {
			return 0, err
		}
	}
	return contribution, nil
}

func (app *application) employerIMSSContribution(concept database.IMSSConcept, dailySalary float64, monthlyBase float64, fiscalYear database.FiscalYear) (float64, error) {
	if !isProgressiveCesantia(concept) {
		return monthlyBase * concept.EmployerPercent, nil
	}
	bracket, found, err := app.db.GetCesantiaBracket(fiscalYear.ID, dailySalary/fiscalYear.UMADaily)
	if err != nil {
		return 0, err
	}
	if found {
		return monthlyBase * bracket.EmployerPercent, nil
	}
	return monthlyBase * concept.EmployerPercent, nil
}

func isProgressiveCesantia(concept database.IMSSConcept) bool {
	return !concept.IsFixedRate && concept.ConceptName == "Cesantía en Edad Avanzada y Vejez"
}

// calculateSBC calculates the Salario Base de Cotización
// This is a simplified version for MVP
func calculateSBC(grossMonthlySalary float64, yearsOfService int, fiscalYear database.FiscalYear) float64 {
	// Simplified: In reality, you'd need to calculate with aguinaldo, prima vacacional, etc.
	// For MVP, we'll use a basic factor

	// Default integration factor (aguinaldo 15 days + prima vac 12 days * 25% = 18 days / 365)
	integrationFactor := 1.0493 // Approximately 4.93% additional for benefits

	// If years of service > 0, adjust slightly (simplified)
	if yearsOfService > 0 {
		// Each year adds slight increase due to more vacation days
		additionalFactor := float64(yearsOfService) * 0.001
		integrationFactor += additionalFactor
	}

	dailySalary := grossMonthlySalary / 30.4
	sbc := dailySalary * integrationFactor

	// Cap at 25 UMAs
	maxSBC := 25 * fiscalYear.UMADaily
	if sbc > maxSBC {
		sbc = maxSBC
	}

	return math.Round(sbc*100) / 100
}
