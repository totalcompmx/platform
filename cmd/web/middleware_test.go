package main

import (
	"bytes"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/jcroyoaun/totalcompmx/internal/assert"
	"github.com/jcroyoaun/totalcompmx/internal/database"
)

func teapotHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestSecurityHeaders(t *testing.T) {
	t.Run("Sets appropriate security headers", func(t *testing.T) {
		app := newTestApplication(t)

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTeapot)
		})

		req := newTestRequest(t, http.MethodGet, "/test")

		res := send(t, req, app.securityHeaders(next))
		assert.Equal(t, res.StatusCode, http.StatusTeapot)
		assert.Equal(t, res.Header.Get("Referrer-Policy"), "origin-when-cross-origin")
		assert.Equal(t, res.Header.Get("X-Content-Type-Options"), "nosniff")
		assert.Equal(t, res.Header.Get("X-Frame-Options"), "deny")
	})
}

func TestRecoverPanic(t *testing.T) {
	t.Run("Allows normal requests to proceed", func(t *testing.T) {
		app := newTestApplication(t)
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTeapot)
		})

		req := newTestRequest(t, http.MethodGet, "/test")

		res := send(t, req, app.recoverPanic(next))
		assert.Equal(t, res.StatusCode, http.StatusTeapot)
	})

	t.Run("Recovers from panic and renders the 500 error page", func(t *testing.T) {
		var buf bytes.Buffer
		app := newTestApplication(t)
		app.logger = slog.New(slog.NewTextHandler(&buf, nil))

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("something went wrong")
		})

		req := newTestRequest(t, http.MethodGet, "/test")

		res := send(t, req, app.recoverPanic(next))
		assert.Equal(t, res.StatusCode, http.StatusInternalServerError)
		assert.True(t, containsPageTag(t, res.Body, "errors/500"))
		assert.True(t, strings.Contains(buf.String(), "level=ERROR"))
		assert.True(t, strings.Contains(buf.String(), `msg="something went wrong"`))
	})
}

func TestLogAccess(t *testing.T) {
	t.Run("Logs the request and response details", func(t *testing.T) {
		var buf bytes.Buffer
		app := newTestApplication(t)
		app.logger = slog.New(slog.NewTextHandler(&buf, nil))

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTeapot)
			w.Write([]byte("I'm a test teapot"))
		})

		req := newTestRequest(t, http.MethodGet, "/test")

		res := send(t, req, app.logAccess(next))
		assert.Equal(t, res.StatusCode, http.StatusTeapot)
		assert.True(t, strings.Contains(buf.String(), "level=INFO"))
		assert.True(t, strings.Contains(buf.String(), "msg=access"))
		assert.True(t, strings.Contains(buf.String(), "request.method=GET"))
		assert.True(t, strings.Contains(buf.String(), "request.url=/test"))
		assert.True(t, strings.Contains(buf.String(), "response.status=418"))
		assert.True(t, strings.Contains(buf.String(), "response.size=17"))
	})
}

func TestPreventCSRF(t *testing.T) {
	t.Run("POST accepts a valid CSRF token", testPreventCSRFValidToken)
	t.Run("POST rejects invalid CSRF tokens or cookies and displays error page", testPreventCSRFInvalidTokens)
}

func testPreventCSRFValidToken(t *testing.T) {
	app := newTestApplication(t)
	validCSRFToken, validCSRFCookie := getValidCSRFData(t)

	next := teapotHandler()
	req := newTestRequest(t, http.MethodPost, "/test")
	req.AddCookie(validCSRFCookie)
	req.PostForm.Add("csrf_token", validCSRFToken)

	res := send(t, req, app.preventCSRF(next))
	assert.Equal(t, res.StatusCode, http.StatusTeapot)
}

func testPreventCSRFInvalidTokens(t *testing.T) {
	for _, tt := range invalidCSRFCases(t) {
		t.Run(tt.name, func(t *testing.T) {
			assertInvalidCSRFRequest(t, tt)
		})
	}
}

type csrfCase struct {
	name       string
	csrfToken  string
	csrfCookie *http.Cookie
}

func invalidCSRFCases(t *testing.T) []csrfCase {
	t.Helper()
	validCSRFToken, validCSRFCookie := getValidCSRFData(t)
	return []csrfCase{
		{name: "Invalid token", csrfToken: "Inval1dT0k3N", csrfCookie: validCSRFCookie},
		{name: "Missing token", csrfToken: "", csrfCookie: validCSRFCookie},
		{name: "Invalid cookie", csrfToken: validCSRFToken, csrfCookie: &http.Cookie{}},
		{name: "Missing cookie", csrfToken: validCSRFToken, csrfCookie: nil},
		{name: "Missing token and cookie", csrfToken: "", csrfCookie: nil},
	}
}

func assertInvalidCSRFRequest(t *testing.T, tt csrfCase) {
	app := newTestApplication(t)
	req := invalidCSRFRequest(t, tt)
	res := send(t, req, app.sessionManager.LoadAndSave(app.preventCSRF(okHandler())))

	assert.Equal(t, res.StatusCode, http.StatusBadRequest)
	assert.True(t, containsPageTag(t, res.Body, "errors/400"))
	assert.True(t, strings.Contains(res.Body, "CSRF token validation failed"))
}

func invalidCSRFRequest(t *testing.T, tt csrfCase) *http.Request {
	req := newTestRequest(t, http.MethodPost, "/test")
	if tt.csrfToken != "" {
		req.PostForm.Add("c", tt.csrfToken)
	}
	if tt.csrfCookie != nil {
		req.AddCookie(tt.csrfCookie)
	}
	return req
}

func TestAuthenticate(t *testing.T) {
	t.Run("Adds valid authenticated user to request context", func(t *testing.T) {
		app := newTestApplication(t)

		session := newTestSession(t, app.sessionManager, map[string]any{
			"authenticatedUserID": testUsers["alice"].id,
		})

		var capturedUser database.User
		var capturedFound bool
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedUser, capturedFound = contextGetAuthenticatedUser(r)
			w.WriteHeader(http.StatusTeapot)
		})

		req := newTestRequest(t, http.MethodGet, "/test")
		req.AddCookie(session.cookie)

		res := send(t, req, app.sessionManager.LoadAndSave(app.authenticate(next)))
		assert.Equal(t, res.StatusCode, http.StatusTeapot)
		assert.True(t, capturedFound)
		assert.Equal(t, capturedUser.ID, testUsers["alice"].id)
		assert.Equal(t, capturedUser.Email, testUsers["alice"].email)
	})

	t.Run("Does not add user when no authenticated user ID in session", func(t *testing.T) {
		app := newTestApplication(t)

		var capturedFound bool
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, capturedFound = contextGetAuthenticatedUser(r)
			w.WriteHeader(http.StatusTeapot)
		})

		req := newTestRequest(t, http.MethodGet, "/test")

		res := send(t, req, app.sessionManager.LoadAndSave(app.authenticate(next)))
		assert.Equal(t, res.StatusCode, http.StatusTeapot)
		assert.False(t, capturedFound)
	})

	t.Run("Does not add user when user ID not found in database", func(t *testing.T) {
		app := newTestApplication(t)

		session := newTestSession(t, app.sessionManager, map[string]any{
			"authenticatedUserID": 999,
		})

		var capturedFound bool
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, capturedFound = contextGetAuthenticatedUser(r)
			w.WriteHeader(http.StatusTeapot)
		})

		req := newTestRequest(t, http.MethodGet, "/test")
		req.AddCookie(session.cookie)

		res := send(t, req, app.sessionManager.LoadAndSave(app.authenticate(next)))
		assert.Equal(t, res.StatusCode, http.StatusTeapot)
		assert.False(t, capturedFound)
	})
}

func TestRequireAuthenticatedUser(t *testing.T) {
	t.Run("Allows authenticated user to proceed and sets cache control header", func(t *testing.T) {
		app := newTestApplication(t)

		session := newTestSession(t, app.sessionManager, map[string]any{
			"authenticatedUserID": testUsers["alice"].id,
		})

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTeapot)
		})

		req := newTestRequest(t, http.MethodGet, "/restricted")
		req.AddCookie(session.cookie)

		res := send(t, req, app.sessionManager.LoadAndSave(app.authenticate(app.requireAuthenticatedUser(next))))
		assert.Equal(t, res.StatusCode, http.StatusTeapot)
		assert.Equal(t, res.Header.Get("Cache-Control"), "no-store")
	})

	t.Run("Redirects unauthenticated user to login and stores redirect path", func(t *testing.T) {
		app := newTestApplication(t)

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTeapot)
		})

		req := newTestRequest(t, http.MethodGet, "/test")

		res := send(t, req, app.sessionManager.LoadAndSave(app.authenticate(app.requireAuthenticatedUser(next))))
		assert.Equal(t, res.StatusCode, http.StatusSeeOther)
		assert.Equal(t, res.Header.Get("Location"), "/login")

		updatedSession := getTestSession(t, app.sessionManager, res.Cookies())
		assert.True(t, updatedSession != nil)
		assert.Equal(t, updatedSession.data["redirectPathAfterLogin"].(string), "/test")
	})
}

func TestRequireAnonymousUser(t *testing.T) {
	t.Run("Allows anonymous user to proceed", func(t *testing.T) {
		app := newTestApplication(t)

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTeapot)
		})

		req := newTestRequest(t, http.MethodGet, "/signup")

		res := send(t, req, app.sessionManager.LoadAndSave(app.authenticate(app.requireAnonymousUser(next))))
		assert.Equal(t, res.StatusCode, http.StatusTeapot)
	})

	t.Run("Redirects authenticated user to home page", func(t *testing.T) {
		app := newTestApplication(t)

		session := newTestSession(t, app.sessionManager, map[string]any{
			"authenticatedUserID": testUsers["alice"].id,
		})
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTeapot)
		})

		req := newTestRequest(t, http.MethodGet, "/test")
		req.AddCookie(session.cookie)

		res := send(t, req, app.sessionManager.LoadAndSave(app.authenticate(app.requireAnonymousUser(next))))
		assert.Equal(t, res.StatusCode, http.StatusSeeOther)
		assert.Equal(t, res.Header.Get("Location"), "/")
	})
}

func TestRequireBasicAuthentication(t *testing.T) {
	t.Run("Allows user with valid basic auth credentials to proceed", testRequireBasicAuthenticationAllowsValidUser)
	t.Run("Renders the 401 error page and WWW-Authenticate header for invalid authentication", testRequireBasicAuthenticationRejectsInvalidUsers)
}

func testRequireBasicAuthenticationAllowsValidUser(t *testing.T) {
	app := newBasicAuthTestApplication(t)
	req := newTestRequest(t, http.MethodGet, "/test")
	req.SetBasicAuth("admin", "placeholder*77")

	res := send(t, req, app.requireBasicAuthentication(teapotHandler()))
	assert.Equal(t, res.StatusCode, http.StatusTeapot)
}

func testRequireBasicAuthenticationRejectsInvalidUsers(t *testing.T) {
	for _, tt := range invalidBasicAuthCases() {
		t.Run(tt.name, func(t *testing.T) {
			assertInvalidBasicAuthentication(t, tt)
		})
	}
}

type basicAuthCase struct {
	name         string
	setAuth      bool
	authUsername string
	authPassword string
}

func invalidBasicAuthCases() []basicAuthCase {
	return []basicAuthCase{
		{name: "No basic auth credentials provided", setAuth: false},
		{name: "Invalid username provided", setAuth: true, authUsername: "wronguser", authPassword: "placeholder*77"},
		{name: "Invalid password provided", setAuth: true, authUsername: "admin", authPassword: "wrongpassword"},
	}
}

func assertInvalidBasicAuthentication(t *testing.T, tt basicAuthCase) {
	app := newBasicAuthTestApplication(t)
	req := newBasicAuthRequest(t, tt)

	res := send(t, req, app.requireBasicAuthentication(teapotHandler()))
	assert.Equal(t, res.StatusCode, http.StatusUnauthorized)
	assert.Equal(t, res.Header.Get("WWW-Authenticate"), `Basic realm="restricted", charset="UTF-8"`)
	assert.True(t, containsPageTag(t, res.Body, "errors/401"))
}

func newBasicAuthTestApplication(t *testing.T) *application {
	app := newTestApplication(t)
	app.config.basicAuth.username = "admin"
	app.config.basicAuth.hashedPassword = "$2a$04$HLvpR86.wXVT.2KHHkUbFe4/ou3wYGnc9FD7VcKaixofed5enOS.W"
	return app
}

func newBasicAuthRequest(t *testing.T, tt basicAuthCase) *http.Request {
	req := newTestRequest(t, http.MethodGet, "/test")
	if tt.setAuth {
		req.SetBasicAuth(tt.authUsername, tt.authPassword)
	}
	return req
}

func TestRecordAPIUsage(t *testing.T) {
	app := newTestApplication(t)
	userID := testUsers["alice"].id
	req := newTestRequest(t, http.MethodGet, "/api/calculate")

	app.recordAPIUsage(req, userID)
	app.wg.Wait()

	dailyCount, err := app.db.GetDailyAPICallCount(userID)
	assert.Nil(t, err)
	assert.Equal(t, dailyCount, 1)

	user, found, err := app.db.GetUser(userID)
	assert.Nil(t, err)
	assert.True(t, found)
	assert.Equal(t, user.ApiCallsCount, 1)
}
