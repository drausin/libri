#!/usr/bin/env bash

set -eou pipefail

AUTHOR_BENCH_PKGS='github.com/drausin/libri/libri/author/io/enc
    github.com/drausin/libri/libri/author/io/comp
    github.com/drausin/libri/libri/author/io/page'
N_TRIALS=32
RESULT_FILE='author.bench'

rm -f ${RESULT_FILE}

for c in $(seq 1 ${N_TRIALS}); do
    echo -ne "\rrunning fast benchmarks trial ${c} of ${N_TRIALS}..."
    go test -bench='.+/^(small|medium|large).*' -benchmem -cpu 4 -benchtime 0.1s -run 'Benchmark*' ${AUTHOR_BENCH_PKGS} 2>&1 \
    | grep Benchmark >> ${RESULT_FILE}
done
echo "done"

for c in $(seq 1 ${N_TRIALS}); do
    echo -ne "\rrunning slow benchmarks trial ${c} of ${N_TRIALS}..."
    go test -bench='.+/^xlarge.*' -benchmem -cpu 4 -benchtime 3s -run 'Benchmark*' ${AUTHOR_BENCH_PKGS} 2>&1 \
    | grep Benchmark >> ${RESULT_FILE}
done
echo "done"
