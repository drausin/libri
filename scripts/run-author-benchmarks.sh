#!/usr/bin/env bash

set -eou pipefail

AUTHOR_BENCH_PKGS='github.com/drausin/libri/libri/author/io/enc
    github.com/drausin/libri/libri/author/io/comp
    github.com/drausin/libri/libri/author/io/page'
N_TRIALS=32
RAW_RESULT_FILE='author.raw.bench'
RESULT_FILE='author.bench'
TEST_BINARY="pkg.test"

FAST_BENCH_DURATION='0.1s'
SLOW_BENCH_DURATION='3s'

rm -f ${RAW_RESULT_FILE}

for pkg in ${AUTHOR_BENCH_PKGS}; do
    echo "benchmarking ${pkg}"
    echo -n "- compiling test binary ... "
    go test -c ${pkg} -o ${TEST_BINARY}
    echo "done"
    echo "- running benchmarks ${N_TRIALS} times ... "
    echo -n "    - running fast benchmarks ... "
    ./${TEST_BINARY} -test.bench='.+/^(small|medium|large).*' -test.benchmem -test.cpu 4 \
        -test.count ${N_TRIALS} -test.benchtime ${FAST_BENCH_DURATION} \
        -test.run 'Benchmark*' 2>&1 >> ${RAW_RESULT_FILE}
    echo "done"
    echo -n "    - running slow benchmarks ... "
    ./${TEST_BINARY} -test.bench='.+/^xlarge.*' -test.benchmem -test.cpu 4 \
        -test.count ${N_TRIALS} -test.benchtime ${SLOW_BENCH_DURATION} \
        -test.run 'Benchmark*' 2>&1 >> ${RAW_RESULT_FILE}
    rm ${TEST_BINARY}
    echo "done"
    echo -e "done\n"
done

grep -v PASS ${RAW_RESULT_FILE} > ${RESULT_FILE}
rm ${RAW_RESULT_FILE}

echo "benchmarking complete!"
