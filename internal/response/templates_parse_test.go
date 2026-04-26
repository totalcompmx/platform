package response

import (
	"html/template"
	"io/fs"
	"strings"
	"testing"

	"github.com/jcroyoaun/totalcompmx/assets"
	"github.com/jcroyoaun/totalcompmx/internal/funcs"
)

func TestPageTemplatesParse(t *testing.T) {
	for _, page := range parseablePageTemplates(t) {
		t.Run(page, func(t *testing.T) {
			assertPageTemplateParses(t, page)
		})
	}
}

func parseablePageTemplates(t *testing.T) []string {
	t.Helper()

	pages, err := fs.Glob(assets.EmbeddedFiles, "templates/pages/*.tmpl")
	if err != nil {
		t.Fatal(err)
	}

	parseable := make([]string, 0, len(pages))
	for _, page := range pages {
		if !strings.HasSuffix(page, "_old.tmpl") {
			parseable = append(parseable, page)
		}
	}
	return parseable
}

func assertPageTemplateParses(t *testing.T, page string) {
	t.Helper()

	_, err := template.New("").Funcs(funcs.TemplateFuncs).ParseFS(
		assets.EmbeddedFiles,
		"templates/base.tmpl",
		"templates/partials/*.tmpl",
		page,
	)
	if err != nil {
		t.Fatal(err)
	}
}
