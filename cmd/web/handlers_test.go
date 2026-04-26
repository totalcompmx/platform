package main

import (
	"fmt"
	"net/http"
	"regexp"
	"testing"
	"time"

	"github.com/jcroyoaun/totalcompmx/internal/assert"
	"github.com/jcroyoaun/totalcompmx/internal/database"
	"github.com/jcroyoaun/totalcompmx/internal/password"
	"github.com/jcroyoaun/totalcompmx/internal/token"
)

func TestHome(t *testing.T) {
	t.Run("GET renders the home page", func(t *testing.T) {
		app := newTestApplication(t)

		req := newTestRequest(t, http.MethodGet, "/")

		res := send(t, req, app.routes())
		assert.Equal(t, res.StatusCode, http.StatusOK)
		assert.True(t, containsPageTag(t, res.Body, "home"))
	})
}

func TestSignup(t *testing.T) {
	t.Run("GET renders the signup page", func(t *testing.T) {
		app := newTestApplication(t)

		req := newTestRequest(t, http.MethodGet, "/signup")

		res := send(t, req, app.routes())
		assert.Equal(t, res.StatusCode, http.StatusOK)
		assert.True(t, containsPageTag(t, res.Body, "signup"))
		assert.True(t, containsHTMLNode(t, res.Body, `form[method="POST"][action="/signup"]`))
		assert.True(t, containsHTMLNode(t, res.Body, `input[type="hidden"][name="csrf_token"]`))
	})

	t.Run("POST creates new user and redirects", func(t *testing.T) {
		app := newTestApplication(t)

		req := newTestRequest(t, http.MethodPost, "/signup")
		req.PostForm.Add("Email", "zara@example.com")
		req.PostForm.Add("Password", "Zara_pw_fake00")

		res := sendWithCSRFToken(t, req, app.routes())
		assert.Equal(t, res.StatusCode, http.StatusSeeOther)
		assert.Equal(t, res.Header.Get("Location"), "/account/developer")

		user, found, err := app.db.GetUserByEmail("zara@example.com")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, found, true)
		assert.MatchesRegexp(t, user.HashedPassword, `^\$2a\$12\$[./0-9A-Za-z]{53}$`)
	})

	t.Run("POST rejects invalid data and re-displays the form", func(t *testing.T) {
		tests := []struct {
			testName     string
			userEmail    string
			userPassword string
		}{
			{
				testName:     "Rejects empty email",
				userEmail:    "",
				userPassword: "demo789$Test",
			},
			{
				testName:     "Rejects empty password",
				userEmail:    "zoe@example.com",
				userPassword: "",
			},
			{
				testName:     "Rejects invalid email",
				userEmail:    "invalid@example.",
				userPassword: "demo789$Test",
			},
			{
				testName:     "Rejects short password",
				userEmail:    "zoe@example.com",
				userPassword: "k4k3dw9",
			},
			{
				testName:     "Rejects password longer than 72 bytes",
				userEmail:    "zoe@example.com",
				userPassword: "iRbMr5Av5T1DINST1l2pGBBUtW4Qn628N4lN6tFNjW8Ea4fuYiI84j2KH8tKQrF3INkqbKwZh",
			},
			{
				testName:     "Rejects common password",
				userEmail:    "zoe@example.com",
				userPassword: "sunshine",
			},
			{
				testName:     "Rejects duplicate user",
				userEmail:    testUsers["alice"].email,
				userPassword: "pw-fake-abc987",
			},
		}

		for _, tt := range tests {
			t.Run(tt.testName, func(t *testing.T) {
				app := newTestApplication(t)

				req := newTestRequest(t, http.MethodPost, "/signup")
				req.PostForm.Add("Email", tt.userEmail)
				req.PostForm.Add("Password", tt.userPassword)

				res := sendWithCSRFToken(t, req, app.routes())
				assert.Equal(t, res.StatusCode, http.StatusUnprocessableEntity)
				assert.True(t, containsPageTag(t, res.Body, "signup"))
				assert.True(t, containsHTMLNode(t, res.Body, `form[method="POST"][action="/signup"]`))
				assert.True(t, containsHTMLNode(t, res.Body, `input[type="hidden"][name="csrf_token"]`))
			})
		}
	})
}

func TestLogin(t *testing.T) {
	t.Run("GET renders the login page", func(t *testing.T) {
		app := newTestApplication(t)

		req := newTestRequest(t, http.MethodGet, "/login")

		res := send(t, req, app.routes())
		assert.Equal(t, res.StatusCode, http.StatusOK)
		assert.True(t, containsPageTag(t, res.Body, "login"))
		assert.True(t, containsHTMLNode(t, res.Body, `form[method="POST"][action="/login"]`))
		assert.True(t, containsHTMLNode(t, res.Body, `input[type="hidden"][name="csrf_token"]`))
	})

	t.Run("POST authenticates user, renews the session token and redirects", func(t *testing.T) {
		app := newTestApplication(t)

		session := newTestSession(t, app.sessionManager, map[string]any{})

		req := newTestRequest(t, http.MethodPost, "/login")
		req.AddCookie(session.cookie)
		req.PostForm.Add("Email", testUsers["alice"].email)
		req.PostForm.Add("Password", testUsers["alice"].password)

		res := sendWithCSRFToken(t, req, app.routes())
		assert.Equal(t, res.StatusCode, http.StatusSeeOther)
		assert.Equal(t, res.Header.Get("Location"), "/account/developer")

		updatedSession := getTestSession(t, app.sessionManager, res.Cookies())
		assert.True(t, updatedSession != nil)
		assert.True(t, updatedSession.token != session.token)
		assert.Equal(t, updatedSession.data["authenticatedUserID"].(int), testUsers["alice"].id)
	})

	t.Run("POST rejects invalid credentials and re-displays the form", func(t *testing.T) {
		tests := []struct {
			testName     string
			userEmail    string
			userPassword string
		}{
			{
				testName:     "Rejects empty email",
				userEmail:    "",
				userPassword: testUsers["alice"].password,
			},
			{
				testName:     "Rejects empty password",
				userEmail:    testUsers["alice"].email,
				userPassword: "",
			},
			{
				testName:     "Rejects valid email but invalid password",
				userEmail:    testUsers["alice"].email,
				userPassword: "NotARealPass123#",
			},
			{
				testName:     "Rejects invalid email but valid password",
				userEmail:    "zaha@example.com",
				userPassword: testUsers["alice"].password,
			},
			{
				testName:     "Rejects mismatched email and password",
				userEmail:    testUsers["alice"].email,
				userPassword: testUsers["bob"].password,
			},
		}

		for _, tt := range tests {
			t.Run(tt.testName, func(t *testing.T) {
				app := newTestApplication(t)

				req := newTestRequest(t, http.MethodPost, "/login")
				req.PostForm.Add("Email", tt.userEmail)
				req.PostForm.Add("Password", tt.userPassword)

				res := sendWithCSRFToken(t, req, app.routes())
				assert.Equal(t, res.StatusCode, http.StatusUnprocessableEntity)
				assert.True(t, containsPageTag(t, res.Body, "login"))
				assert.True(t, containsHTMLNode(t, res.Body, `form[method="POST"][action="/login"]`))
				assert.True(t, containsHTMLNode(t, res.Body, `input[type="hidden"][name="csrf_token"]`))
			})
		}
	})
}

func TestLogout(t *testing.T) {
	t.Run("Unauthenticates the user, renews the session token and redirects", func(t *testing.T) {
		app := newTestApplication(t)

		session := newTestSession(t, app.sessionManager, map[string]any{
			"authenticatedUserID": testUsers["alice"].id,
		})

		req := newTestRequest(t, http.MethodPost, "/logout")
		req.AddCookie(session.cookie)

		res := sendWithCSRFToken(t, req, app.routes())
		assert.Equal(t, res.StatusCode, http.StatusSeeOther)
		assert.Equal(t, res.Header.Get("Location"), "/")

		updatedSession := getTestSession(t, app.sessionManager, res.Cookies())
		assert.NotNil(t, updatedSession)
		if updatedSession != nil {
			_, found := updatedSession.data["authenticatedUserID"]
			assert.False(t, found)
			assert.NotEqual(t, updatedSession.token, session.token)
		}
	})
}

func TestForgottenPassword(t *testing.T) {
	t.Run("GET renders the forgotten password page", func(t *testing.T) {
		app := newTestApplication(t)

		req := newTestRequest(t, http.MethodGet, "/forgotten-password")

		res := send(t, req, app.routes())
		assert.Equal(t, res.StatusCode, http.StatusOK)
		assert.True(t, containsPageTag(t, res.Body, "forgotten-password"))
		assert.True(t, containsHTMLNode(t, res.Body, `form[method="POST"][action="/forgotten-password"]`))
		assert.True(t, containsHTMLNode(t, res.Body, `input[type="hidden"][name="csrf_token"]`))
	})

	t.Run("POST creates password reset token, sends email and redirects", func(t *testing.T) {
		app := newTestApplication(t)

		req := newTestRequest(t, http.MethodPost, "/forgotten-password")
		req.PostForm.Add("Email", testUsers["alice"].email)

		res := sendWithCSRFToken(t, req, app.routes())
		assert.Equal(t, res.StatusCode, http.StatusSeeOther)
		assert.Equal(t, res.Header.Get("Location"), "/forgotten-password-confirmation")

		assert.Equal(t, len(app.mailer.SentMessages), 1)
		matches := regexp.MustCompile(`/password-reset/([a-z0-9]{26})`).FindStringSubmatch(app.mailer.SentMessages[0])
		assert.Equal(t, len(matches), 2)

		passwordResetToken := matches[1]
		passwordReset, found, err := app.db.GetPasswordReset(token.Hash(passwordResetToken))
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, found, true)
		assert.Equal(t, passwordReset.UserID, testUsers["alice"].id)
	})

	t.Run("POST rejects invalid data and re-displays the form", func(t *testing.T) {
		tests := []struct {
			testName   string
			userEmail  string
			wantStatus int
		}{
			{
				testName:  "Rejects empty email",
				userEmail: "",
			},
			{
				testName:  "Rejects invalid email format",
				userEmail: "invalid@example.",
			},
			{
				testName:  "Rejects non-existent user",
				userEmail: "zion@example.com",
			},
		}

		for _, tt := range tests {
			t.Run(tt.testName, func(t *testing.T) {
				app := newTestApplication(t)

				req := newTestRequest(t, http.MethodPost, "/forgotten-password")
				req.PostForm.Add("Email", tt.userEmail)

				res := sendWithCSRFToken(t, req, app.routes())
				assert.Equal(t, res.StatusCode, http.StatusUnprocessableEntity)
				assert.True(t, containsPageTag(t, res.Body, "forgotten-password"))
				assert.True(t, containsHTMLNode(t, res.Body, `form[method="POST"][action="/forgotten-password"]`))
				assert.True(t, containsHTMLNode(t, res.Body, `input[type="hidden"][name="csrf_token"]`))
			})
		}
	})
}

func TestForgottenPasswordConfirmation(t *testing.T) {
	t.Run("GET renders the forgotten password confirmation page", func(t *testing.T) {
		app := newTestApplication(t)

		req := newTestRequest(t, http.MethodGet, "/forgotten-password-confirmation")

		res := send(t, req, app.routes())
		assert.Equal(t, res.StatusCode, http.StatusOK)
		assert.True(t, containsPageTag(t, res.Body, "forgotten-password-confirmation"))
	})
}

func TestPasswordReset(t *testing.T) {
	t.Run("GET renders the signup page", testPasswordResetGet)
	t.Run("GET with an invalid token doesn't display the form", testPasswordResetInvalidToken)
	t.Run("POST updates the password and deletes all tokens for a user", testPasswordResetPostUpdatesPassword)
	t.Run("POST rejects invalid data and re-displays the form", testPasswordResetRejectsInvalidData)
}

func testPasswordResetGet(t *testing.T) {
	app := newTestApplication(t)
	plaintextToken := insertPasswordResetForUser(t, app, testUsers["alice"].id)
	req := newTestRequest(t, http.MethodGet, "/password-reset/"+plaintextToken)

	res := send(t, req, app.routes())
	assert.Equal(t, res.StatusCode, http.StatusOK)
	assertPasswordResetForm(t, res.Body, plaintextToken)
}

func testPasswordResetInvalidToken(t *testing.T) {
	app := newTestApplication(t)
	req := newTestRequest(t, http.MethodGet, "/password-reset/Inval1dT0k3N")

	res := send(t, req, app.routes())
	assert.Equal(t, res.StatusCode, http.StatusUnprocessableEntity)
	assert.True(t, containsPageTag(t, res.Body, "password-reset"))
	assert.False(t, containsHTMLNode(t, res.Body, `form[method="POST"][action^="/password-reset/"]`))
}

func testPasswordResetPostUpdatesPassword(t *testing.T) {
	app := newTestApplication(t)
	plaintextToken := insertPasswordResetForUser(t, app, testUsers["alice"].id)
	newPassword := "NewValidPassword123!"
	req := passwordResetPostRequest(t, plaintextToken, newPassword)

	res := sendWithCSRFToken(t, req, app.routes())
	assert.Equal(t, res.StatusCode, http.StatusSeeOther)
	assert.Equal(t, res.Header.Get("Location"), "/password-reset-confirmation")
	assertPasswordUpdated(t, app, newPassword)
	assertPasswordResetCount(t, app, testUsers["alice"].id, 0)
}

func testPasswordResetRejectsInvalidData(t *testing.T) {
	for _, tt := range invalidPasswordResetCases() {
		t.Run(tt.testName, func(t *testing.T) {
			assertInvalidPasswordReset(t, tt.newPassword)
		})
	}
}

type invalidPasswordResetCase struct {
	testName    string
	newPassword string
}

func invalidPasswordResetCases() []invalidPasswordResetCase {
	return []invalidPasswordResetCase{
		{testName: "Rejects empty password", newPassword: ""},
		{testName: "Rejects short password", newPassword: "k4k3dw9"},
		{testName: "Rejects password longer than 72 bytes", newPassword: "iRbMr5Av5T1DINST1l2pGBBUtW4Qn628N4lN6tFNjW8Ea4fuYiI84j2KH8tKQrF3INkqbKwZh"},
		{testName: "Rejects common password", newPassword: "sunshine"},
	}
}

func assertInvalidPasswordReset(t *testing.T, newPassword string) {
	app := newTestApplication(t)
	plaintextToken := insertPasswordResetForUser(t, app, testUsers["alice"].id)
	req := passwordResetPostRequest(t, plaintextToken, newPassword)

	res := sendWithCSRFToken(t, req, app.routes())
	assert.Equal(t, res.StatusCode, http.StatusUnprocessableEntity)
	assertPasswordResetForm(t, res.Body, plaintextToken)
}

func insertPasswordResetForUser(t *testing.T, app *application, userID int) string {
	t.Helper()
	plaintextToken := token.New()
	err := app.db.InsertPasswordReset(token.Hash(plaintextToken), userID, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return plaintextToken
}

func passwordResetPostRequest(t *testing.T, plaintextToken string, newPassword string) *http.Request {
	req := newTestRequest(t, http.MethodPost, "/password-reset/"+plaintextToken)
	req.PostForm.Add("NewPassword", newPassword)
	return req
}

func assertPasswordResetForm(t *testing.T, body string, plaintextToken string) {
	t.Helper()
	assert.True(t, containsPageTag(t, body, "password-reset"))
	assert.True(t, containsHTMLNode(t, body, `form[method="POST"]`))
	assert.True(t, containsHTMLNode(t, body, `input[type="hidden"][name="csrf_token"]`))
}

func assertPasswordUpdated(t *testing.T, app *application, newPassword string) {
	t.Helper()
	user := mustUserByEmail(t, app, testUsers["alice"].email)
	passwordMatches, err := password.Matches(newPassword, user.HashedPassword)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, passwordMatches)
}

func mustUserByEmail(t *testing.T, app *application, email string) database.User {
	t.Helper()
	user, found, err := app.db.GetUserByEmail(email)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal(fmt.Errorf("test user with email %s not found", email))
	}
	return user
}

func assertPasswordResetCount(t *testing.T, app *application, userID int, expected int) {
	t.Helper()
	store, ok := app.db.(*fakeStore)
	if !ok {
		t.Fatal("test application is not using fakeStore")
	}
	count := store.passwordResetCount(userID)
	assert.Equal(t, count, expected)
}

func TestPasswordResetConfirmation(t *testing.T) {
	t.Run("GET renders the password reset confirmation page", func(t *testing.T) {
		app := newTestApplication(t)

		req := newTestRequest(t, http.MethodGet, "/password-reset-confirmation")

		res := send(t, req, app.routes())
		assert.Equal(t, res.StatusCode, http.StatusOK)
		assert.True(t, containsPageTag(t, res.Body, "password-reset-confirmation"))
	})
}

func TestRestricted(t *testing.T) {
	t.Run("Unauthenticated users are redirected to the login page", func(t *testing.T) {
		app := newTestApplication(t)

		req := newTestRequest(t, http.MethodGet, "/restricted")

		res := send(t, req, app.routes())
		assert.Equal(t, res.StatusCode, http.StatusSeeOther)
		assert.Equal(t, res.Header.Get("Location"), "/login")
	})

	t.Run("Authenticated users are shown the restricted page", func(t *testing.T) {
		app := newTestApplication(t)

		session := newTestSession(t, app.sessionManager, map[string]any{
			"authenticatedUserID": testUsers["alice"].id,
		})

		req := newTestRequest(t, http.MethodGet, "/restricted")
		req.AddCookie(session.cookie)

		res := send(t, req, app.routes())
		assert.Equal(t, res.StatusCode, http.StatusOK)
		assert.True(t, containsPageTag(t, res.Body, "restricted"))
	})
}

func TestRestrictedBasicAuth(t *testing.T) {
	t.Run("Unauthenticated users get the 401 error page and appropriate WWW-Authenticate header", func(t *testing.T) {
		app := newTestApplication(t)

		req := newTestRequest(t, http.MethodGet, "/restricted-basic-auth")

		res := send(t, req, app.routes())
		assert.Equal(t, res.StatusCode, http.StatusUnauthorized)
		assert.Equal(t, res.Header.Get("WWW-Authenticate"), `Basic realm="restricted", charset="UTF-8"`)
		assert.True(t, containsPageTag(t, res.Body, "errors/401"))
	})

	t.Run("Authenticated users are shown the restricted page", func(t *testing.T) {
		app := newTestApplication(t)

		authUsername := "admin"
		authPassword := "placeholder*77"
		authHashedPassword := "$2a$04$MTmOEATIPE7akymfiaOqyuQemmXp6VAY8pn6yRf3Ya5REVK78umcu"

		app.config.basicAuth.username = authUsername
		app.config.basicAuth.hashedPassword = authHashedPassword

		req := newTestRequest(t, http.MethodGet, "/restricted-basic-auth")
		req.SetBasicAuth(authUsername, authPassword)

		res := send(t, req, app.routes())
		assert.Equal(t, res.StatusCode, http.StatusOK)
		assert.True(t, containsPageTag(t, res.Body, "restricted"))
	})
}
