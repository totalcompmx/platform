package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/jcroyoaun/totalcompmx/internal/assert"
	"github.com/jcroyoaun/totalcompmx/internal/database"
	"github.com/jcroyoaun/totalcompmx/internal/pdf"
	"github.com/jcroyoaun/totalcompmx/internal/smtp"

	"github.com/alexedwards/scs/v2"
)

func TestErrorFallbackBranches(t *testing.T) {
	t.Run("notification email send error is logged", func(t *testing.T) {
		app := newTestApplication(t)
		app.config.notifications.email = "ops@example.com"
		restore := stubSendMail(func(*smtp.Mailer, string, any, ...string) error {
			return errors.New("mail failed")
		})
		defer restore()

		app.reportServerError(newTestRequest(t, http.MethodGet, "/test"), errors.New("handler failed"))
	})

	t.Run("server error falls back to plain HTTP error", func(t *testing.T) {
		app := newTestApplication(t)
		restore := stubResponsePage(func(http.ResponseWriter, int, any, string) error {
			return errors.New("render failed")
		})
		defer restore()

		res := httptest.NewRecorder()
		app.serverError(res, newTestRequest(t, http.MethodGet, "/test"), errors.New("handler failed"))

		assert.Equal(t, res.Code, http.StatusInternalServerError)
	})

	t.Run("page error handlers delegate to server error", func(t *testing.T) {
		app := newTestApplication(t)
		restore := stubResponsePage(func(http.ResponseWriter, int, any, string) error {
			return errors.New("render failed")
		})
		defer restore()

		app.notFound(httptest.NewRecorder(), newTestRequest(t, http.MethodGet, "/missing"))
		app.badRequest(httptest.NewRecorder(), newTestRequest(t, http.MethodGet, "/bad"), errors.New("bad"))
	})

	t.Run("basic auth render error delegates to server error", func(t *testing.T) {
		app := newTestApplication(t)
		restore := stubResponsePageWithHeaders(func(http.ResponseWriter, int, any, http.Header, string) error {
			return errors.New("render failed")
		})
		defer restore()

		app.basicAuthenticationRequired(httptest.NewRecorder(), newTestRequest(t, http.MethodGet, "/restricted"))
	})
}

func TestHandlerBranchCoverage(t *testing.T) {
	t.Run("session renewal failures", func(t *testing.T) {
		app := newTestApplication(t)
		restore := stubRenewSessionToken(func() error {
			return errors.New("renew failed")
		})
		defer restore()
		req := loadedRequest(t, app, http.MethodPost, "/clear")
		app.clearSession(httptest.NewRecorder(), req)
		app.logout(httptest.NewRecorder(), req)
		assert.False(t, app.loginUser(httptest.NewRecorder(), req, testUsers["alice"].id))
	})

	t.Run("active fiscal year failures", func(t *testing.T) {
		app := newTestApplication(t)
		app.db.(*fakeStore).errors["GetActiveFiscalYear"] = errors.New("fiscal failed")
		app.homeGet(httptest.NewRecorder(), newTestRequest(t, http.MethodGet, "/"))

		app = newTestApplication(t)
		app.db.(*fakeStore).activeFiscalFound = false
		app.homeGet(httptest.NewRecorder(), newTestRequest(t, http.MethodGet, "/"))
	})

	t.Run("home POST parse and calculation errors", func(t *testing.T) {
		app := newTestApplication(t)
		app.homePost(httptest.NewRecorder(), malformedFormRequest(t, http.MethodPost, "/"))

		app = newTestApplication(t)
		app.db.(*fakeStore).activeFiscalFound = false
		app.homePost(httptest.NewRecorder(), homePostRequest(t))

		app = newTestApplication(t)
		app.db.(*fakeStore).errors["GetISRBrackets"] = errors.New("brackets failed")
		app.homePost(httptest.NewRecorder(), loadedFormRequest(t, app, homePostRequest(t)))
	})

	t.Run("home result builder returns calculation errors", func(t *testing.T) {
		app := newTestApplication(t)
		payload := newHomePostPayload(homePostRequest(t).PostForm)
		app.db.(*fakeStore).errors["GetISRBrackets"] = errors.New("brackets failed")

		_, err := app.buildHomeResults(payload, testFiscalYear())

		assert.NotNil(t, err)

		app = newTestApplication(t)
		app.db.(*fakeStore).errors["GetISRBrackets"] = errors.New("brackets failed")
		res := sendWithCSRFToken(t, homePostRequest(t), app.routes())
		assert.Equal(t, res.StatusCode, http.StatusInternalServerError)
	})

	t.Run("home payload helpers", func(t *testing.T) {
		app := newTestApplication(t)
		empty := newHomePostPayload(map[string][]string{})
		assert.Equal(t, empty.numPackages(), 2)
		_, ok := empty.salary(0)
		assert.False(t, ok)
		assert.Equal(t, parseInt("", 7), 7)
		assertFloatValue(t, monthlySalary(10, "daily", 40), 300)
		assertFloatValue(t, monthlySalary(10, "weekly", 40), 43.3)
		assertFloatValue(t, monthlySalary(10, "biweekly", 40), 21.7)
		assertFloatValue(t, monthlySalary(10, "unknown", 40), 10)
		checked, intValue := checkedInt(0, nil, nil, 15)
		assert.False(t, checked)
		assert.Equal(t, intValue, 15)
		checked, floatValue := checkedFloat(0, nil, nil, 1.5)
		assert.False(t, checked)
		assertFloatValue(t, floatValue, 1.5)
		_, ok = empty.otherBenefit(0, 0, "")
		assert.False(t, ok)

		results, err := app.buildHomeResults(empty, testFiscalYear())
		assert.Nil(t, err)
		assert.Equal(t, len(results.Results), 0)
	})

	t.Run("signup failure branches", func(t *testing.T) {
		app := newTestApplication(t)
		form := signupForm{Email: "new@example.com", Password: "goodPass123!"}
		app.signupPost(httptest.NewRecorder(), malformedFormRequest(t, http.MethodPost, "/signup"), &form)

		app.db.(*fakeStore).errors["GetUserByEmail"] = errors.New("lookup failed")
		assert.False(t, app.prepareSignup(httptest.NewRecorder(), loadedRequest(t, app, http.MethodPost, "/signup"), &form))

		app = newTestApplication(t)
		app.db.(*fakeStore).errors["InsertUser"] = errors.New("insert failed")
		_, _, ok := app.finishSignup(httptest.NewRecorder(), loadedRequest(t, app, http.MethodPost, "/signup"), &form)
		assert.False(t, ok)

		app = newTestApplication(t)
		restoreRenew := stubRenewSessionToken(func() error {
			return errors.New("renew failed")
		})
		_, _, ok = app.finishSignup(httptest.NewRecorder(), loadedRequest(t, app, http.MethodPost, "/signup"), &form)
		restoreRenew()
		assert.False(t, ok)

		app = newTestApplication(t)
		app.db.(*fakeStore).errors["InsertEmailVerificationToken"] = errors.New("token failed")
		_, _, ok = app.finishSignup(httptest.NewRecorder(), loadedRequest(t, app, http.MethodPost, "/signup"), &form)
		assert.False(t, ok)

		_, err := app.createSignupUser(&signupForm{Email: "long@example.com", Password: strings.Repeat("x", 73)})
		assert.NotNil(t, err)

		app = newTestApplication(t)
		restoreRenew = stubRenewSessionToken(func() error {
			return errors.New("renew failed")
		})
		req := loadedRequest(t, app, http.MethodPost, "/signup")
		req.PostForm.Set("Email", "new@example.com")
		req.PostForm.Set("Password", "goodPass123!")
		app.signupPost(httptest.NewRecorder(), req, &signupForm{})
		restoreRenew()
	})

	t.Run("login failure branches", func(t *testing.T) {
		app := newTestApplication(t)
		form := loginForm{Email: testUsers["alice"].email, Password: testUsers["alice"].password}
		app.loginPost(httptest.NewRecorder(), malformedFormRequest(t, http.MethodPost, "/login"), &form)

		app.db.(*fakeStore).errors["GetUserByEmail"] = errors.New("lookup failed")
		_, ok := app.validLoginUser(httptest.NewRecorder(), loadedRequest(t, app, http.MethodPost, "/login"), &form)
		assert.False(t, ok)

		app = newTestApplication(t)
		store := app.db.(*fakeStore)
		user := store.users[testUsers["alice"].id]
		user.HashedPassword = "not-a-bcrypt-hash"
		store.users[user.ID] = user
		_, ok = app.validLoginUser(httptest.NewRecorder(), loadedRequest(t, app, http.MethodPost, "/login"), &form)
		assert.False(t, ok)

		req := loadedRequest(t, app, http.MethodGet, "/login")
		app.sessionManager.Put(req.Context(), "redirectPathAfterLogin", "/after")
		app.redirectAfterLogin(httptest.NewRecorder(), req)

		app = newTestApplication(t)
		restoreRenew := stubRenewSessionToken(func() error {
			return errors.New("renew failed")
		})
		req = loadedRequest(t, app, http.MethodPost, "/login")
		req.PostForm.Set("Email", testUsers["alice"].email)
		req.PostForm.Set("Password", testUsers["alice"].password)
		app.loginPost(httptest.NewRecorder(), req, &loginForm{})
		restoreRenew()
	})

	t.Run("forgotten password failure branches", func(t *testing.T) {
		app := newTestApplication(t)
		form := forgottenPasswordForm{Email: testUsers["alice"].email}
		app.forgottenPasswordPost(httptest.NewRecorder(), malformedFormRequest(t, http.MethodPost, "/forgotten-password"), &form)

		app.db.(*fakeStore).errors["GetUserByEmail"] = errors.New("lookup failed")
		_, ok := app.validForgottenPasswordUser(httptest.NewRecorder(), loadedRequest(t, app, http.MethodPost, "/forgotten-password"), &form)
		assert.False(t, ok)

		app = newTestApplication(t)
		app.db.(*fakeStore).errors["InsertPasswordReset"] = errors.New("reset failed")
		assert.False(t, app.sendPasswordResetEmail(httptest.NewRecorder(), loadedRequest(t, app, http.MethodPost, "/forgotten-password"), mustUser(t, app, testUsers["alice"].id)))

		app = newTestApplication(t)
		restore := stubSendMail(func(*smtp.Mailer, string, any, ...string) error {
			return errors.New("mail failed")
		})
		assert.False(t, app.sendPasswordResetEmail(httptest.NewRecorder(), loadedRequest(t, app, http.MethodPost, "/forgotten-password"), mustUser(t, app, testUsers["alice"].id)))
		restore()

		app = newTestApplication(t)
		app.db.(*fakeStore).errors["InsertPasswordReset"] = errors.New("reset failed")
		req := loadedRequest(t, app, http.MethodPost, "/forgotten-password")
		req.PostForm.Set("Email", testUsers["alice"].email)
		app.forgottenPasswordPost(httptest.NewRecorder(), req, &forgottenPasswordForm{})
	})

	t.Run("password reset failure branches", func(t *testing.T) {
		app := newTestApplication(t)
		app.db.(*fakeStore).errors["GetPasswordReset"] = errors.New("reset lookup failed")
		_, ok := app.validPasswordReset(httptest.NewRecorder(), loadedRequest(t, app, http.MethodGet, "/password-reset/token"), "token")
		assert.False(t, ok)

		app = newTestApplication(t)
		reset := database.PasswordReset{UserID: testUsers["alice"].id}
		app.passwordResetPost(httptest.NewRecorder(), malformedFormRequest(t, http.MethodPost, "/password-reset/token"), &passwordResetForm{}, reset)
		app.passwordResetPost(httptest.NewRecorder(), loadedRequest(t, app, http.MethodPost, "/password-reset/token"), &passwordResetForm{NewPassword: "short"}, reset)
		assert.False(t, app.updateResetPassword(httptest.NewRecorder(), loadedRequest(t, app, http.MethodPost, "/password-reset/token"), testUsers["alice"].id, strings.Repeat("x", 73)))

		app.db.(*fakeStore).errors["UpdateUserHashedPassword"] = errors.New("update failed")
		assert.False(t, app.updateResetPassword(httptest.NewRecorder(), loadedRequest(t, app, http.MethodPost, "/password-reset/token"), testUsers["alice"].id, "goodPass123!"))

		app = newTestApplication(t)
		app.db.(*fakeStore).errors["DeletePasswordResets"] = errors.New("delete failed")
		assert.False(t, app.updateResetPassword(httptest.NewRecorder(), loadedRequest(t, app, http.MethodPost, "/password-reset/token"), testUsers["alice"].id, "goodPass123!"))

		app = newTestApplication(t)
		app.db.(*fakeStore).errors["UpdateUserHashedPassword"] = errors.New("update failed")
		req := loadedRequest(t, app, http.MethodPost, "/password-reset/token")
		req.PostForm.Set("NewPassword", "goodPass123!")
		app.passwordResetPost(httptest.NewRecorder(), req, &passwordResetForm{}, database.PasswordReset{UserID: testUsers["alice"].id})
	})

	t.Run("render-only handler errors", func(t *testing.T) {
		app := newTestApplication(t)
		restore := stubResponsePage(func(http.ResponseWriter, int, any, string) error {
			return errors.New("render failed")
		})
		defer restore()

		req := loadedRequest(t, app, http.MethodGet, "/")
		app.forgottenPasswordConfirmation(httptest.NewRecorder(), req)
		app.passwordResetConfirmation(httptest.NewRecorder(), req)
		app.restricted(httptest.NewRecorder(), req)
		app.privacy(httptest.NewRecorder(), req)
		app.terms(httptest.NewRecorder(), req)
		app.developersPage(httptest.NewRecorder(), req)
	})

	t.Run("developer and verification errors", func(t *testing.T) {
		app := newTestApplication(t)
		app.accountDeveloper(httptest.NewRecorder(), loadedRequest(t, app, http.MethodGet, "/account/developer"))

		req := authenticatedRequest(t, app, http.MethodGet, "/account/developer")
		req = loadExistingSessionRequest(t, app, req)
		app.db.(*fakeStore).errors["GetUser"] = errors.New("get failed")
		app.accountDeveloper(httptest.NewRecorder(), contextSetAuthenticatedUser(loadedRequest(t, app, http.MethodGet, "/account/developer"), database.User{ID: testUsers["alice"].id}))

		app = newTestApplication(t)
		delete(app.db.(*fakeStore).users, testUsers["alice"].id)
		app.accountDeveloper(httptest.NewRecorder(), contextSetAuthenticatedUser(loadedRequest(t, app, http.MethodGet, "/account/developer"), database.User{ID: testUsers["alice"].id}))

		app = newTestApplication(t)
		app.generateAPIKey(httptest.NewRecorder(), loadedRequest(t, app, http.MethodPost, "/account/api-key"))

		req = requestWithAuthenticatedUser(t, app, http.MethodPost, "/account/api-key")
		restoreRandom := stubReadRandom(func([]byte) (int, error) { return 0, errors.New("rand failed") })
		app.generateAPIKey(httptest.NewRecorder(), req)
		restoreRandom()

		app.db.(*fakeStore).errors["UpdateUserAPIKey"] = errors.New("api key failed")
		app.generateAPIKey(httptest.NewRecorder(), req)

		app = newTestApplication(t)
		app.db.(*fakeStore).errors["GetUserIDFromVerificationToken"] = errors.New("verify lookup failed")
		app.verifyEmail(httptest.NewRecorder(), newTestRequest(t, http.MethodGet, "/verify-email/token"))

		app = newTestApplication(t)
		plaintextToken := insertVerificationToken(t, app, testUsers["alice"].id)
		app.db.(*fakeStore).errors["VerifyUserEmail"] = errors.New("verify failed")
		req = newTestRequest(t, http.MethodGet, "/verify-email/"+plaintextToken)
		req.SetPathValue("plaintextToken", plaintextToken)
		app.verifyEmail(httptest.NewRecorder(), req)
	})

	t.Run("resend verification errors", func(t *testing.T) {
		app := newTestApplication(t)
		req := authenticatedRequest(t, app, http.MethodPost, "/account/resend-verification")
		req = loadExistingSessionRequest(t, app, req)
		app.db.(*fakeStore).errors["GetUser"] = errors.New("get failed")
		app.resendVerificationEmail(httptest.NewRecorder(), req)

		app = newTestApplication(t)
		req = authenticatedRequest(t, app, http.MethodPost, "/account/resend-verification")
		req = loadExistingSessionRequest(t, app, req)
		delete(app.db.(*fakeStore).users, testUsers["alice"].id)
		app.resendVerificationEmail(httptest.NewRecorder(), req)

		app = newTestApplication(t)
		req = authenticatedRequest(t, app, http.MethodPost, "/account/resend-verification")
		req = loadExistingSessionRequest(t, app, req)
		app.db.(*fakeStore).errors["DeleteEmailVerificationTokensForUser"] = errors.New("delete failed")
		app.resendVerificationEmail(httptest.NewRecorder(), req)

		app = newTestApplication(t)
		req = authenticatedRequest(t, app, http.MethodPost, "/account/resend-verification")
		req = loadExistingSessionRequest(t, app, req)
		app.db.(*fakeStore).errors["InsertEmailVerificationToken"] = errors.New("insert failed")
		app.resendVerificationEmail(httptest.NewRecorder(), req)
	})

	t.Run("API calculation errors", func(t *testing.T) {
		app := apiTestApplication(t)
		res := httptest.NewRecorder()
		app.apiCalculate(res, jsonRequestBody(t, "{"))
		assert.Equal(t, res.Code, http.StatusBadRequest)

		app = apiTestApplication(t)
		app.db.(*fakeStore).errors["GetActiveFiscalYear"] = errors.New("fiscal failed")
		app.apiCalculate(httptest.NewRecorder(), jsonRequestBody(t, `{"salary":100}`))

		app = apiTestApplication(t)
		app.db.(*fakeStore).activeFiscalFound = false
		app.apiCalculate(httptest.NewRecorder(), jsonRequestBody(t, `{"salary":100}`))

		app = apiTestApplication(t)
		app.db.(*fakeStore).errors["GetISRBrackets"] = errors.New("brackets failed")
		app.apiCalculate(httptest.NewRecorder(), jsonRequestBody(t, `{"salary":100}`))

		restoreJSON := stubResponseJSON(func(http.ResponseWriter, int, any) error {
			return errors.New("json failed")
		})
		app = apiTestApplication(t)
		app.writeAPICalculateResponse(httptest.NewRecorder(), jsonRequestBody(t, `{"salary":100}`), apiCalculateRequest{Salary: 100}, database.SalaryCalculation{}, testFiscalYear())
		app.writeJSONError(httptest.NewRecorder(), jsonRequestBody(t, `{"salary":100}`), http.StatusBadRequest, "bad")
		restoreJSON()
	})

	t.Run("calculator failures", func(t *testing.T) {
		app := newTestApplication(t)
		app.salaryCalculatorPost(httptest.NewRecorder(), malformedFormRequest(t, http.MethodPost, "/calculator"), &calculatorForm{})

		app = newTestApplication(t)
		app.db.(*fakeStore).errors["GetActiveFiscalYear"] = errors.New("fiscal failed")
		app.renderCalculatorForm(httptest.NewRecorder(), loadedRequest(t, app, http.MethodGet, "/calculator"), http.StatusOK, &calculatorForm{})

		app = newTestApplication(t)
		app.db.(*fakeStore).activeFiscalFound = false
		app.salaryCalculatorPost(httptest.NewRecorder(), loadedCalculatorRequest(t, app, 50000, 1), &calculatorForm{})

		app = newTestApplication(t)
		app.db.(*fakeStore).errors["GetISRBrackets"] = errors.New("brackets failed")
		app.salaryCalculatorPost(httptest.NewRecorder(), loadedCalculatorRequest(t, app, 50000, 1), &calculatorForm{})
	})

	t.Run("PDF export errors", func(t *testing.T) {
		app := newTestApplication(t)
		session := newTestSession(t, app.sessionManager, map[string]any{"comparisonResults": "bad"})
		req := newTestRequest(t, http.MethodGet, "/export-pdf")
		req.AddCookie(session.cookie)
		res := send(t, req, app.routes())
		assert.Equal(t, res.StatusCode, http.StatusInternalServerError)

		app = newTestApplication(t)
		session = newTestSession(t, app.sessionManager, map[string]any{"comparisonResults": []PackageResult{}})
		req = newTestRequest(t, http.MethodGet, "/export-pdf")
		req.AddCookie(session.cookie)
		res = send(t, req, app.routes())
		assert.Equal(t, res.StatusCode, http.StatusBadRequest)

		app = newTestApplication(t)
		session = newTestSession(t, app.sessionManager, map[string]any{
			"comparisonResults": []PackageResult{{PackageName: "One", SalaryCalculation: &database.SalaryCalculation{}}},
			"packageInputs":     "bad",
		})
		req = newTestRequest(t, http.MethodGet, "/export-pdf")
		req.AddCookie(session.cookie)
		res = send(t, req, app.routes())
		assert.Equal(t, res.StatusCode, http.StatusInternalServerError)

		app = newTestApplication(t)
		session = newTestSession(t, app.sessionManager, map[string]any{
			"comparisonResults": []PackageResult{{PackageName: "One", SalaryCalculation: &database.SalaryCalculation{}}},
			"packageInputs":     []PackageInput{{Name: "One"}},
		})
		req = newTestRequest(t, http.MethodGet, "/export-pdf")
		req.AddCookie(session.cookie)
		restore := stubPDFGenerator(t, []byte("%PDF"), nil)
		res = send(t, req, app.routes())
		restore()
		assert.Equal(t, res.StatusCode, http.StatusOK)

		app = newTestApplication(t)
		app.db.(*fakeStore).errors["GetActiveFiscalYear"] = errors.New("fiscal failed")
		_, ok := app.pdfFiscalYear(httptest.NewRecorder(), loadedRequest(t, app, http.MethodGet, "/export-pdf"))
		assert.False(t, ok)

		app = newTestApplication(t)
		app.db.(*fakeStore).activeFiscalFound = false
		_, ok = app.pdfFiscalYear(httptest.NewRecorder(), loadedRequest(t, app, http.MethodGet, "/export-pdf"))
		assert.False(t, ok)

		app = newTestApplication(t)
		restore = stubPDFGenerator(t, nil, errors.New("pdf failed"))
		session = newTestSession(t, app.sessionManager, pdfSessionData())
		req = newTestRequest(t, http.MethodGet, "/export-pdf")
		req.AddCookie(session.cookie)
		res = send(t, req, app.routes())
		restore()
		assert.Equal(t, res.StatusCode, http.StatusInternalServerError)

		app = newTestApplication(t)
		app.db.(*fakeStore).activeFiscalFound = false
		session = newTestSession(t, app.sessionManager, map[string]any{
			"comparisonResults": []PackageResult{{PackageName: "One", SalaryCalculation: &database.SalaryCalculation{}}},
			"packageInputs":     []PackageInput{{Name: "One"}},
		})
		req = newTestRequest(t, http.MethodGet, "/export-pdf")
		req.AddCookie(session.cookie)
		_, ok = app.pdfExportData(httptest.NewRecorder(), loadExistingSessionRequest(t, app, req))
		assert.False(t, ok)

		assert.Equal(t, pdfPackageInput(nil, 1), pdf.PackageInput{})
		app.writePDFResponse(errorResponseWriter{header: http.Header{}}, []byte("%PDF"), database.FiscalYear{})
		app.writeStaticContent(errorResponseWriter{header: http.Header{}}, "text/plain", "body")
	})
}

func TestMiddlewareBranchCoverage(t *testing.T) {
	t.Run("authenticate handles DB error", func(t *testing.T) {
		app := newTestApplication(t)
		session := newTestSession(t, app.sessionManager, map[string]any{"authenticatedUserID": testUsers["alice"].id})
		req := newTestRequest(t, http.MethodGet, "/test")
		req.AddCookie(session.cookie)
		app.db.(*fakeStore).errors["GetUser"] = errors.New("get failed")

		res := send(t, req, app.sessionManager.LoadAndSave(app.authenticate(okHandler())))

		assert.Equal(t, res.StatusCode, http.StatusInternalServerError)
	})

	t.Run("basic auth hash error", func(t *testing.T) {
		app := newTestApplication(t)
		app.config.basicAuth.username = "admin"
		app.config.basicAuth.hashedPassword = "bad-hash"
		req := newTestRequest(t, http.MethodGet, "/test")
		req.SetBasicAuth("admin", "password")

		res := send(t, req, app.requireBasicAuthentication(okHandler()))

		assert.Equal(t, res.StatusCode, http.StatusInternalServerError)
	})

	t.Run("API auth branches", func(t *testing.T) {
		app := newTestApplication(t)
		key, message := bearerAPIKey(authRequest(""))
		assert.Equal(t, key, "")
		assert.Equal(t, message, "Missing Authorization header. Use: Authorization: Bearer YOUR_API_KEY")
		key, message = bearerAPIKey(authRequest("Token abc"))
		assert.Equal(t, key, "")
		assert.Equal(t, message, "Invalid Authorization format. Use: Authorization: Bearer YOUR_API_KEY")
		key, message = bearerAPIKey(authRequest("Bearer "))
		assert.Equal(t, key, "")
		assert.Equal(t, message, "API key is empty")

		app.db.(*fakeStore).errors["GetUserByAPIKey"] = errors.New("api lookup failed")
		_, ok := app.authenticatedAPIUser(httptest.NewRecorder(), authRequest("Bearer api-key"))
		assert.False(t, ok)

		app = newTestApplication(t)
		_, ok = app.authenticatedAPIUser(httptest.NewRecorder(), authRequest("Bearer missing"))
		assert.False(t, ok)

		app = newTestApplication(t)
		app.db.UpdateUserAPIKey(testUsers["alice"].id, "api-key")
		app.db.(*fakeStore).errors["GetDailyAPICallCount"] = errors.New("count failed")
		_, ok = app.authenticatedAPIUser(httptest.NewRecorder(), authRequest("Bearer api-key"))
		assert.False(t, ok)

		app = newTestApplication(t)
		app.db.UpdateUserAPIKey(testUsers["alice"].id, "api-key")
		app.db.(*fakeStore).apiCallCounts[testUsers["alice"].id] = 10
		_, ok = app.authenticatedAPIUser(httptest.NewRecorder(), authRequest("Bearer api-key"))
		assert.False(t, ok)

		app = newTestApplication(t)
		user := mustUser(t, app, testUsers["alice"].id)
		user.EmailVerified = false
		assert.True(t, app.allowAPIRequest(httptest.NewRecorder(), authRequest("Bearer api-key"), user))
	})

	t.Run("API JSON error branches", func(t *testing.T) {
		app := newTestApplication(t)
		restoreJSON := stubResponseJSON(func(http.ResponseWriter, int, any) error {
			return errors.New("json failed")
		})
		defer restoreJSON()

		req := newTestRequest(t, http.MethodGet, "/api")
		app.writeAPIRateLimitError(httptest.NewRecorder(), req, 10)
		app.writeJSONError(httptest.NewRecorder(), req, http.StatusUnauthorized, "nope")
	})

	t.Run("API usage logging errors", func(t *testing.T) {
		app := newTestApplication(t)
		app.db.(*fakeStore).errors["LogAPICall"] = errors.New("log failed")
		app.db.(*fakeStore).errors["IncrementAPICallsCount"] = errors.New("increment failed")

		app.recordAPIUsage(newTestRequest(t, http.MethodGet, "/api"), testUsers["alice"].id)
		app.wg.Wait()
	})

}

func TestPayrollBranchCoverage(t *testing.T) {
	t.Run("RESICO branches", func(t *testing.T) {
		app := newTestApplication(t)
		fiscalYear := testFiscalYear()
		app.db.(*fakeStore).errors["GetRESICOBracket"] = errors.New("resico failed")
		_, err := app.calculateRESICO(1000, 0, nil, 20, fiscalYear)
		assert.NotNil(t, err)

		app = newTestApplication(t)
		app.db.(*fakeStore).resicoFound = false
		_, err = app.calculateRESICO(1000, 0, nil, 20, fiscalYear)
		assert.NotNil(t, err)

		app = newTestApplication(t)
		benefits := []OtherBenefit{
			{Name: "Taxable", Amount: 10, Currency: "USD", Cadence: "monthly"},
			{Name: "Tax free", Amount: 5, TaxFree: true, IsPercentage: true, Cadence: "annual"},
		}
		result, err := app.calculateRESICO(1000, 2, benefits, 20, fiscalYear)
		assert.Nil(t, err)
		assert.Equal(t, len(result.OtherBenefits), 2)

		var totals benefitTotals
		totals.add("monthly", 1)
		totals.add("annual", 2)
		assertFloatValue(t, totals.MonthlyNet, 1)
		assertFloatValue(t, totals.AnnualNet, 2)
	})

	t.Run("salary benefit branches", func(t *testing.T) {
		app := newTestApplication(t)
		fiscalYear := testFiscalYear()
		input := salaryBenefitsInput{
			GrossMonthlySalary: 50000,
			HasAguinaldo:       true,
			AguinaldoDays:      15,
			HasPrimaVacacional: true,
			VacationDays:       12,
			PrimaVacacionalPct: 25,
			HasFondoAhorro:     true,
			FondoAhorroPct:     13,
			OtherBenefits: []OtherBenefit{
				{Name: "Monthly", Amount: 100, Cadence: "monthly"},
				{Name: "Annual", Amount: 1000, Cadence: "annual"},
				{Name: "Free", Amount: 100, TaxFree: true, Cadence: "monthly"},
			},
			ExchangeRate: 20,
		}
		result := database.SalaryCalculation{GrossSalary: 50000, NetSalary: 40000, SBC: 1000}
		assert.Nil(t, app.applySalaryBenefits(&result, input, fiscalYear))

		app.db.(*fakeStore).errors["GetISRBrackets"] = errors.New("isr failed")
		_, err := app.salaryOtherBenefitISR("monthly", 100, 50000, fiscalYear.ID)
		assert.NotNil(t, err)

		app = newTestApplication(t)
		app.db.(*fakeStore).errors["GetISRBrackets"] = errors.New("isr failed")
		_, err = app.salaryOtherBenefit(OtherBenefit{Name: "Taxable", Amount: 100, Cadence: "monthly"}, 600000, 20, 50000, fiscalYear)
		assert.NotNil(t, err)

		app = newTestApplication(t)
		app.db.(*fakeStore).errors["GetISRBrackets"] = errors.New("isr failed")
		app.db.(*fakeStore).errorOnCall["GetISRBrackets"] = 2
		_, err = app.calculateSalaryWithBenefits(50000, false, 0, false, 0, false, 0, 0, false, 0, false, []OtherBenefit{{Name: "Taxable", Amount: 100, Cadence: "monthly"}}, 20, fiscalYear)
		assert.NotNil(t, err)

		app = newTestApplication(t)
		app.db.(*fakeStore).errors["GetIMSSConcepts"] = errors.New("imss failed")
		result = database.SalaryCalculation{GrossSalary: 50000, NetSalary: 40000, SBC: 1000}
		err = app.applySalaryBenefits(&result, salaryBenefitsInput{GrossMonthlySalary: 50000}, fiscalYear)
		assert.NotNil(t, err)

		app = newTestApplication(t)
		isr, err := app.salaryOtherBenefitISR("annual", 1000, 50000, fiscalYear.ID)
		assertFloatValue(t, mustISR(t, isr, err), calculateTaxArt174(50000, 1000, testISRBrackets()))
		isr, err = app.exemptAnnualISR(50000, 100, 1000, fiscalYear.ID)
		assertFloatValue(t, mustISR(t, isr, err), 0)
		app.db.(*fakeStore).errors["GetISRBrackets"] = errors.New("isr failed")
		_, err = app.exemptAnnualISR(50000, 2000, 1000, fiscalYear.ID)
		assert.NotNil(t, err)

		app = newTestApplication(t)
		app.db.(*fakeStore).errors["GetISRBrackets"] = errors.New("isr failed")
		err = app.applyAnnualBenefits(&database.SalaryCalculation{}, salaryBenefitsInput{GrossMonthlySalary: 50000, HasAguinaldo: true, AguinaldoDays: 100}, fiscalYear)
		assert.NotNil(t, err)

		app = newTestApplication(t)
		app.db.(*fakeStore).errors["GetISRBrackets"] = errors.New("isr failed")
		err = app.applyAnnualBenefits(&database.SalaryCalculation{}, salaryBenefitsInput{GrossMonthlySalary: 50000, HasPrimaVacacional: true, VacationDays: 365, PrimaVacacionalPct: 100}, fiscalYear)
		assert.NotNil(t, err)

		app = newTestApplication(t)
		app.db.(*fakeStore).errors["GetISRBrackets"] = errors.New("isr failed")
		err = app.applySalaryBenefits(&database.SalaryCalculation{GrossSalary: 50000, NetSalary: 40000, SBC: 1000}, salaryBenefitsInput{GrossMonthlySalary: 50000, HasAguinaldo: true, AguinaldoDays: 100}, fiscalYear)
		assert.NotNil(t, err)

		app = newTestApplication(t)
		result = database.SalaryCalculation{FondoAhorroEmployee: 100}
		err = app.applyAnnualBenefits(&result, salaryBenefitsInput{HasFondoAhorro: true}, fiscalYear)
		assert.Nil(t, err)
		assertFloatValue(t, result.FondoAhorroYearly, 2400)
	})

	t.Run("base salary and IMSS error branches", func(t *testing.T) {
		app := newTestApplication(t)
		fiscalYear := testFiscalYear()
		_, err := app.calculateSalary(5000, 0, fiscalYear)
		assert.Nil(t, err)

		app.db.(*fakeStore).errors["GetISRBrackets"] = errors.New("isr failed")
		_, err = app.calculateSalary(5000, 0, fiscalYear)
		assert.NotNil(t, err)

		app = newTestApplication(t)
		app.db.(*fakeStore).errors["GetIMSSConcepts"] = errors.New("imss failed")
		_, err = app.calculateSalary(50000, 0, fiscalYear)
		assert.NotNil(t, err)
		_, err = app.calculateIMSSWorker(50000, fiscalYear)
		assert.NotNil(t, err)
		_, err = app.calculateIMSSEmployer(50000, fiscalYear)
		assert.NotNil(t, err)

		app = newTestApplication(t)
		_, err = app.sumIMSSContributions(50000, fiscalYear, testIMSSConcepts(), func(database.IMSSConcept, float64, float64, database.FiscalYear) (float64, error) {
			return 0, errors.New("contribution failed")
		})
		assert.NotNil(t, err)

		app.db.(*fakeStore).errors["GetCesantiaBracket"] = errors.New("cesantia failed")
		_, err = app.workerIMSSContribution(testIMSSConcepts()[1], 1000, 10000, fiscalYear)
		assert.NotNil(t, err)
		_, err = app.employerIMSSContribution(testIMSSConcepts()[1], 1000, 10000, fiscalYear)
		assert.NotNil(t, err)

		app = newTestApplication(t)
		app.db.(*fakeStore).cesantiaFound = false
		value, err := app.employerIMSSContribution(testIMSSConcepts()[1], 1000, 10000, fiscalYear)
		assert.Nil(t, err)
		assertFloatValue(t, value, 315)
		assertFloatValue(t, calculateSBC(1000000, 0, fiscalYear), 25*fiscalYear.UMADaily)
	})
}

func malformedFormRequest(t *testing.T, method string, path string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader("%"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func loadedFormRequest(t *testing.T, app *application, req *http.Request) *http.Request {
	t.Helper()
	ctx, err := app.sessionManager.Load(req.Context(), "")
	if err != nil {
		t.Fatal(err)
	}
	return req.WithContext(ctx)
}

func loadedRequest(t *testing.T, app *application, method string, path string) *http.Request {
	t.Helper()
	return loadedFormRequest(t, app, newTestRequest(t, method, path))
}

func loadedCalculatorRequest(t *testing.T, app *application, salary float64, years int) *http.Request {
	t.Helper()
	req := loadedRequest(t, app, http.MethodPost, "/calculator")
	req.PostForm.Set("GrossMonthlySalary", fmt.Sprintf("%.2f", salary))
	req.PostForm.Set("YearsOfService", strconv.Itoa(years))
	return req
}

func requestWithAuthenticatedUser(t *testing.T, app *application, method string, path string) *http.Request {
	t.Helper()
	req := loadedRequest(t, app, method, path)
	return contextSetAuthenticatedUser(req, mustUser(t, app, testUsers["alice"].id))
}

func loadExistingSessionRequest(t *testing.T, app *application, req *http.Request) *http.Request {
	t.Helper()
	cookie := req.Cookies()[0]
	ctx, err := app.sessionManager.Load(req.Context(), cookie.Value)
	if err != nil {
		t.Fatal(err)
	}
	return req.WithContext(ctx)
}

func jsonRequestBody(t *testing.T, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/calculate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func authRequest(header string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api", nil)
	if header != "" {
		req.Header.Set("Authorization", header)
	}
	return req
}

func mustISR(t *testing.T, value float64, err error) float64 {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func stubResponsePage(fn func(http.ResponseWriter, int, any, string) error) func() {
	original := responsePage
	responsePage = fn
	return func() {
		responsePage = original
	}
}

func stubResponsePageWithHeaders(fn func(http.ResponseWriter, int, any, http.Header, string) error) func() {
	original := responsePageWithHeaders
	responsePageWithHeaders = fn
	return func() {
		responsePageWithHeaders = original
	}
}

func stubResponseJSON(fn func(http.ResponseWriter, int, any) error) func() {
	original := responseJSON
	responseJSON = fn
	return func() {
		responseJSON = original
	}
}

func stubSendMail(fn func(*smtp.Mailer, string, any, ...string) error) func() {
	original := sendMail
	sendMail = fn
	return func() {
		sendMail = original
	}
}

func stubReadRandom(fn func([]byte) (int, error)) func() {
	original := readRandom
	readRandom = fn
	return func() {
		readRandom = original
	}
}

func stubRenewSessionToken(fn func() error) func() {
	original := renewSessionToken
	renewSessionToken = func(*scs.SessionManager, context.Context) error {
		return fn()
	}
	return func() {
		renewSessionToken = original
	}
}

type errorResponseWriter struct {
	header http.Header
}

func (w errorResponseWriter) Header() http.Header {
	return w.header
}

func (w errorResponseWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func (w errorResponseWriter) WriteHeader(int) {}
