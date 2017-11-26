#!/usr/bin/env bash

# Download and install RocksDB on the *nix system.

ROCKSDB_VERSION='5.8'

# download
wget https://github.com/facebook/rocksdb/archive/v${ROCKSDB_VERSION}.zip -O /tmp/rocksdb-${ROCKSDB_VERSION}.zip
unzip /tmp/rocksdb-${ROCKSDB_VERSION}.zip -d /tmp
cd /tmp/rocksdb-${ROCKSDB_VERSION}

# creates librocksdb.a
make static_lib

# installs librocksdb.a at /usr/local/lib/librocksdb.a and source at /usr/local/include/rocksdb/
make install
md5sum /usr/local/lib/librocksdb.a

# clean up, keeping
rm /tmp/rocksdb-${ROCKSDB_VERSION}.zip
rm -r /tmp/rocksdb-${ROCKSDB_VERSION}
