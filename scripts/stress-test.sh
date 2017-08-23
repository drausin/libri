#!/usr/bin/env bash

set -ou pipefail

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
TEST_BINARY="pkg.test"
OUTPUT_DIR="${DIR}/../artifacts/stress-test"
ORIG_GOMAXPROCS=${GOMAXPROCS:-}
N_TRIALS=16
PKGS=$(go list ./... | grep -v /vendor/)

rm -rf ${OUTPUT_DIR} ${TEST_BINARY}
mkdir -p ${OUTPUT_DIR}

# stress test indiv packages
errored=false
for pkg in ${PKGS}; do
    pkg_dir=$(echo ${pkg} | sed -r "s|github.com/drausin/libri/||g")
    pushd ${pkg_dir} > /dev/null
    echo "stress testing ${pkg}"
    echo -n "- compiling test binary ... "
    if [[ ${pkg} == github.com/drausin/libri/libri/acceptance ]]; then
        go test -c -tags acceptance -o ${TEST_BINARY}
    else
        go test -c -race -o ${TEST_BINARY}
    fi
    echo "done"
    pkg_base_name=$(echo ${pkg_dir} | sed "s|/|-|g")

    # run tests if test binary exists (i.e., package has tests in it)
    if [[ -f ${TEST_BINARY} ]]; then
        for c in $(seq 1 ${N_TRIALS}) ; do
            export GOMAXPROCS=$[ 1 + $[ RANDOM % 64 ]]
            LOG_FILE="${OUTPUT_DIR}/${pkg_base_name}.t${c}.p${GOMAXPROCS}.log"
            echo -ne "\r- trial ${c} of ${N_TRIALS} with GOMAXPROCS=${GOMAXPROCS} ... "
            ./${TEST_BINARY} -test.v &> ${LOG_FILE}
            if [[ $? -ne 0 ]]; then
                echo "ERROR found, please consult ${LOG_FILE}"
                errored=true
            fi
        done
        rm ${TEST_BINARY}
        echo -e "done\n"
    fi
    popd > /dev/null
done

export GOMAXPROCS=${ORIG_GOMAXPROCS}

if [[ ${errored} = "true" ]]; then
    echo 'One or more stress tests had errors.'
    exit 1
fi

echo 'All stress tests passed.'

