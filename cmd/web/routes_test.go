package main

import (
	"net/http"
	"testing"

	"github.com/jcroyoaun/totalcompmx/internal/assert"
)

func TestRoutes(t *testing.T) {
	t.Run("Serves CSS file with appropriate headers and content", func(t *testing.T) {
		app := newTestApplication(t)

		req := newTestRequest(t, http.MethodGet, "/static/css/tokens.css")

		res := send(t, req, app.routes())
		assert.Equal(t, res.StatusCode, http.StatusOK)
		assert.Equal(t, res.Header.Get("Content-Type"), "text/css; charset=utf-8")
		assert.True(t, len(res.Body) > 0)
	})

	t.Run("Renders the 404 error page for non-existent routes", func(t *testing.T) {
		app := newTestApplication(t)

		req := newTestRequest(t, http.MethodGet, "/nonexistent")

		res := send(t, req, app.routes())
		assert.Equal(t, res.StatusCode, http.StatusNotFound)
		assert.True(t, containsPageTag(t, res.Body, "errors/404"))
	})

	t.Run("Sends a 405 response for routes with a matching route pattern but no matching HTTP method", func(t *testing.T) {
		app := newTestApplication(t)

		req := newTestRequest(t, http.MethodTrace, "/")

		res := send(t, req, app.routes())
		assert.Equal(t, res.StatusCode, http.StatusMethodNotAllowed)
	})
}
