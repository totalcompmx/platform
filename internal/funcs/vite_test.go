package funcs

import (
	"errors"
	"strings"
	"testing"

	"github.com/jcroyoaun/totalcompmx/internal/assert"
)

func TestViteTags(t *testing.T) {
	t.Run("renders dev server tags when configured", func(t *testing.T) {
		restore := stubVite("http://localhost:5173/", nil, errors.New("manifest unavailable"))
		defer restore()

		tags, err := viteTags("src/entries/home.ts")

		assert.Nil(t, err)
		body := string(tags)
		assert.True(t, strings.Contains(body, `src="http://localhost:5173/@vite/client"`))
		assert.True(t, strings.Contains(body, `src="http://localhost:5173/src/entries/home.ts"`))
	})

	t.Run("renders fallback tags when no manifest is available", func(t *testing.T) {
		restore := stubVite("", nil, errors.New("manifest unavailable"))
		defer restore()

		tags, err := viteTags("src/entries/home.ts")

		assert.Nil(t, err)
		assert.Equal(t, string(tags), "")
	})

	t.Run("renders production tags from manifest", func(t *testing.T) {
		restore := stubVite("", []byte(`{
			"_shared.js": {
				"file": "assets/shared.123.js",
				"css": ["assets/shared.123.css"]
			},
			"src/entries/home.ts": {
				"file": "assets/home.123.js",
				"css": ["assets/home.123.css"],
				"imports": ["_shared.js"],
				"isEntry": true
			}
		}`), nil)
		defer restore()

		tags, err := viteTags("src/entries/home.ts")

		assert.Nil(t, err)
		body := string(tags)
		assert.True(t, strings.Contains(body, `<link rel="stylesheet" href="/static/dist/assets/home.123.css">`))
		assert.True(t, strings.Contains(body, `<link rel="stylesheet" href="/static/dist/assets/shared.123.css">`))
		assert.True(t, strings.Contains(body, `<link rel="modulepreload" href="/static/dist/assets/shared.123.js">`))
		assert.True(t, strings.Contains(body, `<script type="module" src="/static/dist/assets/home.123.js"></script>`))
	})

	t.Run("returns an error when manifest exists without entry", func(t *testing.T) {
		restore := stubVite("", []byte(`{}`), nil)
		defer restore()

		_, err := viteTags("src/entries/missing.ts")

		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), "vite manifest entry not found: src/entries/missing.ts"))
	})

	t.Run("returns an error for invalid manifest JSON", func(t *testing.T) {
		restore := stubVite("", []byte(`{`), nil)
		defer restore()

		_, err := viteTags("src/entries/home.ts")

		assert.NotNil(t, err)
	})
}

func TestViteManifestTagsHandlesDuplicateAndMissingImports(t *testing.T) {
	manifest := viteManifest{
		"src/entries/home.ts": {
			File:    "assets/home.123.js",
			CSS:     []string{"assets/home.123.css"},
			Imports: []string{"_shared.js", "_shared.js", "_missing.js"},
		},
		"_shared.js": {
			CSS:     []string{"assets/home.123.css", "assets/shared.123.css"},
			Imports: []string{"_missing.js"},
		},
	}

	tags, err := viteManifestTags(manifest, "src/entries/home.ts")

	assert.Nil(t, err)
	body := strings.Join(tags, "\n")
	assert.Equal(t, strings.Count(body, "assets/home.123.css"), 1)
	assert.True(t, strings.Contains(body, `<link rel="stylesheet" href="/static/dist/assets/shared.123.css">`))
	assert.True(t, strings.Contains(body, `<script type="module" src="/static/dist/assets/home.123.js"></script>`))
}

func TestViteFallbackTags(t *testing.T) {
	tags := viteFallbackTags([]string{"/static/css/main.css", "/static/js/legacy.js"})

	assert.Equal(t, strings.Join(tags, "\n"), strings.Join([]string{
		`<link rel="stylesheet" href="/static/css/main.css">`,
		`<script src="/static/js/legacy.js" defer></script>`,
	}, "\n"))
}

func stubVite(devServerURL string, manifest []byte, manifestErr error) func() {
	originalDevServerURL := getViteDevServerURL
	originalManifestReader := readViteManifest

	getViteDevServerURL = func() string {
		return devServerURL
	}
	readViteManifest = func() ([]byte, error) {
		return manifest, manifestErr
	}

	return func() {
		getViteDevServerURL = originalDevServerURL
		readViteManifest = originalManifestReader
	}
}
