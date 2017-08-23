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

GOOS=linux go build \
    --ldflags '-extldflags "-lpthread -static"' \
    -a \
    -installsuffix cgo \
    -o ${OUTPUT_FILE} \
    libri/main.go
