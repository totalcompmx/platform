package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jcroyoaun/totalcompmx/internal/database"
	"github.com/jcroyoaun/totalcompmx/internal/response"

	"github.com/justinas/nosurf"
	"github.com/tomasen/realip"
	"golang.org/x/crypto/bcrypt"
)

func (app *application) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			pv := recover()
			if pv != nil {
				app.serverError(w, r, fmt.Errorf("%v", pv))
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func (app *application) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		w.Header().Set("Referrer-Policy", "origin-when-cross-origin")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "deny")

		next.ServeHTTP(w, r)
	})
}

func (app *application) logAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mw := response.NewMetricsResponseWriter(w)
		next.ServeHTTP(mw, r)

		var (
			ip     = realip.FromRequest(r)
			method = r.Method
			url    = r.URL.String()
			proto  = r.Proto
		)

		userAttrs := slog.Group("user", "ip", ip)
		requestAttrs := slog.Group("request", "method", method, "url", url, "proto", proto)
		responseAttrs := slog.Group("response", "status", mw.StatusCode, "size", mw.BytesCount)

		app.logger.Info("access", userAttrs, requestAttrs, responseAttrs)
	})
}

func (app *application) preventCSRF(next http.Handler) http.Handler {
	csrfHandler := nosurf.New(next)

	csrfHandler.SetBaseCookie(http.Cookie{
		HttpOnly: true,
		Path:     "/",
		MaxAge:   86400,
		SameSite: http.SameSiteLaxMode,
		Secure:   app.config.cookie.secure,
	})

	csrfHandler.SetFailureHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.badRequest(w, r, errors.New("CSRF token validation failed"))
	}))

	return csrfHandler
}

func (app *application) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authenticatedRequest, ok := app.authenticatedRequest(w, r)
		if !ok {
			return
		}

		next.ServeHTTP(w, authenticatedRequest)
	})
}

func (app *application) authenticatedRequest(w http.ResponseWriter, r *http.Request) (*http.Request, bool) {
	id := app.sessionManager.GetInt(r.Context(), "authenticatedUserID")
	if id == 0 {
		return r, true
	}

	user, found, err := app.db.GetUser(id)
	if err != nil {
		app.serverError(w, r, err)
		return nil, false
	}

	if found {
		return contextSetAuthenticatedUser(r, user), true
	}

	return r, true
}

func (app *application) requireAuthenticatedUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, found := contextGetAuthenticatedUser(r)
		if !found {
			app.sessionManager.Put(r.Context(), "redirectPathAfterLogin", r.URL.Path)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		w.Header().Add("Cache-Control", "no-store")

		next.ServeHTTP(w, r)
	})
}

func (app *application) requireAnonymousUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, found := contextGetAuthenticatedUser(r)

		if found {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return

		}

		next.ServeHTTP(w, r)
	})
}

func (app *application) requireBasicAuthentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		valid, err := app.validBasicAuthentication(r)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		if !valid {
			app.basicAuthenticationRequired(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (app *application) validBasicAuthentication(r *http.Request) (bool, error) {
	username, plaintextPassword, ok := r.BasicAuth()
	if !ok {
		return false, nil
	}
	if app.config.basicAuth.username != username {
		return false, nil
	}
	return app.validBasicAuthPassword(plaintextPassword)
}

func (app *application) validBasicAuthPassword(plaintextPassword string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(app.config.basicAuth.hashedPassword), []byte(plaintextPassword))
	switch {
	case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
		return false, nil
	case err != nil:
		return false, err
	default:
		return true, nil
	}
}

// requireAPIKey validates the API key in the Authorization header (stateless)
func (app *application) requireAPIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := app.authenticatedAPIUser(w, r)
		if !ok {
			return
		}

		app.recordAPIUsage(r, user.ID)
		next.ServeHTTP(w, contextSetAuthenticatedUser(r, user))
	})
}

func (app *application) authenticatedAPIUser(w http.ResponseWriter, r *http.Request) (database.User, bool) {
	apiKey, ok := app.requestAPIKey(w, r)
	if !ok {
		return database.User{}, false
	}

	user, ok := app.apiUserForKey(w, r, apiKey)
	if !ok {
		return database.User{}, false
	}
	if !app.allowAPIRequest(w, r, user) {
		return database.User{}, false
	}

	return user, true
}

func (app *application) requestAPIKey(w http.ResponseWriter, r *http.Request) (string, bool) {
	apiKey, authErr := bearerAPIKey(r)
	if authErr != "" {
		app.writeJSONError(w, r, http.StatusUnauthorized, authErr)
		return "", false
	}
	return apiKey, true
}

func (app *application) apiUserForKey(w http.ResponseWriter, r *http.Request, apiKey string) (database.User, bool) {
	user, found, err := app.db.GetUserByAPIKey(apiKey)
	if err != nil {
		app.serverError(w, r, err)
		return database.User{}, false
	}
	if !found {
		app.writeJSONError(w, r, http.StatusUnauthorized, "Invalid API key")
		return database.User{}, false
	}

	return user, true
}

func bearerAPIKey(r *http.Request) (string, string) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", "Missing Authorization header. Use: Authorization: Bearer YOUR_API_KEY"
	}

	apiKey, ok := strings.CutPrefix(authHeader, "Bearer ")
	if !ok {
		return "", "Invalid Authorization format. Use: Authorization: Bearer YOUR_API_KEY"
	}
	if apiKey == "" {
		return "", "API key is empty"
	}

	return apiKey, ""
}

func (app *application) allowAPIRequest(w http.ResponseWriter, r *http.Request, user database.User) bool {
	if user.EmailVerified {
		return true
	}

	dailyCount, err := app.db.GetDailyAPICallCount(user.ID)
	if err != nil {
		app.serverError(w, r, err)
		return false
	}
	if dailyCount >= 10 {
		app.writeAPIRateLimitError(w, r, dailyCount)
		return false
	}

	return true
}

func (app *application) writeAPIRateLimitError(w http.ResponseWriter, r *http.Request, dailyCount int) {
	err := responseJSON(w, http.StatusTooManyRequests, map[string]any{
		"error":   "Daily API limit exceeded",
		"message": "You have reached your daily limit of 10 API calls. Verify your email to unlock 100 calls/month.",
		"limit":   10,
		"used":    dailyCount,
		"type":    "unverified_user",
		"action":  "Please verify your email to increase your limit.",
	})
	if err != nil {
		app.serverError(w, r, err)
	}
}

func (app *application) recordAPIUsage(r *http.Request, userID int) {
	if err := app.db.LogAPICall(userID); err != nil {
		app.logger.Error("failed to log API call", "error", err, "user_id", userID)
	}

	app.backgroundTask(r, func() error {
		return app.db.IncrementAPICallsCount(userID)
	})
}
