package main

import (
	"bytes"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jcroyoaun/totalcompmx/internal/assert"
	"github.com/jcroyoaun/totalcompmx/internal/database"
	"github.com/jcroyoaun/totalcompmx/internal/pdf"
	"github.com/jcroyoaun/totalcompmx/internal/token"
)

func TestHomePostStoresComparisonResults(t *testing.T) {
	app := newTestApplication(t)
	req := homePostRequest(t)

	res := sendWithCSRFToken(t, req, app.routes())

	assert.Equal(t, res.StatusCode, http.StatusSeeOther)
	session := getTestSession(t, app.sessionManager, res.Cookies())
	assert.NotNil(t, session)
	assert.NotNil(t, session.data["comparisonResults"])
	assert.NotNil(t, session.data["packageInputs"])
	assert.NotNil(t, session.data["bestPackage"])
}

func TestHomePostRejectsMissingSalary(t *testing.T) {
	app := newTestApplication(t)
	req := newTestRequest(t, http.MethodPost, "/")
	req.PostForm.Add("GrossMonthlySalary[]", "0")

	res := sendWithCSRFToken(t, req, app.routes())

	assert.Equal(t, res.StatusCode, http.StatusUnprocessableEntity)
}

func TestHomeGetUsesStoredSessionResults(t *testing.T) {
	app := newTestApplication(t)
	session := newTestSession(t, app.sessionManager, map[string]any{
		"packageInputs":     []PackageInput{{Name: "Stored"}},
		"comparisonResults": []PackageResult{{PackageName: "Stored", SalaryCalculation: &database.SalaryCalculation{YearlyNet: 1}}},
		"bestPackage":       &PackageResult{PackageName: "Stored", SalaryCalculation: &database.SalaryCalculation{YearlyNet: 1}},
		"fiscalYear":        testFiscalYear(),
	})
	req := newTestRequest(t, http.MethodGet, "/")
	req.AddCookie(session.cookie)

	res := send(t, req, app.routes())

	assert.Equal(t, res.StatusCode, http.StatusOK)
}

func TestClearSessionRemovesComparisonData(t *testing.T) {
	app := newTestApplication(t)
	session := newTestSession(t, app.sessionManager, map[string]any{
		"packageInputs":     []PackageInput{{Name: "Stored"}},
		"comparisonResults": []PackageResult{{PackageName: "Stored"}},
		"bestPackage":       &PackageResult{PackageName: "Stored"},
		"fiscalYear":        testFiscalYear(),
	})
	req := newTestRequest(t, http.MethodPost, "/clear")
	req.AddCookie(session.cookie)

	res := sendWithCSRFToken(t, req, app.routes())

	assert.Equal(t, res.StatusCode, http.StatusSeeOther)
	updated := getTestSession(t, app.sessionManager, res.Cookies())
	assert.NotNil(t, updated)
	assert.Nil(t, updated.data["comparisonResults"])
}

func TestCalculatorRoutes(t *testing.T) {
	t.Run("GET renders calculator", func(t *testing.T) {
		app := newTestApplication(t)
		req := newTestRequest(t, http.MethodGet, "/calculator")

		res := send(t, req, app.routes())

		assert.Equal(t, res.StatusCode, http.StatusOK)
	})

	t.Run("POST renders result", func(t *testing.T) {
		app := newTestApplication(t)
		req := newTestRequest(t, http.MethodPost, "/calculator")
		req.PostForm.Add("GrossMonthlySalary", "50000")
		req.PostForm.Add("YearsOfService", "3")

		res := sendWithCSRFToken(t, req, app.routes())

		assert.Equal(t, res.StatusCode, http.StatusInternalServerError)
	})

	t.Run("POST rejects invalid input", func(t *testing.T) {
		app := newTestApplication(t)
		req := newTestRequest(t, http.MethodPost, "/calculator")
		req.PostForm.Add("GrossMonthlySalary", "-1")
		req.PostForm.Add("YearsOfService", "-1")

		res := sendWithCSRFToken(t, req, app.routes())

		assert.Equal(t, res.StatusCode, http.StatusUnprocessableEntity)
	})
}

func TestStaticContentRoutes(t *testing.T) {
	for _, route := range []string{"/privacy", "/terms", "/developers", "/robots.txt", "/sitemap.xml"} {
		t.Run(route, func(t *testing.T) {
			app := newTestApplication(t)
			req := newTestRequest(t, http.MethodGet, route)

			res := send(t, req, app.routes())

			assert.Equal(t, res.StatusCode, http.StatusOK)
		})
	}
}

func TestDeveloperAccountRoutes(t *testing.T) {
	t.Run("renders account", func(t *testing.T) {
		app := newTestApplication(t)
		req := authenticatedRequest(t, app, http.MethodGet, "/account/developer")

		res := send(t, req, app.routes())

		assert.Equal(t, res.StatusCode, http.StatusOK)
	})

	t.Run("generates api key", func(t *testing.T) {
		app := newTestApplication(t)
		req := authenticatedRequest(t, app, http.MethodPost, "/account/api-key")

		res := sendWithCSRFToken(t, req, app.routes())

		assert.Equal(t, res.StatusCode, http.StatusSeeOther)
		user := mustUser(t, app, testUsers["alice"].id)
		assert.True(t, user.ApiKey.Valid)
	})
}

func TestEmailVerificationRoutes(t *testing.T) {
	t.Run("verifies token", func(t *testing.T) {
		app := newTestApplication(t)
		plaintextToken := insertVerificationToken(t, app, testUsers["alice"].id)
		req := newTestRequest(t, http.MethodGet, "/verify-email/"+plaintextToken)

		res := send(t, req, app.routes())

		assert.Equal(t, res.StatusCode, http.StatusOK)
		assert.True(t, mustUser(t, app, testUsers["alice"].id).EmailVerified)
	})

	t.Run("rejects invalid token", func(t *testing.T) {
		app := newTestApplication(t)
		req := newTestRequest(t, http.MethodGet, "/verify-email/invalid")

		res := send(t, req, app.routes())

		assert.Equal(t, res.StatusCode, http.StatusBadRequest)
	})
}

func TestResendVerificationRoutes(t *testing.T) {
	t.Run("resends for unverified user", func(t *testing.T) {
		app := newTestApplication(t)
		req := authenticatedRequest(t, app, http.MethodPost, "/account/resend-verification")

		res := sendWithCSRFToken(t, req, app.routes())
		app.wg.Wait()

		assert.Equal(t, res.StatusCode, http.StatusSeeOther)
		assert.Equal(t, len(app.mailer.SentMessages), 1)
	})

	t.Run("redirects for verified user", func(t *testing.T) {
		app := newTestApplication(t)
		markUserVerified(t, app, testUsers["alice"].id)
		req := authenticatedRequest(t, app, http.MethodPost, "/account/resend-verification")

		res := sendWithCSRFToken(t, req, app.routes())

		assert.Equal(t, res.StatusCode, http.StatusSeeOther)
		assert.Equal(t, len(app.mailer.SentMessages), 0)
	})
}

func TestAPIKeyMiddlewareAndCalculateRoute(t *testing.T) {
	t.Run("calculates salary with valid API key", func(t *testing.T) {
		app := apiTestApplication(t)
		req := jsonAPIRequest(t, `{"salary":50000,"regime":"sueldos_salarios"}`)

		res := send(t, req, app.routes())

		assert.Equal(t, res.StatusCode, http.StatusOK)
		assert.True(t, strings.Contains(res.Body, `"success": true`))
	})

	t.Run("calculates resico with valid API key", func(t *testing.T) {
		app := apiTestApplication(t)
		req := jsonAPIRequest(t, `{"salary":50000,"regime":"resico","unpaid_vacation_days":2}`)

		res := send(t, req, app.routes())

		assert.Equal(t, res.StatusCode, http.StatusOK)
	})

	t.Run("rejects bad authorization", func(t *testing.T) {
		app := newTestApplication(t)
		req := jsonAPIRequest(t, `{"salary":50000}`)
		req.Header.Del("Authorization")

		res := send(t, req, app.routes())

		assert.Equal(t, res.StatusCode, http.StatusUnauthorized)
	})

	t.Run("rejects invalid salary", func(t *testing.T) {
		app := apiTestApplication(t)
		req := jsonAPIRequest(t, `{"salary":0}`)

		res := send(t, req, app.routes())

		assert.Equal(t, res.StatusCode, http.StatusBadRequest)
	})
}

func TestExportPDFRoute(t *testing.T) {
	app := newTestApplication(t)
	restore := stubPDFGenerator(t, []byte("%PDF"), nil)
	defer restore()
	session := newTestSession(t, app.sessionManager, pdfSessionData())
	req := newTestRequest(t, http.MethodGet, "/export-pdf")
	req.AddCookie(session.cookie)

	res := send(t, req, app.routes())

	assert.Equal(t, res.StatusCode, http.StatusOK)
	assert.Equal(t, res.Header.Get("Content-Type"), "application/pdf")
}

func TestExportPDFRejectsMissingSessionData(t *testing.T) {
	app := newTestApplication(t)
	req := newTestRequest(t, http.MethodGet, "/export-pdf")

	res := send(t, req, app.routes())

	assert.Equal(t, res.StatusCode, http.StatusBadRequest)
}

func TestGenerateExportPDFReportsErrors(t *testing.T) {
	app := newTestApplication(t)
	restore := stubPDFGenerator(t, nil, errors.New("pdf failed"))
	defer restore()
	req := newTestRequest(t, http.MethodGet, "/export-pdf")

	_, ok := app.generateExportPDF(nilResponseWriter{}, req, pdfExportData{})

	assert.False(t, ok)
}

func TestPayrollHelpers(t *testing.T) {
	app := newTestApplication(t)
	fiscalYear := testFiscalYear()
	assertFloatValue(t, calculateISR(15000, testISRBrackets()), 2000)
	assertFloatValue(t, calculateISR(0, testISRBrackets()), 0)
	assertFloatValue(t, calculateTaxArt174(50000, 0, testISRBrackets()), 0)
	assert.True(t, monthlyFondoDeduction(100000, 13, fiscalYear) <= (fiscalYear.UMAAnnual*1.3)/12)
	assertFloatValue(t, imssBaseForCalculation(1000, database.IMSSConcept{}, fiscalYear), 1000)
	assert.True(t, isProgressiveCesantia(database.IMSSConcept{ConceptName: "Cesantía en Edad Avanzada y Vejez"}))
	_, err := app.workerIMSSContribution(testIMSSConcepts()[1], 1000, 10000, fiscalYear)
	if err != nil {
		t.Fatal(err)
	}
}

func homePostRequest(t *testing.T) *http.Request {
	req := newTestRequest(t, http.MethodPost, "/")
	req.PostForm.Add("PackageName[]", "Salary")
	req.PostForm.Add("PackageName[]", "Resico")
	req.PostForm.Add("Regime[]", "sueldos_salarios")
	req.PostForm.Add("Regime[]", "resico")
	req.PostForm.Add("GrossMonthlySalary[]", "50000")
	req.PostForm.Add("GrossMonthlySalary[]", "3000")
	req.PostForm.Add("Currency[]", "MXN")
	req.PostForm.Add("Currency[]", "USD")
	req.PostForm.Add("ExchangeRate[]", "20")
	req.PostForm.Add("ExchangeRate[]", "20")
	req.PostForm.Add("PaymentFrequency[]", "monthly")
	req.PostForm.Add("PaymentFrequency[]", "hourly")
	req.PostForm.Add("HoursPerWeek[]", "40")
	req.PostForm.Add("HoursPerWeek[]", "20")
	req.PostForm.Add("HasAguinaldo[]", "0")
	req.PostForm.Add("AguinaldoDays[]", "15")
	req.PostForm.Add("HasValesDespensa[]", "0")
	req.PostForm.Add("ValesDespensaAmount[]", "1000")
	req.PostForm.Add("HasPrimaVacacional[]", "0")
	req.PostForm.Add("VacationDays[]", "12")
	req.PostForm.Add("PrimaVacacionalPercent[]", "25")
	req.PostForm.Add("HasFondoAhorro[]", "0")
	req.PostForm.Add("FondoAhorroPercent[]", "13")
	req.PostForm.Add("HasInfonavitCredit[]", "0")
	req.PostForm.Add("UnpaidVacationDays[]", "0")
	req.PostForm.Add("UnpaidVacationDays[]", "2")
	req.PostForm.Add("HasEquity[]", "0")
	req.PostForm.Add("InitialEquityUSD[]", "10000")
	req.PostForm.Add("HasRefreshers[]", "0")
	req.PostForm.Add("RefresherMinUSD[]", "3000")
	req.PostForm.Add("RefresherMaxUSD[]", "2000")
	req.PostForm.Add("OtherBenefitName-0[]", "Bonus")
	req.PostForm.Add("OtherBenefitAmount-0[]", "10")
	req.PostForm.Add("OtherBenefitType-0[]", "percentage")
	req.PostForm.Add("OtherBenefitCurrency-0[]", "MXN")
	req.PostForm.Add("OtherBenefitCadence-0[]", "monthly")
	req.PostForm.Add("OtherBenefitTaxFree-0[]", "1")
	return req
}

func authenticatedRequest(t *testing.T, app *application, method string, path string) *http.Request {
	session := newTestSession(t, app.sessionManager, map[string]any{
		"authenticatedUserID": testUsers["alice"].id,
	})
	req := newTestRequest(t, method, path)
	req.AddCookie(session.cookie)
	return req
}

func insertVerificationToken(t *testing.T, app *application, userID int) string {
	plaintextToken := token.New()
	err := app.db.InsertEmailVerificationToken(userID, token.Hash(plaintextToken))
	if err != nil {
		t.Fatal(err)
	}
	return plaintextToken
}

func mustUser(t *testing.T, app *application, id int) database.User {
	user, found, err := app.db.GetUser(id)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	return user
}

func markUserVerified(t *testing.T, app *application, id int) {
	store := app.db.(*fakeStore)
	user := store.users[id]
	user.EmailVerified = true
	user.EmailVerifiedAt = sql.NullTime{Time: time.Now(), Valid: true}
	store.users[id] = user
}

func apiTestApplication(t *testing.T) *application {
	app := newTestApplication(t)
	err := app.db.UpdateUserAPIKey(testUsers["alice"].id, "api-key")
	if err != nil {
		t.Fatal(err)
	}
	markUserVerified(t, app, testUsers["alice"].id)
	return app
}

func jsonAPIRequest(t *testing.T, body string) *http.Request {
	req, err := http.NewRequest(http.MethodPost, "/api/v1/calculate", bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer api-key")
	return req
}

func pdfSessionData() map[string]any {
	calc := &database.SalaryCalculation{YearlyNet: 1000}
	return map[string]any{
		"comparisonResults": []PackageResult{{PackageName: "One", SalaryCalculation: calc}},
		"packageInputs":     []PackageInput{{Name: "One", OtherBenefits: []OtherBenefit{{Name: "Bonus", Amount: 1}}}},
		"fiscalYear":        testFiscalYear(),
	}
}

func stubPDFGenerator(t *testing.T, bytes []byte, err error) func() {
	original := generateComparisonReport
	generateComparisonReport = func(packages []pdf.PackageResult, fiscalYear database.FiscalYear) ([]byte, error) {
		return bytes, err
	}
	return func() {
		generateComparisonReport = original
	}
}

type nilResponseWriter struct{}

func (nilResponseWriter) Header() http.Header {
	return http.Header{}
}

func (nilResponseWriter) Write(bytes []byte) (int, error) {
	return len(bytes), nil
}

func (nilResponseWriter) WriteHeader(statusCode int) {}

func assertFloatValue(t *testing.T, got float64, want float64) {
	t.Helper()
	if got != want {
		t.Fatalf("got %f; want %f", got, want)
	}
}
