#!/usr/bin/env bash

set -eou pipefail

GIT_HOOKS_DIR="scripts/git-hooks"

for filepath in ${GIT_HOOKS_DIR}/* ; do
    filename=$(basename ${filepath})
    ln -fs ${filepath} .git/hooks/${filename}
    echo "installed ${filename}"
done
