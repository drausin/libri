package main

import (
	"github.com/drausin/libri/libri/cmd"
	"github.com/blang/semver"
)

// LibriVersion is the current version of this repo.
var LibriVersion = semver.Version{Major: 0, Minor: 1, Patch: 0}

func main() {
	cmd.Execute()
}
