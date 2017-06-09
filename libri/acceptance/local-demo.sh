#!/usr/bin/env bash

set -eou pipefail
set -x

docker-compose up -d

LIBRARIANS='librarian-0:20100 librarian-1:20101 librarian-2:20102'

docker-compose run tester test health -a "${LIBRARIANS}"

#docker-compose run tester test io -a "${LIBRARIANS}"

#export LIBRI_PASSPHRASE='demo passphrase'  # picked up subsequent commands

#KEYCHAIN_DIR='~/.libri/demo-keychains'
#docker rm --rm ${IMAGE} author init -k "${KEYCHAIN_DIR}"


#DATA_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )/data"

#ls ${DATA_DIR}

#docker rm --rm ${IMAGE} author upload -k "${KEYCHAIN_DIR}" -f

docker-compose down

