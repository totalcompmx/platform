package version

import (
	"fmt"
	"runtime/debug"
)

var readBuildInfo = debug.ReadBuildInfo

func Get() string {
	bi, ok := readBuildInfo()
	if ok {
		return bi.Main.Version
	}

	return "unavailable"
}

func GetRevision() string {
	revision, modified := readRevision()
	return formatRevision(revision, modified)
}

func readRevision() (string, bool) {
	var revision string
	var modified bool

	bi, ok := readBuildInfo()
	if !ok {
		return revision, modified
	}

	for _, s := range bi.Settings {
		applyRevisionSetting(s, &revision, &modified)
	}

	return revision, modified
}

func applyRevisionSetting(s debug.BuildSetting, revision *string, modified *bool) {
	switch s.Key {
	case "vcs.revision":
		*revision = s.Value
	case "vcs.modified":
		*modified = s.Value == "true"
	}
}

func formatRevision(revision string, modified bool) string {
	if revision == "" {
		return "unavailable"
	}

	if modified {
		return fmt.Sprintf("%s+dirty", revision)
	}

	return revision
}
