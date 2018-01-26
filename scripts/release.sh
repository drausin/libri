#!/usr/bin/env bash

set -eou pipefail


usage() {
    echo 'Libri release script'
    echo 'Usage: '
    echo '  release [start|finish]'
    echo
}

start() {
    echo -e 'Starting libri release\n'
    release_version=$(grep "semverString = " version/version.go | sed -r "s|.+semverString = \"(.+)\"$|\1|g")
    echo "enter version to start releasing [${release_version}]:"
    read release_version
    echo "enter next version after this release (e.g., 0.4.0):"
    read next_version

    release_branch="release/${release_version}"
    set +x
    git checkout develop
    git pull
    git checkout -b ${release_branch} develop
    set -x

    echo 'please draft release notes in CHANGELOG.md [press ENTER when finished]'
    read

    echo 'please copy the CHANGELOG.md release notes into a new Github release draft (https://github.com/drausin/libri/releases/new) [press ENTER when finished]'
    read

    set +x
    git ci -am "prepare for v${release_version} release"
    git checkout develop
    sed -i -r "s|semverString = \".+\"|semverString = \"${next_version}\"|g" version/version.go
    git commit -am "bump to next (in progress) version"
    git push origin develop
    set -x
    echo -e 'Started libri release. Run\n'
    echo '  ./release.sh finish\n'
    echo 'to finish when ready.'
}

finish() {
    echo -e 'Finishing libri release\n'
    release_version=$(git branch --list 'release/*' --sort=-committerdate | head -1 | sed -r "s|^release/(.+)$|\1|g")
    echo "enter version to finish releasing [${release_version}]:"
    read release_version

    release_branch="release/${release_version}"
    set +x
    git checkout master
    git merge --no-ff ${release_branch}
    git tag -a "v${release_version}" -m "release v${release_version}"
    git checkout develop
    git pull
    git merge --no-ff ${release_branch} -s recursive -X ours
    git branch -d ${release_branch}
    git push origin develop
    git checkout master
    git push origin master --tags
    set -x
    echo "Finished libri release. Please attach the Github draft release notes to tag v${release_version}"

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








