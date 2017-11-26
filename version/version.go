package version

import (
	"os"
	"strings"
	"time"

	"github.com/blang/semver"
	"github.com/drausin/libri/libri/common/errors"
)

// Current contains the current build info.
var Current BuildInfo

// these variables are populated by ldflags during builds and fall back to population from git repo
// when they're not set (e.g., during tests)
var (
	// GitBranch is the current git branch
	GitBranch string

	// GitRevision is the current git commit hash.
	GitRevision string

	// BuildDate is the date of the build.
	BuildDate string
)

var semverString = "0.2.0"

const (
	develop         = "develop"
	master          = "master"
	snapshot        = "snapshot"
	buildDateFormat = "2006-01-02" // ISO 8601 date format
)

var branchPrefixes = []string{
	"feature/",
	"release/",
	"bugfix/",
}

// BuildInfo contains info about the current build.
type BuildInfo struct {
	Version     semver.Version
	GitBranch   string
	GitRevision string
	BuildDate   string
}

func init() {
	wd, err := os.Getwd()
	errors.MaybePanic(err)
	g := git{dir: wd}

	if GitBranch == "" {
		GitBranch = g.Branch()
	}
	if GitRevision == "" {
		GitRevision, err = g.Commit()
		errors.MaybePanic(err)
	}
	if BuildDate == "" {
		BuildDate = time.Now().UTC().Format(buildDateFormat)
	}
	Version := semver.MustParse(semverString)
	if GitBranch == master {
		// no pre-release tags to add
	} else if GitBranch == develop {
		Version.Pre = []semver.PRVersion{{VersionStr: snapshot}}
	} else {
		Version.Pre = []semver.PRVersion{{VersionStr: stripPrefixes(GitBranch)}}
	}
	Current = BuildInfo{
		Version:     Version,
		GitBranch:   GitBranch,
		GitRevision: GitRevision,
		BuildDate:   BuildDate,
	}
}

func stripPrefixes(branch string) string {
	for _, prefix := range branchPrefixes {
		if strings.HasPrefix(branch, prefix) {
			return strings.TrimPrefix(branch, prefix)
		}
	}
	return branch
}
