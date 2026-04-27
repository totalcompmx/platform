package funcs

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"strings"

	"github.com/jcroyoaun/totalcompmx/assets"
)

const viteManifestPath = "static/dist/manifest.json"

var getViteDevServerURL = func() string {
	return os.Getenv("VITE_DEV_SERVER_URL")
}

var readViteManifest = func() ([]byte, error) {
	return assets.EmbeddedFiles.ReadFile(viteManifestPath)
}

type viteManifest map[string]viteManifestChunk

type viteManifestChunk struct {
	File    string   `json:"file"`
	CSS     []string `json:"css"`
	Imports []string `json:"imports"`
	IsEntry bool     `json:"isEntry"`
}

func viteTags(entry string, fallbackPaths ...string) (template.HTML, error) {
	if devServerURL := normalizedViteDevServerURL(); devServerURL != "" {
		return template.HTML(strings.Join(viteDevTags(devServerURL, entry), "\n")), nil
	}

	manifest, ok, err := loadViteManifest()
	if err != nil {
		return "", err
	}
	if !ok {
		return template.HTML(strings.Join(viteFallbackTags(fallbackPaths), "\n")), nil
	}

	tags, err := viteManifestTags(manifest, entry)
	if err != nil {
		return "", err
	}
	return template.HTML(strings.Join(tags, "\n")), nil
}

func normalizedViteDevServerURL() string {
	return strings.TrimRight(getViteDevServerURL(), "/")
}

func viteDevTags(devServerURL string, entry string) []string {
	return []string{
		viteModuleScriptTag(devServerURL + "/@vite/client"),
		viteModuleScriptTag(devServerURL + "/" + strings.TrimPrefix(entry, "/")),
	}
}

func loadViteManifest() (viteManifest, bool, error) {
	data, err := readViteManifest()
	if err != nil {
		return nil, false, nil
	}

	var manifest viteManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, false, err
	}

	return manifest, true, nil
}

func viteManifestTags(manifest viteManifest, entry string) ([]string, error) {
	chunk, ok := manifest[entry]
	if !ok {
		return nil, missingViteEntryError(entry)
	}

	tags := make([]string, 0)
	for _, cssFile := range viteCSSFiles(manifest, entry) {
		tags = append(tags, viteStylesheetTag(viteAssetPath(cssFile)))
	}
	for _, importedChunk := range viteImportedChunks(manifest, entry) {
		if importedChunk.File != "" {
			tags = append(tags, viteModulePreloadTag(viteAssetPath(importedChunk.File)))
		}
	}
	tags = append(tags, viteModuleScriptTag(viteAssetPath(chunk.File)))

	return tags, nil
}

func missingViteEntryError(entry string) error {
	return fmt.Errorf("vite manifest entry not found: %s", entry)
}

func viteCSSFiles(manifest viteManifest, entry string) []string {
	chunks := []viteManifestChunk{manifest[entry]}
	chunks = append(chunks, viteImportedChunks(manifest, entry)...)

	seen := make(map[string]bool)
	cssFiles := make([]string, 0)
	for _, chunk := range chunks {
		for _, cssFile := range chunk.CSS {
			if seen[cssFile] {
				continue
			}
			seen[cssFile] = true
			cssFiles = append(cssFiles, cssFile)
		}
	}

	return cssFiles
}

func viteImportedChunks(manifest viteManifest, entry string) []viteManifestChunk {
	seen := make(map[string]bool)
	chunks := make([]viteManifestChunk, 0)

	var visit func(name string)
	visit = func(name string) {
		chunk, ok := manifest[name]
		if !ok {
			return
		}
		for _, importName := range chunk.Imports {
			if seen[importName] {
				continue
			}
			seen[importName] = true
			visit(importName)
			if importedChunk, ok := manifest[importName]; ok {
				chunks = append(chunks, importedChunk)
			}
		}
	}

	visit(entry)
	return chunks
}

func viteFallbackTags(paths []string) []string {
	tags := make([]string, 0, len(paths))
	for _, path := range paths {
		if strings.HasSuffix(path, ".css") {
			tags = append(tags, viteStylesheetTag(path))
			continue
		}
		tags = append(tags, viteScriptTag(path))
	}
	return tags
}

func viteAssetPath(file string) string {
	return "/static/dist/" + strings.TrimPrefix(file, "/")
}

func viteStylesheetTag(path string) string {
	return `<link rel="stylesheet" href="` + htmlAttr(path) + `">`
}

func viteModulePreloadTag(path string) string {
	return `<link rel="modulepreload" href="` + htmlAttr(path) + `">`
}

func viteModuleScriptTag(path string) string {
	return `<script type="module" src="` + htmlAttr(path) + `"></script>`
}

func viteScriptTag(path string) string {
	return `<script src="` + htmlAttr(path) + `" defer></script>`
}

func htmlAttr(value string) string {
	return template.HTMLEscapeString(value)
}
