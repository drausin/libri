#!/usr/bin/env bash

set -eou pipefail
#set -x  # useful for debugging

# local and filesystem constants
LOCAL_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
LOCAL_TEST_DATA_DIR="${LOCAL_DIR}/data"
LOCAL_TEST_LOGS_DIR="${LOCAL_DIR}/logs"
mkdir -p "${LOCAL_TEST_LOGS_DIR}"

# get test data if it doesn't exist
if [[ ! -e "${LOCAL_TEST_DATA_DIR}" ]]; then
    ./get-test-data.sh
fi

# container command contants
IMAGE="daedalus2718/libri:latest"
KEYCHAIN_DIR="~/.libri/keychains"  # inside container
CONTAINER_TEST_DATA_DIR="/test-data"
LIBRI_PASSPHRASE="test passphrase"
N_LIBRARIANS=3

# clean up any existing libri containers
docker ps | grep -e 'libri' | awk '{print $1}' | xargs docker stop -t 3

echo "starting librarian peers..."
librarian_addrs=""
librarian_containers=""
for c in $(seq 0 $((${N_LIBRARIANS} - 1))); do
    port=$((20100+c))
    name="librarian-${c}"
    docker run --rm --name "${name}" --net=host -d -p ${port}:${port} ${IMAGE} \
        librarian start \
        --nSubscriptions 2 \
        --publicPort ${port} \
        --localPort ${port} \
        --bootstraps localhost:20100
    librarian_addrs="localhost:${port} ${librarian_addrs}"
    librarian_containers="${librarian_containers} ${name}"
done

echo
echo "testing librarians health..."
docker run --rm --net=host ${IMAGE} test health -a "${librarian_addrs}"

echo
echo "testing librarians upload/download..."
docker run --rm --net=host ${IMAGE} test io -a "${librarian_addrs}" -n 4

echo
echo "starting author container..."
docker run -d --rm --name author --net=host \
    -v ${LOCAL_TEST_DATA_DIR}:${CONTAINER_TEST_DATA_DIR} \
    -e LIBRI_PASSPHRASE="${LIBRI_PASSPHRASE}" \
    --entrypoint=sleep ${IMAGE} 3600  # use sleep just to keep it running

echo
echo "initializing author..."
docker exec author libri author init -k "${KEYCHAIN_DIR}"  # uses pre-stored passphrase in env var

echo
echo "uploading & downloading local files..."
for file in $(ls ${LOCAL_TEST_DATA_DIR}); do
    up_file="${CONTAINER_TEST_DATA_DIR}/${file}"
    docker exec author libri author upload \
        -k "${KEYCHAIN_DIR}" \
        -a "${librarian_addrs}" \
        -f "${up_file}" |& \
        tee ${LOCAL_TEST_LOGS_DIR}/${file}.log

    log_file="${LOCAL_TEST_LOGS_DIR}/${file}.log"
    down_file="${CONTAINER_TEST_DATA_DIR}/downloaded.${file}"
    envelope_key=$(grep envelope_key ${log_file} | sed -r 's/^.*"envelope_key": "(\w+)".*$/\1/g')
    docker exec author libri author download \
        -k "${KEYCHAIN_DIR}" \
        -a "${librarian_addrs}" \
        -f "${down_file}" \
        -e "${envelope_key}"

    # verify md5s (locally, since it's simpler)
    up_md5=$(md5sum "${LOCAL_TEST_DATA_DIR}/${file}" | awk '{print $1}')
    down_md5=$(md5sum "${LOCAL_TEST_DATA_DIR}/downloaded.${file}" | awk '{print $1}')
    [[ "${up_md5}" = "${down_md5}" ]]
done

echo
echo "cleaning up..."
rm ${LOCAL_TEST_DATA_DIR}/downloaded.*
rm ${LOCAL_TEST_LOGS_DIR}/*
docker stop -t 3 author ${librarian_containers}

echo
echo "All tests passed."
