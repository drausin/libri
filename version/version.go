package version

import (
	"github.com/blang/semver"
)

// Version is the current version of this repo.
var Version semver.Version

var versionString = "0.1.0"
var isSnapshot = false
var snapshot = semver.PRVersion{VersionStr: "snapshot"}

func init() {
	Version = semver.MustParse(versionString)
	if isSnapshot {
		Version.Pre = []semver.PRVersion{snapshot}
	}
}
