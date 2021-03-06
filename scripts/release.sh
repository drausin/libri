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
    echo -n "enter version to start releasing [${release_version}]: "
    read entered_release_version
    if [[ ! -z "${entered_release_version}" ]]; then
        release_version="${entered_release_version}"
    fi
    echo -n "enter next version after this release (e.g., 0.4.0): "
    read next_version

    release_branch="release/${release_version}"
    set -x
    git checkout develop
    git pull
    git checkout -b ${release_branch} develop
    set +x

    echo 'please draft release notes in CHANGELOG.md [press ENTER when finished]'
    read

    echo 'please copy the CHANGELOG.md release notes into a new Github release draft (https://github.com/drausin/libri/releases/new) [press ENTER when finished]'
    read

    sed -i -r "s/librarian_libri_version = \"(.+)\"/librarian_libri_version = \"${release_version}\"/g" deploy/cloud/terraform/gcp/variables.tf
    echo "updated Terraform GCP libri version to ${release_version}"

    set -x
    git ci -am "prepare for v${release_version} release"
    git checkout develop
    sed -i -r "s|semverString = \".+\"|semverString = \"${next_version}\"|g" version/version.go
    git commit -am "bump to next (in progress) version"
    git push origin develop
    set +x
    echo -e 'Started libri release. We recommend the following tests before finishing the release:\n'
    echo -e '   - run an intensive build\n'
    echo -e '       git checkout develop-intensive-build && git pull && git push origin develop-intensive-build\n'
    echo -e '   - test local minikube k8s cluster\n'
    echo -e '       scripts/minikube-test.sh'
    echo ''
    echo -e 'Run\n'
    echo -e '  ./release.sh finish\n'
    echo 'to finish when ready.'
}

finish() {
    echo -e 'Finishing libri release\n'
    release_version=$(git branch --list 'release/*' --sort=-committerdate | head -1 | sed -r "s|^.+*release/(.+)$|\1|g")
    echo -n "enter version to finish releasing [${release_version}]: "
    read entered_release_version
    if [[ ! -z "${entered_release_version}" ]]; then
        release_version="${entered_release_version}"
    fi

    release_branch="release/${release_version}"
    set -x
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
    set +x
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








