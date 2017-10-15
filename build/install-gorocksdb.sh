#!/usr/bin/env bash

CGO_CFLAGS="-I/usr/local/include/rocksdb" \
CGO_LDFLAGS="-L/usr/local/lib/librocksdb.a -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy" \
go get github.com/tecbot/gorocksdb
