package response

import (
	"bytes"
	"html/template"
	"net/http"

	"github.com/jcroyoaun/totalcompmx/assets"
	"github.com/jcroyoaun/totalcompmx/internal/funcs"
)

func Page(w http.ResponseWriter, status int, data any, pagePath string) error {
	return PageWithHeaders(w, status, data, nil, pagePath)
}

func PageWithHeaders(w http.ResponseWriter, status int, data any, headers http.Header, pagePath string) error {
	patterns := []string{"base.tmpl", "partials/*.tmpl", pagePath}

	return NamedTemplateWithHeaders(w, status, data, headers, "base", patterns...)
}

func NamedTemplate(w http.ResponseWriter, status int, data any, templateName string, patterns ...string) error {
	return NamedTemplateWithHeaders(w, status, data, nil, templateName, patterns...)
}

func NamedTemplateWithHeaders(w http.ResponseWriter, status int, data any, headers http.Header, templateName string, patterns ...string) error {
	ts, err := parseEmbeddedTemplate(patterns)
	if err != nil {
		return err
	}

	buf, err := executeTemplate(ts, templateName, data)
	if err != nil {
		return err
	}

	writeHeaders(w, headers)
	w.WriteHeader(status)
	_, err = buf.WriteTo(w)
	return err
}

func parseEmbeddedTemplate(patterns []string) (*template.Template, error) {
	return template.New("").Funcs(funcs.TemplateFuncs).ParseFS(assets.EmbeddedFiles, embeddedTemplatePatterns(patterns)...)
}

func embeddedTemplatePatterns(patterns []string) []string {
	embeddedPatterns := make([]string, len(patterns))
	for i := range patterns {
		embeddedPatterns[i] = "templates/" + patterns[i]
	}

	return embeddedPatterns
}

func executeTemplate(ts *template.Template, templateName string, data any) (*bytes.Buffer, error) {
	buf := new(bytes.Buffer)
	err := ts.ExecuteTemplate(buf, templateName, data)
	return buf, err
}

func writeHeaders(w http.ResponseWriter, headers http.Header) {
	for key, values := range headers {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
}
