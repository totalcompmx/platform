package main

import (
	"strings"

	"testing"

	"github.com/jcroyoaun/totalcompmx/internal/assert"
)

func TestServeHTTP(t *testing.T) {

	t.Run("Invalid port configuration causes an error", func(t *testing.T) {
		app := newTestApplication(t)
		app.config.httpPort = -1

		err := app.serveHTTP()
		assert.NotNil(t, err)
	})
}

func TestServeAutoHTTPS(t *testing.T) {

	t.Run("Rejects localhost domain", func(t *testing.T) {
		app := newTestApplication(t)
		app.config.autoHTTPS.domain = "localhost"
		app.config.autoHTTPS.email = "test@example.com"
		app.config.autoHTTPS.staging = true

		err := app.serveAutoHTTPS()
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), "localhost"))
	})

	t.Run("Rejects localhost with port", func(t *testing.T) {
		app := newTestApplication(t)
		app.config.autoHTTPS.domain = "localhost:8080"
		app.config.autoHTTPS.email = "test@example.com"
		app.config.autoHTTPS.staging = true

		err := app.serveAutoHTTPS()
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), "localhost"))
	})
}
