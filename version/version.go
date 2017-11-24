package version

import (
	"github.com/blang/semver"
)

// BuildInfo contains info about the current build.
type BuildInfo struct {
	Version     semver.Version
	GitBranch   string
	GitRevision string
	BuildDate   string
}

func init() {
	Version = semver.MustParse(version)
	if branch == "develop" {
		Version.Pre = []semver.PRVersion{{VersionStr: "snapshot"}}
	}
}
