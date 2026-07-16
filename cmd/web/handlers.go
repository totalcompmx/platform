package main

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/jcroyoaun/totalcompmx/internal/database"
	"github.com/jcroyoaun/totalcompmx/internal/equity"
	"github.com/jcroyoaun/totalcompmx/internal/fiscalyear"
	"github.com/jcroyoaun/totalcompmx/internal/password"
	"github.com/jcroyoaun/totalcompmx/internal/pdf"
	"github.com/jcroyoaun/totalcompmx/internal/request"
	"github.com/jcroyoaun/totalcompmx/internal/response"
	"github.com/jcroyoaun/totalcompmx/internal/token"
	"github.com/jcroyoaun/totalcompmx/internal/validator"
)

var generateComparisonReport = pdf.GenerateComparisonReport
var responsePage = response.Page
var responsePageWithHeaders = response.PageWithHeaders
var responseJSON = response.JSON
var errNoActiveFiscalYear = errors.New("no active fiscal year found")

type OtherBenefit struct {
	Name         string
	Amount       float64
	TaxFree      bool
	Currency     string
	Cadence      string // monthly, annual, etc.
	IsPercentage bool   // true if Amount is a percentage of gross annual salary
}

type PackageResult struct {
	PackageName string
	*database.SalaryCalculation
	EquityConfig   *equity.EquityConfig
	EquitySchedule []equity.YearlyEquity
}

type PackageInput struct {
	Name                   string
	Regime                 string
	Currency               string
	ExchangeRate           string
	PaymentFrequency       string
	HoursPerWeek           string
	GrossMonthlySalary     string
	HasAguinaldo           bool
	AguinaldoDays          string
	HasValesDespensa       bool
	ValesDespensaAmount    string
	HasPrimaVacacional     bool
	VacationDays           string
	PrimaVacacionalPercent string
	HasFondoAhorro         bool
	FondoAhorroPercent     string
	UnpaidVacationDays     string // RESICO only: days off without pay
	OtherBenefits          []OtherBenefit
	// Equity fields
	HasEquity        bool
	InitialEquityUSD string
	HasRefreshers    bool
	RefresherMinUSD  string
	RefresherMaxUSD  string
}

func (app *application) clearSession(w http.ResponseWriter, r *http.Request) {
	// Clear all session data
	err := renewSessionToken(app.sessionManager, r.Context())
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	// Remove all calculator-related session data
	app.sessionManager.Remove(r.Context(), "packageInputs")
	app.sessionManager.Remove(r.Context(), "comparisonResults")
	app.sessionManager.Remove(r.Context(), "bestPackage")
	app.sessionManager.Remove(r.Context(), "fiscalYear")

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (app *application) home(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		app.homeGet(w, r)
		return
	}
	app.homePost(w, r)
}

type homeForm struct {
	PackageNames []string            `form:"-"`
	Validator    validator.Validator `form:"-"`
}

func (app *application) homeGet(w http.ResponseWriter, r *http.Request) {
	data := app.newTemplateData(r)

	fiscalYear, ok := app.requireActiveFiscalYear(w, r)
	if !ok {
		return
	}
	data["FiscalYear"] = fiscalYear
	app.populateHomeGetData(r, data)
	app.renderHome(w, r, http.StatusOK, data)
}

func (app *application) populateHomeGetData(r *http.Request, data map[string]any) {
	if app.sessionManager.Exists(r.Context(), "comparisonResults") {
		data["PackageInputs"] = app.sessionManager.Get(r.Context(), "packageInputs")
		data["Results"] = app.sessionManager.Get(r.Context(), "comparisonResults")
		data["BestPackage"] = app.sessionManager.Get(r.Context(), "bestPackage")
		app.applySessionFiscalYear(r, data)
	} else {
		var form homeForm
		form.PackageNames = []string{"Paquete 1", "Paquete 2"}
		data["Form"] = form
	}
}

func (app *application) applySessionFiscalYear(r *http.Request, data map[string]any) {
	if sessionFiscalYear := app.sessionManager.Get(r.Context(), "fiscalYear"); sessionFiscalYear != nil {
		data["FiscalYear"] = sessionFiscalYear
	}
}

func (app *application) homePost(w http.ResponseWriter, r *http.Request) {
	fiscalYear, payload, ok := app.readHomePost(w, r)
	if !ok {
		return
	}

	results, err := app.buildHomeResults(payload, fiscalYear)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	app.storeHomeResults(r, results, fiscalYear)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

type homePostPayload struct {
	form                    map[string][]string
	packageNames            []string
	regimes                 []string
	salaries                []string
	currencies              []string
	exchangeRates           []string
	paymentFrequencies      []string
	hoursPerWeek            []string
	hasAguinaldo            []string
	aguinaldoDays           []string
	hasValesDespensa        []string
	valesDespensaAmounts    []string
	hasPrimaVacacional      []string
	vacationDays            []string
	primaVacacionalPercents []string
	hasFondoAhorro          []string
	fondoAhorroPercents     []string
	hasInfonavitCredit      []string
	unpaidVacationDays      []string
	hasEquity               []string
	initialEquityUSD        []string
	hasRefreshers           []string
	refresherMinUSD         []string
	refresherMaxUSD         []string
}

type homePackageSalary struct {
	Original     string
	Monthly      float64
	Currency     string
	ExchangeRate float64
	PaymentFreq  string
	HoursPerWeek string
}

type homeBenefits struct {
	HasAguinaldo       bool
	AguinaldoDays      int
	HasValesDespensa   bool
	ValesAmount        float64
	HasPrimaVacacional bool
	VacationDays       int
	PrimaPercent       float64
	HasFondoAhorro     bool
	FondoPercent       float64
	HasInfonavit       bool
	UnpaidVacationDays int
}

type homeBuildResults struct {
	PackageInputs []PackageInput
	Results       []PackageResult
	BestPackage   *PackageResult
}

func (app *application) readHomePost(w http.ResponseWriter, r *http.Request) (database.FiscalYear, homePostPayload, bool) {
	if err := r.ParseForm(); err != nil {
		app.badRequest(w, r, err)
		return database.FiscalYear{}, homePostPayload{}, false
	}

	fiscalYear, ok := app.requireActiveFiscalYear(w, r)
	payload := newHomePostPayload(r.Form)
	if !ok {
		return database.FiscalYear{}, homePostPayload{}, false
	}
	if !payload.hasValidPackage() {
		app.renderInvalidHomeForm(w, r, fiscalYear)
		return database.FiscalYear{}, homePostPayload{}, false
	}
	return fiscalYear, payload, true
}

func (app *application) requireActiveFiscalYear(w http.ResponseWriter, r *http.Request) (database.FiscalYear, bool) {
	fiscalYear, err := app.activeFiscalYear()
	if err != nil {
		app.serverError(w, r, err)
		return database.FiscalYear{}, false
	}
	return fiscalYear, true
}

func (app *application) activeFiscalYear() (database.FiscalYear, error) {
	fiscalYear, found, err := app.db.GetActiveFiscalYear()
	if err != nil {
		return database.FiscalYear{}, err
	}
	if !found {
		return database.FiscalYear{}, errNoActiveFiscalYear
	}
	return fiscalYear, nil
}

func (app *application) renderInvalidHomeForm(w http.ResponseWriter, r *http.Request, fiscalYear database.FiscalYear) {
	var form homeForm
	form.Validator.AddFieldError("GrossMonthlySalary", "Debes ingresar al menos un salario válido para comparar")
	data := app.newTemplateData(r)
	data["FiscalYear"] = fiscalYear
	data["Form"] = form
	app.renderHome(w, r, http.StatusUnprocessableEntity, data)
}

func (app *application) renderHome(w http.ResponseWriter, r *http.Request, status int, data map[string]any) {
	app.renderPage(w, r, status, data, "pages/home.tmpl")
}

func (app *application) renderFormPage(w http.ResponseWriter, r *http.Request, status int, page string, form any) {
	data := app.newTemplateData(r)
	data["Form"] = form
	app.renderPage(w, r, status, data, page)
}

func (app *application) renderPage(w http.ResponseWriter, r *http.Request, status int, data map[string]any, page string) {
	err := responsePage(w, status, data, page)
	if err != nil {
		app.serverError(w, r, err)
	}
}

func (app *application) renderStaticPage(w http.ResponseWriter, r *http.Request, page string) {
	app.renderPage(w, r, http.StatusOK, app.newTemplateData(r), page)
}

func newHomePostPayload(form map[string][]string) homePostPayload {
	return homePostPayload{
		form:                    form,
		packageNames:            form["PackageName[]"],
		regimes:                 form["Regime[]"],
		salaries:                form["GrossMonthlySalary[]"],
		currencies:              form["Currency[]"],
		exchangeRates:           form["ExchangeRate[]"],
		paymentFrequencies:      form["PaymentFrequency[]"],
		hoursPerWeek:            form["HoursPerWeek[]"],
		hasAguinaldo:            form["HasAguinaldo[]"],
		aguinaldoDays:           form["AguinaldoDays[]"],
		hasValesDespensa:        form["HasValesDespensa[]"],
		valesDespensaAmounts:    form["ValesDespensaAmount[]"],
		hasPrimaVacacional:      form["HasPrimaVacacional[]"],
		vacationDays:            form["VacationDays[]"],
		primaVacacionalPercents: form["PrimaVacacionalPercent[]"],
		hasFondoAhorro:          form["HasFondoAhorro[]"],
		fondoAhorroPercents:     form["FondoAhorroPercent[]"],
		hasInfonavitCredit:      form["HasInfonavitCredit[]"],
		unpaidVacationDays:      form["UnpaidVacationDays[]"],
		hasEquity:               form["HasEquity[]"],
		initialEquityUSD:        form["InitialEquityUSD[]"],
		hasRefreshers:           form["HasRefreshers[]"],
		refresherMinUSD:         form["RefresherMinUSD[]"],
		refresherMaxUSD:         form["RefresherMaxUSD[]"],
	}
}

func (payload homePostPayload) numPackages() int {
	if len(payload.salaries) == 0 {
		return 2
	}
	return len(payload.salaries)
}

func (payload homePostPayload) hasValidPackage() bool {
	for i := 0; i < payload.numPackages(); i++ {
		if parseFloat(indexedValue(payload.salaries, i, "0"), 0) > 0 {
			return true
		}
	}
	return false
}

func (payload homePostPayload) salary(index int, defaultExchangeRate float64) (homePackageSalary, bool) {
	salary := homePackageSalary{
		Original:     indexedValue(payload.salaries, index, ""),
		Currency:     indexedValue(payload.currencies, index, "MXN"),
		ExchangeRate: parseFloat(indexedValue(payload.exchangeRates, index, ""), defaultExchangeRate),
		PaymentFreq:  indexedValue(payload.paymentFrequencies, index, "monthly"),
		HoursPerWeek: indexedValue(payload.hoursPerWeek, index, ""),
	}
	salary.Monthly = parseFloat(salary.Original, 0)
	if salary.Monthly <= 0 {
		return salary, false
	}
	if salary.Currency == "USD" {
		salary.Monthly *= salary.ExchangeRate
	}
	salary.Monthly = monthlySalary(salary.Monthly, salary.PaymentFreq, parseFloat(salary.HoursPerWeek, 40.0))
	return salary, true
}

func (payload homePostPayload) benefits(index int, regime string) homeBenefits {
	if regime == "resico" {
		return homeBenefits{UnpaidVacationDays: parseInt(indexedValue(payload.unpaidVacationDays, index, ""), 0)}
	}
	benefits := homeBenefits{}
	benefits.HasAguinaldo, benefits.AguinaldoDays = checkedInt(index, payload.hasAguinaldo, payload.aguinaldoDays, 15)
	benefits.HasValesDespensa, benefits.ValesAmount = checkedFloat(index, payload.hasValesDespensa, payload.valesDespensaAmounts, 0)
	benefits.HasPrimaVacacional, benefits.VacationDays = checkedInt(index, payload.hasPrimaVacacional, payload.vacationDays, 12)
	_, benefits.PrimaPercent = checkedFloat(index, payload.hasPrimaVacacional, payload.primaVacacionalPercents, 25)
	benefits.HasFondoAhorro, benefits.FondoPercent = checkedFloat(index, payload.hasFondoAhorro, payload.fondoAhorroPercents, 13)
	benefits.HasInfonavit = containsIndex(payload.hasInfonavitCredit, index)
	return benefits
}

func (payload homePostPayload) otherBenefits(index int) []OtherBenefit {
	names := payload.form[fmt.Sprintf("OtherBenefitName-%d[]", index)]
	benefits := make([]OtherBenefit, 0, len(names))
	for i, name := range names {
		if benefit, ok := payload.otherBenefit(index, i, name); ok {
			benefits = append(benefits, benefit)
		}
	}
	return benefits
}

func (payload homePostPayload) otherBenefit(packageIndex, benefitIndex int, name string) (OtherBenefit, bool) {
	amounts := payload.form[fmt.Sprintf("OtherBenefitAmount-%d[]", packageIndex)]
	amount := parseFloat(indexedValue(amounts, benefitIndex, ""), 0)
	if name == "" || amount <= 0 {
		return OtherBenefit{}, false
	}
	cadence := indexedValue(payload.form[fmt.Sprintf("OtherBenefitCadence-%d[]", packageIndex)], benefitIndex, "monthly")
	isPercentage := indexedValue(payload.form[fmt.Sprintf("OtherBenefitType-%d[]", packageIndex)], benefitIndex, "") == "percentage"
	if isPercentage {
		cadence = "annual"
	}
	return OtherBenefit{
		Name:         name,
		Amount:       amount,
		TaxFree:      indexedValue(payload.form[fmt.Sprintf("OtherBenefitTaxFree-%d[]", packageIndex)], benefitIndex, "") == "true",
		Currency:     indexedValue(payload.form[fmt.Sprintf("OtherBenefitCurrency-%d[]", packageIndex)], benefitIndex, "MXN"),
		Cadence:      cadence,
		IsPercentage: isPercentage,
	}, true
}

// packageSpec converts one parsed form package into the canonical calculation
// input shared with the JSON API.
func (payload homePostPayload) packageSpec(index int, name string, regime string, salary homePackageSalary, benefits homeBenefits, otherBenefits []OtherBenefit) packageSpec {
	return packageSpec{
		Name:                   name,
		Regime:                 regime,
		Salary:                 parseFloat(salary.Original, 0),
		Currency:               salary.Currency,
		ExchangeRate:           salary.ExchangeRate,
		PaymentFrequency:       salary.PaymentFreq,
		HoursPerWeek:           parseFloat(salary.HoursPerWeek, 40.0),
		HasAguinaldo:           benefits.HasAguinaldo,
		AguinaldoDays:          benefits.AguinaldoDays,
		HasValesDespensa:       benefits.HasValesDespensa,
		ValesDespensaAmount:    benefits.ValesAmount,
		HasPrimaVacacional:     benefits.HasPrimaVacacional,
		VacationDays:           benefits.VacationDays,
		PrimaVacacionalPercent: benefits.PrimaPercent,
		HasFondoAhorro:         benefits.HasFondoAhorro,
		FondoAhorroPercent:     benefits.FondoPercent,
		HasInfonavitCredit:     benefits.HasInfonavit,
		UnpaidVacationDays:     benefits.UnpaidVacationDays,
		OtherBenefits:          otherBenefits,
		HasEquity:              containsIndex(payload.hasEquity, index),
		InitialEquityUSD:       parseFloat(indexedValue(payload.initialEquityUSD, index, ""), 0),
		HasRefreshers:          containsIndex(payload.hasRefreshers, index),
		RefresherMinUSD:        parseFloat(indexedValue(payload.refresherMinUSD, index, ""), 0),
		RefresherMaxUSD:        parseFloat(indexedValue(payload.refresherMaxUSD, index, ""), 0),
	}
}

func (payload homePostPayload) packageInput(index int, name string, salary homePackageSalary, benefits homeBenefits, otherBenefits []OtherBenefit) PackageInput {
	return PackageInput{
		Name:                   name,
		Regime:                 indexedValue(payload.regimes, index, "sueldos_salarios"),
		Currency:               salary.Currency,
		ExchangeRate:           indexedValue(payload.exchangeRates, index, ""),
		PaymentFrequency:       salary.PaymentFreq,
		HoursPerWeek:           salary.HoursPerWeek,
		GrossMonthlySalary:     salary.Original,
		HasAguinaldo:           benefits.HasAguinaldo,
		AguinaldoDays:          fmt.Sprintf("%d", benefits.AguinaldoDays),
		HasValesDespensa:       benefits.HasValesDespensa,
		ValesDespensaAmount:    fmt.Sprintf("%.2f", benefits.ValesAmount),
		HasPrimaVacacional:     benefits.HasPrimaVacacional,
		VacationDays:           fmt.Sprintf("%d", benefits.VacationDays),
		PrimaVacacionalPercent: fmt.Sprintf("%.2f", benefits.PrimaPercent),
		HasFondoAhorro:         benefits.HasFondoAhorro,
		FondoAhorroPercent:     fmt.Sprintf("%.2f", benefits.FondoPercent),
		UnpaidVacationDays:     fmt.Sprintf("%d", benefits.UnpaidVacationDays),
		OtherBenefits:          otherBenefits,
		HasEquity:              containsIndex(payload.hasEquity, index),
		InitialEquityUSD:       indexedValue(payload.initialEquityUSD, index, ""),
		HasRefreshers:          containsIndex(payload.hasRefreshers, index),
		RefresherMinUSD:        indexedValue(payload.refresherMinUSD, index, ""),
		RefresherMaxUSD:        indexedValue(payload.refresherMaxUSD, index, ""),
	}
}

func (app *application) buildHomeResults(payload homePostPayload, fiscalYear database.FiscalYear) (homeBuildResults, error) {
	results := homeBuildResults{}
	for i := 0; i < payload.numPackages(); i++ {
		if err := app.addHomePackage(&results, payload, fiscalYear, i); err != nil {
			return homeBuildResults{}, err
		}
	}
	return results, nil
}

func (app *application) addHomePackage(results *homeBuildResults, payload homePostPayload, fiscalYear database.FiscalYear, index int) error {
	result, input, ok, err := app.buildHomePackage(payload, fiscalYear, index)
	if err != nil {
		return err
	}
	if ok {
		results.add(result, input)
	}
	return nil
}

func (app *application) buildHomePackage(payload homePostPayload, fiscalYear database.FiscalYear, index int) (PackageResult, PackageInput, bool, error) {
	salary, ok := payload.salary(index, fiscalYear.USDMXNRate)
	if !ok {
		return PackageResult{}, PackageInput{}, false, nil
	}
	regime := indexedValue(payload.regimes, index, "sueldos_salarios")
	benefits := payload.benefits(index, regime)
	otherBenefits := payload.otherBenefits(index)
	name := indexedValue(payload.packageNames, index, fmt.Sprintf("Paquete %d", index+1))
	spec := payload.packageSpec(index, name, regime, salary, benefits, otherBenefits)
	result, err := app.calculatePackage(spec, fiscalYear)
	if err != nil {
		return PackageResult{}, PackageInput{}, false, err
	}
	return result, payload.packageInput(index, name, salary, benefits, otherBenefits), true, nil
}

func (results *homeBuildResults) add(result PackageResult, input PackageInput) {
	results.Results = append(results.Results, result)
	results.PackageInputs = append(results.PackageInputs, input)
	if results.BestPackage == nil || result.YearlyNet > results.BestPackage.SalaryCalculation.YearlyNet {
		results.BestPackage = &result
	}
}

func (app *application) storeHomeResults(r *http.Request, results homeBuildResults, fiscalYear database.FiscalYear) {
	app.sessionManager.Put(r.Context(), "packageInputs", results.PackageInputs)
	app.sessionManager.Put(r.Context(), "comparisonResults", results.Results)
	app.sessionManager.Put(r.Context(), "bestPackage", results.BestPackage)
	app.sessionManager.Put(r.Context(), "fiscalYear", fiscalYear)
}

func indexedValue(values []string, index int, fallback string) string {
	if index < len(values) && values[index] != "" {
		return values[index]
	}
	return fallback
}

func parseFloat(value string, fallback float64) float64 {
	if value == "" {
		return fallback
	}
	result := fallback
	fmt.Sscanf(value, "%f", &result)
	return result
}

func parseInt(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	result := fallback
	fmt.Sscanf(value, "%d", &result)
	return result
}

func monthlySalary(amount float64, frequency string, hoursPerWeek float64) float64 {
	if frequency == "hourly" {
		return amount * hoursPerWeek * 4.33
	}
	multipliers := map[string]float64{"daily": 30, "weekly": 4.33, "biweekly": 2.17, "monthly": 1}
	multiplier := multipliers[frequency]
	if multiplier == 0 {
		multiplier = 1
	}
	return amount * multiplier
}

func checkedInt(index int, selections []string, values []string, fallback int) (bool, int) {
	checked := containsIndex(selections, index)
	if checked {
		return true, parseInt(indexedValue(values, index, ""), fallback)
	}
	return false, fallback
}

func checkedFloat(index int, selections []string, values []string, fallback float64) (bool, float64) {
	checked := containsIndex(selections, index)
	if checked {
		return true, parseFloat(indexedValue(values, index, ""), fallback)
	}
	return false, fallback
}

func containsIndex(values []string, index int) bool {
	want := strconv.Itoa(index)
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func (app *application) signup(w http.ResponseWriter, r *http.Request) {
	form := signupForm{}
	if r.Method == http.MethodGet {
		app.renderFormPage(w, r, http.StatusOK, "pages/signup.tmpl", form)
		return
	}

	app.signupPost(w, r, &form)
}

type signupForm struct {
	Email     string              `form:"Email"`
	Password  string              `form:"Password"`
	Validator validator.Validator `form:"-"`
}

func (app *application) signupPost(w http.ResponseWriter, r *http.Request, form *signupForm) {
	if err := request.DecodePostForm(r, form); err != nil {
		app.badRequest(w, r, err)
		return
	}

	if !app.prepareSignup(w, r, form) {
		return
	}

	id, plaintextToken, ok := app.finishSignup(w, r, form)
	if !ok {
		return
	}

	app.sessionManager.Put(r.Context(), "authenticatedUserID", id)
	app.sendWelcomeVerificationEmail(r, form.Email, plaintextToken)
	http.Redirect(w, r, "/account/developer", http.StatusSeeOther)
}

func (app *application) prepareSignup(w http.ResponseWriter, r *http.Request, form *signupForm) bool {
	_, found, err := app.db.GetUserByEmail(form.Email)
	if err != nil {
		app.serverError(w, r, err)
		return false
	}

	validateSignupForm(form, found)
	if !form.Validator.HasErrors() {
		return true
	}

	app.renderFormPage(w, r, http.StatusUnprocessableEntity, "pages/signup.tmpl", form)
	return false
}

func validateSignupForm(form *signupForm, found bool) {
	form.Validator.CheckField(form.Email != "", "Email", "Email is required")
	form.Validator.CheckField(validator.Matches(form.Email, validator.RgxEmail), "Email", "Must be a valid email address")
	form.Validator.CheckField(!found, "Email", "Email is already in use")
	form.Validator.CheckField(form.Password != "", "Password", "Password is required")
	form.Validator.CheckField(len(form.Password) >= 8, "Password", "Password is too short")
	form.Validator.CheckField(len(form.Password) <= 72, "Password", "Password is too long")
	form.Validator.CheckField(validator.NotIn(form.Password, password.CommonPasswords...), "Password", "Password is too common")
}

func (app *application) finishSignup(w http.ResponseWriter, r *http.Request, form *signupForm) (int, string, bool) {
	id, err := app.createSignupUser(form)
	if err != nil {
		app.serverError(w, r, err)
		return 0, "", false
	}

	if err = renewSessionToken(app.sessionManager, r.Context()); err != nil {
		app.serverError(w, r, err)
		return 0, "", false
	}

	plaintextToken, err := app.insertVerificationToken(id)
	if err != nil {
		app.serverError(w, r, err)
		return 0, "", false
	}

	return id, plaintextToken, true
}

func (app *application) createSignupUser(form *signupForm) (int, error) {
	hashedPassword, err := password.Hash(form.Password)
	if err != nil {
		return 0, err
	}

	return app.db.InsertUser(form.Email, hashedPassword)
}

func (app *application) insertVerificationToken(userID int) (string, error) {
	plaintextToken := token.New()
	hashedToken := token.Hash(plaintextToken)
	err := app.db.InsertEmailVerificationToken(userID, hashedToken)
	return plaintextToken, err
}

func (app *application) sendWelcomeVerificationEmail(r *http.Request, email string, plaintextToken string) {
	app.backgroundTask(r, func() error {
		data := app.newEmailData()
		data["Email"] = email
		data["VerificationToken"] = plaintextToken
		return sendMail(app.mailer, email, data, "welcome.tmpl")
	})
}

func (app *application) login(w http.ResponseWriter, r *http.Request) {
	form := loginForm{}
	if r.Method == http.MethodGet {
		app.renderFormPage(w, r, http.StatusOK, "pages/login.tmpl", form)
		return
	}

	app.loginPost(w, r, &form)
}

type loginForm struct {
	Email     string              `form:"Email"`
	Password  string              `form:"Password"`
	Validator validator.Validator `form:"-"`
}

func (app *application) loginPost(w http.ResponseWriter, r *http.Request, form *loginForm) {
	if err := request.DecodePostForm(r, form); err != nil {
		app.badRequest(w, r, err)
		return
	}

	user, ok := app.validLoginUser(w, r, form)
	if !ok {
		return
	}

	if !app.loginUser(w, r, user.ID) {
		return
	}

	app.redirectAfterLogin(w, r)
}

func (app *application) validLoginUser(w http.ResponseWriter, r *http.Request, form *loginForm) (database.User, bool) {
	user, found, err := app.db.GetUserByEmail(form.Email)
	if err != nil {
		app.serverError(w, r, err)
		return database.User{}, false
	}

	if err := validateLoginForm(form, user, found); err != nil {
		app.serverError(w, r, err)
		return database.User{}, false
	}

	if !form.Validator.HasErrors() {
		return user, true
	}

	app.renderFormPage(w, r, http.StatusUnprocessableEntity, "pages/login.tmpl", form)
	return database.User{}, false
}

func validateLoginForm(form *loginForm, user database.User, found bool) error {
	form.Validator.CheckField(form.Email != "", "Email", "Email is required")
	form.Validator.CheckField(found, "Email", "Email address could not be found")
	if !found {
		return nil
	}

	passwordMatches, err := password.Matches(form.Password, user.HashedPassword)
	if err != nil {
		return err
	}

	form.Validator.CheckField(form.Password != "", "Password", "Password is required")
	form.Validator.CheckField(passwordMatches, "Password", "Password is incorrect")
	return nil
}

func (app *application) loginUser(w http.ResponseWriter, r *http.Request, userID int) bool {
	if err := renewSessionToken(app.sessionManager, r.Context()); err != nil {
		app.serverError(w, r, err)
		return false
	}

	app.sessionManager.Put(r.Context(), "authenticatedUserID", userID)
	return true
}

func (app *application) redirectAfterLogin(w http.ResponseWriter, r *http.Request) {
	redirectPath := app.sessionManager.PopString(r.Context(), "redirectPathAfterLogin")
	if redirectPath != "" {
		http.Redirect(w, r, redirectPath, http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/account/developer", http.StatusSeeOther)
}

func (app *application) logout(w http.ResponseWriter, r *http.Request) {
	err := renewSessionToken(app.sessionManager, r.Context())
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	app.sessionManager.Remove(r.Context(), "authenticatedUserID")

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (app *application) forgottenPassword(w http.ResponseWriter, r *http.Request) {
	form := forgottenPasswordForm{}
	if r.Method == http.MethodGet {
		app.renderFormPage(w, r, http.StatusOK, "pages/forgotten-password.tmpl", form)
		return
	}

	app.forgottenPasswordPost(w, r, &form)
}

type forgottenPasswordForm struct {
	Email     string              `form:"Email"`
	Validator validator.Validator `form:"-"`
}

func (app *application) forgottenPasswordPost(w http.ResponseWriter, r *http.Request, form *forgottenPasswordForm) {
	if err := request.DecodePostForm(r, form); err != nil {
		app.badRequest(w, r, err)
		return
	}

	user, ok := app.validForgottenPasswordUser(w, r, form)
	if !ok {
		return
	}

	if !app.sendPasswordResetEmail(w, r, user) {
		return
	}

	http.Redirect(w, r, "/forgotten-password-confirmation", http.StatusSeeOther)
}

func (app *application) validForgottenPasswordUser(w http.ResponseWriter, r *http.Request, form *forgottenPasswordForm) (database.User, bool) {
	user, found, err := app.db.GetUserByEmail(form.Email)
	if err != nil {
		app.serverError(w, r, err)
		return database.User{}, false
	}

	validateForgottenPasswordForm(form, found)
	if !form.Validator.HasErrors() {
		return user, true
	}

	app.renderFormPage(w, r, http.StatusUnprocessableEntity, "pages/forgotten-password.tmpl", form)
	return database.User{}, false
}

func validateForgottenPasswordForm(form *forgottenPasswordForm, found bool) {
	form.Validator.CheckField(form.Email != "", "Email", "Email is required")
	form.Validator.CheckField(validator.Matches(form.Email, validator.RgxEmail), "Email", "Must be a valid email address")
	form.Validator.CheckField(found, "Email", "No matching email found")
}

func (app *application) sendPasswordResetEmail(w http.ResponseWriter, r *http.Request, user database.User) bool {
	plaintextToken := token.New()
	hashedToken := token.Hash(plaintextToken)

	if err := app.db.InsertPasswordReset(hashedToken, user.ID, 24*time.Hour); err != nil {
		app.serverError(w, r, err)
		return false
	}

	data := app.newEmailData()
	data["PlaintextToken"] = plaintextToken
	if err := sendMail(app.mailer, user.Email, data, "forgotten-password.tmpl"); err != nil {
		app.serverError(w, r, err)
		return false
	}

	return true
}

func (app *application) forgottenPasswordConfirmation(w http.ResponseWriter, r *http.Request) {
	app.renderStaticPage(w, r, "pages/forgotten-password-confirmation.tmpl")
}

func (app *application) passwordReset(w http.ResponseWriter, r *http.Request) {
	plaintextToken := r.PathValue("plaintextToken")
	passwordReset, ok := app.validPasswordReset(w, r, plaintextToken)
	if !ok {
		return
	}

	form := passwordResetForm{}
	if r.Method == http.MethodGet {
		app.renderPasswordResetForm(w, r, http.StatusOK, form, plaintextToken)
		return
	}

	app.passwordResetPost(w, r, &form, passwordReset)
}

type passwordResetForm struct {
	NewPassword string              `form:"NewPassword"`
	Validator   validator.Validator `form:"-"`
}

func (app *application) validPasswordReset(w http.ResponseWriter, r *http.Request, plaintextToken string) (database.PasswordReset, bool) {
	passwordReset, found, err := app.db.GetPasswordReset(token.Hash(plaintextToken))
	if err != nil {
		app.serverError(w, r, err)
		return database.PasswordReset{}, false
	}

	if !found {
		app.renderInvalidPasswordReset(w, r)
		return database.PasswordReset{}, false
	}

	return passwordReset, true
}

func (app *application) renderInvalidPasswordReset(w http.ResponseWriter, r *http.Request) {
	data := app.newTemplateData(r)
	data["InvalidLink"] = true
	app.renderPage(w, r, http.StatusUnprocessableEntity, data, "pages/password-reset.tmpl")
}

func (app *application) renderPasswordResetForm(w http.ResponseWriter, r *http.Request, status int, form passwordResetForm, plaintextToken string) {
	data := app.newTemplateData(r)
	data["Form"] = form
	data["PlaintextToken"] = plaintextToken
	app.renderPage(w, r, status, data, "pages/password-reset.tmpl")
}

func (app *application) passwordResetPost(w http.ResponseWriter, r *http.Request, form *passwordResetForm, passwordReset database.PasswordReset) {
	if err := request.DecodePostForm(r, form); err != nil {
		app.badRequest(w, r, err)
		return
	}

	validatePasswordResetForm(form)
	if form.Validator.HasErrors() {
		app.renderPasswordResetForm(w, r, http.StatusUnprocessableEntity, *form, r.PathValue("plaintextToken"))
		return
	}

	if !app.updateResetPassword(w, r, passwordReset.UserID, form.NewPassword) {
		return
	}

	http.Redirect(w, r, "/password-reset-confirmation", http.StatusSeeOther)
}

func validatePasswordResetForm(form *passwordResetForm) {
	form.Validator.CheckField(form.NewPassword != "", "NewPassword", "La contraseña es obligatoria")
	form.Validator.CheckField(len(form.NewPassword) >= 8, "NewPassword", "La contraseña debe tener al menos 8 caracteres")
	form.Validator.CheckField(len(form.NewPassword) <= 72, "NewPassword", "La contraseña es demasiado larga (máximo 72 caracteres)")
	form.Validator.CheckField(validator.NotIn(form.NewPassword, password.CommonPasswords...), "NewPassword", "Esta contraseña es muy común. Usa una más segura")
}

func (app *application) updateResetPassword(w http.ResponseWriter, r *http.Request, userID int, newPassword string) bool {
	hashedPassword, err := password.Hash(newPassword)
	if err != nil {
		app.serverError(w, r, err)
		return false
	}

	if err = app.db.UpdateUserHashedPassword(userID, hashedPassword); err != nil {
		app.serverError(w, r, err)
		return false
	}

	if err = app.db.DeletePasswordResets(userID); err != nil {
		app.serverError(w, r, err)
		return false
	}

	return true
}

func (app *application) passwordResetConfirmation(w http.ResponseWriter, r *http.Request) {
	app.renderStaticPage(w, r, "pages/password-reset-confirmation.tmpl")
}

func (app *application) restricted(w http.ResponseWriter, r *http.Request) {
	app.renderStaticPage(w, r, "pages/restricted.tmpl")
}

func (app *application) salaryCalculator(w http.ResponseWriter, r *http.Request) {
	form := calculatorForm{}
	if r.Method == http.MethodGet {
		app.renderCalculatorForm(w, r, http.StatusOK, &form)
		return
	}

	app.salaryCalculatorPost(w, r, &form)
}

type calculatorForm struct {
	GrossMonthlySalary float64             `form:"GrossMonthlySalary"`
	YearsOfService     int                 `form:"YearsOfService"`
	Validator          validator.Validator `form:"-"`
}

func (app *application) salaryCalculatorPost(w http.ResponseWriter, r *http.Request, form *calculatorForm) {
	if !app.readCalculatorForm(w, r, form) {
		return
	}

	result, fiscalYear, ok := app.calculateSalaryResult(w, r, form)
	if !ok {
		return
	}

	app.renderCalculatorResult(w, r, form, result, fiscalYear)
}

func (app *application) readCalculatorForm(w http.ResponseWriter, r *http.Request, form *calculatorForm) bool {
	if err := request.DecodePostForm(r, form); err != nil {
		app.badRequest(w, r, err)
		return false
	}

	validateCalculatorForm(form)
	if !form.Validator.HasErrors() {
		return true
	}

	app.renderCalculatorForm(w, r, http.StatusUnprocessableEntity, form)
	return false
}

func validateCalculatorForm(form *calculatorForm) {
	form.Validator.CheckField(form.GrossMonthlySalary > 0, "GrossMonthlySalary", "El salario debe ser mayor a 0")
	form.Validator.CheckField(form.GrossMonthlySalary <= 1000000, "GrossMonthlySalary", "El salario es demasiado alto")
	form.Validator.CheckField(form.YearsOfService >= 0, "YearsOfService", "Los años de servicio no pueden ser negativos")
}

func (app *application) calculateSalaryResult(w http.ResponseWriter, r *http.Request, form *calculatorForm) (database.SalaryCalculation, database.FiscalYear, bool) {
	fiscalYear, ok := app.requireActiveFiscalYear(w, r)
	if !ok {
		return database.SalaryCalculation{}, database.FiscalYear{}, false
	}

	result, err := app.calculateSalary(form.GrossMonthlySalary, form.YearsOfService, fiscalYear)
	if err != nil {
		app.serverError(w, r, err)
		return database.SalaryCalculation{}, database.FiscalYear{}, false
	}

	return result, fiscalYear, true
}

func (app *application) renderCalculatorResult(w http.ResponseWriter, r *http.Request, form *calculatorForm, result database.SalaryCalculation, fiscalYear database.FiscalYear) {
	data := app.newTemplateData(r)
	data["Form"] = form
	data["Result"] = result
	data["FiscalYear"] = fiscalYear
	app.renderPage(w, r, http.StatusOK, data, "pages/calculator.tmpl")
}

func (app *application) renderCalculatorForm(w http.ResponseWriter, r *http.Request, status int, form *calculatorForm) {
	fiscalYear, ok := app.requireActiveFiscalYear(w, r)
	if !ok {
		return
	}

	data := app.newTemplateData(r)
	data["Form"] = form
	data["FiscalYear"] = fiscalYear
	app.renderPage(w, r, status, data, "pages/calculator.tmpl")
}

// privacy displays the privacy policy (Aviso de Privacidad)
func (app *application) privacy(w http.ResponseWriter, r *http.Request) {
	app.renderStaticPage(w, r, "pages/privacy.tmpl")
}

// terms displays the terms and conditions (Términos y Condiciones)
func (app *application) terms(w http.ResponseWriter, r *http.Request) {
	app.renderStaticPage(w, r, "pages/terms.tmpl")
}

// robotsTxt serves the robots.txt file for SEO
func (app *application) robotsTxt(w http.ResponseWriter, r *http.Request) {
	robotsContent := `User-agent: *
Allow: /

Sitemap: https://totalcomp.mx/sitemap.xml`

	app.writeStaticContent(w, "text/plain; charset=utf-8", robotsContent)
}

// sitemapXML serves the sitemap.xml file for SEO
func (app *application) sitemapXML(w http.ResponseWriter, r *http.Request) {
	sitemapContent := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>https://totalcomp.mx/</loc>
    <lastmod>2025-11-19</lastmod>
    <changefreq>weekly</changefreq>
    <priority>1.0</priority>
  </url>
  <url>
    <loc>https://totalcomp.mx/privacy</loc>
    <lastmod>2025-11-19</lastmod>
    <changefreq>monthly</changefreq>
    <priority>0.5</priority>
  </url>
  <url>
    <loc>https://totalcomp.mx/terms</loc>
    <lastmod>2025-11-19</lastmod>
    <changefreq>monthly</changefreq>
    <priority>0.5</priority>
  </url>
</urlset>`

	app.writeStaticContent(w, "application/xml; charset=utf-8", sitemapContent)
}

func (app *application) writeStaticContent(w http.ResponseWriter, contentType string, body string) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(body)); err != nil {
		app.logger.Error("failed to write static content", "error", err)
	}
}

// accountDeveloper displays the developer dashboard where users can manage their API keys
func (app *application) accountDeveloper(w http.ResponseWriter, r *http.Request) {
	user, ok := app.developerAccountUser(w, r)
	if !ok {
		return
	}

	monthlyCalls, err := app.db.GetMonthlyAPICallCount(user.ID)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	data := app.newTemplateData(r)
	data["User"] = user
	data["MonthlyCallsUsed"] = monthlyCalls
	data["MonthlyCallsLimit"] = apiMonthlyLimitVerified
	if newAPIKey := app.sessionManager.PopString(r.Context(), "newAPIKey"); newAPIKey != "" {
		data["NewAPIKey"] = newAPIKey
	}
	app.renderPage(w, r, http.StatusOK, data, "pages/developer.tmpl")
}

func (app *application) developerAccountUser(w http.ResponseWriter, r *http.Request) (database.User, bool) {
	authenticatedUser, found := contextGetAuthenticatedUser(r)
	if !found {
		app.notFound(w, r)
		return database.User{}, false
	}

	user, found, err := app.db.GetUser(authenticatedUser.ID)
	if err != nil {
		app.serverError(w, r, err)
		return database.User{}, false
	}

	if !found {
		app.notFound(w, r)
		return database.User{}, false
	}

	return user, true
}

// generateAPIKey generates or regenerates an API key for the authenticated user
func (app *application) generateAPIKey(w http.ResponseWriter, r *http.Request) {
	authenticatedUser, found := contextGetAuthenticatedUser(r)
	if !found {
		app.notFound(w, r)
		return
	}
	userID := authenticatedUser.ID

	apiKey, err := app.generateSecureAPIKey()
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	// Only the hash is persisted; the plaintext key is shown to the user once
	// (via the session) and never stored.
	err = app.db.UpdateUserAPIKey(userID, hashAPIKey(apiKey), apiKeyPrefix(apiKey))
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	app.sessionManager.Put(r.Context(), "newAPIKey", apiKey)
	http.Redirect(w, r, "/account/developer", http.StatusSeeOther)
}

// developersPage renders the public marketing page for the API
func (app *application) developersPage(w http.ResponseWriter, r *http.Request) {
	app.renderStaticPage(w, r, "pages/developers.tmpl")
}

// verifyEmail handles the email verification flow
func (app *application) verifyEmail(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.verifiedEmailUserID(w, r)
	if !ok {
		return
	}

	if err := app.db.VerifyUserEmail(userID); err != nil {
		app.serverError(w, r, err)
		return
	}

	app.renderEmailVerificationSuccess(w, r)
}

func (app *application) verifiedEmailUserID(w http.ResponseWriter, r *http.Request) (int, bool) {
	hashedToken := token.Hash(r.PathValue("plaintextToken"))
	userID, found, err := app.db.GetUserIDFromVerificationToken(hashedToken)
	if err != nil {
		app.serverError(w, r, err)
		return 0, false
	}

	if !found {
		app.renderEmailVerificationError(w, r)
		return 0, false
	}

	return userID, true
}

func (app *application) renderEmailVerificationError(w http.ResponseWriter, r *http.Request) {
	data := app.newTemplateData(r)
	data["Message"] = "El enlace de verificación es inválido o ha expirado."
	app.renderPage(w, r, http.StatusBadRequest, data, "pages/email-verification-error.tmpl")
}

func (app *application) renderEmailVerificationSuccess(w http.ResponseWriter, r *http.Request) {
	data := app.newTemplateData(r)
	app.renderPage(w, r, http.StatusOK, data, "pages/email-verification-success.tmpl")
}

// resendVerificationEmail generates a new verification token and sends it to the user's email
func (app *application) resendVerificationEmail(w http.ResponseWriter, r *http.Request) {
	userID := app.sessionManager.GetInt(r.Context(), "authenticatedUserID")
	user, ok := app.verificationEmailUser(w, r, userID)
	if !ok {
		return
	}

	plaintextToken, ok := app.refreshVerificationToken(w, r, userID)
	if !ok {
		return
	}

	app.sendWelcomeVerificationEmail(r, user.Email, plaintextToken)
	app.flashAndRedirect(w, r, "Email de verificación reenviado. Revisa tu bandeja de entrada.", "/account/developer")
}

func (app *application) verificationEmailUser(w http.ResponseWriter, r *http.Request, userID int) (database.User, bool) {
	user, found, err := app.db.GetUser(userID)
	if err != nil {
		app.serverError(w, r, err)
		return database.User{}, false
	}

	if !found {
		app.flashAndRedirect(w, r, "Usuario no encontrado", "/account/developer")
		return database.User{}, false
	}

	if user.EmailVerified {
		app.flashAndRedirect(w, r, "Tu email ya está verificado", "/account/developer")
		return database.User{}, false
	}

	return user, true
}

func (app *application) refreshVerificationToken(w http.ResponseWriter, r *http.Request, userID int) (string, bool) {
	plaintextToken := token.New()
	if err := app.replaceVerificationToken(userID, plaintextToken); err != nil {
		app.serverError(w, r, err)
		return "", false
	}

	return plaintextToken, true
}

func (app *application) replaceVerificationToken(userID int, plaintextToken string) error {
	if err := app.db.DeleteEmailVerificationTokensForUser(userID); err != nil {
		return err
	}

	return app.db.InsertEmailVerificationToken(userID, token.Hash(plaintextToken))
}

func (app *application) flashAndRedirect(w http.ResponseWriter, r *http.Request, message string, path string) {
	app.sessionManager.Put(r.Context(), "flash", message)
	http.Redirect(w, r, path, http.StatusSeeOther)
}

func (app *application) writeJSONError(w http.ResponseWriter, r *http.Request, status int, message string) {
	if err := responseJSON(w, status, map[string]any{"success": false, "error": message}); err != nil {
		app.serverError(w, r, err)
	}
}

// exportPDF generates and downloads a comparison PDF report for all packages
func (app *application) exportPDF(w http.ResponseWriter, r *http.Request) {
	exportData, ok := app.pdfExportData(w, r)
	if !ok {
		return
	}

	pdfBytes, ok := app.generateExportPDF(w, r, exportData)
	if !ok {
		return
	}

	app.writePDFResponse(w, pdfBytes, exportData.fiscalYear)
}

type pdfExportData struct {
	results     []PackageResult
	inputs      []PackageInput
	fiscalYear  database.FiscalYear
	pdfPackages []pdf.PackageResult
}

func (app *application) pdfExportData(w http.ResponseWriter, r *http.Request) (pdfExportData, bool) {
	results, ok := app.pdfComparisonResults(w, r)
	if !ok {
		return pdfExportData{}, false
	}

	packageInputs, ok := app.pdfPackageInputs(w, r)
	if !ok {
		return pdfExportData{}, false
	}

	fiscalYear, ok := app.pdfFiscalYear(w, r)
	if !ok {
		return pdfExportData{}, false
	}

	return pdfExportData{
		results:     results,
		inputs:      packageInputs,
		fiscalYear:  fiscalYear,
		pdfPackages: buildPDFPackages(results, packageInputs),
	}, true
}

func (app *application) pdfComparisonResults(w http.ResponseWriter, r *http.Request) ([]PackageResult, bool) {
	if !app.sessionManager.Exists(r.Context(), "comparisonResults") {
		app.badRequest(w, r, fmt.Errorf("no results in session"))
		return nil, false
	}

	results, ok := app.sessionManager.Get(r.Context(), "comparisonResults").([]PackageResult)
	if !ok {
		app.serverError(w, r, fmt.Errorf("invalid results in session"))
		return nil, false
	}

	if len(results) == 0 {
		app.badRequest(w, r, fmt.Errorf("no packages to compare"))
		return nil, false
	}

	return results, true
}

func (app *application) pdfPackageInputs(w http.ResponseWriter, r *http.Request) ([]PackageInput, bool) {
	packageInputs, ok := app.sessionManager.Get(r.Context(), "packageInputs").([]PackageInput)
	if !ok {
		app.serverError(w, r, fmt.Errorf("invalid package inputs in session"))
		return nil, false
	}

	return packageInputs, true
}

func (app *application) pdfFiscalYear(w http.ResponseWriter, r *http.Request) (database.FiscalYear, bool) {
	fiscalYear, ok := app.sessionManager.Get(r.Context(), "fiscalYear").(database.FiscalYear)
	if ok {
		return fiscalYear, true
	}

	return app.requireActiveFiscalYear(w, r)
}

func buildPDFPackages(results []PackageResult, packageInputs []PackageInput) []pdf.PackageResult {
	pdfPackages := make([]pdf.PackageResult, len(results))
	for i, result := range results {
		pdfPackages[i] = buildPDFPackage(result, packageInputs, i)
	}

	return pdfPackages
}

func buildPDFPackage(result PackageResult, packageInputs []PackageInput, index int) pdf.PackageResult {
	return pdf.PackageResult{
		Name:        result.PackageName,
		Input:       pdfPackageInput(packageInputs, index),
		Calculation: result.SalaryCalculation,
		Equity:      pdfPackageEquity(result),
	}
}

func pdfPackageEquity(result PackageResult) *pdf.PackageEquity {
	if result.EquityConfig == nil {
		return nil
	}

	packageEquity := &pdf.PackageEquity{
		InitialGrantUSD: result.EquityConfig.InitialGrantUSD,
		HasRefreshers:   result.EquityConfig.HasRefreshers,
		RefresherMinUSD: result.EquityConfig.RefresherMinUSD,
		RefresherMaxUSD: result.EquityConfig.RefresherMaxUSD,
		VestingYears:    result.EquityConfig.VestingYears,
		ExchangeRate:    result.EquityConfig.ExchangeRate,
	}

	for _, year := range result.EquitySchedule {
		if year.Year == 0 {
			continue
		}
		packageEquity.Schedule = append(packageEquity.Schedule, pdf.EquityYear{
			Year:               year.Year,
			InitialVestedUSD:   year.InitialGrantVested,
			RefresherVestedUSD: year.RefresherTotal,
			TotalVestedUSD:     year.TotalVested,
			TotalVestedMXN:     year.TotalVestedMXN,
		})
		packageEquity.TotalUSD += year.TotalVested
		packageEquity.TotalMXN += year.TotalVestedMXN
	}

	return packageEquity
}

func pdfPackageInput(packageInputs []PackageInput, index int) pdf.PackageInput {
	if index >= len(packageInputs) {
		return pdf.PackageInput{}
	}

	return convertPDFPackageInput(packageInputs[index])
}

func convertPDFPackageInput(input PackageInput) pdf.PackageInput {
	return pdf.PackageInput{
		Name:                   input.Name,
		Regime:                 input.Regime,
		Currency:               input.Currency,
		ExchangeRate:           input.ExchangeRate,
		PaymentFrequency:       input.PaymentFrequency,
		HoursPerWeek:           input.HoursPerWeek,
		GrossMonthlySalary:     input.GrossMonthlySalary,
		HasAguinaldo:           input.HasAguinaldo,
		AguinaldoDays:          input.AguinaldoDays,
		HasValesDespensa:       input.HasValesDespensa,
		ValesDespensaAmount:    input.ValesDespensaAmount,
		HasPrimaVacacional:     input.HasPrimaVacacional,
		VacationDays:           input.VacationDays,
		PrimaVacacionalPercent: input.PrimaVacacionalPercent,
		HasFondoAhorro:         input.HasFondoAhorro,
		FondoAhorroPercent:     input.FondoAhorroPercent,
		UnpaidVacationDays:     input.UnpaidVacationDays,
		OtherBenefits:          convertPDFOtherBenefits(input.OtherBenefits),
		HasEquity:              input.HasEquity,
		InitialEquityUSD:       input.InitialEquityUSD,
		HasRefreshers:          input.HasRefreshers,
		RefresherMinUSD:        input.RefresherMinUSD,
		RefresherMaxUSD:        input.RefresherMaxUSD,
	}
}

func convertPDFOtherBenefits(otherBenefits []OtherBenefit) []pdf.OtherBenefit {
	pdfOtherBenefits := make([]pdf.OtherBenefit, 0, len(otherBenefits))
	for _, ob := range otherBenefits {
		pdfOtherBenefits = append(pdfOtherBenefits, pdf.OtherBenefit{
			Name:     ob.Name,
			Amount:   ob.Amount,
			TaxFree:  ob.TaxFree,
			Currency: ob.Currency,
			Cadence:  ob.Cadence,
		})
	}

	return pdfOtherBenefits
}

func (app *application) generateExportPDF(w http.ResponseWriter, r *http.Request, exportData pdfExportData) ([]byte, bool) {
	pdfBytes, err := generateComparisonReport(exportData.pdfPackages, exportData.fiscalYear)
	if err != nil {
		app.serverError(w, r, err)
		return nil, false
	}

	return pdfBytes, true
}

func (app *application) writePDFResponse(w http.ResponseWriter, pdfBytes []byte, fiscalYear database.FiscalYear) {
	w.Header().Set("Content-Type", "application/pdf")
	filename := fmt.Sprintf("TotalComp_Comparacion_%d.pdf", fiscalyear.CurrentLabel(fiscalYear))
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(pdfBytes)))

	if _, err := w.Write(pdfBytes); err != nil {
		app.logger.Error("failed to write PDF response", "error", err)
	}
}
