package main

import (
	"net/http"

	"github.com/jcroyoaun/totalcompmx/assets"

	"github.com/alexedwards/flow"
)

func (app *application) routes() http.Handler {
	mux := flow.New()
	mux.NotFound = http.HandlerFunc(app.notFound)

	// Global middleware for ALL routes
	mux.Use(app.logAccess)
	mux.Use(app.recoverPanic)
	mux.Use(app.securityHeaders)

	fileServer := http.FileServer(http.FS(assets.EmbeddedFiles))
	mux.Handle("/static/...", fileServer, "GET")

	// API routes - NO CSRF, NO SESSION (stateless)
	mux.Group(func(mux *flow.Mux) {
		mux.Use(app.requireAPIKey)

		mux.HandleFunc("/api/v1/calculate", app.apiCalculate, "POST")
	})

	// Web routes - WITH session, CSRF, and authentication
	mux.Group(func(mux *flow.Mux) {
		mux.Use(app.sessionManager.LoadAndSave)
		mux.Use(app.preventCSRF)
		mux.Use(app.authenticate)

		// Public pages
		mux.HandleFunc("/", app.home, "GET", "POST")
		mux.HandleFunc("/clear", app.clearSession, "POST")
		mux.HandleFunc("/calculator", app.salaryCalculator, "GET", "POST")
		mux.HandleFunc("/export-pdf", app.exportPDF, "GET")
		mux.HandleFunc("/privacy", app.privacy, "GET")
		mux.HandleFunc("/terms", app.terms, "GET")
		mux.HandleFunc("/developers", app.developersPage, "GET")
		mux.HandleFunc("/robots.txt", app.robotsTxt, "GET")
		mux.HandleFunc("/sitemap.xml", app.sitemapXML, "GET")
		mux.HandleFunc("/verify-email/:plaintextToken", app.verifyEmail, "GET")

		// Anonymous user routes (login/signup)
		mux.Group(func(mux *flow.Mux) {
			mux.Use(app.requireAnonymousUser)

			mux.HandleFunc("/signup", app.signup, "GET", "POST")
			mux.HandleFunc("/login", app.login, "GET", "POST")
			mux.HandleFunc("/forgotten-password", app.forgottenPassword, "GET", "POST")
			mux.HandleFunc("/forgotten-password-confirmation", app.forgottenPasswordConfirmation, "GET")
			mux.HandleFunc("/password-reset/:plaintextToken", app.passwordReset, "GET", "POST")
			mux.HandleFunc("/password-reset-confirmation", app.passwordResetConfirmation, "GET")
		})

		// Authenticated user routes (developer dashboard, logout)
		mux.Group(func(mux *flow.Mux) {
			mux.Use(app.requireAuthenticatedUser)

			mux.HandleFunc("/restricted", app.restricted, "GET")
			mux.HandleFunc("/logout", app.logout, "POST")
			mux.HandleFunc("/account/developer", app.accountDeveloper, "GET")
			mux.HandleFunc("/account/api-key", app.generateAPIKey, "POST")
			mux.HandleFunc("/account/resend-verification", app.resendVerificationEmail, "POST")
		})

		// Basic auth routes
		mux.Group(func(mux *flow.Mux) {
			mux.Use(app.requireBasicAuthentication)

			mux.HandleFunc("/restricted-basic-auth", app.restricted, "GET")
		})
	})

	return mux
}
