package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
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
// with a recognizable "tc_" prefix.
func (app *application) generateSecureAPIKey() (string, error) {
	// Generate 32 random bytes
	b := make([]byte, 32)
	_, err := readRandom(b)
	if err != nil {
		return "", err
	}

	// Encode to base64 URL-safe format; 43 characters carry the full 32 bytes.
	apiKey := base64.URLEncoding.EncodeToString(b)

	return "tc_" + apiKey[:43], nil
}

// hashAPIKey returns the SHA-256 hex digest under which a key is stored.
func hashAPIKey(apiKey string) string {
	digest := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(digest[:])
}

// apiKeyPrefix returns the leading characters of a key, safe to display in the
// dashboard after the plaintext key is gone.
func apiKeyPrefix(apiKey string) string {
	if len(apiKey) < 8 {
		return apiKey
	}
	return apiKey[:8]
}
