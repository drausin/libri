package version

import (
	"github.com/blang/semver"
)

var snapshot = semver.PRVersion{VersionStr: "snapshot"}

// Version is the current version of this repo.
var Version = semver.Version{Major: 0, Minor: 1, Patch: 0}
var isSnapshot = true

func init() {
	if isSnapshot {
		Version.Pre = []semver.PRVersion{snapshot}
	}
}
