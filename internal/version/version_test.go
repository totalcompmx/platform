package version

import (
	"runtime/debug"
	"testing"

	"github.com/jcroyoaun/totalcompmx/internal/assert"
)

func TestGet(t *testing.T) {
	t.Run("Returns version string or unavailable", func(t *testing.T) {
		expectedVersion := "unavailable"
		bi, ok := debug.ReadBuildInfo()
		if ok {
			expectedVersion = bi.Main.Version
		}

		version := Get()
		assert.True(t, version != "")
		assert.Equal(t, version, expectedVersion)
	})
}

func TestGetRevision(t *testing.T) {
	t.Run("Returns revision string or unavailable", func(t *testing.T) {

		revision := GetRevision()
		assert.True(t, revision != "")
		assert.True(t, revision == "unavailable" || len(revision) > 7)
	})
}

func TestGetWithoutBuildInfo(t *testing.T) {
	withBuildInfo(t, nil, false)

	assert.Equal(t, "unavailable", Get())
	assert.Equal(t, "unavailable", GetRevision())
}

func TestGetRevisionFromBuildInfo(t *testing.T) {
	withBuildInfo(t, &debug.BuildInfo{
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "abc123"},
			{Key: "vcs.modified", Value: "true"},
			{Key: "other", Value: "ignored"},
		},
	}, true)

	assert.Equal(t, "abc123+dirty", GetRevision())
}

func TestFormatRevision(t *testing.T) {
	assert.Equal(t, "abc123", formatRevision("abc123", false))
	assert.Equal(t, "abc123+dirty", formatRevision("abc123", true))
	assert.Equal(t, "unavailable", formatRevision("", true))
}

func withBuildInfo(t *testing.T, bi *debug.BuildInfo, ok bool) {
	t.Helper()

	previous := readBuildInfo
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return bi, ok
	}
	t.Cleanup(func() {
		readBuildInfo = previous
	})
}
