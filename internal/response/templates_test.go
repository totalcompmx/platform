package response

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jcroyoaun/totalcompmx/internal/assert"
	"github.com/jcroyoaun/totalcompmx/internal/database"
)

func TestNamedTemplate(t *testing.T) {
	t.Run("Write valid HTML response the correct status code", func(t *testing.T) {
		w := httptest.NewRecorder()

		err := NamedTemplate(w, http.StatusTeapot, "this is a test", "test:data", "testdata/test.tmpl")
		assert.Nil(t, err)
		assert.Equal(t, w.Code, http.StatusTeapot)
		assert.Equal(t, strings.TrimSpace(w.Body.String()), "<strong>this is a test</strong>")
	})
}

func TestPage(t *testing.T) {
	t.Run("Writes embedded page with base template", func(t *testing.T) {
		w := httptest.NewRecorder()

		err := Page(w, http.StatusTeapot, nil, "pages/privacy.tmpl")
		assert.Nil(t, err)
		assert.Equal(t, http.StatusTeapot, w.Code)
		assert.True(t, strings.Contains(w.Body.String(), "Aviso de Privacidad"))
	})
}

func TestPageWithHeaders(t *testing.T) {
	t.Run("Writes embedded page with custom headers", func(t *testing.T) {
		w := httptest.NewRecorder()
		headers := http.Header{"X-Test": []string{"ok"}}

		err := PageWithHeaders(w, http.StatusAccepted, nil, headers, "pages/terms.tmpl")
		assert.Nil(t, err)
		assert.Equal(t, http.StatusAccepted, w.Code)
		assert.Equal(t, "ok", w.Header().Get("X-Test"))
	})
}

func TestHomePageScriptConfig(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]any{
		"CSRFToken":  "csrf-token",
		"FiscalYear": database.FiscalYear{USDMXNRate: 19.1234},
	}

	err := Page(w, http.StatusOK, data, "pages/home.tmpl")

	assert.Nil(t, err)
	body := w.Body.String()
	assert.True(t, strings.Contains(body, `/static/dist/assets/home`) || !strings.Contains(body, `type="module"`))
	assert.True(t, strings.Contains(body, `csrfToken: "csrf-token"`))
	assert.True(t, strings.Contains(body, `usdMxnRate: "19.1234"`))
}

func TestNamedTemplateWithHeaders(t *testing.T) {
	t.Run("Write valid HTML response the correct status code", func(t *testing.T) {
		w := httptest.NewRecorder()

		err := NamedTemplateWithHeaders(w, http.StatusTeapot, "this is a test", nil, "test:data", "testdata/test.tmpl")
		assert.Nil(t, err)
		assert.Equal(t, w.Code, http.StatusTeapot)
		assert.Equal(t, strings.TrimSpace(w.Body.String()), "<strong>this is a test</strong>")
	})

	t.Run("Write valid HTML response with custom headers", func(t *testing.T) {
		w := httptest.NewRecorder()

		headers := http.Header{
			"X-Custom-Header": []string{"custom-value"},
			"X-Request-ID":    []string{"12345"},
			"X-Multiple":      []string{"value1", "value2", "value3"},
		}

		err := NamedTemplateWithHeaders(w, http.StatusTeapot, "this is a test", headers, "test:data", "testdata/test.tmpl")
		assert.Nil(t, err)
		assert.Equal(t, w.Code, http.StatusTeapot)
		assert.Equal(t, strings.TrimSpace(w.Body.String()), "<strong>this is a test</strong>")
		assert.Equal(t, w.Header().Get("X-Custom-Header"), "custom-value")
		assert.Equal(t, w.Header().Get("X-Request-ID"), "12345")
		assert.Equal(t, w.Header().Values("X-Multiple"), []string{"value1", "value2", "value3"})
	})

	t.Run("Check functions are available to the templates", func(t *testing.T) {
		w := httptest.NewRecorder()

		err := NamedTemplateWithHeaders(w, http.StatusTeapot, nil, nil, "test:function", "testdata/test.tmpl")
		assert.Nil(t, err)
		assert.Equal(t, strings.TrimSpace(w.Body.String()), "<strong>THIS IS ANOTHER TEST</strong>")
	})

	t.Run("Returns error for non-existent template name", func(t *testing.T) {
		w := httptest.NewRecorder()

		err := NamedTemplateWithHeaders(w, http.StatusTeapot, nil, nil, "test:non-existent-template", "testdata/test.tmpl")
		assert.NotNil(t, err)
	})

	t.Run("Returns error for non-existent template pattern", func(t *testing.T) {
		w := httptest.NewRecorder()

		err := NamedTemplateWithHeaders(w, http.StatusTeapot, nil, nil, "test:data", "testdata/non-existent-file.tmpl")
		assert.NotNil(t, err)
	})
}
