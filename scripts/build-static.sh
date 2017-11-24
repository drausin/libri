#!/usr/bin/env bash

set -eou pipefail

# Build a static binary for use in Docker container. This script is mainly intended for CircleCI
# builds and does't at the moment work locally on OSX.
#
# Usage:
#
#   ./build-static path/to/output/binary
#
# where "path/to/output/binary" is the path to write the output binary to.
#

OUTPUT_FILE=${1}

GIT_BRANCH_VAR="version.GitBranch=$(git symbolic-ref -q --short HEAD)"
GIT_REVISION_VAR="version.GitRevision=$(git rev-parse --short HEAD)"
BUILD_DATE_VAR="version.BuildDate=$(date -u +"%Y-%m-%d")"
VERSION_VARS="-X ${GIT_BRANCH_VAR} -X ${GIT_REVISION_VAR} -X ${BUILD_DATE_VAR}"

GOOS=linux go build \
    -ldflags "-extldflags '-lpthread -static' ${VERSION_VARS}" \
    -a \
    -installsuffix cgo \
    -o ${OUTPUT_FILE} \
    libri/main.go
