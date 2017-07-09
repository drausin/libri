#!/usr/bin/env bash

set -eou pipefail

# this will only work on CircleCI/in build container
OUT_DIR="artifacts/cover"
cd /go/src/github.com/drausin/libri
mkdir -p ${OUT_DIR}

PKGS=$(go list ./... | grep -v /vendor/)
echo ${PKGS} | sed 's| |\n|g' | xargs -I {} bash -c '
    COVER_FILE=${OUT_DIR}/$(echo {} | sed -r "s|github.com/drausin/libri/||g" | sed "s|/|-|g").cov &&
    go test -race -coverprofile=${COVER_FILE} {}
'

# merge profiles together, removing results from auto-generated code
gocovmerge ${OUT_DIR}/*.cov | grep -v '.pb.go:' > ${OUT_DIR}/test-coverage-merged.cov
