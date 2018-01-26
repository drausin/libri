#!/usr/bin/env bash

set -eou pipefail

release_version=$(grep "semverString = " version/version.go | sed -r "s|.+semverString = \"(.+)\"$|\1|g")

usage() {
    echo 'Libri release script'
    echo 'Usage: '
    echo '  release [start|finish]'
    echo
}

start() {
    echo -n "Libri release script\n"
    echo "enter version to release [${release_version}]:"
    read release_version
    echo "enter next version after this release (e.g., 0.4.0):"
    read next_version

    release_branch="release/${release_version}"
    echo "cutting release branch ${release_branch} from develop..."
    git checkout -b ${release_branch} develop

    echo 'please draft release notes in CHANGELOG.md [press ENTER when finished]'
    read

    echo 'please copy the CHANGELOG.md release notes into a new Github release (https://github.com/drausin/libri/releases/new) [press ENTER when finished]'
    read

    set +x
    git ci -am "prepare for v${release_version} release"
    git checkout develop
    sed -i -r "s|semverString = \".+\"|semverString = \"${next_version}\"|g" version/version.go
    git commit -am "bump to next (in progress) version"
    git push origin develop
    set -x
    echo ''
}

cmd=${1}

case ${cmd} in
    start)
        start ;;
    finish)
        finish ;;
    *)
        usage ;;
esac








