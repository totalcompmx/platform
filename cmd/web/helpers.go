package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/jcroyoaun/totalcompmx/internal/version"

	"github.com/justinas/nosurf"
)

var readRandom = rand.Read

func (app *application) newTemplateData(r *http.Request) map[string]any {
	data := map[string]any{
		"CSRFToken": nosurf.Token(r),
		"Version":   version.Get(),
	}
	authenticatedUser, found := contextGetAuthenticatedUser(r)
	if found {
		data["AuthenticatedUser"] = authenticatedUser
	}

	return data
}

func (app *application) newEmailData() map[string]any {
	data := map[string]any{
		"BaseURL": app.config.baseURL,
	}

	return data
}

func (app *application) backgroundTask(r *http.Request, fn func() error) {
	app.wg.Add(1)
	go app.runBackgroundTask(r, fn)
}

func (app *application) runBackgroundTask(r *http.Request, fn func() error) {
	defer app.wg.Done()
	defer app.recoverBackgroundTask(r)

	err := fn()
	if err != nil {
		app.reportServerError(r, err)
	}
}

func (app *application) recoverBackgroundTask(r *http.Request) {
	pv := recover()
	if pv != nil {
		app.reportServerError(r, fmt.Errorf("%v", pv))
	}
}

// generateSecureAPIKey generates a cryptographically secure random API key
func (app *application) generateSecureAPIKey() (string, error) {
	// Generate 32 random bytes
	b := make([]byte, 32)
	_, err := readRandom(b)
	if err != nil {
		return "", err
	}

	// Encode to base64 URL-safe format and remove padding
	apiKey := base64.URLEncoding.EncodeToString(b)

	// Return first 43 characters (standard for 32-byte base64)
	return apiKey[:43], nil
}
