#!/usr/bin/env bash

set -eou pipefail
set -x

IMAGE=daedalus2718/libri:latest
LOCAL_PORT=20100

for c in $(seq 0 0); do
    public_port=$((20100 + $c))
    docker run -d -p 127.0.0.1:${public_port}:${LOCAL_PORT} --name "librarian-${c}" ${IMAGE} \
        librarian start \
        --nSubscriptions 2 \
        --publicPort ${public_port} \
        --localPort ${LOCAL_PORT} \
        --bootstraps "localhost:20100"
done

#LIBRARIANS='localhost:20100 localhost:20101 localhost:20102'
#docker run --rm ${IMAGE} test health -a "${LIBRARIANS}"

#docker run --rm ${IMAGE} test io -a "${LIBRARIANS}"

#export LIBRI_PASSPHRASE='demo passphrase'  # picked up subsequent commands

#KEYCHAIN_DIR='~/.libri/demo-keychains'
#docker rm --rm ${IMAGE} author init -k "${KEYCHAIN_DIR}"


#DATA_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )/data"

#ls ${DATA_DIR}

#docker rm --rm ${IMAGE} author upload -k "${KEYCHAIN_DIR}" -f

