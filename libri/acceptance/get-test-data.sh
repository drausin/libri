#!/usr/bin/env bash

set -eou pipefail

DATA_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )/data"
mkdir -p ${DATA_DIR}
pushd ${DATA_DIR}

wget --quiet https://www.whatsapp.com/security/WhatsApp-Security-Whitepaper.pdf

popd