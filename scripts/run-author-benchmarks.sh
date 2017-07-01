#!/usr/bin/env bash

set -eou pipefail

AUTHOR_BENCH_PKGS='github.com/drausin/libri/libri/author/io/enc
    github.com/drausin/libri/libri/author/io/comp
    github.com/drausin/libri/libri/author/io/page'

RESULT_FILE='author.bench'
rm -f ${RESULT_FILE}

for c in {1..100}; do
    echo
    echo "running benchmarks trial ${c} of 100..."
    go test -bench='.+\/(small|medium|large).*' -benchmem -cpu 4 -benchtime 0.1s -run 'Benchmark*' ${AUTHOR_BENCH_PKGS} 2>&1 \
    | grep Benchmark | tee -a ${RESULT_FILE}
done

for c in {1..100}; do
    echo
    echo "running benchmarks trial ${c} of 100..."
    go test -bench='.+/xlarge.*' -benchmem -cpu 4 -benchtime 1s -run 'Benchmark*' ${AUTHOR_BENCH_PKGS} 2>&1 \
    | grep Benchmark | tee -a ${RESULT_FILE}
done

