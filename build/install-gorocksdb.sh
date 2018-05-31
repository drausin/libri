#!/usr/bin/env bash

# TODO (drausin) make this use vendored gorocksdb

CGO_CFLAGS="-I/usr/local/include/rocksdb" \
CGO_LDFLAGS="-L/usr/local/lib/librocksdb.a -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy" \
go get github.com/tecbot/gorocksdb
