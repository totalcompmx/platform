package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/jcroyoaun/totalcompmx/internal/assert"
)

// TestAPIWebParity guarantees the JSON API and the web form produce identical
// calculations for identical inputs, since both decode into packageSpec.
func TestAPIWebParity(t *testing.T) {
	app := newTestApplication(t)
	fiscalYear := testFiscalYear()

	form := url.Values{
		"PackageName[]":            {"Oferta"},
		"Regime[]":                 {"sueldos_salarios"},
		"GrossMonthlySalary[]":     {"60000"},
		"Currency[]":               {"MXN"},
		"ExchangeRate[]":           {"19.5"},
		"PaymentFrequency[]":       {"monthly"},
		"HasAguinaldo[]":           {"0"},
		"AguinaldoDays[]":          {"30"},
		"HasValesDespensa[]":       {"0"},
		"ValesDespensaAmount[]":    {"2000"},
		"HasPrimaVacacional[]":     {"0"},
		"VacationDays[]":           {"12"},
		"PrimaVacacionalPercent[]": {"25"},
		"HasFondoAhorro[]":         {"0"},
		"FondoAhorroPercent[]":     {"13"},
		"HasInfonavitCredit[]":     {"0"},
		"HasEquity[]":              {"0"},
		"InitialEquityUSD[]":       {"40000"},
		"HasRefreshers[]":          {"0"},
		"RefresherMinUSD[]":        {"8000"},
		"RefresherMaxUSD[]":        {"12000"},
		"OtherBenefitName-0[]":     {"Bono", "Gym"},
		"OtherBenefitAmount-0[]":   {"1000", "500"},
		"OtherBenefitCadence-0[]":  {"monthly", "annual"},
		"OtherBenefitCurrency-0[]": {"MXN", "USD"},
		"OtherBenefitTaxFree-0[]":  {"2"},
	}

	payload := newHomePostPayload(form)
	webResults, err := app.buildHomeResults(payload, fiscalYear)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(webResults.Results), 1)
	webResult := webResults.Results[0]

	apiRequest := apiPackageRequest{
		Name:                   "Oferta",
		Salary:                 60000,
		Regime:                 "sueldos_salarios",
		Currency:               "MXN",
		ExchangeRate:           19.5,
		PaymentFrequency:       "monthly",
		HasAguinaldo:           true,
		AguinaldoDays:          intPtr(30),
		HasValesDespensa:       true,
		ValesDespensaAmount:    2000,
		HasPrimaVacacional:     true,
		VacationDays:           intPtr(12),
		PrimaVacacionalPercent: floatPtr(25),
		HasFondoAhorro:         true,
		FondoAhorroPercent:     floatPtr(13),
		HasInfonavitCredit:     true,
		HasEquity:              true,
		InitialEquityUSD:       40000,
		HasRefreshers:          true,
		RefresherMinUSD:        8000,
		RefresherMaxUSD:        12000,
		OtherBenefits: []apiOtherBenefit{
			{Name: "Bono", Amount: 1000, Cadence: "monthly", Currency: "MXN"},
			{Name: "Gym", Amount: 500, Cadence: "annual", Currency: "USD", TaxFree: true},
		},
	}

	spec, fieldErrors := apiRequest.spec(fiscalYear, "", 1)
	if len(fieldErrors) > 0 {
		t.Fatalf("unexpected validation errors: %+v", fieldErrors)
	}

	apiResult, err := app.calculatePackage(spec, fiscalYear)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(*webResult.SalaryCalculation, *apiResult.SalaryCalculation) {
		t.Fatalf("web and API calculations diverge:\nweb: %+v\napi: %+v", *webResult.SalaryCalculation, *apiResult.SalaryCalculation)
	}
	if !reflect.DeepEqual(webResult.EquitySchedule, apiResult.EquitySchedule) {
		t.Fatalf("web and API equity schedules diverge:\nweb: %+v\napi: %+v", webResult.EquitySchedule, apiResult.EquitySchedule)
	}
}

func TestAPIWebParityRESICO(t *testing.T) {
	app := newTestApplication(t)
	fiscalYear := testFiscalYear()

	form := url.Values{
		"Regime[]":             {"resico"},
		"GrossMonthlySalary[]": {"45000"},
		"UnpaidVacationDays[]": {"5"},
	}

	payload := newHomePostPayload(form)
	webResults, err := app.buildHomeResults(payload, fiscalYear)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(webResults.Results), 1)

	apiRequest := apiPackageRequest{Salary: 45000, Regime: "resico", UnpaidVacationDays: 5}
	spec, fieldErrors := apiRequest.spec(fiscalYear, "", 1)
	if len(fieldErrors) > 0 {
		t.Fatalf("unexpected validation errors: %+v", fieldErrors)
	}
	apiResult, err := app.calculatePackage(spec, fiscalYear)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(*webResults.Results[0].SalaryCalculation, *apiResult.SalaryCalculation) {
		t.Fatalf("web and API RESICO calculations diverge")
	}
}

func TestAPICalculateValidation(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantWord string
	}{
		{"missing regime", `{"salary":50000}`, "regime"},
		{"unknown regime", `{"salary":50000,"regime":"banana"}`, "regime"},
		{"zero salary", `{"salary":0,"regime":"resico"}`, "salary"},
		{"negative salary", `{"salary":-1,"regime":"resico"}`, "salary"},
		{"bad currency", `{"salary":100,"regime":"resico","currency":"EUR"}`, "currency"},
		{"bad frequency", `{"salary":100,"regime":"resico","payment_frequency":"yearly"}`, "payment_frequency"},
		{"vales without amount", `{"salary":100,"regime":"sueldos_salarios","has_vales_despensa":true}`, "vales_despensa_amount"},
		{"equity without grant", `{"salary":100,"regime":"sueldos_salarios","has_equity":true}`, "initial_equity_usd"},
		{"benefit without name", `{"salary":100,"regime":"resico","other_benefits":[{"amount":10}]}`, "other_benefits[0].name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := apiTestApplication(t)
			req := jsonAPIRequest(t, tt.body)

			res := send(t, req, app.routes())

			assert.Equal(t, res.StatusCode, http.StatusBadRequest)
			assert.True(t, strings.Contains(res.Body, "Validation failed"))
			assert.True(t, strings.Contains(res.Body, tt.wantWord))
		})
	}
}

func TestAPICalculateAcceptsSueldosAlias(t *testing.T) {
	app := apiTestApplication(t)
	req := jsonAPIRequest(t, `{"salary":50000,"regime":"sueldos"}`)

	res := send(t, req, app.routes())

	assert.Equal(t, res.StatusCode, http.StatusOK)
	assert.True(t, strings.Contains(res.Body, `"regime": "sueldos_salarios"`))
}

func TestAPICalculateRESICOOutOfBrackets(t *testing.T) {
	app := apiTestApplication(t)
	app.db.(*fakeStore).resicoFound = false
	req := jsonAPIRequest(t, `{"salary":5000000,"regime":"resico"}`)

	res := send(t, req, app.routes())

	assert.Equal(t, res.StatusCode, http.StatusBadRequest)
	assert.True(t, strings.Contains(res.Body, "RESICO"))
}

func TestAPICompare(t *testing.T) {
	t.Run("returns per-package results and the best package", func(t *testing.T) {
		app := apiTestApplication(t)
		body := `{"packages":[
			{"name":"Oferta A","salary":50000,"regime":"sueldos_salarios"},
			{"name":"Oferta B","salary":70000,"regime":"sueldos_salarios"}
		]}`
		req := jsonAPIRequestPath(t, "/api/v1/compare", body)

		res := send(t, req, app.routes())

		assert.Equal(t, res.StatusCode, http.StatusOK)
		assert.True(t, strings.Contains(res.Body, `"success": true`))
		assert.True(t, strings.Contains(res.Body, `"name": "Oferta B"`))
		assert.True(t, strings.Contains(res.Body, `"index": 1`))
	})

	t.Run("rejects fewer than two packages", func(t *testing.T) {
		app := apiTestApplication(t)
		req := jsonAPIRequestPath(t, "/api/v1/compare", `{"packages":[{"salary":50000,"regime":"resico"}]}`)

		res := send(t, req, app.routes())

		assert.Equal(t, res.StatusCode, http.StatusBadRequest)
		assert.True(t, strings.Contains(res.Body, "packages"))
	})

	t.Run("rejects more than ten packages", func(t *testing.T) {
		app := apiTestApplication(t)
		packages := make([]string, 11)
		for i := range packages {
			packages[i] = `{"salary":50000,"regime":"resico"}`
		}
		body := fmt.Sprintf(`{"packages":[%s]}`, strings.Join(packages, ","))
		req := jsonAPIRequestPath(t, "/api/v1/compare", body)

		res := send(t, req, app.routes())

		assert.Equal(t, res.StatusCode, http.StatusBadRequest)
	})

	t.Run("labels validation errors with the package index", func(t *testing.T) {
		app := apiTestApplication(t)
		body := `{"packages":[
			{"salary":50000,"regime":"sueldos_salarios"},
			{"salary":50000,"regime":"banana"}
		]}`
		req := jsonAPIRequestPath(t, "/api/v1/compare", body)

		res := send(t, req, app.routes())

		assert.Equal(t, res.StatusCode, http.StatusBadRequest)
		assert.True(t, strings.Contains(res.Body, "packages[1].regime"))
	})
}

func TestAPIQuotas(t *testing.T) {
	t.Run("unverified users are limited to 10 calls per day", func(t *testing.T) {
		app := newTestApplication(t)
		err := app.db.UpdateUserAPIKey(testUsers["alice"].id, hashAPIKey("api-key"), apiKeyPrefix("api-key"))
		if err != nil {
			t.Fatal(err)
		}
		app.db.(*fakeStore).apiCallCounts[testUsers["alice"].id] = apiDailyLimitUnverified

		res := send(t, jsonAPIRequest(t, `{"salary":100,"regime":"resico"}`), app.routes())

		assert.Equal(t, res.StatusCode, http.StatusTooManyRequests)
		assert.True(t, strings.Contains(res.Body, "unverified_user"))
	})

	t.Run("verified users are limited to 100 calls per month", func(t *testing.T) {
		app := apiTestApplication(t)
		app.db.(*fakeStore).apiCallCounts[testUsers["alice"].id] = apiMonthlyLimitVerified

		res := send(t, jsonAPIRequest(t, `{"salary":100,"regime":"resico"}`), app.routes())

		assert.Equal(t, res.StatusCode, http.StatusTooManyRequests)
		assert.True(t, strings.Contains(res.Body, "verified_user"))
	})

	t.Run("verified users below the monthly limit pass", func(t *testing.T) {
		app := apiTestApplication(t)
		app.db.(*fakeStore).apiCallCounts[testUsers["alice"].id] = apiMonthlyLimitVerified - 1

		res := send(t, jsonAPIRequest(t, `{"salary":100,"regime":"resico"}`), app.routes())

		assert.Equal(t, res.StatusCode, http.StatusOK)
	})
}

func TestAPIKeyHashing(t *testing.T) {
	app := newTestApplication(t)

	plaintext, err := app.generateSecureAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, strings.HasPrefix(plaintext, "tc_"))
	assert.Equal(t, len(plaintext), 46)

	err = app.db.UpdateUserAPIKey(testUsers["alice"].id, hashAPIKey(plaintext), apiKeyPrefix(plaintext))
	if err != nil {
		t.Fatal(err)
	}

	stored := mustUser(t, app, testUsers["alice"].id)
	assert.True(t, stored.ApiKey.String != plaintext)
	assert.Equal(t, stored.ApiKeyPrefix.String, plaintext[:8])

	// The plaintext key still authenticates because the middleware hashes it.
	user, ok := app.authenticatedAPIUser(nilResponseWriter{}, authRequest("Bearer "+plaintext))
	assert.True(t, ok)
	assert.Equal(t, user.ID, testUsers["alice"].id)
}

func TestAPICORS(t *testing.T) {
	t.Run("preflight needs no API key and gets CORS headers", func(t *testing.T) {
		app := newTestApplication(t)
		req, err := http.NewRequest(http.MethodOptions, "/api/v1/calculate", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Origin", "https://example.com")
		req.Header.Set("Access-Control-Request-Method", "POST")

		res := send(t, req, app.routes())

		assert.Equal(t, res.StatusCode, http.StatusNoContent)
		assert.Equal(t, res.Header.Get("Access-Control-Allow-Origin"), "*")
		assert.True(t, strings.Contains(res.Header.Get("Access-Control-Allow-Headers"), "Authorization"))
	})

	t.Run("API responses carry the allow-origin header", func(t *testing.T) {
		app := apiTestApplication(t)
		req := jsonAPIRequest(t, `{"salary":100,"regime":"resico"}`)

		res := send(t, req, app.routes())

		assert.Equal(t, res.StatusCode, http.StatusOK)
		assert.Equal(t, res.Header.Get("Access-Control-Allow-Origin"), "*")
	})
}

func TestAPIOpenAPISpecRoute(t *testing.T) {
	app := newTestApplication(t)
	req := newTestRequest(t, http.MethodGet, "/api/v1/openapi.json")

	res := send(t, req, app.routes())

	assert.Equal(t, res.StatusCode, http.StatusOK)
	assert.True(t, strings.Contains(res.Body, "openapi"))
	assert.True(t, strings.Contains(res.Body, "/api/v1/calculate"))
	assert.True(t, strings.Contains(res.Body, "/api/v1/compare"))
}

// TestAPIHonorsExplicitZeroBenefitValues pins the omitted-vs-zero contract:
// omitted fields get the legal defaults, an explicit 0 is honored (parity with
// typing 0 in the web form).
func TestAPIHonorsExplicitZeroBenefitValues(t *testing.T) {
	app := apiTestApplication(t)

	req := jsonAPIRequest(t, `{"salary":30000,"regime":"sueldos_salarios","has_aguinaldo":true,"aguinaldo_days":0}`)
	res := send(t, req, app.routes())
	assert.Equal(t, res.StatusCode, http.StatusOK)
	assert.True(t, strings.Contains(res.Body, `"aguinaldo_gross": 0`))

	// Wait for the background usage-count goroutine before reusing the store.
	app.wg.Wait()

	req = jsonAPIRequest(t, `{"salary":30000,"regime":"sueldos_salarios","has_aguinaldo":true}`)
	res = send(t, req, app.routes())
	assert.Equal(t, res.StatusCode, http.StatusOK)
	assert.True(t, !strings.Contains(res.Body, `"aguinaldo_gross": 0`))
}

func TestAPIRejectsUnpaidVacationDaysOutsideRESICO(t *testing.T) {
	app := apiTestApplication(t)
	req := jsonAPIRequest(t, `{"salary":30000,"regime":"sueldos_salarios","unpaid_vacation_days":5}`)

	res := send(t, req, app.routes())

	assert.Equal(t, res.StatusCode, http.StatusBadRequest)
	assert.True(t, strings.Contains(res.Body, "only applies to the resico regime"))
}

func TestAPIRejectsUnknownFields(t *testing.T) {
	app := apiTestApplication(t)
	req := jsonAPIRequest(t, `{"salary":30000,"regime":"resico","salaray_typo":1}`)

	res := send(t, req, app.routes())

	assert.Equal(t, res.StatusCode, http.StatusBadRequest)
	assert.True(t, strings.Contains(res.Body, "unknown key"))
}

func TestAPIWrongMethodStillGetsCORSAndJSON(t *testing.T) {
	app := newTestApplication(t)
	req := newTestRequest(t, http.MethodGet, "/api/v1/calculate")

	res := send(t, req, app.routes())

	assert.Equal(t, res.StatusCode, http.StatusMethodNotAllowed)
	assert.Equal(t, res.Header.Get("Access-Control-Allow-Origin"), "*")
}

func TestAPIValidationEdgeCases(t *testing.T) {
	tooManyBenefits := `[` + strings.Repeat(`{"name":"b","amount":1},`, 20) + `{"name":"b","amount":1}]`

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantWord   string
	}{
		{"salary above cap", `{"salary":2000000000,"regime":"resico"}`, 400, "must not exceed"},
		{"negative exchange rate", `{"salary":100,"regime":"resico","exchange_rate":-1}`, 400, "must not be negative"},
		{"excessive exchange rate", `{"salary":100,"regime":"resico","exchange_rate":20000}`, 400, "must not exceed 10000"},
		{"hourly defaults hours to 40", `{"salary":100,"regime":"sueldos_salarios","payment_frequency":"hourly"}`, 200, `"payment_frequency": "hourly"`},
		{"hourly with explicit hours", `{"salary":100,"regime":"sueldos_salarios","payment_frequency":"hourly","hours_per_week":30}`, 200, `"success": true`},
		{"hourly with impossible hours", `{"salary":100,"regime":"sueldos_salarios","payment_frequency":"hourly","hours_per_week":200}`, 400, "hours_per_week"},
		{"unpaid days out of range", `{"salary":100,"regime":"resico","unpaid_vacation_days":400}`, 400, "between 0 and 365"},
		{"aguinaldo days out of range", `{"salary":100,"regime":"sueldos_salarios","has_aguinaldo":true,"aguinaldo_days":400}`, 400, "aguinaldo_days"},
		{"prima percent out of range", `{"salary":100,"regime":"sueldos_salarios","has_prima_vacacional":true,"prima_vacacional_percent":150}`, 400, "prima_vacacional_percent"},
		{"too many other benefits", `{"salary":100,"regime":"resico","other_benefits":` + tooManyBenefits + `}`, 400, "must not contain more than 20"},
		{"invalid benefit cadence", `{"salary":100,"regime":"resico","other_benefits":[{"name":"b","amount":1,"cadence":"weekly"}]}`, 400, "cadence"},
		{"percentage benefit above 100", `{"salary":100,"regime":"resico","other_benefits":[{"name":"b","amount":150,"is_percentage":true}]}`, 400, "between 0 and 100 when is_percentage"},
		{"benefit amount above cap", `{"salary":100,"regime":"resico","other_benefits":[{"name":"b","amount":2000000000}]}`, 400, "must not exceed"},
		{"refreshers without amounts", `{"salary":100,"regime":"sueldos_salarios","has_equity":true,"initial_equity_usd":1000,"has_refreshers":true}`, 400, "refresher_min_usd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := apiTestApplication(t)
			res := send(t, jsonAPIRequest(t, tt.body), app.routes())

			assert.Equal(t, res.StatusCode, tt.wantStatus)
			assert.True(t, strings.Contains(res.Body, tt.wantWord))
		})
	}
}

func TestAPICalculateSerializesBenefitsAndEquity(t *testing.T) {
	app := apiTestApplication(t)
	body := `{
		"salary": 50000, "regime": "sueldos_salarios",
		"other_benefits": [
			{"name": "Bono", "amount": 1000, "cadence": "monthly"},
			{"name": "Utilidades", "amount": 10, "is_percentage": true, "tax_free": true}
		],
		"has_equity": true, "initial_equity_usd": 20000,
		"has_refreshers": true, "refresher_min_usd": 5000, "refresher_max_usd": 10000
	}`

	res := send(t, jsonAPIRequest(t, body), app.routes())

	assert.Equal(t, res.StatusCode, http.StatusOK)
	assert.True(t, strings.Contains(res.Body, `"name": "Bono"`))
	assert.True(t, strings.Contains(res.Body, `"cadence": "annual"`))
	assert.True(t, strings.Contains(res.Body, `"initial_grant_usd": 20000`))
	assert.True(t, strings.Contains(res.Body, `"total_vested_mxn"`))
}

func TestAPICompareErrorBranches(t *testing.T) {
	t.Run("malformed JSON body", func(t *testing.T) {
		app := apiTestApplication(t)
		res := send(t, jsonAPIRequestPath(t, "/api/v1/compare", `{"packages":`), app.routes())

		assert.Equal(t, res.StatusCode, http.StatusBadRequest)
	})

	t.Run("missing fiscal year", func(t *testing.T) {
		app := apiTestApplication(t)
		app.db.(*fakeStore).activeFiscalFound = false
		body := `{"packages":[{"salary":100,"regime":"resico"},{"salary":200,"regime":"resico"}]}`

		res := send(t, jsonAPIRequestPath(t, "/api/v1/compare", body), app.routes())

		assert.Equal(t, res.StatusCode, http.StatusInternalServerError)
	})

	t.Run("calculation error mid-comparison", func(t *testing.T) {
		app := apiTestApplication(t)
		app.db.(*fakeStore).resicoFound = false
		body := `{"packages":[{"salary":100,"regime":"sueldos_salarios"},{"salary":200,"regime":"resico"}]}`

		res := send(t, jsonAPIRequestPath(t, "/api/v1/compare", body), app.routes())

		assert.Equal(t, res.StatusCode, http.StatusBadRequest)
		assert.True(t, strings.Contains(res.Body, "RESICO"))
	})
}

func TestAPICalculateServerErrorIsJSON(t *testing.T) {
	app := apiTestApplication(t)
	app.db.(*fakeStore).errors["GetISRBrackets"] = fmt.Errorf("brackets exploded")

	res := send(t, jsonAPIRequest(t, `{"salary":100,"regime":"sueldos_salarios"}`), app.routes())

	assert.Equal(t, res.StatusCode, http.StatusInternalServerError)
	assert.True(t, strings.Contains(res.Body, `"success": false`))
}

func TestAPINotFoundIsJSON(t *testing.T) {
	app := newTestApplication(t)
	res := send(t, newTestRequest(t, http.MethodGet, "/api/v1/nope"), app.routes())

	assert.Equal(t, res.StatusCode, http.StatusNotFound)
	assert.True(t, strings.Contains(res.Body, "could not be found"))
	assert.Equal(t, res.Header.Get("Access-Control-Allow-Origin"), "*")
}

func TestAPIOpenAPISpecErrorBranches(t *testing.T) {
	t.Run("missing embedded spec", func(t *testing.T) {
		app := newTestApplication(t)
		original := openAPISpecFile
		openAPISpecFile = "static/api/does-not-exist.json"
		defer func() { openAPISpecFile = original }()

		res := send(t, newTestRequest(t, http.MethodGet, "/api/v1/openapi.json"), app.routes())

		assert.Equal(t, res.StatusCode, http.StatusInternalServerError)
	})

	t.Run("write failure is logged", func(t *testing.T) {
		app := newTestApplication(t)
		app.apiOpenAPISpec(failingSpecWriter{}, newTestRequest(t, http.MethodGet, "/api/v1/openapi.json"))
	})
}

func TestAccountDeveloperNewKeyAndErrors(t *testing.T) {
	t.Run("shows the freshly generated key once", func(t *testing.T) {
		app := newTestApplication(t)
		session := newTestSession(t, app.sessionManager, map[string]any{
			"authenticatedUserID": testUsers["alice"].id,
			"newAPIKey":           "tc_freshly-generated-key",
		})
		req := newTestRequest(t, http.MethodGet, "/account/developer")
		req.AddCookie(session.cookie)

		res := send(t, req, app.routes())

		assert.Equal(t, res.StatusCode, http.StatusOK)
		assert.True(t, strings.Contains(res.Body, "tc_freshly-generated-key"))
	})

	t.Run("monthly count failure surfaces as server error", func(t *testing.T) {
		app := newTestApplication(t)
		app.db.(*fakeStore).errors["GetMonthlyAPICallCount"] = fmt.Errorf("count failed")
		session := newTestSession(t, app.sessionManager, map[string]any{
			"authenticatedUserID": testUsers["alice"].id,
		})
		req := newTestRequest(t, http.MethodGet, "/account/developer")
		req.AddCookie(session.cookie)

		res := send(t, req, app.routes())

		assert.Equal(t, res.StatusCode, http.StatusInternalServerError)
	})
}

func TestAPIMonthlyQuotaErrorBranches(t *testing.T) {
	t.Run("monthly count lookup failure", func(t *testing.T) {
		app := apiTestApplication(t)
		app.db.(*fakeStore).errors["GetMonthlyAPICallCount"] = fmt.Errorf("count failed")

		res := send(t, jsonAPIRequest(t, `{"salary":100,"regime":"resico"}`), app.routes())

		assert.Equal(t, res.StatusCode, http.StatusInternalServerError)
	})

	t.Run("monthly limit error write failure", func(t *testing.T) {
		app := newTestApplication(t)
		restore := stubResponseJSON(func(http.ResponseWriter, int, any) error {
			return fmt.Errorf("json failed")
		})
		defer restore()

		app.writeAPIMonthlyLimitError(nilResponseWriter{}, newTestRequest(t, http.MethodGet, "/api/v1/calculate"), 100)
	})
}

type failingSpecWriter struct{}

func (failingSpecWriter) Header() http.Header        { return http.Header{} }
func (failingSpecWriter) Write([]byte) (int, error)  { return 0, fmt.Errorf("write failed") }
func (failingSpecWriter) WriteHeader(statusCode int) {}

func intPtr(v int) *int           { return &v }
func floatPtr(v float64) *float64 { return &v }

func jsonAPIRequestPath(t *testing.T, path string, body string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer api-key")
	return req
}
